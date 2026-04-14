package pptx

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path"
	"path/filepath"
	"sort"
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

	slideFiles, err := readSlideFilesInOrder(tempDir)
	if err != nil {
		return stats, err
	}
	if cfg.ExcludePagesEnabled {
		if err := config.ValidateExcludePagesAgainstTotal(cfg.ExcludePages, len(slideFiles)); err != nil {
			return stats, err
		}
	}
	excludePages := config.BuildExcludePageSet(cfg.ExcludePagesEnabled, cfg.ExcludePages)

	for slideIndex, slideFile := range slideFiles {
		pageNumber := slideIndex + 1
		ownerXML := filepath.ToSlash(strings.TrimPrefix(slideFile, tempDir+string(os.PathSeparator)))
		relsPath := filepath.Join(tempDir, "ppt", "slides", "_rels", filepath.Base(slideFile)+".rels")

		xmlDoc := etree.NewDocument()
		if err := xmlDoc.ReadFromFile(slideFile); err != nil {
			return stats, err
		}
		instances := collectSlideInstances(xmlDoc.Root(), ownerXML, filepath.ToSlash(strings.TrimPrefix(relsPath, tempDir+string(os.PathSeparator))))
		if _, excluded := excludePages[pageNumber]; excluded {
			stats.TotalImages += len(instances)
			stats.SkippedImages += len(instances)
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
		ctx := relContext{doc: relDoc, root: relRoot, path: relsPath, ownerXML: ownerXML, byID: relByID}

		for _, instance := range instances {
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

func readSlideFilesInOrder(tempDir string) ([]string, error) {
	presentationPath := filepath.Join(tempDir, "ppt", "presentation.xml")
	relsPath := filepath.Join(tempDir, "ppt", "_rels", "presentation.xml.rels")

	presentationDoc := etree.NewDocument()
	if err := presentationDoc.ReadFromFile(presentationPath); err != nil {
		return readSlideFilesFallback(tempDir), nil
	}
	if presentationDoc.Root() == nil {
		return readSlideFilesFallback(tempDir), nil
	}

	relsDoc := etree.NewDocument()
	if err := relsDoc.ReadFromFile(relsPath); err != nil {
		return readSlideFilesFallback(tempDir), nil
	}
	if relsDoc.Root() == nil {
		return readSlideFilesFallback(tempDir), nil
	}

	relTargets := map[string]string{}
	for _, rel := range relsDoc.Root().ChildElements() {
		if common.LocalName(rel.Tag) != "Relationship" {
			continue
		}
		relType, _ := common.GetAttr(rel, "Type")
		if !strings.HasSuffix(strings.ToLower(relType), "/slide") {
			continue
		}
		relID, okID := common.GetAttr(rel, "Id")
		target, okTarget := common.GetAttr(rel, "Target")
		if okID && okTarget {
			relTargets[relID] = target
		}
	}

	var ordered []string
	for _, sldID := range findElementsByLocalName(presentationDoc.Root(), "sldId") {
		relID, ok := common.GetAttr(sldID, "r:id")
		if !ok {
			continue
		}
		target, ok := relTargets[relID]
		if !ok {
			continue
		}
		relPath := common.ResolveTarget("ppt/presentation.xml", target)
		slidePath := filepath.Join(tempDir, filepath.FromSlash(relPath))
		if _, err := os.Stat(slidePath); err == nil {
			ordered = append(ordered, slidePath)
		}
	}
	if len(ordered) == 0 {
		return readSlideFilesFallback(tempDir), nil
	}
	return ordered, nil
}

func findElementsByLocalName(root *etree.Element, localName string) []*etree.Element {
	if root == nil {
		return nil
	}
	var found []*etree.Element
	var walk func(*etree.Element)
	walk = func(element *etree.Element) {
		if common.LocalName(element.Tag) == localName {
			found = append(found, element)
		}
		for _, child := range element.ChildElements() {
			walk(child)
		}
	}
	walk(root)
	return found
}

func readSlideFilesFallback(tempDir string) []string {
	slideFiles, _ := filepath.Glob(filepath.Join(tempDir, "ppt", "slides", "slide*.xml"))
	sort.Slice(slideFiles, func(i, j int) bool {
		return slideFileOrder(slideFiles[i]) < slideFileOrder(slideFiles[j])
	})
	return slideFiles
}

func slideFileOrder(filePath string) int {
	base := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	value, err := strconv.Atoi(strings.TrimPrefix(strings.ToLower(base), "slide"))
	if err != nil {
		return 1 << 30
	}
	return value
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

func buildScaledInstanceConfig(cfg config.WatermarkConfig, sourceBlob []byte, format string, displayWidthEMU, displayHeightEMU int) (config.WatermarkConfig, string, int, int, error) {
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
