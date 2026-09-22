package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func isShare(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(s, "vless://") ||
		strings.HasPrefix(s, "vmess://") ||
		strings.HasPrefix(s, "trojan://") ||
		strings.HasPrefix(s, "ss://") ||
		strings.HasPrefix(s, "hysteria2://") ||
		strings.HasPrefix(s, "hy2://")
}

func rejectUnsupported(s string) error {
	low := strings.ToLower(strings.TrimSpace(s))
	switch {
	case strings.HasPrefix(low, "hysteria://"):
		return fmt.Errorf("Hysteria v1 Xray не умеет — нужен hy2://, vless, vmess, trojan или ss")
	case strings.HasPrefix(low, "tuic://"):
		return fmt.Errorf("TUIC Xray не умеет — нужен hy2, vless, vmess, trojan или ss")
	case strings.HasPrefix(low, "wireguard://"), strings.HasPrefix(low, "wg://"):
		return fmt.Errorf("WireGuard-ключ пока не разбираю — нужен vless, vmess, trojan, ss или hy2")
	}
	return nil
}

func splitRemark(raw string) (body, remark string) {
	raw = strings.TrimSpace(raw)
	i := strings.Index(raw, "#")
	if i < 0 {
		return raw, ""
	}
	body = strings.TrimSpace(raw[:i])
	rest := raw[i+1:]
	if j := strings.Index(rest, "#"); j >= 0 {
		rest = rest[:j]
	}
	remark, err := url.QueryUnescape(rest)
	if err != nil {
		remark = rest
	}
	return body, strings.TrimSpace(remark)
}

func decodeB64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, s)
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	if pad := len(s) % 4; pad != 0 {
		s += strings.Repeat("=", 4-pad)
	}
	return base64.StdEncoding.DecodeString(s)
}

func expandSubBody(body string) string {
	body = strings.TrimSpace(body)
	if line, err := FirstShareLine(body); err == nil && line != "" {
		return body
	}
	compact := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, body)
	b, err := decodeB64(compact)
	if err != nil {
		return body
	}
	s := string(b)
	if line, err := FirstShareLine(s); err == nil && line != "" {
		return s
	}
	return body
}

func FirstShareLine(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", fmt.Errorf("подписка пустая")
	}
	var unsupported error
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := rejectUnsupported(line); err != nil {
			if unsupported == nil {
				unsupported = err
			}
			continue
		}
		if isShare(line) {
			return line, nil
		}
	}
	if isShare(body) {
		return body, nil
	}
	if unsupported != nil {
		return "", unsupported
	}
	return "", fmt.Errorf("в ответе нет ключа vless/vmess/trojan/ss/hy2")
}

func FirstVLESS(body string) (string, error) {
	return FirstShareLine(body)
}

func ParseLink(raw string) (*Node, error) {
	raw = strings.TrimSpace(raw)
	if err := rejectUnsupported(raw); err != nil {
		return nil, err
	}
	low := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(low, "vless://"):
		return ParseVLESS(raw)
	case strings.HasPrefix(low, "vmess://"):
		return ParseVMess(raw)
	case strings.HasPrefix(low, "trojan://"):
		return ParseTrojan(raw)
	case strings.HasPrefix(low, "ss://"):
		return ParseSS(raw)
	case strings.HasPrefix(low, "hysteria2://"), strings.HasPrefix(low, "hy2://"):
		return ParseHysteria(raw)
	default:
		return nil, fmt.Errorf("не ключ")
	}
}

func ParseVLESS(raw string) (*Node, error) {
	body, remark := splitRemark(raw)
	if !strings.HasPrefix(strings.ToLower(body), "vless://") {
		return nil, fmt.Errorf("не vless")
	}
	u, err := url.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("разобрать vless: %w", err)
	}
	q := u.Query()
	n := &Node{
		Proto:    "vless",
		UUID:     u.User.Username(),
		Host:     u.Hostname(),
		Port:     portOf(u, 443),
		Flow:     q.Get("flow"),
		Security: strings.ToLower(q.Get("security")),
		Network:  firstNonEmpty(q.Get("type"), q.Get("network")),
		SNI:      firstNonEmpty(q.Get("sni"), q.Get("serverName")),
		FP:       firstNonEmpty(q.Get("fp"), q.Get("fingerprint")),
		PBK:      firstNonEmpty(q.Get("pbk"), q.Get("publicKey")),
		SID:      q.Get("sid"),
		Spx:      q.Get("spx"),
		Enc:      firstNonEmpty(q.Get("encryption"), "none"),
		Path:     firstNonEmpty(q.Get("path"), q.Get("serviceName")),
		HostHdr:  q.Get("host"),
		ALPN:     q.Get("alpn"),
		Service:  firstNonEmpty(q.Get("serviceName"), q.Get("service")),
		Mode:     q.Get("mode"),
		Header:   q.Get("headerType"),
		Insecure: truthy(firstNonEmpty(q.Get("allowInsecure"), q.Get("insecure"))),
		Remark:   remark,
	}
	normalizeTransport(n)
	if n.UUID == "" || n.Host == "" {
		return nil, fmt.Errorf("битый ключ: нет uuid или хоста")
	}
	if n.Security == "reality" && n.PBK == "" {
		return nil, fmt.Errorf("для reality нужен pbk")
	}
	return n, nil
}

