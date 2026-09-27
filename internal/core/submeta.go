package core

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type subMeta struct {
	newURL    string
	newDomain string
	fallback  string
}

func isHTTPSource(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func fetchSubRotate(source string) (string, string, error) {
	body, hdr, final, err := httpGetSub(source)
	used := source
	if err != nil {
		fb := readFallback()
		if fb == "" || sameSub(fb, source) {
			return "", "", err
		}
		body, hdr, final, err = httpGetSub(fb)
		if err != nil {
			return "", "", err
		}
		used = fb
	}
	expanded := expandSubPayload(body)
	meta := parseSubMeta(hdr, body, expanded)
	if meta.fallback != "" {
		_ = writeFallback(meta.fallback)
	}
	stored := applySubMeta(used, final, meta)
	if !sameSub(stored, used) && len(AllShareLines(expanded)) == 0 {
		body2, hdr2, final2, err2 := httpGetSub(stored)
		if err2 == nil {
			body = body2
			expanded = expandSubPayload(body2)
			meta2 := parseSubMeta(hdr2, body2, expanded)
			if meta2.fallback != "" {
				_ = writeFallback(meta2.fallback)
			}
			stored = applySubMeta(stored, final2, meta2)
		}
	}
	return expanded, StripSubHash(stored), nil
}

func applySubMeta(source, final string, meta subMeta) string {
	if meta.newURL != "" {
		return meta.newURL
	}
	base := source
	if strings.TrimSpace(final) != "" {
		base = final
	}
	if meta.newDomain != "" {
		if u, err := swapHost(base, meta.newDomain); err == nil {
			return u
		}
	}
	if strings.TrimSpace(final) != "" && !sameSub(final, source) {
		return final
	}
	return source
}

func httpGetSub(raw string) (string, http.Header, string, error) {
	raw = StripSubHash(strings.TrimSpace(raw))
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || !allowedSubURL(u) {
		return "", nil, "", fmt.Errorf("плохая ссылка подписки")
	}
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return "", nil, "", err
	}
	req.Header.Set("User-Agent", UserAgent())
	cli := &http.Client{
		Timeout: 20 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("слишком много переходов")
			}
			if req.URL == nil || !allowedSubURL(req.URL) {
				return fmt.Errorf("подписка ушла на закрытый адрес")
			}
			return nil
		},
	}
	res, err := cli.Do(req)
	if err != nil {
		return "", nil, "", fmt.Errorf("скачать подписку: %w", err)
	}
	defer res.Body.Close()
	final := raw
	if res.Request != nil && res.Request.URL != nil {
		final = res.Request.URL.String()
	}
	if res.StatusCode != http.StatusOK {
		return "", res.Header, final, fmt.Errorf("подписка HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", res.Header, final, err
	}
	return string(b), res.Header, final, nil
}

func parseSubMeta(h http.Header, bodies ...string) subMeta {
	var m subMeta
	if h != nil {
		m.newURL = cleanSubURL(firstHeader(h, "new-url", "subscription-url"))
		m.newDomain = cleanDomain(firstHeader(h, "new-domain", "subscription-domain"))
		m.fallback = cleanSubURL(firstHeader(h, "fallback-url"))
	}
	for _, body := range bodies {
		nu, nd, fb := metaFromBody(body)
		if nu != "" {
			m.newURL = nu
		}
		if nd != "" {
			m.newDomain = nd
		}
		if fb != "" {
			m.fallback = fb
		}
	}
	return m
}

func firstHeader(h http.Header, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(h.Get(k)); v != "" {
			return v
		}
	}
	return ""
}

func metaFromBody(body string) (newURL, newDomain, fallback string) {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "#") {
			continue
		}
		key, val := splitMetaKV(strings.TrimSpace(strings.TrimPrefix(line, "#")))
		switch key {
		case "new-url", "subscription-url":
			if u := cleanSubURL(val); u != "" {
				newURL = u
			}
		case "new-domain", "subscription-domain":
			if d := cleanDomain(val); d != "" {
				newDomain = d
			}
		case "fallback-url":
			if u := cleanSubURL(val); u != "" {
				fallback = u
			}
		}
	}
	return
}

