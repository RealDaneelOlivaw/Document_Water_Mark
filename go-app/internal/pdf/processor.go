package pdf

import (
	"bytes"
	"fmt"
	"image/png"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"pptwatermark/goapp/internal/config"
	"pptwatermark/goapp/internal/openxml/common"
	"pptwatermark/goapp/internal/watermark"
)

type Processor struct {
	renderer *watermark.Renderer
}

func New() *Processor {
	return &Processor{renderer: &watermark.Renderer{}}
}

func (p *Processor) ProcessFile(inputPath, outputPath string, cfg config.WatermarkConfig) (common.ProcessStats, error) {
	if err := cfg.Validate(); err != nil {
		return common.ProcessStats{}, err
	}

	pageDims, err := api.PageDimsFile(inputPath)
	if err != nil {
		return common.ProcessStats{}, err
	}
	pageCount := len(pageDims)
	stats := common.ProcessStats{TotalImages: pageCount}
	if pageCount == 0 {
		return stats, fmt.Errorf("pdf contains no pages")
	}

	if cfg.ExcludePagesEnabled {
		if err := config.ValidateExcludePagesAgainstTotal(cfg.ExcludePages, pageCount); err != nil {
			return stats, err
		}
	}
	excludePages := config.BuildExcludePageSet(cfg.ExcludePagesEnabled, cfg.ExcludePages)
	selectedPages := buildSelectedPages(pageCount, excludePages)
	stats.WatermarkedImages = len(selectedPages)
	stats.SkippedImages = pageCount - len(selectedPages)
	if len(selectedPages) == 0 {
		return stats, copyFile(inputPath, outputPath)
	}

	watermarks := map[int]*model.Watermark{}
	overlayCache := map[string][]byte{}
	for _, pageNumber := range selectedPages {
		dim := pageDims[pageNumber-1]
		overlay, err := p.overlayForPage(cfg, dim, overlayCache)
		if err != nil {
			return stats, err
		}
		wm, err := api.ImageWatermarkForReader(
			bytes.NewReader(overlay),
			"scale:1 abs, pos:c, rot:0, op:1",
			true,
			false,
			types.POINTS,
		)
		if err != nil {
			return stats, err
		}
		watermarks[pageNumber] = wm
	}

	conf := model.NewDefaultConfiguration()
	conf.OptimizeDuplicateContentStreams = false
	if err := api.AddWatermarksMapFile(inputPath, outputPath, watermarks, conf); err != nil {
		return stats, err
	}
	return stats, nil
}

func (p *Processor) overlayForPage(cfg config.WatermarkConfig, dim types.Dim, cache map[string][]byte) ([]byte, error) {
	width := maxInt(int(math.Round(dim.Width)), 1)
	height := maxInt(int(math.Round(dim.Height)), 1)
	key := strings.Join([]string{
		strconv.Itoa(width),
		strconv.Itoa(height),
		cfg.Text,
		cfg.FontName,
		strconv.Itoa(cfg.FontSizePt),
		strconv.Itoa(cfg.Opacity),
		cfg.Color,
		fmt.Sprintf("%.2f", cfg.Rotation),
		cfg.Position,
		fmt.Sprintf("%.3f", cfg.GapXRatio),
		fmt.Sprintf("%.3f", cfg.GapYRatio),
		strconv.Itoa(cfg.MarginXPt),
		strconv.Itoa(cfg.MarginYPt),
	}, "|")
	if cached := cache[key]; cached != nil {
		return cached, nil
	}

	overlay, err := p.renderer.CreateOverlay(cfg, width, height)
	if err != nil {
		return nil, err
	}
	buffer := &bytes.Buffer{}
	if err := png.Encode(buffer, overlay); err != nil {
		return nil, err
	}
	cache[key] = buffer.Bytes()
	return cache[key], nil
}

func buildSelectedPages(pageCount int, excludePages map[int]struct{}) []int {
	selected := make([]int, 0, pageCount)
	for page := 1; page <= pageCount; page++ {
		if _, excluded := excludePages[page]; excluded {
			continue
		}
		selected = append(selected, page)
	}
	return selected
}

func copyFile(sourcePath, targetPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer target.Close()

	if _, err := target.ReadFrom(source); err != nil {
		return err
	}
	return target.Close()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
