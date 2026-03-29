package webflow

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"wfkit/internal/utils"
	cookie "wfkit/pkg/cookie"

	"github.com/PuerkitoBio/goquery"
)

const (
	cacheTTL    = 5 * time.Minute // время жизни кэша
	csrfMetaTag = "meta[name='_csrf']"
	userAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
	baseApiUrl  = "https://%s.design.webflow.com/api/sites/%s"
	pageApiUrl  = "%s/api/pages/%s"
)

// Глобальный HTTP клиент для переиспользования соединений (Connection Pooling)
var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// GlobalCode представляет глобальный код сайта
type GlobalCode struct {
	Meta map[string]string `json:"meta"`
}

// Page представляет страницу сайта
type Page struct {
	ID       string `json:"_id"`
	PostBody string `json:"postBody"`
	Title    string `json:"title"`
	Slug     string `json:"slug"`
}

// cacheEntry представляет кэшированные данные авторизации
type cacheEntry struct {
	Cookies string
	Token   string
	Expires time.Time
}

var (
	cache     = make(map[string]cacheEntry)
	cacheLock sync.RWMutex
)

type apiError struct {
	Method    string
	Status    int
	Message   string
	Name      string
	Path      string
	ErrorEnum string
	Raw       string
}

func (e *apiError) Error() string {
	if isIncompatibleClientVersionError(e) {
		return "Webflow Designer session is outdated. Refresh your open Webflow Designer tab and retry."
	}

	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = strings.TrimSpace(e.Raw)
	}
	if message == "" {
		message = "Webflow request failed"
	}

	return fmt.Sprintf("%s request failed with status %d: %s", e.Method, e.Status, message)
}

// GetCsrfTokenAndCookies получает CSRF-токен и cookie, используя кэш.
func GetCsrfTokenAndCookies(ctx context.Context, baseUrl string) (string, string, error) {
	return getCsrfTokenAndCookies(ctx, baseUrl, true)
}

func CheckAuthentication(ctx context.Context, baseUrl string) error {
	_, _, err := getCsrfTokenAndCookies(ctx, baseUrl, false)
	return err
}

func InvalidateAuthCache(baseURL string) {
	domain := extractDomain(baseURL)
	cacheLock.Lock()
	delete(cache, domain)
	cacheLock.Unlock()
}

func getCsrfTokenAndCookies(ctx context.Context, baseUrl string, openOnMissingToken bool) (string, string, error) {
	domain := extractDomain(baseUrl)

	cacheLock.RLock()
	if entry, ok := cache[domain]; ok && !entry.isExpired() {
		cacheLock.RUnlock()
		return entry.Token, entry.Cookies, nil
	}
	cacheLock.RUnlock()

	cookies, err := GetCookiesForWebflow(domain)
	if err != nil {
		return "", "", fmt.Errorf("failed to get cookies: %w", err)
	}

	token, err := fetchCsrfToken(ctx, baseUrl, cookies, openOnMissingToken)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch CSRF token: %w", err)
	}

	cacheLock.Lock()
	cache[domain] = cacheEntry{
		Cookies: cookies,
		Token:   token,
		Expires: time.Now().Add(cacheTTL),
	}
	cacheLock.Unlock()

	return token, cookies, nil
}

// fetchCsrfToken выполняет HTTP-запрос для получения CSRF-токена.
func fetchCsrfToken(ctx context.Context, url, cookies string, openOnMissingToken bool) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	setupRequestHeaders(req, cookies)

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	token := doc.Find(csrfMetaTag).AttrOr("content", "")
	if token == "" {
		if openOnMissingToken {
			if err := openBrowser(url); err != nil {
				utils.CPrint(fmt.Sprintf("Failed to open browser: %v", err), "red")
			}
		}
		return "", fmt.Errorf("CSRF token not found - please login to Webflow in your browser")
	}

	return token, nil
}

func sanitizeCookieValue(value string) string {
	value = strings.ReplaceAll(value, "\n", "")
	value = strings.ReplaceAll(value, "\r", "")
	return url.QueryEscape(value)
}

