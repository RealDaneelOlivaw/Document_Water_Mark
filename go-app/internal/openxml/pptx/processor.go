package pptx

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/beevik/etree"

	"pptwatermark/goapp/internal/config"
	"pptwatermark/goapp/internal/openxml/common"
	"pptwatermark/goapp/internal/watermark"
)

type Processor struct {
	renderer *watermark.Renderer
}

type imageInstance struct {
	element     *etree.Element
	ownerXML    string
	relsPath    string
	rID         string
	widthEMU    int
	heightEMU   int
	sizeIsKnown bool
}

type relContext struct {
	doc      *etree.Document
	root     *etree.Element
	path     string
	ownerXML string
	byID     map[string]*etree.Element
}

func New() *Processor {
	return &Processor{renderer: &watermark.Renderer{}}
}

func (p *Processor) ProcessFile(inputPath, outputPath string, cfg config.WatermarkConfig) (common.ProcessStats, error) {
	if err := cfg.Validate(); err != nil {
		return common.ProcessStats{}, err
	}

	tempDir, err := common.UnzipToTemp(inputPath, "pptx_go_")
	if err != nil {
		return common.ProcessStats{}, err
	}
	defer os.RemoveAll(tempDir)

	slideWidth, slideHeight := readSlideSize(filepath.Join(tempDir, "ppt", "presentation.xml"))
	slideArea := slideWidth * slideHeight
	mediaDir := filepath.Join(tempDir, "ppt", "media")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return common.ProcessStats{}, err
	}

	mediaCache := map[string]string{}
	renderCache := map[string][]byte{}
	stats := common.ProcessStats{}

	slideFiles, err := filepath.Glob(filepath.Join(tempDir, "ppt", "slides", "slide*.xml"))
	if err != nil {
		return stats, err
	}

	for _, slideFile := range slideFiles {
		ownerXML := filepath.ToSlash(strings.TrimPrefix(slideFile, tempDir+string(os.PathSeparator)))
		relsPath := filepath.Join(tempDir, "ppt", "slides", "_rels", filepath.Base(slideFile)+".rels")
		if _, err := os.Stat(relsPath); err != nil {
			continue
		}

		xmlDoc := etree.NewDocument()
		if err := xmlDoc.ReadFromFile(slideFile); err != nil {
			return stats, err
		}
		relDoc, relRoot, relByID, err := loadRelationships(relsPath)
		if err != nil {
			return stats, err
		}
		ctx := relContext{doc: relDoc, root: relRoot, path: relsPath, ownerXML: ownerXML, byID: relByID}

		for _, instance := range collectSlideInstances(xmlDoc.Root(), ownerXML, filepath.ToSlash(strings.TrimPrefix(relsPath, tempDir+string(os.PathSeparator)))) {
			stats.TotalImages++
			if cfg.MinAreaPct > 0 && slideArea > 0 && instance.sizeIsKnown {
				pct := (float64(instance.widthEMU*instance.heightEMU) / float64(slideArea)) * 100.0
				if pct < float64(cfg.MinAreaPct) {
					stats.SkippedImages++
					continue
				}
			}
			rel := ctx.byID[instance.rID]
			if rel == nil {
				stats.SkippedImages++
				continue
			}
			target, ok := common.GetAttr(rel, "Target")
			if !ok {
				stats.SkippedImages++
				continue
			}
			mediaRelPath := common.ResolveTarget(ownerXML, target)
			mediaPath := filepath.Join(tempDir, filepath.FromSlash(mediaRelPath))
			sourceBlob, err := os.ReadFile(mediaPath)
			if err != nil {
				stats.SkippedImages++
				continue
			}
			format := common.DetectFormat(mediaPath, sourceBlob)
			cfgScaled, renderKey, widthPx, heightPx, err := buildScaledInstanceConfig(cfg, mediaPath, sourceBlob, format, instance.widthEMU, instance.heightEMU)
			if err != nil {
				stats.SkippedImages++
				continue
			}
			newMediaRelPath, ok := mediaCache[renderKey]
			if !ok {
				rendered := renderCache[renderKey]
				if rendered == nil {
					rendered, err = common.RenderWatermarkedBlob(p.renderer, sourceBlob, format, cfgScaled, widthPx, heightPx)
					if err != nil {
						stats.SkippedImages++
						continue
					}
					renderCache[renderKey] = rendered
				}
				newMediaRelPath, err = writeGeneratedMedia(tempDir, "ppt/media", mediaPath, rendered)
				if err != nil {
					stats.SkippedImages++
					continue
				}
				mediaCache[renderKey] = newMediaRelPath
			}
			newRID := addRelationship(&ctx, newMediaRelPath)
			common.SetAttr(instance.element, "r:embed", newRID, "embed")
			stats.WatermarkedImages++
		}

		if err := xmlDoc.WriteToFile(slideFile); err != nil {
			return stats, err
		}
		if err := relDoc.WriteToFile(relsPath); err != nil {
			return stats, err
		}
	}

	if err := common.ZipDir(tempDir, outputPath); err != nil {
		return stats, err
	}
	return stats, nil
}

func collectSlideInstances(root *etree.Element, ownerXML, relsPath string) []imageInstance {
	var instances []imageInstance
	var walk func(element *etree.Element)
	walk = func(element *etree.Element) {
		if common.LocalName(element.Tag) == "blip" {
			if rID, ok := common.GetAttr(element, "r:embed", "embed"); ok && rID != "" {
				width, height, known := findShapeSize(element)
				instances = append(instances, imageInstance{
					element:     element,
					ownerXML:    ownerXML,
					relsPath:    relsPath,
					rID:         rID,
					widthEMU:    width,
					heightEMU:   height,
					sizeIsKnown: known,
				})
			}
		}
		for _, child := range element.ChildElements() {
			walk(child)
		}
	}
	walk(root)
	return instances
}

