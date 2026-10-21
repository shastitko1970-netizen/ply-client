package core

import (
	"net"
	"strings"
	"testing"
)

func TestNormalizeBypassKeepsProcessCase(t *testing.T) {
	got := NormalizeBypass([]string{"Steam.EXE", "self/", `C:\Games\cs2.exe`, "steam.exe", "ply.EXE", "WOW.EXE", "wow.exe"})
	joined := strings.Join(got, "\n")
	if strings.Contains(joined, "steam.exe") {
		t.Fatalf("dedupe must keep the first casing, got %q", joined)
	}
	if strings.Contains(strings.ToLower(joined), "ply.exe") || strings.Contains(joined, "self/") {
		t.Fatalf("own process leaked: %q", joined)
	}
	if !strings.Contains(joined, "Steam.exe") {
		t.Fatalf("bare name must keep its letters and use a lowercase .exe suffix, got %q", joined)
	}
	if strings.Contains(joined, "Steam.EXE") {
		t.Fatalf("Xray only strips a lowercase .exe suffix, got %q", joined)
	}
	if !strings.Contains(joined, "C:/Games/cs2.exe") {
		t.Fatalf("path case must stay, got %q", joined)
	}
	if !strings.Contains(joined, "WOW.exe") {
		t.Fatalf("second distinct name keeps its own letters, got %q", joined)
	}
}

func TestNormalizeBypassResolvesDiskCase(t *testing.T) {
	prev := bypassCaseFn
	bypassCaseFn = func(s string) string {
		switch strings.ToLower(s) {
		case "wow.exe":
			return "Wow.exe"
		case "c:/program files/hoyoplay/":
			return "C:/Program Files/HoYoPlay/"
		default:
			return ""
		}
	}
	t.Cleanup(func() { bypassCaseFn = prev })

	got := NormalizeBypass([]string{"wow.EXE", `c:\Program Files\HoYoPlay\`, "Wow.exe"})
	if len(got) != 2 || got[0] != "Wow.exe" || got[1] != "C:/Program Files/HoYoPlay/" {
		t.Fatalf("%q", got)
	}
}

func TestNormalizeBypassUppercasesDrive(t *testing.T) {
	got := NormalizeBypass([]string{`c:/Windows/System32/curl.exe`})
	if len(got) != 1 || got[0] != "C:/Windows/System32/curl.exe" {
		t.Fatalf("%q", got)
	}
}

func TestStableParentStripsVersionedFolder(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`C:\Program Files\HoYoPlay\1.4.2.199\HYP.exe`, "C:/Program Files/HoYoPlay/"},
		{`C:\ProgramData\Battle.net\Agent\Agent.9370\Agent.exe`, "C:/ProgramData/Battle.net/Agent/"},
		{`C:\Program Files (x86)\Battle.net\Battle.net.5383\Battle.net.exe`, "C:/Program Files (x86)/Battle.net/"},
		{`D:\Games\ZenlessZoneZero\ZenlessZoneZero.exe`, ""},
		{`C:\1.2.3\game.exe`, ""},
		{`C:\Program Files\HoYoPlay\`, ""},
	}
	for _, tc := range cases {
		got, ok := stableParent(tc.in)
		if tc.want == "" {
			if ok {
				t.Fatalf("%s -> %s", tc.in, got)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Fatalf("%s -> %q ok=%v", tc.in, got, ok)
		}
	}
}

func TestPresetCatalogIsSpecific(t *testing.T) {
	for _, name := range []string{"HYP.exe", "HYPHelper.exe", "launcher.exe", "ZenlessZoneZero.exe", "HYUpdater.exe"} {
		if !containsStr(HoyoProcesses, name) {
			t.Fatalf("missing hoyo process %s", name)
		}
	}
	for _, name := range []string{"Battle.net.exe", "Agent.exe", "Wow.exe", "WowClassic.exe", "WowT.exe"} {
		if !containsStr(BlizzardProcesses, name) {
			t.Fatalf("missing blizzard process %s", name)
		}
	}
	for _, d := range []string{"domain:zenlesszonezero.com", "domain:hoyoverse.com", "domain:mihoyo.com", "domain:hoyolab.com"} {
		if !containsStr(HoyoDomains, d) {
			t.Fatalf("missing hoyo domain %s", d)
		}
	}
	for _, d := range []string{"domain:battle.net", "domain:blizzard.com"} {
		if !containsStr(BlizzardDomains, d) {
			t.Fatalf("missing blizzard domain %s", d)
		}
	}
	for _, junk := range []string{"akamai", "cloudflare", "google", "amazon", "akamaized"} {
		for _, d := range append(append([]string{}, HoyoDomains...), BlizzardDomains...) {
			if strings.Contains(d, junk) {
				t.Fatalf("unverified CDN domain %s", d)
			}
		}
	}
	if len(BlizzardIPv4) < 10 {
		t.Fatal("expected AS57976 aggregates")
	}
	for _, cidr := range BlizzardIPv4 {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			t.Fatal(cidr, err)
		}
	}
	if !containsStr(BlizzardIPv4, "24.105.0.0/18") || !containsStr(BlizzardIPv4, "137.221.64.0/18") || !containsStr(BlizzardIPv4, "5.42.160.0/19") {
		t.Fatalf("missing well-known Blizzard blocks: %v", BlizzardIPv4)
	}
}

func TestRunningHitSuggestsStableFolder(t *testing.T) {
	hit := runningHit(`C:\Program Files\HoYoPlay\1.4.2.199\HYP.exe`)
	if hit.Kind != "folder" || hit.Path != "C:/Program Files/HoYoPlay/" || hit.Name != "HoYoPlay" {
		t.Fatalf("%+v", hit)
	}
	plain := runningHit(`D:\Games\ZenlessZoneZero\ZenlessZoneZero.exe`)
	if plain.Kind != "running" || plain.Path != "D:/Games/ZenlessZoneZero/ZenlessZoneZero.exe" || plain.Name != "ZenlessZoneZero.exe" {
		t.Fatalf("%+v", plain)
	}
	agent := runningHit(`C:\ProgramData\Battle.net\Agent\Agent.9370\Agent.exe`)
	if agent.Kind != "folder" || agent.Path != "C:/ProgramData/Battle.net/Agent/" {
		t.Fatalf("%+v", agent)
	}
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
