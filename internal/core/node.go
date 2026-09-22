package core

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const UA = "Ply/1.7"

type Node struct {
	Proto    string
	UUID     string
	Password string
	Method   string
	Host     string
	Port     int
	Flow     string
	Security string
	Network  string
	SNI      string
	FP       string
	PBK      string
	SID      string
	Spx      string
	Enc      string
	Path     string
	HostHdr  string
	ALPN     string
	Service  string
	Mode     string
	Header   string
	AlterID  int
	Insecure bool
	Remark   string
}

func (n *Node) Label() string {
	if n == nil {
		return ""
	}
	p := n.Proto
	if p == "" {
		p = "vless"
	}
	extra := n.Network
	if n.Security != "" && n.Security != "none" {
		if extra != "" {
			extra += "+"
		}
		extra += n.Security
	}
	if extra == "" {
		return p
	}
	return p + " · " + extra
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
	if err := rejectUnsupported(source); err != nil {
		return nil, err
	}
	switch {
	case isShare(source):
		return ParseLink(source)
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		body, err := FetchSub(source)
		if err != nil {
			return nil, err
		}
		line, err := FirstShareLine(expandSubBody(body))
		if err != nil {
			return nil, err
		}
		return ParseLink(line)
	default:
		return nil, fmt.Errorf("нужна ссылка подписки или ключ vless/vmess/trojan/ss")
	}
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
			n.outbound(),
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

func (n *Node) outbound() map[string]any {
	mux := map[string]any{"enabled": false, "concurrency": -1}
	switch n.Proto {
	case "vmess":
		return map[string]any{
			"tag":      "proxy",
			"protocol": "vmess",
			"settings": map[string]any{
				"vnext": []any{
					map[string]any{
						"address": n.Host,
						"port":    n.Port,
						"users": []any{
							map[string]any{
								"id": n.UUID, "alterId": n.AlterID,
								"security": firstNonEmpty(n.Enc, "auto"),
							},
						},
					},
				},
			},
			"streamSettings": n.streamSettings(),
			"mux":            mux,
		}
	case "trojan":
		return map[string]any{
			"tag":      "proxy",
			"protocol": "trojan",
			"settings": map[string]any{
				"servers": []any{
					map[string]any{
						"address": n.Host, "port": n.Port,
						"password": firstNonEmpty(n.Password, n.UUID),
					},
				},
			},
			"streamSettings": n.streamSettings(),
			"mux":            mux,
		}
	case "shadowsocks":
		return map[string]any{
			"tag":      "proxy",
			"protocol": "shadowsocks",
			"settings": map[string]any{
				"servers": []any{
					map[string]any{
						"address": n.Host, "port": n.Port,
						"method": n.Method, "password": n.Password,
					},
				},
			},
			"streamSettings": n.streamSettings(),
			"mux":            mux,
		}
	default:
		user := map[string]any{"id": n.UUID, "encryption": firstNonEmpty(n.Enc, "none")}
		if n.Flow != "" {
			user["flow"] = n.Flow
		}
		return map[string]any{
			"tag":      "proxy",
			"protocol": "vless",
			"settings": map[string]any{
				"vnext": []any{
					map[string]any{
						"address": n.Host,
						"port":    n.Port,
						"users":   []any{user},
					},
				},
			},
			"streamSettings": n.streamSettings(),
			"mux":            mux,
		}
	}
}

func (n *Node) streamSettings() map[string]any {
	netw := n.Network
	if netw == "tcp" || netw == "" {
		netw = "raw"
	}
	ss := map[string]any{"network": netw}

	switch n.Security {
	case "tls":
		ss["security"] = "tls"
		tls := map[string]any{"allowInsecure": n.Insecure}
		if n.SNI != "" {
			tls["serverName"] = n.SNI
		}
		if n.FP != "" {
			tls["fingerprint"] = n.FP
		}
		if n.ALPN != "" {
			parts := strings.Split(n.ALPN, ",")
			alpn := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					alpn = append(alpn, p)
				}
			}
			if len(alpn) > 0 {
				tls["alpn"] = alpn
			}
		}
		ss["tlsSettings"] = tls
	case "reality":
		ss["security"] = "reality"
		ss["realitySettings"] = map[string]any{
			"serverName":  n.SNI,
			"fingerprint": firstNonEmpty(n.FP, "chrome"),
			"publicKey":   n.PBK,
			"shortId":     n.SID,
			"spiderX":     n.Spx,
			"show":        false,
		}
	default:
		ss["security"] = "none"
	}

	switch netw {
	case "ws":
		ws := map[string]any{}
		if n.Path != "" {
			ws["path"] = n.Path
		}
		if n.HostHdr != "" {
			ws["host"] = n.HostHdr
		}
		ss["wsSettings"] = ws
	case "grpc":
		grpc := map[string]any{"serviceName": n.Service}
		if n.Mode == "multi" {
			grpc["multiMode"] = true
		}
		ss["grpcSettings"] = grpc
	case "httpupgrade":
		hu := map[string]any{}
		if n.Path != "" {
			hu["path"] = n.Path
		}
		if n.HostHdr != "" {
			hu["host"] = n.HostHdr
		}
		ss["httpupgradeSettings"] = hu
	case "xhttp":
		xh := map[string]any{}
		if n.Path != "" {
			xh["path"] = n.Path
		}
		if n.HostHdr != "" {
			xh["host"] = n.HostHdr
		}
		if n.Mode != "" {
			xh["mode"] = n.Mode
		}
		ss["xhttpSettings"] = xh
	case "h2":
		h2 := map[string]any{}
		if n.Path != "" {
			h2["path"] = n.Path
		}
		if n.HostHdr != "" {
			h2["host"] = []string{n.HostHdr}
		}
		ss["httpSettings"] = h2
	case "raw":
		if n.Header == "http" {
			ss["tcpSettings"] = map[string]any{
				"header": map[string]any{"type": "http"},
			}
		}
	}
	return ss
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}