// GetCookiesForWebflow извлекает cookie для домена.
func GetCookiesForWebflow(domain string) (string, error) {
	cookies, err := cookie.GetAllCookies([]string{domain})
	if err != nil {
		return "", fmt.Errorf("failed to load cookies: %w", err)
	}

	var cookieHeaderParts []string
	for _, c := range cookies {
		sanitizedValue := sanitizeCookieValue(c.Value)
		cookieHeaderParts = append(cookieHeaderParts, fmt.Sprintf("%s=%s", c.Name, sanitizedValue))
	}
	cookieHeader := strings.Join(cookieHeaderParts, "; ")

	return cookieHeader, nil
}

// HTTP-запросы

func doGet(ctx context.Context, url, cookies, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}
	setupRequestHeaders(req, cookies)
	req.Header.Set("x-xsrf-token", token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute GET request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, parseAPIError("GET", resp.StatusCode, body)
	}

	return resp, nil
}

func doPost(ctx context.Context, url, cookies, token string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create POST request: %w", err)
	}
	setupRequestHeaders(req, cookies)
	req.Header.Set("x-xsrf-token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute POST request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, parseAPIError("POST", resp.StatusCode, body)
	}

	return resp, nil
}

func doPut(ctx context.Context, url, cookies, token string, body interface{}) (*http.Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	setupRequestHeaders(req, cookies)
	req.Header.Set("x-xsrf-token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute PUT request: %w", err)
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, parseAPIError("PUT", resp.StatusCode, body)
	}

	return resp, nil
}

func setupRequestHeaders(req *http.Request, cookies string) {
	req.Header.Set("Cookie", cookies)
	req.Header.Set("User-Agent", userAgent)
}

// GetGlobalCode получает глобальный код сайта.
func GetGlobalCode(ctx context.Context, siteName, token, cookies string) (GlobalCode, error) {
	url := fmt.Sprintf(baseApiUrl, siteName, siteName+"/code")
	resp, err := doGet(ctx, url, cookies, token)
	if err != nil {
		return GlobalCode{}, fmt.Errorf("failed to get global code: %w", err)
	}
	defer resp.Body.Close()

	var data GlobalCode
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return GlobalCode{}, fmt.Errorf("failed to decode global code response: %w", err)
	}
	return data, nil
}

// UpdateGlobalCode обновляет глобальный код сайта.
func UpdateGlobalCode(ctx context.Context, siteName, token, cookies string, head, postBody string) error {
	url := fmt.Sprintf(baseApiUrl, siteName, siteName+"/code")
	body := map[string]string{
		"head":     head,
		"postBody": postBody,
	}
	resp, err := doPut(ctx, url, cookies, token, body)
	if err != nil {
		return fmt.Errorf("failed to update global code: %w", err)
	}
	resp.Body.Close()
	return nil
}

// PublishSite публикует сайт в staging по умолчанию.
func PublishSite(ctx context.Context, siteName, token, cookies string) error {
	return PublishSiteTargets(ctx, siteName, token, cookies, []string{fmt.Sprintf("%s.webflow.io", siteName)})
}

// PublishSiteTargets публикует сайт на указанные домены.
func PublishSiteTargets(ctx context.Context, siteName, token, cookies string, publishTargets []string) error {
	if len(publishTargets) == 0 {
		return fmt.Errorf("failed to publish site: no publish targets selected")
	}

	url := fmt.Sprintf(baseApiUrl, siteName, siteName+"/queue-publish")
	body := map[string]interface{}{
		"publishTarget": publishTargets,
		"meta":          map[string]string{"designerMode": "design"},
	}
	resp, err := doPost(ctx, url, cookies, token, body)
	if err != nil {
		return fmt.Errorf("failed to publish site: %w", err)
	}
	resp.Body.Close()
	return nil
}

