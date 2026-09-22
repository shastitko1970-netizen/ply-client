package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const UA = "Ply/1.6"

type Node struct {
	UUID, Host, Flow, Security, Network, SNI, FP, PBK, SID, Spx, Enc string
	Port                                                             int
}

func StripSubHash(raw string) string {
	t := strings.TrimSpace(raw)
	if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") {
		if i := strings.Index(t, "#"); i >= 0 {
			return t[:i]
		}
	}
	return t
}

func FirstVLESS(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", fmt.Errorf("подписка пустая")
	}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "vless://") {
			return line, nil
		}
	}
	if strings.HasPrefix(body, "vless://") {
		return body, nil
	}
	return "", fmt.Errorf("в ответе нет vless://")
}

func ParseVLESS(raw string) (*Node, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "vless://") {
		return nil, fmt.Errorf("не vless")
	}
	if i := strings.Index(raw, "#"); i >= 0 {
		raw = raw[:i]
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("разобрать vless: %w", err)
	}
	port := 443
	if u.Port() != "" {
		port, _ = strconv.Atoi(u.Port())
	}
	q := u.Query()
	n := &Node{
		UUID:     u.User.Username(),
		Host:     u.Hostname(),
		Port:     port,
		Flow:     q.Get("flow"),
		Security: q.Get("security"),
		Network:  q.Get("type"),
		SNI:      firstNonEmpty(q.Get("sni"), q.Get("serverName")),
		FP:       firstNonEmpty(q.Get("fp"), q.Get("fingerprint"), "firefox"),
		PBK:      firstNonEmpty(q.Get("pbk"), q.Get("publicKey")),
		SID:      q.Get("sid"),
		Spx:      q.Get("spx"),
		Enc:      firstNonEmpty(q.Get("encryption"), "none"),
	}
	if n.Network == "tcp" || n.Network == "" {
		n.Network = "raw"
	}
	if n.UUID == "" || n.Host == "" || n.PBK == "" || n.Security != "reality" {
		return nil, fmt.Errorf("битый ключ: uuid/host/pbk/reality")
	}
	if n.Flow == "" {
		n.Flow = "xtls-rprx-vision"
	}
	return n, nil
}

func (n *Node) ServerIPv4() string {
	if n == nil {
		return ""
	}
	if ip := net.ParseIP(n.Host); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4.String()
		}
	}
	ips, err := net.LookupIP(n.Host)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String()
		}
	}
	return ""
}

func FetchSub(sub string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, StripSubHash(sub), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", UA)
	cli := &http.Client{Timeout: 20 * time.Second}
	res, err := cli.Do(req)
	if err != nil {
		return "", fmt.Errorf("скачать подписку: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("подписка HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Resolve(source string) (*Node, error) {
	source = strings.TrimSpace(source)
	var vless string
	var err error
	switch {
	case strings.HasPrefix(source, "vless://"):
		vless = source
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		var body string
		body, err = FetchSub(source)
		if err != nil {
			return nil, err
		}
		vless, err = FirstVLESS(body)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("нужна ссылка кабинета или vless://")
	}
	return ParseVLESS(vless)
}

func RenderXray(n *Node, port int, split bool) ([]byte, error) {
	rules := []any{}
	if ip := net.ParseIP(n.Host); ip != nil {
		rules = append(rules, map[string]any{"type": "field", "ip": []string{n.Host}, "outboundTag": "direct"})
	}
	rules = append(rules, map[string]any{"type": "field", "ip": []string{"geoip:private"}, "outboundTag": "direct"})
	if split {
		rules = append(rules,
			map[string]any{"type": "field", "domain": RussiaDirectDomains(), "outboundTag": "direct"},
			map[string]any{"type": "field", "ip": RussiaDirectIPs(), "outboundTag": "direct"},
		)
	}
	rules = append(rules,
		map[string]any{"type": "field", "network": "udp", "port": "443", "outboundTag": "block"},
		map[string]any{"type": "field", "port": "0-65535", "outboundTag": "proxy"},
	)

	dns := map[string]any{
		"servers":       []any{"1.1.1.1", "8.8.8.8"},
		"queryStrategy": "UseIPv4",
	}
	if split {
		dns = map[string]any{
			"servers": []any{
				map[string]any{
					"address":      "77.88.8.8",
					"domains":      RussiaDirectDomains(),
					"skipFallback": true,
				},
				"1.1.1.1",
				"8.8.8.8",
			},
			"queryStrategy": "UseIPv4",
		}
	}

	strategy := "AsIs"
	if split {
		strategy = "IPIfNonMatch"
	}

	cfg := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"dns": dns,
		"inbounds": []any{
			map[string]any{
				"tag":      "tun",
				"protocol": "tun",
				"settings": map[string]any{
					"name":                   "ply0",
					"desc":                   "Ply",
					"mtu":                    1500,
					"gateway":                []string{"198.18.0.1/16"},
					"dns":                    []string{"1.1.1.1", "8.8.8.8"},
					"autoSystemRoutingTable": []string{"0.0.0.0/1", "128.0.0.0/1"},
					"autoOutboundsInterface": "auto",
				},
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []string{"http", "tls", "quic"},
				},
			},
			map[string]any{
				"tag":      "socks",
				"port":     port,
				"listen":   "127.0.0.1",
				"protocol": "mixed",
				"sniffing": map[string]any{
					"enabled":      true,
					"destOverride": []string{"http", "tls", "quic"},
				},
				"settings": map[string]any{"auth": "noauth", "udp": true},
			},
		},
		"outbounds": []any{
			map[string]any{
				"tag":      "proxy",
				"protocol": "vless",
				"settings": map[string]any{
					"vnext": []any{
						map[string]any{
							"address": n.Host,
							"port":    n.Port,
							"users": []any{
								map[string]any{
									"id": n.UUID, "encryption": n.Enc, "flow": n.Flow,
								},
							},
						},
					},
				},
				"streamSettings": map[string]any{
					"network":  n.Network,
					"security": "reality",
					"realitySettings": map[string]any{
						"serverName": n.SNI, "fingerprint": n.FP, "publicKey": n.PBK,
						"shortId": n.SID, "spiderX": n.Spx, "show": false,
					},
				},
				"mux": map[string]any{"enabled": false, "concurrency": -1},
			},
			map[string]any{"tag": "direct", "protocol": "freedom"},
			map[string]any{"tag": "block", "protocol": "blackhole"},
		},
		"routing": map[string]any{
			"domainStrategy": strategy,
			"rules":          rules,
		},
	}
	return json.MarshalIndent(cfg, "", "  ")
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