func ParseVMess(raw string) (*Node, error) {
	body, remark := splitRemark(raw)
	if !strings.HasPrefix(strings.ToLower(body), "vmess://") {
		return nil, fmt.Errorf("не vmess")
	}
	payload := body[len("vmess://"):]
	decoded, err := decodeB64(payload)
	if err != nil {
		return nil, fmt.Errorf("разобрать vmess: %w", err)
	}
	var obj map[string]any
	if err := json.Unmarshal(decoded, &obj); err != nil {
		return nil, fmt.Errorf("vmess json: %w", err)
	}
	tls := strings.ToLower(anyStr(obj["tls"]))
	sec := "none"
	if tls == "tls" || tls == "xtls" {
		sec = "tls"
	} else if tls == "reality" {
		sec = "reality"
	}
	n := &Node{
		Proto:    "vmess",
		UUID:     anyStr(obj["id"]),
		Host:     anyStr(obj["add"]),
		Port:     anyInt(obj["port"], 443),
		Enc:      firstNonEmpty(anyStr(obj["scy"]), anyStr(obj["security"]), "auto"),
		Network:  firstNonEmpty(anyStr(obj["net"]), "tcp"),
		Security: sec,
		SNI:      firstNonEmpty(anyStr(obj["sni"]), anyStr(obj["serverName"])),
		FP:       firstNonEmpty(anyStr(obj["fp"]), anyStr(obj["fingerprint"])),
		PBK:      firstNonEmpty(anyStr(obj["pbk"]), anyStr(obj["publicKey"])),
		SID:      anyStr(obj["sid"]),
		Path:     anyStr(obj["path"]),
		HostHdr:  anyStr(obj["host"]),
		ALPN:     anyStr(obj["alpn"]),
		Service:  firstNonEmpty(anyStr(obj["serviceName"]), anyStr(obj["path"])),
		Header:   anyStr(obj["type"]),
		AlterID:  anyInt(obj["aid"], 0),
		Insecure: truthy(anyStr(obj["allowInsecure"])),
		Remark:   firstNonEmpty(remark, anyStr(obj["ps"])),
	}
	normalizeTransport(n)
	if n.UUID == "" || n.Host == "" {
		return nil, fmt.Errorf("битый vmess: нет id или адреса")
	}
	return n, nil
}

func ParseTrojan(raw string) (*Node, error) {
	body, remark := splitRemark(raw)
	if !strings.HasPrefix(strings.ToLower(body), "trojan://") {
		return nil, fmt.Errorf("не trojan")
	}
	u, err := url.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("разобрать trojan: %w", err)
	}
	pass, _ := u.User.Password()
	if pass == "" {
		pass = u.User.Username()
	}
	q := u.Query()
	sec := strings.ToLower(q.Get("security"))
	if sec == "" {
		sec = "tls"
	}
	n := &Node{
		Proto:    "trojan",
		Password: pass,
		UUID:     pass,
		Host:     u.Hostname(),
		Port:     portOf(u, 443),
		Security: sec,
		Network:  firstNonEmpty(q.Get("type"), q.Get("network"), "tcp"),
		SNI:      firstNonEmpty(q.Get("sni"), q.Get("peer"), q.Get("serverName")),
		FP:       firstNonEmpty(q.Get("fp"), q.Get("fingerprint")),
		Path:     q.Get("path"),
		HostHdr:  q.Get("host"),
		ALPN:     q.Get("alpn"),
		Service:  firstNonEmpty(q.Get("serviceName"), q.Get("service")),
		Mode:     q.Get("mode"),
		Header:   q.Get("headerType"),
		Insecure: truthy(firstNonEmpty(q.Get("allowInsecure"), q.Get("insecure"))),
		Remark:   remark,
	}
	normalizeTransport(n)
	if n.Password == "" || n.Host == "" {
		return nil, fmt.Errorf("битый trojan: нет пароля или хоста")
	}
	return n, nil
}

func ParseSS(raw string) (*Node, error) {
	body, remark := splitRemark(raw)
	if !strings.HasPrefix(strings.ToLower(body), "ss://") {
		return nil, fmt.Errorf("не ss")
	}
	rest := body[5:]
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		rest = rest[:i]
	}
	var method, password, host string
	port := 443
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		userPart, err := url.PathUnescape(rest[:at])
		if err != nil {
			userPart = rest[:at]
		}
		decoded := userPart
		if !strings.Contains(userPart, ":") {
			if b, err := decodeB64(userPart); err == nil && strings.Contains(string(b), ":") {
				decoded = string(b)
			}
		}
		method, password = splitOnce(decoded, ":")
		host, port = splitHostPort(rest[at+1:], 443)
	} else {
		b, err := decodeB64(rest)
		if err != nil {
			return nil, fmt.Errorf("разобрать ss: %w", err)
		}
		s := string(b)
		at := strings.LastIndex(s, "@")
		if at < 0 {
			return nil, fmt.Errorf("битый ss")
		}
		method, password = splitOnce(s[:at], ":")
		host, port = splitHostPort(s[at+1:], 443)
	}
	if method == "" || password == "" || host == "" {
		return nil, fmt.Errorf("битый ss: method/password/host")
	}
	return &Node{
		Proto:    "shadowsocks",
		Method:   method,
		Password: password,
		Host:     host,
		Port:     port,
		Network:  "raw",
		Security: "none",
		Remark:   remark,
	}, nil
}

