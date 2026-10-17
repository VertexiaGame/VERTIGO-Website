package models

import (
	"strings"
	"testing"
)

func TestNormalizeBannerEffect(t *testing.T) {
	cases := map[string]string{
		"":          BannerEffectNone,
		"none":      BannerEffectNone,
		"Blur":      BannerEffectBlur,
		" BLUR ":    BannerEffectBlur,
		"darken":    BannerEffectDarken,
		"zoom":      BannerEffectZoom,
		"drift":     BannerEffectDrift,
		"grayscale": BannerEffectNone,
		"sepia":     BannerEffectNone,
		"<script>":  BannerEffectNone,
	}
	for in, want := range cases {
		if got := NormalizeBannerEffect(in); got != want {
			t.Fatalf("NormalizeBannerEffect(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBannerEffectFilter(t *testing.T) {
	animated := map[string]string{
		BannerEffectZoom:  "prfbnr-zoom",
		BannerEffectDrift: "prfbnr-drift",
	}
	for effect, keyframe := range animated {
		if got := string(BannerEffectFilter(effect)); !strings.Contains(got, keyframe) {
			t.Fatalf("BannerEffectFilter(%q) missing %q: %q", effect, keyframe, got)
		}
	}
	if got := string(BannerEffectFilter(BannerEffectBlur)); !strings.Contains(got, "blur(8px)") {
		t.Fatalf("blur filter wrong: %q", got)
	}
	if got := string(BannerEffectFilter(BannerEffectDarken)); !strings.Contains(got, "brightness(0.65)") {
		t.Fatalf("darken filter wrong: %q", got)
	}
	if got := string(BannerEffectFilter("nope")); got != "" {
		t.Fatalf("unknown effect should be empty, got %q", got)
	}
	if got := string(BannerEffectFilter("")); got != "" {
		t.Fatalf("empty effect should be empty, got %q", got)
	}
}

func TestTextBgStyle(t *testing.T) {
	got := string((&User{BannerTextBgColor: "FF0000", BannerTextBgBorder: "00FF00", BannerTextBgText: "0000FF", BannerTextBgOpacity: 50}).TextBgStyle())
	if got != "background:rgba(255,0,0,0.50);border-color:#00FF00;color:#0000FF;" {
		t.Fatalf("text bg style = %q", got)
	}
	def := string((&User{BannerTextBgOpacity: 78}).TextBgStyle())
	if def != "background:rgba(240,240,240,0.78);border-color:#A9A9A9;color:#1A202C;" {
		t.Fatalf("text bg default style = %q", def)
	}
	clamped := string((&User{BannerTextBgColor: "zzz", BannerTextBgOpacity: 250}).TextBgStyle())
	if clamped != "background:rgba(240,240,240,1.00);border-color:#A9A9A9;color:#1A202C;" {
		t.Fatalf("text bg clamped style = %q", clamped)
	}
}

func TestBannerEffectName(t *testing.T) {
	if got := BannerEffectName(BannerEffectZoom); got != "Zoom" {
		t.Fatalf("effect name = %q", got)
	}
	if got := BannerEffectName("bogus"); got != "None" {
		t.Fatalf("unknown effect name = %q", got)
	}
}
