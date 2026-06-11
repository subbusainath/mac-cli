package tui

import (
	"strings"
	"testing"
)

func TestLerpHexEndpoints(t *testing.T) {
	if got := lerpHex("#7C3AED", "#3B82F6", 0); got != "#7C3AED" {
		t.Fatalf("t=0 got %s", got)
	}
	if got := lerpHex("#7C3AED", "#3B82F6", 1); got != "#3B82F6" {
		t.Fatalf("t=1 got %s", got)
	}
}

func TestBannerFrameContainsLogoAndTagline(t *testing.T) {
	frame := renderBannerFrame(1.0) // final frame: full gradient + full tagline
	if !strings.Contains(frame, "█") {
		t.Fatal("logo missing")
	}
	for _, ch := range []string{"M", "y", "A", "g", "e", "n", "t", "i", "c", "C", "L", "I"} {
		if !strings.Contains(frame, ch) {
			t.Fatalf("tagline incomplete, missing %q", ch)
		}
	}
}

func TestBannerFrameZeroShowsMACOnly(t *testing.T) {
	frame := renderBannerFrame(0)
	if strings.Contains(frame, "Agentic") {
		t.Fatal("tagline must not be expanded at t=0")
	}
}