func findShapeSize(element *etree.Element) (int, int, bool) {
	bestW, bestH := int(common.EMUPerInch*6), int(common.EMUPerInch*4)
	bestArea := 0
	found := false
	for current := element; current != nil; current = current.Parent() {
		ext := findDescendantByLocalName(current, "ext")
		if ext == nil {
			continue
		}
		cx, _ := strconv.Atoi(ext.SelectAttrValue("cx", "0"))
		cy, _ := strconv.Atoi(ext.SelectAttrValue("cy", "0"))
		if cx > 0 && cy > 0 {
			area := cx * cy
			if area > bestArea {
				bestArea = area
				bestW = cx
				bestH = cy
				found = true
			}
		}
	}
	return bestW, bestH, found
}

func findDescendantByLocalName(element *etree.Element, localName string) *etree.Element {
	if common.LocalName(element.Tag) == localName {
		return element
	}
	for _, child := range element.ChildElements() {
		if found := findDescendantByLocalName(child, localName); found != nil {
			return found
		}
	}
	return nil
}

func loadRelationships(path string) (*etree.Document, *etree.Element, map[string]*etree.Element, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromFile(path); err != nil {
		return nil, nil, nil, err
	}
	root := doc.Root()
	byID := map[string]*etree.Element{}
	for _, child := range root.ChildElements() {
		if common.LocalName(child.Tag) != "Relationship" {
			continue
		}
		if id, ok := common.GetAttr(child, "Id"); ok {
			byID[id] = child
		}
	}
	return doc, root, byID, nil
}

func addRelationship(ctx *relContext, mediaRelPath string) string {
	nextID := nextRelationshipID(ctx.byID)
	rel := ctx.root.CreateElement("Relationship")
	rel.CreateAttr("Id", nextID)
	rel.CreateAttr("Type", "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image")
	rel.CreateAttr("Target", common.RelativeTarget(ctx.ownerXML, mediaRelPath))
	ctx.byID[nextID] = rel
	return nextID
}

func nextRelationshipID(existing map[string]*etree.Element) string {
	maxID := 0
	for key := range existing {
		if strings.HasPrefix(key, "rId") {
			if value, err := strconv.Atoi(strings.TrimPrefix(key, "rId")); err == nil && value > maxID {
				maxID = value
			}
		}
	}
	return "rId" + strconv.Itoa(maxID+1)
}

func writeGeneratedMedia(tempDir, mediaRoot, sourcePath string, blob []byte) (string, error) {
	sourceExt := filepath.Ext(sourcePath)
	targetDir := filepath.Join(tempDir, filepath.FromSlash(mediaRoot))
	existing, _ := filepath.Glob(filepath.Join(targetDir, "image_generated_*"+sourceExt))
	next := len(existing) + 1
	targetName := fmt.Sprintf("image_generated_%03d%s", next, sourceExt)
	fullPath := filepath.Join(targetDir, targetName)
	if err := os.WriteFile(fullPath, blob, 0o644); err != nil {
		return "", err
	}
	return path.Join(mediaRoot, targetName), nil
}

func readSlideSize(presentationPath string) (int, int) {
	doc := etree.NewDocument()
	if err := doc.ReadFromFile(presentationPath); err != nil {
		return 0, 0
	}
	root := doc.Root()
	for _, child := range root.ChildElements() {
		if common.LocalName(child.Tag) != "sldSz" {
			continue
		}
		cx, _ := strconv.Atoi(child.SelectAttrValue("cx", "0"))
		cy, _ := strconv.Atoi(child.SelectAttrValue("cy", "0"))
		return cx, cy
	}
	return 0, 0
}

func buildScaledInstanceConfig(cfg config.WatermarkConfig, mediaPath string, sourceBlob []byte, format string, displayWidthEMU, displayHeightEMU int) (config.WatermarkConfig, string, int, int, error) {
	cfgInfo, _, err := image.DecodeConfig(bytes.NewReader(sourceBlob))
	if err != nil {
		return config.WatermarkConfig{}, "", 0, 0, err
	}
	widthPx := cfgInfo.Width
	heightPx := cfgInfo.Height
	if displayWidthEMU <= 0 {
		displayWidthEMU = int(common.EMUPerInch * 6)
	}
	if displayHeightEMU <= 0 {
		displayHeightEMU = int(common.EMUPerInch * 4)
	}
	metrics := common.ComputeRenderMetrics(cfg, widthPx, heightPx, displayWidthEMU, displayHeightEMU, common.EMUPerInch)
	scaled := common.BuildScaledConfig(cfg, metrics)
	key := strings.Join([]string{
		common.HashBytes(sourceBlob),
		format,
		strconv.Itoa(displayWidthEMU),
		strconv.Itoa(displayHeightEMU),
		strconv.Itoa(scaled.FontSizePt),
		fmt.Sprintf("%.3f", scaled.GapXRatio),
		fmt.Sprintf("%.3f", scaled.GapYRatio),
		fmt.Sprintf("%.2f", scaled.Rotation),
		scaled.Color,
		scaled.Position,
		strconv.Itoa(scaled.MarginXPt),
		strconv.Itoa(scaled.MarginYPt),
	}, "|")
	return scaled, key, widthPx, heightPx, nil
}
