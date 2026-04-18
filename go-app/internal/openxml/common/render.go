package common

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/disintegration/imaging"
	"golang.org/x/image/bmp"
	_ "golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	_ "golang.org/x/image/tiff"

	"pptwatermark/goapp/internal/config"
	"pptwatermark/goapp/internal/watermark"
)

const (
	EMUPerInch  = 914400.0
	TwipPerInch = 1440.0
	EMUPerTwip  = 635.0
)

type ProcessStats struct {
	TotalImages       int
	WatermarkedImages int
	SkippedImages     int
}

type RenderMetrics struct {
	FontSizePx int
	MarginXPx  int
	MarginYPx  int
}

func ComputeRenderMetrics(cfg config.WatermarkConfig, imagePxWidth, imagePxHeight, displayWidthUnits, displayHeightUnits int, unitsPerInch float64) RenderMetrics {
	displayWIn := math.Max(float64(displayWidthUnits)/unitsPerInch, 0.1)
	displayHIn := math.Max(float64(displayHeightUnits)/unitsPerInch, 0.1)
	dpiX := float64(imagePxWidth) / displayWIn
	dpiY := float64(imagePxHeight) / displayHIn
	avgDPI := (dpiX + dpiY) / 2.0
	return RenderMetrics{
		FontSizePx: maxInt(int(math.Round(float64(cfg.FontSizePt)*avgDPI/72.0)), 1),
		MarginXPx:  maxInt(int(math.Round(float64(cfg.MarginXPt)*dpiX/72.0)), 0),
		MarginYPx:  maxInt(int(math.Round(float64(cfg.MarginYPt)*dpiY/72.0)), 0),
	}
}

func BuildScaledConfig(cfg config.WatermarkConfig, metrics RenderMetrics) config.WatermarkConfig {
	scaled := cfg
	scaled.FontSizePt = maxInt(metrics.FontSizePx, 1)
	scaled.MarginXPt = maxInt(metrics.MarginXPx, 0)
	scaled.MarginYPt = maxInt(metrics.MarginYPx, 0)
	return scaled
}

func RenderWatermarkedBlob(renderer *watermark.Renderer, source []byte, format string, cfg config.WatermarkConfig, width, height int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(source))
	if err != nil {
		return nil, err
	}

	original := imaging.Clone(img)
	overlay, err := renderer.CreateOverlay(cfg, width, height)
	if err != nil {
		return nil, err
	}
	result := image.NewNRGBA(original.Bounds())
	draw.Draw(result, result.Bounds(), original, image.Point{}, draw.Src)
	draw.Draw(result, result.Bounds(), overlay, image.Point{}, draw.Over)

	buffer := &bytes.Buffer{}
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		err = jpeg.Encode(buffer, result, &jpeg.Options{Quality: 95})
	case "png":
		err = png.Encode(buffer, result)
	case "gif":
		err = gif.Encode(buffer, result, nil)
	case "bmp":
		err = bmp.Encode(buffer, result)
	case "tiff", "tif":
		err = tiff.Encode(buffer, result, nil)
	default:
		err = png.Encode(buffer, result)
	}
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func DetectFormat(sourcePath string, source []byte) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(sourcePath)), ".")
	if ext != "" {
		return ext
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(source))
	if err == nil {
		return format
	}
	return "png"
}

func HashBytes(value []byte) string {
	sum := md5.Sum(value)
	return hex.EncodeToString(sum[:])
}

func UniqueOutputPath(inputPath string) string {
	dir := filepath.Dir(inputPath)
	ext := filepath.Ext(inputPath)
	base := strings.TrimSuffix(filepath.Base(inputPath), ext)
	candidate := filepath.Join(dir, base+"_watermarked"+ext)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for idx := 2; ; idx++ {
		next := filepath.Join(dir, base+"_watermarked_"+strconv.Itoa(idx)+ext)
		if _, err := os.Stat(next); os.IsNotExist(err) {
			return next
		}
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
