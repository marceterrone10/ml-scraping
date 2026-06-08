package scraper

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type browserCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Secure   bool    `json:"secure"`
	HTTPOnly bool    `json:"httpOnly"`
	Expires  float64 `json:"expirationDate"`
}

// LoadCookies reads cookies from JSON or Netscape (cookies.txt) format.
func LoadCookies(path string) ([]*http.Cookie, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cookies file: %w", err)
	}

	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("cookies file is empty")
	}

	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ".") || !strings.HasPrefix(trimmed, "[") {
		return parseNetscapeCookies(trimmed)
	}
	return parseJSONCookies(trimmed)
}

func parseJSONCookies(data string) ([]*http.Cookie, error) {
	var exported []browserCookie
	if err := json.Unmarshal([]byte(data), &exported); err != nil {
		return nil, fmt.Errorf("parse cookies JSON: %w", err)
	}
	if len(exported) == 0 {
		return nil, fmt.Errorf("cookies file is empty")
	}

	cookies := make([]*http.Cookie, 0, len(exported))
	for _, c := range exported {
		if cookie := browserCookieToHTTP(c); cookie != nil {
			cookies = append(cookies, cookie)
		}
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no valid cookies found")
	}
	return cookies, nil
}

func parseNetscapeCookies(data string) ([]*http.Cookie, error) {
	scanner := bufio.NewScanner(strings.NewReader(data))
	cookies := make([]*http.Cookie, 0)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			continue
		}

		domain := fields[0]
		path := fields[2]
		secure := strings.EqualFold(fields[3], "true")
		name := fields[5]
		value := fields[6]

		if name == "" || value == "" {
			continue
		}
		if path == "" {
			path = "/"
		}

		cookie := &http.Cookie{
			Name:   name,
			Value:  value,
			Domain: strings.TrimPrefix(domain, "."),
			Path:   path,
			Secure: secure,
		}

		if exp, err := parseNetscapeExpiry(fields[4]); err == nil && exp > 0 {
			cookie.Expires = time.Unix(exp, 0)
		}

		cookies = append(cookies, cookie)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse cookies file: %w", err)
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("no valid cookies found in Netscape format")
	}
	return cookies, nil
}

func parseNetscapeExpiry(raw string) (int64, error) {
	var exp int64
	_, err := fmt.Sscanf(raw, "%d", &exp)
	return exp, err
}

func browserCookieToHTTP(c browserCookie) *http.Cookie {
	if c.Name == "" || c.Value == "" {
		return nil
	}
	domain := strings.TrimPrefix(c.Domain, ".")
	if domain == "" {
		domain = "mercadolibre.com.ar"
	}
	path := c.Path
	if path == "" {
		path = "/"
	}
	cookie := &http.Cookie{
		Name:     c.Name,
		Value:    c.Value,
		Domain:   domain,
		Path:     path,
		Secure:   c.Secure,
		HttpOnly: c.HTTPOnly,
	}
	if c.Expires > 0 {
		cookie.Expires = time.Unix(int64(c.Expires), 0)
	}
	return cookie
}
