package config

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const (
	DefaultText       = "\u8d85\u5353\u822a\u79d1 \u5e7f\u5dde\u7814\u7a76\u9662"
	DefaultFontName   = "\u5fae\u8f6f\u96c5\u9ed1"
	DefaultFontSizePt = 12
	DefaultOpacity    = 10
	DefaultColor      = "#000000"
	DefaultRotation   = 30.0
	DefaultPosition   = "tile"
	DefaultGapXRatio  = 0.1
	DefaultGapYRatio  = 0.05
	DefaultMarginXPt  = 1
	DefaultMarginYPt  = 1
	DefaultMinAreaPct = 3
)

var (
	AllowedPositions = []string{"top_left", "center", "bottom_right", "tile"}
	hexColorPattern  = regexp.MustCompile(`^#(?:[0-9a-fA-F]{6})$`)
)

type WatermarkConfig struct {
	Text       string
	FontName   string
	FontSizePt int
	Opacity    int
	Color      string
	Rotation   float64
	Position   string
	GapXRatio  float64
	GapYRatio  float64
	MarginXPt  int
	MarginYPt  int
	MinAreaPct int
}

func Default() WatermarkConfig {
	return WatermarkConfig{
		Text:       DefaultText,
		FontName:   DefaultFontName,
		FontSizePt: DefaultFontSizePt,
		Opacity:    DefaultOpacity,
		Color:      DefaultColor,
		Rotation:   DefaultRotation,
		Position:   DefaultPosition,
		GapXRatio:  DefaultGapXRatio,
		GapYRatio:  DefaultGapYRatio,
		MarginXPt:  DefaultMarginXPt,
		MarginYPt:  DefaultMarginYPt,
		MinAreaPct: DefaultMinAreaPct,
	}
}

func (c WatermarkConfig) Validate() error {
	if strings.TrimSpace(c.Text) == "" {
		return errors.New("watermark text cannot be empty")
	}
	if strings.TrimSpace(c.FontName) == "" {
		return errors.New("font name cannot be empty")
	}
	if c.FontSizePt <= 0 {
		return errors.New("font size must be greater than 0")
	}
	if c.Opacity < 0 || c.Opacity > 100 {
		return errors.New("opacity must be in range 0-100")
	}
	if !hexColorPattern.MatchString(c.Color) {
		return errors.New("color must be a 6-digit hex string like #RRGGBB")
	}
	if !containsString(AllowedPositions, c.Position) {
		return fmt.Errorf("position must be one of %s", strings.Join(AllowedPositions, ", "))
	}
	if c.GapXRatio < 0 || c.GapYRatio < 0 {
		return errors.New("gap ratios must be greater than or equal to 0")
	}
	if c.MinAreaPct < 0 || c.MinAreaPct > 100 {
		return errors.New("min area percent must be in range 0-100")
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