// GetPagesListFromDom получает список страниц сайта.
func GetPagesListFromDom(ctx context.Context, siteName, token, cookies string) ([]Page, error) {
	url := fmt.Sprintf(baseApiUrl, siteName, siteName+"/dom")
	resp, err := doGet(ctx, url, cookies, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get pages list: %w", err)
	}
	defer resp.Body.Close()

	var data struct {
		Pages []Page `json:"pages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode pages list response: %w", err)
	}
	return data.Pages, nil
}

// CreateStaticPage создает новую статическую страницу сайта.
func CreateStaticPage(ctx context.Context, siteName, baseURL, token, cookies, title, slug string) (Page, error) {
	url := fmt.Sprintf(baseApiUrl, siteName, siteName+"/pages")
	body := map[string]string{
		"title": title,
		"slug":  slug,
	}

	resp, err := doPost(ctx, url, cookies, token, body)
	if err != nil {
		if isIncompatibleClientVersionError(err) {
			InvalidateAuthCache(baseURL)
			refreshedToken, refreshedCookies, refreshErr := GetCsrfTokenAndCookies(ctx, baseURL)
			if refreshErr == nil {
				resp, err = doPost(ctx, url, refreshedCookies, refreshedToken, body)
				if err == nil {
					token = refreshedToken
					cookies = refreshedCookies
				}
			}
		}
		if err != nil {
			return Page{}, fmt.Errorf("failed to create page %s: %w", title, err)
		}
	}
	defer resp.Body.Close()

	var payload struct {
		Page Page `json:"page"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Page{}, fmt.Errorf("failed to decode create page response: %w", err)
	}
	if payload.Page.ID == "" {
		return Page{}, fmt.Errorf("failed to create page %s: missing page id in response", title)
	}

	if err := InitializePageSecrets(ctx, baseURL, token, cookies, payload.Page.ID, "", ""); err != nil {
		return Page{}, fmt.Errorf("failed to initialize page %s secrets: %w", title, err)
	}

	return payload.Page, nil
}

// InitializePageSecrets initializes page-level custom code storage for a page.
func InitializePageSecrets(ctx context.Context, baseURL, token, cookies, pageID, head, postBody string) error {
	url := fmt.Sprintf("%s/api/pages/%s/secrets", strings.TrimRight(baseURL, "/"), pageID)
	body := map[string]string{
		"head":     head,
		"postBody": postBody,
	}

	resp, err := doPut(ctx, url, cookies, token, body)
	if err != nil {
		if isIncompatibleClientVersionError(err) {
			InvalidateAuthCache(baseURL)
			refreshedToken, refreshedCookies, refreshErr := GetCsrfTokenAndCookies(ctx, baseURL)
			if refreshErr == nil {
				resp, err = doPut(ctx, url, refreshedCookies, refreshedToken, body)
			}
		}
		if err != nil {
			return fmt.Errorf("failed to initialize page secrets for %s: %w", pageID, err)
		}
	}
	resp.Body.Close()
	return nil
}

// PutFullPageObject обновляет страницу сайта.
func PutFullPageObject(ctx context.Context, baseUrl, token, cookies string, page Page) error {
	url := fmt.Sprintf(pageApiUrl, baseUrl, page.ID)
	resp, err := doPut(ctx, url, cookies, token, page)
	if err != nil {
		if isIncompatibleClientVersionError(err) {
			InvalidateAuthCache(baseUrl)
			refreshedToken, refreshedCookies, refreshErr := GetCsrfTokenAndCookies(ctx, baseUrl)
			if refreshErr == nil {
				resp, retryErr := doPut(ctx, url, refreshedCookies, refreshedToken, page)
				if retryErr == nil {
					resp.Body.Close()
					return nil
				}
				err = retryErr
			}
		}
		return fmt.Errorf("failed to update page %s: %w", page.Title, err)
	}
	resp.Body.Close()
	return nil
}

func parseAPIError(method string, status int, body []byte) error {
	raw := strings.TrimSpace(string(body))
	apiErr := &apiError{
		Method: method,
		Status: status,
		Raw:    raw,
	}

	var payload struct {
		Message   string `json:"msg"`
		Name      string `json:"name"`
		Path      string `json:"path"`
		ErrorEnum string `json:"errorEnum"`
		Meta      struct {
			Code string `json:"code"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		apiErr.Message = payload.Message
		apiErr.Name = payload.Name
		apiErr.Path = payload.Path
		apiErr.ErrorEnum = payload.ErrorEnum
		if apiErr.ErrorEnum == "" {
			apiErr.ErrorEnum = payload.Meta.Code
		}
	}

	return apiErr
}

func isIncompatibleClientVersionError(err error) bool {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorEnum == "IncompatibleClientVersion" ||
			strings.Contains(apiErr.Name, "IncompatibleClientVersion") ||
			strings.Contains(strings.ToLower(apiErr.Message), "incompatible client version")
	}
	return false
}

// extractDomain извлекает домен из URL.
func extractDomain(url string) string {
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	return strings.Split(url, "/")[0]
}

// isExpired проверяет истечение срока действия кэша.
func (e cacheEntry) isExpired() bool {
	return time.Now().After(e.Expires)
}

// openBrowser открывает URL в браузере.
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}
