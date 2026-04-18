package picture

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"pptwatermark/goapp/internal/config"
	"pptwatermark/goapp/internal/openxml/common"
	"pptwatermark/goapp/internal/watermark"
)

var supportedExtensions = map[string]struct{}{
	".jpg":  {},
	".jpeg": {},
	".png":  {},
	".bmp":  {},
	".gif":  {},
	".tif":  {},
	".tiff": {},
}

type Processor struct {
	renderer *watermark.Renderer
}

func New() *Processor {
	return &Processor{renderer: &watermark.Renderer{}}
}

func IsSupportedExtension(ext string) bool {
	_, ok := supportedExtensions[strings.ToLower(ext)]
	return ok
}

func (p *Processor) ProcessFile(inputPath, outputPath string, cfg config.WatermarkConfig) (common.ProcessStats, error) {
	stats := common.ProcessStats{TotalImages: 1}

	if err := cfg.Validate(); err != nil {
		stats.SkippedImages = 1
		return stats, err
	}

	ext := strings.ToLower(filepath.Ext(inputPath))
	if !IsSupportedExtension(ext) {
		stats.SkippedImages = 1
		return stats, fmt.Errorf("unsupported image type: %s", ext)
	}

	sourceBlob, err := os.ReadFile(inputPath)
	if err != nil {
		stats.SkippedImages = 1
		return stats, err
	}

	cfgInfo, _, err := image.DecodeConfig(bytes.NewReader(sourceBlob))
	if err != nil {
		stats.SkippedImages = 1
		return stats, err
	}
	if cfgInfo.Width <= 0 || cfgInfo.Height <= 0 {
		stats.SkippedImages = 1
		return stats, fmt.Errorf("invalid image dimensions")
	}

	format := common.DetectFormat(inputPath, sourceBlob)
	metrics := common.ComputeRenderMetrics(
		cfg,
		cfgInfo.Width,
		cfgInfo.Height,
		cfgInfo.Width,
		cfgInfo.Height,
		72.0,
	)
	scaledCfg := common.BuildScaledConfig(cfg, metrics)

	rendered, err := common.RenderWatermarkedBlob(
		p.renderer,
		sourceBlob,
		format,
		scaledCfg,
		cfgInfo.Width,
		cfgInfo.Height,
	)
	if err != nil {
		stats.SkippedImages = 1
		return stats, err
	}

	if err := os.WriteFile(outputPath, rendered, 0o644); err != nil {
		stats.SkippedImages = 1
		return stats, err
	}

	stats.WatermarkedImages = 1
	return stats, nil
}
