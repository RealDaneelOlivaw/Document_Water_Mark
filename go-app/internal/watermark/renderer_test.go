package watermark

import (
	"testing"

	"pptwatermark/goapp/internal/config"
)

func TestBuildFontCandidatesHasFallbacks(t *testing.T) {
	candidates := BuildFontCandidates("微软雅黑")
	if len(candidates) == 0 {
		t.Fatal("expected font candidates")
	}
}

func TestRendererCreatesOverlay(t *testing.T) {
	cfg := config.Default()
	cfg.FontName = "Arial"
	cfg.Text = "CONFIDENTIAL"
	cfg.FontSizePt = 14

	if _, err := LoadFace(cfg.FontName, float64(cfg.FontSizePt)); err != nil {
		t.Skipf("skipping renderer test because no usable test font is available: %v", err)
	}

	renderer := &Renderer{}
	overlay, err := renderer.CreateOverlay(cfg, 240, 160)
	if err != nil {
		t.Fatalf("create overlay failed: %v", err)
	}
	if overlay.Bounds().Dx() != 240 || overlay.Bounds().Dy() != 160 {
		t.Fatalf("unexpected overlay size: %+v", overlay.Bounds())
	}
}
