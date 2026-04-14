package config

import "testing"

func TestDefaultConfigMatchesPlan(t *testing.T) {
	cfg := Default()
	if cfg.FontSizePt != 12 {
		t.Fatalf("expected default font size 12, got %d", cfg.FontSizePt)
	}
	if cfg.Opacity != 10 {
		t.Fatalf("expected default opacity 10, got %d", cfg.Opacity)
	}
	if cfg.GapXRatio != 0.1 {
		t.Fatalf("expected default gap x ratio 0.1, got %.2f", cfg.GapXRatio)
	}
	if cfg.GapYRatio != 0.05 {
		t.Fatalf("expected default gap y ratio 0.05, got %.2f", cfg.GapYRatio)
	}
	if cfg.MinAreaPct != 3 {
		t.Fatalf("expected default min area pct 3, got %d", cfg.MinAreaPct)
	}
}

func TestValidateRejectsEmptyText(t *testing.T) {
	cfg := Default()
	cfg.Text = "   "
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error for empty text")
	}
}
