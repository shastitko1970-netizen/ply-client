package core

import "testing"

func TestStripSubHash(t *testing.T) {
	u := "https://apimef.example.net/key#PaperVPN"
	if got := StripSubHash(u); got != "https://apimef.example.net/key" {
		t.Fatalf("got %q", got)
	}
}

func TestParseDoubleHashVLESS(t *testing.T) {
	raw := "vless://0de5863f-fb38-473b-95b6-1a355c5345fd@2.27.175.32:443?type=tcp&security=reality&encryption=none&flow=xtls-rprx-vision&fp=firefox&sni=www.elastic.co&sid=&pbk=3rdiCNo7h8FvYrC7WdPYcTt7M2g8PhlRy9eCI2hLDB0#887c535c-aa8a-49de-94c9-eb947b2c810a@papervpn.io#PaperVPN_LLM"
	n, err := ParseVLESS(raw)
	if err != nil {
		t.Fatal(err)
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
