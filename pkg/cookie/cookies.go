package cookie

import (
	"context"
	"strings"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all"
)

// Config определяет конфигурацию для извлечения cookies.
type Config struct {
	SanitizeValues bool // Очищать ли значения cookies
}

// DefaultConfig возвращает конфигурацию по умолчанию.
func DefaultConfig() Config {
	return Config{
		SanitizeValues: true,
	}
}

// GetAllCookies собирает cookies из всех поддерживаемых браузеров.
func GetAllCookies(domains []string, config ...Config) ([]Cookie, error) {
	cfg := DefaultConfig()
	if len(config) > 0 {
		cfg = config[0]
	}

	var allCookies []Cookie

	// kooky.ReadCookies can read cookies from all stores that match filters
	// If no filter is given, it reads all. Let's build a custom filter to match any of the domains.
	domainFilter := kooky.FilterFunc(func(c *kooky.Cookie) bool {
		if len(domains) == 0 {
			return true
		}
		for _, domain := range domains {
			if strings.HasSuffix(c.Domain, domain) || c.Domain == domain {
				return true
			}
		}
		return false
	})

	kookyCookies, err := kooky.ReadCookies(context.Background(), domainFilter, kooky.Valid)
	if err != nil {
		if len(kookyCookies) == 0 {
			return nil, err
		}
	}

	for _, kc := range kookyCookies {
		value := kc.Value
		if cfg.SanitizeValues {
			value = SanitizeCookieValue(value)
		}

		c := Cookie{
			Domain:   kc.Domain,
			Path:     kc.Path,
			Secure:   kc.Secure,
			Name:     kc.Name,
			Value:    value,
			HttpOnly: kc.HttpOnly,
			SameSite: 0, // Not perfectly mapped by kooky.Cookie.http.Cookie
		}

		if !kc.Expires.IsZero() {
			exp := kc.Expires.Unix()
			c.Expires = &exp
		}

		allCookies = append(allCookies, c)
	}

	return allCookies, nil
}

// SanitizeCookieValue очищает значение cookie, оставляя только печатные ASCII символы.
func SanitizeCookieValue(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
		}
	}
	return b.String()
}