func splitMetaKV(rest string) (string, string) {
	i := strings.IndexAny(rest, ": \t")
	if i <= 0 {
		return "", ""
	}
	return strings.ToLower(strings.TrimSpace(rest[:i])), strings.TrimSpace(rest[i+1:])
}

func cleanSubURL(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'`")
	if strings.HasPrefix(strings.ToLower(s), "base64:") {
		if b, err := decodeB64(s[len("base64:"):]); err == nil {
			s = strings.TrimSpace(string(b))
		}
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || !allowedSubURL(u) {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

func cleanDomain(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'`")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimSuffix(s, "/")
	if s == "" || strings.ContainsAny(s, "/?# \t") || strings.Contains(s, "..") {
		return ""
	}
	host := s
	if h, _, err := net.SplitHostPort(s); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return ""
	}
	for _, r := range host {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {
			continue
		}
		return ""
	}
	return s
}

func allowedSubURL(u *url.URL) bool {
	if u == nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || host == "localhost" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return os.Getenv("PLY_ALLOW_LOCAL_SUB") == "1"
		}
	}
	return true
}

func sameSub(a, b string) bool {
	a = StripSubHash(strings.TrimSpace(a))
	b = StripSubHash(strings.TrimSpace(b))
	ua, ea := url.Parse(a)
	ub, eb := url.Parse(b)
	if ea != nil || eb != nil || ua == nil || ub == nil || ua.Host == "" || ub.Host == "" {
		return a == b
	}
	pa, pb := strings.TrimRight(ua.Path, "/"), strings.TrimRight(ub.Path, "/")
	return strings.EqualFold(ua.Scheme, ub.Scheme) && strings.EqualFold(ua.Host, ub.Host) && pa == pb && ua.RawQuery == ub.RawQuery
}

func swapHost(raw, domain string) (string, error) {
	domain = cleanDomain(domain)
	if domain == "" {
		return "", fmt.Errorf("домен")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("ссылка")
	}
	if _, _, err := net.SplitHostPort(domain); err == nil {
		u.Host = domain
	} else if port := u.Port(); port != "" {
		u.Host = net.JoinHostPort(domain, port)
	} else {
		u.Host = domain
	}
	u.Fragment = ""
	if !allowedSubURL(u) {
		return "", fmt.Errorf("домен")
	}
	return u.String(), nil
}

func expandSubPayload(body string) string {
	if line, err := FirstShareLine(body); err == nil && line != "" {
		return body
	}
	if s := expandSubBody(body); s != body {
		return s
	}
	var b64 strings.Builder
	var head []string
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if strings.HasPrefix(t, "#") {
			head = append(head, t)
			continue
		}
		b64.WriteString(t)
	}
	if b64.Len() == 0 {
		return body
	}
	raw, err := decodeB64(b64.String())
	if err != nil {
		return body
	}
	s := string(raw)
	if _, err := FirstShareLine(s); err != nil {
		return body
	}
	if len(head) == 0 {
		return s
	}
	return strings.Join(head, "\n") + "\n" + s
}

func fallbackPath() string {
	dir, err := DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "fallback.txt")
}

func readFallback() string {
	p := fallbackPath()
	if p == "" {
		return ""
	}
	b, err := readSecret(p)
	if err != nil {
		return ""
	}
	return cleanSubURL(strings.TrimSpace(string(b)))
}

func writeFallback(u string) error {
	u = cleanSubURL(u)
	if u == "" {
		return nil
	}
	p := fallbackPath()
	if p == "" {
		return fmt.Errorf("нет папки данных")
	}
	return writeSecret(p, []byte(u+"\n"))
}

func keepSubURL(stored string) {
	stored = StripSubHash(strings.TrimSpace(stored))
	if isHTTPSource(stored) {
		_ = SaveURL(stored)
	}
}