func collapseHopPorts(raw string) string {
	at := strings.LastIndex(raw, "@")
	if at < 0 {
		return raw
	}
	rest := raw[at+1:]
	cut := len(rest)
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		cut = i
	}
	hp := rest[:cut]
	if i := strings.Index(hp, ","); i >= 0 {
		hp = hp[:i]
	}
	return raw[:at+1] + hp + rest[cut:]
}

func ParseHysteria(raw string) (*Node, error) {
	body, remark := splitRemark(raw)
	low := strings.ToLower(body)
	if !strings.HasPrefix(low, "hysteria2://") && !strings.HasPrefix(low, "hy2://") {
		return nil, fmt.Errorf("не hysteria2")
	}
	body = collapseHopPorts(body)
	u, err := url.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("разобрать hy2: %w", err)
	}
	q := u.Query()
	auth := ""
	if u.User != nil {
		auth = u.User.Username()
		if p, ok := u.User.Password(); ok && p != "" {
			auth = auth + ":" + p
		}
	}
	if auth == "" {
		auth = q.Get("auth")
	}
	obfs := strings.ToLower(q.Get("obfs"))
	obfsPass := firstNonEmpty(q.Get("obfs-password"), q.Get("obfsPassword"))
	if obfs != "" && obfs != "salamander" {
		return nil, fmt.Errorf("obfs %s Xray не умеет — только salamander", obfs)
	}
	n := &Node{
		Proto:    "hysteria",
		Password: auth,
		Host:     u.Hostname(),
		Port:     portOf(u, 443),
		Security: "tls",
		Network:  "hysteria",
		SNI:      firstNonEmpty(q.Get("sni"), q.Get("peer")),
		FP:       firstNonEmpty(q.Get("fp"), q.Get("fingerprint"), "chrome"),
		ALPN:     firstNonEmpty(q.Get("alpn"), "h3"),
		Insecure: truthy(firstNonEmpty(q.Get("insecure"), q.Get("allowInsecure"))),
		Obfs:     obfs,
		ObfsPass: obfsPass,
		Pin:      firstNonEmpty(q.Get("pinSHA256"), q.Get("pinsha256"), q.Get("pin")),
		Remark:   remark,
	}
	if n.SNI == "" && net.ParseIP(n.Host) == nil {
		n.SNI = n.Host
	}
	if n.Password == "" || n.Host == "" {
		return nil, fmt.Errorf("битый hy2: нет пароля или хоста")
	}
	return n, nil
}

func normalizeTransport(n *Node) {
	netw := strings.ToLower(n.Network)
	switch netw {
	case "", "tcp":
		n.Network = "raw"
	case "websocket":
		n.Network = "ws"
	case "h2", "http":
		n.Network = "h2"
	case "splithttp":
		n.Network = "xhttp"
	case "grpc", "gun":
		n.Network = "grpc"
	default:
		n.Network = netw
	}
	if n.Security == "" && n.PBK != "" {
		n.Security = "reality"
	}
	if n.Security == "" {
		n.Security = "none"
	}
	if n.Network != "raw" {
		n.Flow = ""
	}
	if n.Flow == "" && n.Network == "raw" && n.Security == "reality" && n.Proto == "vless" {
		n.Flow = "xtls-rprx-vision"
	}
	if n.FP == "" {
		if n.Security == "reality" {
			n.FP = "firefox"
		} else if n.Security == "tls" {
			n.FP = "chrome"
		}
	}
	if n.SNI == "" && n.Security != "none" && net.ParseIP(n.Host) == nil {
		n.SNI = n.Host
	}
	if n.Network == "grpc" && n.Service == "" && n.Path != "" {
		n.Service = strings.TrimPrefix(n.Path, "/")
	}
}

func portOf(u *url.URL, def int) int {
	if u.Port() == "" {
		return def
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil || p <= 0 {
		return def
	}
	return p
}

func splitHostPort(s string, def int) (string, int) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", def
	}
	host, ps, err := net.SplitHostPort(s)
	if err != nil {
		return s, def
	}
	p, err := strconv.Atoi(ps)
	if err != nil || p <= 0 {
		return host, def
	}
	return host, p
}

func splitOnce(s, sep string) (string, string) {
	i := strings.Index(s, sep)
	if i < 0 {
		return s, ""
	}
	return s[:i], s[i+len(sep):]
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func anyStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(t)
	}
}

func anyInt(v any, def int) int {
	s := anyStr(v)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
