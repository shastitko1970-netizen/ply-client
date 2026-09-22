package core

import "testing"

func TestNormalizePrefs(t *testing.T) {
	p := NormalizePrefs(Prefs{Theme: "neon", Density: "huge", MTU: 9999, Core: "sing-box", Silent: true, Split: false})
	if p.Theme != "system" || p.Density != "compact" || p.MTU != 1400 || p.Core != "xray" {
		t.Fatalf("%+v", p)
	}
	if !p.Silent || p.Split {
		t.Fatalf("flags %+v", p)
	}
}

func TestUseCoreRejectsSecond(t *testing.T) {
	if err := UseCore("xray"); err != nil {
		t.Fatal(err)
	}
	if err := UseCore("sing-box"); err == nil {
		t.Fatal("second core must stay unloaded")
	}
}

func TestDefaultMTU(t *testing.T) {
	if EffectiveMTU() != 1400 && EffectiveMTU() != 1280 && EffectiveMTU() != 1500 {
		t.Fatal(EffectiveMTU())
	}
}
