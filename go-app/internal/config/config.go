package config

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	DefaultText       = "超卓航科广州研究院"
	DefaultFontName   = "微软雅黑"
	DefaultFontSizePt = 12
	DefaultOpacity    = 6
	DefaultColor      = "#000000"
	DefaultRotation   = 0.0
	DefaultPosition   = "tile"
	DefaultGapXRatio  = 0.2
	DefaultGapYRatio  = 1.5
	DefaultMarginXPt  = 2
	DefaultMarginYPt  = 2
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

	ExcludePagesEnabled   bool
	ExcludePages          []int
	DeleteOriginalEnabled bool
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
	if c.ExcludePagesEnabled {
		if len(c.ExcludePages) == 0 {
			return errors.New("exclude pages cannot be empty when enabled")
		}
		prev := 0
		for idx, page := range c.ExcludePages {
			if page <= 0 {
				return fmt.Errorf("exclude page %d must be greater than 0", page)
			}
			if idx > 0 && page <= prev {
				return errors.New("exclude pages must be strictly ascending with no duplicates")
			}
			prev = page
		}
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

func ParseExcludePages(raw string) ([]int, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, errors.New("exclude pages cannot be empty")
	}

	replacer := strings.NewReplacer(
		"\uFF0C", ",",
		"\u3001", ",",
		"\uFF1B", ",",
		";", ",",
		"\n", ",",
		"\t", ",",
		" ", "",
	)
	normalized := replacer.Replace(text)
	segments := strings.Split(normalized, ",")
	pageSet := map[int]struct{}{}

	for _, segment := range segments {
		token := strings.TrimSpace(segment)
		if token == "" {
			return nil, errors.New("exclude pages contains empty segment")
		}

		if strings.Contains(token, "-") {
			parts := strings.Split(token, "-")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				return nil, fmt.Errorf("invalid page range: %s", token)
			}
			start, err := strconv.Atoi(parts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid page value in range: %s", token)
			}
			end, err := strconv.Atoi(parts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid page value in range: %s", token)
			}
			if start <= 0 || end <= 0 {
				return nil, fmt.Errorf("page range must use positive numbers: %s", token)
			}
			if end < start {
				return nil, fmt.Errorf("page range end must be greater than or equal to start: %s", token)
			}
			for page := start; page <= end; page++ {
				pageSet[page] = struct{}{}
			}
			continue
		}

		page, err := strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("invalid page value: %s", token)
		}
		if page <= 0 {
			return nil, fmt.Errorf("page value must be greater than 0: %d", page)
		}
		pageSet[page] = struct{}{}
	}

	pages := make([]int, 0, len(pageSet))
	for page := range pageSet {
		pages = append(pages, page)
	}
	sort.Ints(pages)
	return pages, nil
}

func ValidateExcludePagesAgainstTotal(pages []int, totalPages int) error {
	if totalPages <= 0 {
		return errors.New("total pages must be greater than 0")
	}
	for _, page := range pages {
		if page <= 0 {
			return fmt.Errorf("page value must be greater than 0: %d", page)
		}
		if page > totalPages {
			return fmt.Errorf("exclude page %d exceeds total pages %d", page, totalPages)
		}
	}
	return nil
}

func BuildExcludePageSet(enabled bool, pages []int) map[int]struct{} {
	if !enabled || len(pages) == 0 {
		return nil
	}
	pageSet := make(map[int]struct{}, len(pages))
	for _, page := range pages {
		pageSet[page] = struct{}{}
	}
	return pageSet
}
