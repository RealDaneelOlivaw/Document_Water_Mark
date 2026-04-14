package watermark

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"pptwatermark/goapp/internal/config"
)

type Renderer struct{}

func (r *Renderer) CreateOverlay(cfg config.WatermarkConfig, width, height int) (*image.NRGBA, error) {
	if cfg.Position == "tile" {
		return r.createTiledOverlay(cfg, width, height)
	}
	overlay := image.NewNRGBA(image.Rect(0, 0, width, height))
	mark, err := r.CreateWatermarkImage(cfg)
	if err != nil {
		return nil, err
	}
	bounds := visibleBounds(mark)
	x, y := resolvePosition(cfg, bounds, width, height)
	pasteWithClipping(overlay, mark, x, y)
	return overlay, nil
}

func (r *Renderer) CreateWatermarkImage(cfg config.WatermarkConfig) (*image.NRGBA, error) {
	face, err := LoadFace(cfg.FontName, float64(cfg.FontSizePt))
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.ReplaceAll(cfg.Text, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		lines = []string{" "}
	}

	lineSpacing := maxInt(int(math.Round(float64(cfg.FontSizePt)*0.25)), 4)
	paddingX := maxInt(int(math.Round(float64(cfg.FontSizePt)*0.45)), 12)
	paddingY := maxInt(int(math.Round(float64(cfg.FontSizePt)*0.55)), 14)

	metrics := face.Metrics()
	ascent := metrics.Ascent.Round()
	descent := metrics.Descent.Round()
	lineHeight := maxInt(ascent+descent, 1)

	maxWidth := 1
	drawer := &font.Drawer{Face: face}
	for _, line := range lines {
		width := drawer.MeasureString(line).Ceil()
		if width > maxWidth {
			maxWidth = width
		}
	}

	totalHeight := lineHeight*len(lines) + lineSpacing*maxInt(len(lines)-1, 0)
	canvas := image.NewNRGBA(image.Rect(0, 0, maxWidth+paddingX*2, totalHeight+paddingY*2))
	textColor := parseColor(cfg.Color, cfg.Opacity)
	drawer = &font.Drawer{
		Dst:  canvas,
		Src:  image.NewUniform(textColor),
		Face: face,
	}

	baseline := paddingY + ascent
	for _, line := range lines {
		drawer.Dot = fixed.P(paddingX, baseline)
		drawer.DrawString(line)
		baseline += lineHeight + lineSpacing
	}

	if cfg.Rotation != 0 {
		canvas = imaging.Rotate(canvas, cfg.Rotation, color.NRGBA{0, 0, 0, 0})
		border := maxInt(int(math.Round(float64(cfg.FontSizePt)*0.18)), 6)
		withBorder := image.NewNRGBA(image.Rect(0, 0, canvas.Bounds().Dx()+border*2, canvas.Bounds().Dy()+border*2))
		draw.Draw(withBorder, image.Rect(border, border, border+canvas.Bounds().Dx(), border+canvas.Bounds().Dy()), canvas, image.Point{}, draw.Over)
		canvas = withBorder
	}
	return canvas, nil
}

func (r *Renderer) createTiledOverlay(cfg config.WatermarkConfig, width, height int) (*image.NRGBA, error) {
	overlay := image.NewNRGBA(image.Rect(0, 0, width, height))
	mark, err := r.CreateWatermarkImage(cfg)
	if err != nil {
		return nil, err
	}
	bounds := visibleBounds(mark)
	visibleWidth := maxInt(bounds.Dx(), 1)
	visibleHeight := maxInt(bounds.Dy(), 1)
	stepX := maxInt(int(math.Round(float64(visibleWidth)*(1.0+cfg.GapXRatio))), 1)
	stepY := maxInt(int(math.Round(float64(visibleHeight)*(1.0+cfg.GapYRatio))), 1)
	startX := cfg.MarginXPt - bounds.Min.X
	startY := cfg.MarginYPt - bounds.Min.Y

	for y := startY; y < height+visibleHeight+stepY; y += stepY {
		for x := startX; x < width+visibleWidth+stepX; x += stepX {
			pasteWithClipping(overlay, mark, x, y)
		}
	}
	return overlay, nil
}

func visibleBounds(img *image.NRGBA) image.Rectangle {
	bounds := img.Bounds()
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y
	found := false
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			found = true
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}
	if !found {
		return bounds
	}
	return image.Rect(minX, minY, maxX, maxY)
}

func resolvePosition(cfg config.WatermarkConfig, bbox image.Rectangle, width, height int) (int, int) {
	visibleWidth := bbox.Dx()
	visibleHeight := bbox.Dy()
	switch cfg.Position {
	case "top_left":
		return cfg.MarginXPt - bbox.Min.X, cfg.MarginYPt - bbox.Min.Y
	case "center":
		return maxInt((width-visibleWidth)/2-bbox.Min.X, -bbox.Min.X), maxInt((height-visibleHeight)/2-bbox.Min.Y, -bbox.Min.Y)
	case "bottom_right":
		return width - visibleWidth - cfg.MarginXPt - bbox.Min.X, height - visibleHeight - cfg.MarginYPt - bbox.Min.Y
	default:
		return cfg.MarginXPt - bbox.Min.X, cfg.MarginYPt - bbox.Min.Y
	}
}

func pasteWithClipping(canvas *image.NRGBA, mark *image.NRGBA, x, y int) {
	destLeft := maxInt(x, 0)
	destTop := maxInt(y, 0)
	destRight := minInt(x+mark.Bounds().Dx(), canvas.Bounds().Dx())
	destBottom := minInt(y+mark.Bounds().Dy(), canvas.Bounds().Dy())
	if destLeft >= destRight || destTop >= destBottom {
		return
	}
	src := image.Point{X: maxInt(0, -x), Y: maxInt(0, -y)}
	dest := image.Rect(destLeft, destTop, destRight, destBottom)
	draw.Draw(canvas, dest, mark, src, draw.Over)
}

func parseColor(hex string, opacity int) color.NRGBA {
	fmtHex := strings.TrimPrefix(hex, "#")
	var r, g, b uint64
	if len(fmtHex) == 6 {
		r, _ = strconv.ParseUint(fmtHex[0:2], 16, 8)
		g, _ = strconv.ParseUint(fmtHex[2:4], 16, 8)
		b, _ = strconv.ParseUint(fmtHex[4:6], 16, 8)
	}
	alpha := uint8(math.Round(255 * (float64(opacity) / 100.0)))
	return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: alpha}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
