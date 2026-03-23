package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Options struct {
	ListenHost string
	ListenPort int
	ScriptURL  string
	TargetURL  string
}

func Serve(ctx context.Context, opts Options) error {
	target, err := url.Parse(opts.TargetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	proxyHandler := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.SetXForwarded()
			r.Out.Host = target.Host
			r.Out.Header.Del("Accept-Encoding")

			scheme := "http"
			if r.In.TLS != nil {
				scheme = "https"
			}

			r.Out.Header.Set("X-WF-Proxy-Origin", fmt.Sprintf("%s://%s", scheme, r.In.Host))
			r.Out.Header.Set("X-WF-Proxy-Target-Origin", target.Scheme+"://"+target.Host)
			r.Out.Header.Set("X-WF-Proxy-Host", r.In.Host)
			r.Out.Header.Set("X-WF-Proxy-Scheme", scheme)
		},
		ModifyResponse: func(resp *http.Response) error {
			rewriteRedirectLocation(resp)
			rewriteSetCookies(resp)

			if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
				return nil
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read proxied HTML: %w", err)
			}
			resp.Body.Close()

			rewritten := rewriteHTML(string(body), responseOptions{
				proxyOrigin:  resp.Request.Header.Get("X-WF-Proxy-Origin"),
				scriptURL:    opts.ScriptURL,
				targetOrigin: resp.Request.Header.Get("X-WF-Proxy-Target-Origin"),
			})

			resp.Body = io.NopCloser(strings.NewReader(rewritten))
			resp.ContentLength = int64(len(rewritten))
			resp.Header.Del("Content-Encoding")
			resp.Header.Del("ETag")
			resp.Header.Del("Content-Security-Policy")
			resp.Header.Del("Content-Security-Policy-Report-Only")
			resp.Header.Set("Content-Length", strconv.Itoa(len(rewritten)))
			return nil
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			http.Error(rw, fmt.Sprintf("proxy error: %v", err), http.StatusBadGateway)
		},
	}

	addr := net.JoinHostPort(opts.ListenHost, strconv.Itoa(opts.ListenPort))
	server := &http.Server{
		Addr:              addr,
		Handler:           proxyHandler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
}

type responseOptions struct {
	proxyOrigin  string
	scriptURL    string
	targetOrigin string
}

func rewriteHTML(html string, opts responseOptions) string {
	if opts.targetOrigin != "" && opts.proxyOrigin != "" {
		html = strings.ReplaceAll(html, opts.targetOrigin, opts.proxyOrigin)
		if targetURL, err := url.Parse(opts.targetOrigin); err == nil {
			if proxyURL, proxyErr := url.Parse(opts.proxyOrigin); proxyErr == nil {
				html = strings.ReplaceAll(html, "//"+targetURL.Host, "//"+proxyURL.Host)
			}
		}
	}

	html = stripManagedScripts(html)

	injected := strings.Join([]string{
		fmt.Sprintf(`<script type="module" src="%s"></script>`, buildViteClientURL(opts.scriptURL)),
		fmt.Sprintf(`<script data-script-id="global-script" type="module" defer src="%s"></script>`, opts.scriptURL),
	}, "\n")

	if strings.Contains(strings.ToLower(html), "</body>") {
		return replaceClosingTag(html, "</body>", injected+"\n</body>")
	}
	if strings.Contains(strings.ToLower(html), "</head>") {
		return replaceClosingTag(html, "</head>", injected+"\n</head>")
	}
	return injected + "\n" + html
}

func stripManagedScripts(html string) string {
	patterns := []string{
		`<script[^>]*data-script-id=["'](?:global-script|page-script|p-script|wf-loader)["'][^>]*>.*?</script>`,
		`<script[^>]*src=["'][^"']*/@vite/client["'][^>]*>.*?</script>`,
		`<meta[^>]*http-equiv=["']Content-Security-Policy(?:-Report-Only)?["'][^>]*>`,
	}
	cleaned := html
	for _, pattern := range patterns {
		re := regexp.MustCompile("(?is)" + pattern)
		cleaned = re.ReplaceAllString(cleaned, "")
	}
	return cleaned
}

func buildViteClientURL(scriptURL string) string {
	u, err := url.Parse(scriptURL)
	if err != nil {
		return strings.TrimRight(scriptURL, "/") + "/@vite/client"
	}
	u.Path = "/@vite/client"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func replaceClosingTag(html, tag, replacement string) string {
	lower := strings.ToLower(html)
	idx := strings.LastIndex(lower, tag)
	if idx == -1 {
		return html
	}
	return html[:idx] + replacement + html[idx+len(tag):]
}

func rewriteRedirectLocation(resp *http.Response) {
	location := resp.Header.Get("Location")
	if location == "" {
		return
	}

	targetOrigin := resp.Request.Header.Get("X-WF-Proxy-Target-Origin")
	proxyOrigin := resp.Request.Header.Get("X-WF-Proxy-Origin")
	if targetOrigin == "" || proxyOrigin == "" {
		return
	}

	if strings.HasPrefix(location, targetOrigin) {
		resp.Header.Set("Location", proxyOrigin+strings.TrimPrefix(location, targetOrigin))
		return
	}

	targetURL, err := url.Parse(targetOrigin)
	if err != nil {
		return
	}

	if strings.HasPrefix(location, "//"+targetURL.Host) {
		resp.Header.Set("Location", strings.TrimSuffix(proxyOrigin, "/")+strings.TrimPrefix(location, "//"+targetURL.Host))
	}
}

func rewriteSetCookies(resp *http.Response) {
	cookies := resp.Cookies()
	if len(cookies) == 0 {
		return
	}

	proxyHost := resp.Request.Header.Get("X-WF-Proxy-Host")
	proxyScheme := resp.Request.Header.Get("X-WF-Proxy-Scheme")
	host := proxyHost
	if strings.Contains(host, ":") {
		if parsedHost, _, err := net.SplitHostPort(proxyHost); err == nil {
			host = parsedHost
		}
	}

	resp.Header.Del("Set-Cookie")
	for _, cookie := range cookies {
		if cookie == nil {
			continue
		}

		// Browsers reject Domain=.webflow.io for localhost responses, so force host-only cookies.
		cookie.Domain = ""
		if cookie.Path == "" {
			cookie.Path = "/"
		}

		// Local proxy runs on plain HTTP by default, so Secure cookies would be ignored by the browser.
		if proxyScheme == "http" && (host == "localhost" || net.ParseIP(host) != nil) {
			cookie.Secure = false
		}

		resp.Header.Add("Set-Cookie", cookie.String())
	}
}
