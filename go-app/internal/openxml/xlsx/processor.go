package xlsx

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
	rID         string
	widthEMU    int
	heightEMU   int
	sizeIsKnown bool
}

type relContext struct {
	doc      *etree.Document
	root     *etree.Element
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

	tempDir, err := common.UnzipToTemp(inputPath, "xlsx_go_")
	if err != nil {
		return common.ProcessStats{}, err
	}
	defer os.RemoveAll(tempDir)

	mediaDir := filepath.Join(tempDir, "xl", "media")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return common.ProcessStats{}, err
	}

	drawingFiles, err := filepath.Glob(filepath.Join(tempDir, "xl", "drawings", "*.xml"))
	if err != nil {
		return common.ProcessStats{}, err
	}

	stats := common.ProcessStats{}
	mediaCache := map[string]string{}
	renderCache := map[string][]byte{}

	for _, drawingFile := range drawingFiles {
		ownerXML := filepath.ToSlash(strings.TrimPrefix(drawingFile, tempDir+string(os.PathSeparator)))
		relsPath := filepath.Join(filepath.Dir(drawingFile), "_rels", filepath.Base(drawingFile)+".rels")

		xmlDoc := etree.NewDocument()
		if err := xmlDoc.ReadFromFile(drawingFile); err != nil {
			return stats, err
		}
		instances := collectDrawingInstances(xmlDoc.Root())
		if len(instances) == 0 {
			continue
		}

		if _, err := os.Stat(relsPath); err != nil {
			stats.TotalImages += len(instances)
			stats.SkippedImages += len(instances)
			continue
		}

		relDoc, relRoot, relByID, err := loadRelationships(relsPath)
		if err != nil {
			return stats, err
		}
		ctx := relContext{doc: relDoc, root: relRoot, ownerXML: ownerXML, byID: relByID}
		maxShapeArea := maxInstanceArea(instances)

		for _, instance := range instances {
			stats.TotalImages++
			if cfg.MinAreaPct > 0 && maxShapeArea > 0 && instance.sizeIsKnown {
				pct := (float64(instance.widthEMU*instance.heightEMU) / float64(maxShapeArea)) * 100.0
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
			if !ok || strings.EqualFold(rel.SelectAttrValue("TargetMode", ""), "External") {
				stats.SkippedImages++
				continue
			}

			mediaRelPath := strings.TrimPrefix(common.ResolveTarget(ownerXML, target), "/")
			mediaPath := filepath.Join(tempDir, filepath.FromSlash(mediaRelPath))
			sourceBlob, err := os.ReadFile(mediaPath)
			if err != nil {
				stats.SkippedImages++
				continue
			}

			format := common.DetectFormat(mediaPath, sourceBlob)
			cfgScaled, renderKey, widthPx, heightPx, err := buildScaledInstanceConfig(cfg, sourceBlob, format, instance.widthEMU, instance.heightEMU)
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
				newMediaRelPath, err = writeGeneratedMedia(tempDir, "xl/media", mediaPath, rendered)
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

		if err := xmlDoc.WriteToFile(drawingFile); err != nil {
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

func collectDrawingInstances(root *etree.Element) []imageInstance {
	var instances []imageInstance
	var walk func(*etree.Element)
	walk = func(element *etree.Element) {
		if common.LocalName(element.Tag) == "blip" {
			if rID, ok := common.GetAttr(element, "r:embed", "embed"); ok && rID != "" {
				width, height, known := findShapeSize(element)
				instances = append(instances, imageInstance{
					element:     element,
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
	if root != nil {
		walk(root)
	}
	return instances
}

func findShapeSize(element *etree.Element) (int, int, bool) {
	for current := element; current != nil; current = current.Parent() {
		local := common.LocalName(current.Tag)
		if local != "oneCellAnchor" && local != "twoCellAnchor" && local != "absoluteAnchor" {
			continue
		}
		ext := findDescendantByLocalName(current, "ext")
		if ext == nil {
			break
		}
		cx, _ := strconv.Atoi(ext.SelectAttrValue("cx", "0"))
		cy, _ := strconv.Atoi(ext.SelectAttrValue("cy", "0"))
		if cx > 0 && cy > 0 {
			return cx, cy, true
		}
		break
	}
	return int(common.EMUPerInch * 2), int(common.EMUPerInch * 2), false
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

func maxInstanceArea(instances []imageInstance) int {
	maxArea := 0
	for _, instance := range instances {
		if !instance.sizeIsKnown || instance.widthEMU <= 0 || instance.heightEMU <= 0 {
			continue
		}
		if area := instance.widthEMU * instance.heightEMU; area > maxArea {
			maxArea = area
		}
	}
	return maxArea
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
	if sourceExt == "" {
		sourceExt = ".png"
	}
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

func buildScaledInstanceConfig(cfg config.WatermarkConfig, sourceBlob []byte, format string, displayWidthEMU, displayHeightEMU int) (config.WatermarkConfig, string, int, int, error) {
	cfgInfo, _, err := image.DecodeConfig(bytes.NewReader(sourceBlob))
	if err != nil {
		return config.WatermarkConfig{}, "", 0, 0, err
	}
	widthPx := cfgInfo.Width
	heightPx := cfgInfo.Height
	if displayWidthEMU <= 0 {
		displayWidthEMU = int(common.EMUPerInch * 2)
	}
	if displayHeightEMU <= 0 {
		displayHeightEMU = int(common.EMUPerInch * 2)
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
