package core

import (
	"encoding/base64"
	"strings"
	"testing"
)

const paper = "vless://0de5863f-fb38-473b-95b6-1a355c5345fd@2.27.175.32:443?type=tcp&security=reality&encryption=none&flow=xtls-rprx-vision&fp=firefox&sni=www.elastic.co&sid=&pbk=3rdiCNo7h8FvYrC7WdPYcTt7M2g8PhlRy9eCI2hLDB0"

func TestStripSubHash(t *testing.T) {
	u := "https://apimef.example.net/key#PaperVPN"
	if got := StripSubHash(u); got != "https://apimef.example.net/key" {
		t.Fatalf("got %q", got)
	}
}

func TestParseDoubleHashVLESS(t *testing.T) {
	raw := paper + "#887c535c-aa8a-49de-94c9-eb947b2c810a@papervpn.io#PaperVPN_LLM"
	n, err := ParseVLESS(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.Proto != "vless" {
		t.Fatalf("proto %s", n.Proto)
	}
	if n.Host != "2.27.175.32" || n.Port != 443 {
		t.Fatalf("addr %s:%d", n.Host, n.Port)
	}
	if n.SNI != "www.elastic.co" {
		t.Fatalf("sni %s", n.SNI)
	}
	if n.Flow != "xtls-rprx-vision" || n.Network != "raw" {
		t.Fatalf("flow/net %s %s", n.Flow, n.Network)
	}
	if n.PBK != "3rdiCNo7h8FvYrC7WdPYcTt7M2g8PhlRy9eCI2hLDB0" {
		t.Fatalf("pbk %s", n.PBK)
	}
	if n.ServerIPv4() != "2.27.175.32" {
		t.Fatalf("server ip %s", n.ServerIPv4())
	}
}

func TestParseVlessWS(t *testing.T) {
	raw := "vless://11111111-1111-4111-8111-111111111111@cdn.example.com:443?type=ws&security=tls&path=/vless&host=cdn.example.com&sni=cdn.example.com&fp=chrome#ws"
	n, err := ParseLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.Network != "ws" || n.Security != "tls" || n.Path != "/vless" || n.Flow != "" {
		t.Fatalf("ws node %+v", n)
	}
	b, err := RenderXray(n, 10808, false)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, need := range []string{`"protocol": "vless"`, `"wsSettings"`, `"path": "/vless"`, `"security": "tls"`} {
		if !strings.Contains(s, need) {
			t.Fatalf("missing %s in %s", need, s)
		}
	}
	if strings.Contains(s, "realitySettings") {
		t.Fatal("ws+tls should not use reality")
	}
}

func TestParseVMess(t *testing.T) {
	js := `{"v":"2","ps":"n1","add":"1.2.3.4","port":443,"id":"11111111-1111-4111-8111-111111111111","aid":0,"scy":"auto","net":"ws","type":"none","host":"a.example","path":"/vm","tls":"tls","sni":"a.example","fp":"chrome"}`
	raw := "vmess://" + base64.StdEncoding.EncodeToString([]byte(js))
	n, err := ParseLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.Proto != "vmess" || n.Host != "1.2.3.4" || n.Network != "ws" || n.Security != "tls" {
		t.Fatalf("%+v", n)
	}
	b, _ := RenderXray(n, 10808, false)
	if !strings.Contains(string(b), `"protocol": "vmess"`) {
		t.Fatal(string(b))
	}
}

func TestParseTrojan(t *testing.T) {
	raw := "trojan://secret-pass@node.example.net:443?security=tls&sni=node.example.net&type=tcp#eu"
	n, err := ParseLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.Proto != "trojan" || n.Password != "secret-pass" || n.Security != "tls" {
		t.Fatalf("%+v", n)
	}
	b, _ := RenderXray(n, 10808, false)
	if !strings.Contains(string(b), `"protocol": "trojan"`) {
		t.Fatal(string(b))
	}
}

func TestParseSS(t *testing.T) {
	user := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:hunter2"))
	raw := "ss://" + user + "@10.1.2.3:8388#home"
	n, err := ParseLink(raw)
	if err != nil {
		t.Fatal(err)
	}
	if n.Proto != "shadowsocks" || n.Method != "aes-256-gcm" || n.Password != "hunter2" || n.Host != "10.1.2.3" || n.Port != 8388 {
		t.Fatalf("%+v", n)
	}
	legacy := "ss://" + base64.StdEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:abc@8.8.8.8:443"))
	n2, err := ParseLink(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if n2.Host != "8.8.8.8" || n2.Method != "chacha20-ietf-poly1305" {
		t.Fatalf("legacy %+v", n2)
	}
}

func TestRejectHysteria(t *testing.T) {
	_, err := ParseLink("hy2://pass@host:443?sni=x")
	if err == nil || !strings.Contains(err.Error(), "Hysteria") {
		t.Fatalf("err %v", err)
	}
}

func TestFirstShareFromBase64Sub(t *testing.T) {
	body := paper + "\nvmess://xxx"
	line, err := FirstShareLine(body)
	if err != nil || !strings.HasPrefix(line, "vless://") {
		t.Fatalf("%q %v", line, err)
	}
	enc := base64.StdEncoding.EncodeToString([]byte(paper + "\n"))
	got := expandSubBody(enc)
	line, err = FirstShareLine(got)
	if err != nil || !strings.HasPrefix(line, "vless://") {
		t.Fatalf("b64 sub %q %v", line, err)
	}
}

func TestFirstShareSkipsHysteria(t *testing.T) {
	body := "hy2://pass@h:443\n" + paper + "\n"
	line, err := FirstShareLine(body)
	if err != nil || !strings.HasPrefix(line, "vless://") {
		t.Fatalf("%q %v", line, err)
	}
}

func TestFirstVLESS(t *testing.T) {
	body := "not a key\nvless://abc@h:443?security=reality&pbk=x#n\n"
	got, err := FirstVLESS(body)
	if err != nil {
		t.Fatal(err)
	}
	if got[:8] != "vless://" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderXrayHasTUN(t *testing.T) {
	n, err := ParseVLESS(paper)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RenderXray(n, 10808, false)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, need := range []string{`"protocol": "tun"`, `"desc": "Ply"`, `"autoSystemRoutingTable"`, `"0.0.0.0/1"`, `"128.0.0.0/1"`, `"mux"`, `"realitySettings"`} {
		if !strings.Contains(s, need) {
			t.Fatalf("config missing %s", need)
		}
	}
	if !strings.Contains(s, `"enabled": false`) {
		t.Fatal("mux should be off")
	}
	if !strings.Contains(s, "2.27.175.32") {
		t.Fatal("server ip should be direct")
	}
	if strings.Contains(s, "geoip:ru") {
		t.Fatal("full tunnel should not bypass ru")
	}
}

func TestRenderXraySplitRussia(t *testing.T) {
	n, err := ParseVLESS(paper)
	if err != nil {
		t.Fatal(err)
	}
	b, err := RenderXray(n, 10808, true)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, need := range []string{
		"geoip:ru",
		"geosite:tld-ru",
		"geosite:category-ru",
		"geosite:yandex",
		"geosite:vk",
		".ru$",
		"xn--p1ai",
		"IPIfNonMatch",
		"77.88.8.8",
		"domain:vk.com",
	} {
		if !strings.Contains(s, need) {
			t.Fatalf("split config missing %s", need)
		}
	}
}

func TestReadSplitDefaultOn(t *testing.T) {
	if !ReadSplit() {
		t.Fatal("missing file should mean Russia-direct on")
	}
}
