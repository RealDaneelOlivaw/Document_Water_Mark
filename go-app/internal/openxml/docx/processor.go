package docx

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/beevik/etree"

	"pptwatermark/goapp/internal/compat/office"
	"pptwatermark/goapp/internal/config"
	"pptwatermark/goapp/internal/openxml/common"
	"pptwatermark/goapp/internal/watermark"
)

var vmlDimensionPattern = regexp.MustCompile(`(width|height)\s*:\s*([0-9.]+)\s*([a-zA-Z]+)`)

const bodyImageMarkerPrefix = "PWTM_BODY_IMG_"

type Processor struct {
	renderer *watermark.Renderer
}

type imageInstance struct {
	element     *etree.Element
	ownerXML    string
	rID         string
	attrName    string
	marker      string
	widthEMU    int
	heightEMU   int
	sizeIsKnown bool
}

type relContext struct {
	doc      *etree.Document
	root     *etree.Element
	ownerXML string
	path     string
	byID     map[string]*etree.Element
}

func New() *Processor {
	return &Processor{renderer: &watermark.Renderer{}}
}

func (p *Processor) ProcessFile(inputPath, outputPath string, cfg config.WatermarkConfig) (common.ProcessStats, error) {
	if err := cfg.Validate(); err != nil {
		return common.ProcessStats{}, err
	}

	resolvedInput := inputPath
	cleanup := func() {}
	if _, err := unzipCheck(inputPath); err != nil {
		normalized, clean, normalizeErr := office.NormalizeDOCX(inputPath)
		if normalizeErr != nil {
			return common.ProcessStats{}, normalizeErr
		}
		resolvedInput = normalized
		cleanup = clean
	}
	defer cleanup()

	tempDir, err := common.UnzipToTemp(resolvedInput, "docx_go_")
	if err != nil {
		return common.ProcessStats{}, err
	}
	defer os.RemoveAll(tempDir)

	pageArea := readDocPageAreaEMU(filepath.Join(tempDir, "word", "document.xml"))
	mediaDir := filepath.Join(tempDir, "word", "media")
	if err := os.MkdirAll(mediaDir, 0o755); err != nil {
		return common.ProcessStats{}, err
	}

	excludePages := config.BuildExcludePageSet(cfg.ExcludePagesEnabled, cfg.ExcludePages)
	bodyPageMap := map[string]int{}
	if cfg.ExcludePagesEnabled {
		totalPages, pageMap, pageMapErr := buildBodyImagePageMap(tempDir)
		if pageMapErr != nil {
			return common.ProcessStats{}, pageMapErr
		}
		if err := config.ValidateExcludePagesAgainstTotal(cfg.ExcludePages, totalPages); err != nil {
			return common.ProcessStats{}, err
		}
		bodyPageMap = pageMap
	}

	ownerFiles, err := filepath.Glob(filepath.Join(tempDir, "word", "*.xml"))
	if err != nil {
		return common.ProcessStats{}, err
	}

	stats := common.ProcessStats{}
	mediaCache := map[string]string{}
	renderCache := map[string][]byte{}

	for _, ownerFile := range ownerFiles {
		if strings.Contains(ownerFile, string(filepath.Separator)+"_rels"+string(filepath.Separator)) {
			continue
		}
		ownerXML := filepath.ToSlash(strings.TrimPrefix(ownerFile, tempDir+string(os.PathSeparator)))
		xmlDoc := etree.NewDocument()
		if err := xmlDoc.ReadFromFile(ownerFile); err != nil {
			return stats, err
		}
		instances := collectInstances(xmlDoc.Root(), ownerXML)

		relsPath := filepath.Join(tempDir, "word", "_rels", filepath.Base(ownerFile)+".rels")
		if _, err := os.Stat(relsPath); err != nil {
			stats.TotalImages += len(instances)
			stats.SkippedImages += len(instances)
			continue
		}

		relDoc, relRoot, relByID, err := loadRelationships(relsPath)
		if err != nil {
			return stats, err
		}
		ctx := relContext{doc: relDoc, root: relRoot, ownerXML: ownerXML, path: relsPath, byID: relByID}

		for _, instance := range instances {
			stats.TotalImages++
			if cfg.ExcludePagesEnabled && ownerXML == "word/document.xml" {
				pageNumber, ok := bodyPageMap[instance.marker]
				if !ok || pageNumber <= 0 {
					return stats, fmt.Errorf("failed to resolve page number for DOCX body image marker %q", instance.marker)
				}
				if _, excluded := excludePages[pageNumber]; excluded {
					stats.SkippedImages++
					continue
				}
			}
			if cfg.MinAreaPct > 0 && pageArea > 0 && instance.sizeIsKnown {
				pct := (float64(instance.widthEMU*instance.heightEMU) / float64(pageArea)) * 100.0
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
				newMediaRelPath, err = writeGeneratedMedia(tempDir, "word/media", mediaPath, rendered)
				if err != nil {
					stats.SkippedImages++
					continue
				}
				mediaCache[renderKey] = newMediaRelPath
			}

			newRID := addRelationship(&ctx, newMediaRelPath)
			common.SetAttr(instance.element, instance.attrName, newRID, strings.TrimPrefix(instance.attrName, "r:"))
			stats.WatermarkedImages++
		}

		if err := xmlDoc.WriteToFile(ownerFile); err != nil {
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

func unzipCheck(inputPath string) (string, error) {
	tempDir, err := common.UnzipToTemp(inputPath, "docx_check_")
	if err != nil {
		return "", err
	}
	_ = os.RemoveAll(tempDir)
	return tempDir, nil
}

func collectInstances(root *etree.Element, ownerXML string) []imageInstance {
	var instances []imageInstance
	var walk func(*etree.Element)
	walk = func(element *etree.Element) {
		switch common.LocalName(element.Tag) {
		case "blip":
			if rID, ok := common.GetAttr(element, "r:embed", "embed"); ok && rID != "" {
				width, height, known := findDrawingSize(element)
				instances = append(instances, imageInstance{
					element:     element,
					ownerXML:    ownerXML,
					rID:         rID,
					attrName:    "r:embed",
					marker:      findDrawingMarker(element),
					widthEMU:    width,
					heightEMU:   height,
					sizeIsKnown: known,
				})
			}
		case "imagedata":
			if rID, ok := common.GetAttr(element, "r:id", "id"); ok && rID != "" {
				width, height, known := findVMLSize(element)
				instances = append(instances, imageInstance{
					element:     element,
					ownerXML:    ownerXML,
					rID:         rID,
					attrName:    "r:id",
					marker:      findVMLMarker(element),
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

func buildBodyImagePageMap(tempDir string) (int, map[string]int, error) {
	documentPath := filepath.Join(tempDir, "word", "document.xml")
	markers, err := assignBodyImageMarkers(documentPath)
	if err != nil {
		return 0, nil, err
	}

	markedDocxPath, cleanup, err := createMarkedDocx(tempDir)
	if err != nil {
		return 0, nil, err
	}
	defer cleanup()

	totalPages, pageMap, err := office.ExtractDOCXBodyImagePages(markedDocxPath, bodyImageMarkerPrefix)
	if err != nil {
		return 0, nil, err
	}
	for _, marker := range markers {
		page, ok := pageMap[marker]
		if !ok || page <= 0 {
			return 0, nil, fmt.Errorf("failed to map DOCX marker %q to a valid page number", marker)
		}
	}
	return totalPages, pageMap, nil
}

func assignBodyImageMarkers(documentPath string) ([]string, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromFile(documentPath); err != nil {
		return nil, err
	}
	root := doc.Root()
	if root == nil {
		return nil, fmt.Errorf("invalid DOCX document.xml: root is empty")
	}

	markers := make([]string, 0)
	nextIndex := 1
	var walk func(*etree.Element) error
	walk = func(element *etree.Element) error {
		switch common.LocalName(element.Tag) {
		case "blip":
			if rID, ok := common.GetAttr(element, "r:embed", "embed"); ok && rID != "" {
				marker := fmt.Sprintf("%s%d", bodyImageMarkerPrefix, nextIndex)
				if !setDrawingMarker(element, marker) {
					return fmt.Errorf("failed to assign marker for DrawingML body image #%d", nextIndex)
				}
				markers = append(markers, marker)
				nextIndex++
			}
		case "imagedata":
			if rID, ok := common.GetAttr(element, "r:id", "id"); ok && rID != "" {
				marker := fmt.Sprintf("%s%d", bodyImageMarkerPrefix, nextIndex)
				if !setVMLMarker(element, marker) {
					return fmt.Errorf("failed to assign marker for VML body image #%d", nextIndex)
				}
				markers = append(markers, marker)
				nextIndex++
			}
		}

		for _, child := range element.ChildElements() {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}

	if err := walk(root); err != nil {
		return nil, err
	}
	if err := doc.WriteToFile(documentPath); err != nil {
		return nil, err
	}
	return markers, nil
}

func createMarkedDocx(tempDir string) (string, func(), error) {
	tempFile, err := os.CreateTemp("", "ppt_watermark_marked_docx_*.docx")
	if err != nil {
		return "", func() {}, err
	}
	docxPath := tempFile.Name()
	if closeErr := tempFile.Close(); closeErr != nil {
		_ = os.Remove(docxPath)
		return "", func() {}, closeErr
	}
	cleanup := func() { _ = os.Remove(docxPath) }

	if err := common.ZipDir(tempDir, docxPath); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return docxPath, cleanup, nil
}

func setDrawingMarker(element *etree.Element, marker string) bool {
	for current := element; current != nil; current = current.Parent() {
		local := common.LocalName(current.Tag)
		if local != "inline" && local != "anchor" {
			continue
		}
		docPr := findDescendantByLocalName(current, "docPr")
		if docPr == nil {
			return false
		}
		common.SetAttr(docPr, "descr", marker)
		common.SetAttr(docPr, "title", marker)
		return true
	}
	return false
}

func setVMLMarker(element *etree.Element, marker string) bool {
	for current := element.Parent(); current != nil; current = current.Parent() {
		if common.LocalName(current.Tag) != "shape" {
			continue
		}
		common.SetAttr(current, "o:title", marker, "title")
		return true
	}
	return false
}

func findDrawingMarker(element *etree.Element) string {
	for current := element; current != nil; current = current.Parent() {
		local := common.LocalName(current.Tag)
		if local != "inline" && local != "anchor" {
			continue
		}
		docPr := findDescendantByLocalName(current, "docPr")
		if docPr == nil {
			return ""
		}
		if marker, ok := common.GetAttr(docPr, "descr", "title", "name"); ok {
			return marker
		}
		return ""
	}
	return ""
}

func findVMLMarker(element *etree.Element) string {
	for current := element.Parent(); current != nil; current = current.Parent() {
		if common.LocalName(current.Tag) != "shape" {
			continue
		}
		if marker, ok := common.GetAttr(current, "o:title", "title"); ok {
			return marker
		}
		return ""
	}
	return ""
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

func findDrawingSize(element *etree.Element) (int, int, bool) {
	bestW, bestH := int(common.EMUPerInch*6), int(common.EMUPerInch*4)
	found := false
	for current := element; current != nil; current = current.Parent() {
		local := common.LocalName(current.Tag)
		if local != "inline" && local != "anchor" {
			continue
		}
		for _, child := range current.ChildElements() {
			if common.LocalName(child.Tag) == "extent" {
				cx, _ := strconv.Atoi(child.SelectAttrValue("cx", "0"))
				cy, _ := strconv.Atoi(child.SelectAttrValue("cy", "0"))
				if cx > 0 && cy > 0 {
					bestW = cx
					bestH = cy
					found = true
				}
				return bestW, bestH, found
			}
		}
	}
	return bestW, bestH, found
}

func findVMLSize(element *etree.Element) (int, int, bool) {
	for current := element.Parent(); current != nil; current = current.Parent() {
		if common.LocalName(current.Tag) != "shape" {
			continue
		}
		style := current.SelectAttrValue("style", "")
		width := 0
		height := 0
		for _, match := range vmlDimensionPattern.FindAllStringSubmatch(style, -1) {
			emu := convertLengthToEMU(match[2], match[3])
			switch match[1] {
			case "width":
				width = emu
			case "height":
				height = emu
			}
		}
		if width > 0 && height > 0 {
			return width, height, true
		}
		break
	}
	return int(common.EMUPerInch * 6), int(common.EMUPerInch * 4), false
}

func convertLengthToEMU(rawValue, unit string) int {
	value, err := strconv.ParseFloat(rawValue, 64)
	if err != nil {
		return 0
	}
	switch strings.ToLower(unit) {
	case "in":
		return int(value * common.EMUPerInch)
	case "pt":
		return int(value * (common.EMUPerInch / 72.0))
	case "cm":
		return int(value * 360000)
	case "mm":
		return int(value * 36000)
	case "px":
		return int(value * 9525)
	default:
		return 0
	}
}

func readDocPageAreaEMU(documentPath string) int {
	doc := etree.NewDocument()
	if err := doc.ReadFromFile(documentPath); err != nil {
		return 0
	}
	root := doc.Root()
	if root == nil {
		return 0
	}
	for _, element := range root.FindElements(".//*") {
		if common.LocalName(element.Tag) != "pgSz" {
			continue
		}
		widthTwip, _ := strconv.Atoi(element.SelectAttrValue("w:w", element.SelectAttrValue("w", "0")))
		heightTwip, _ := strconv.Atoi(element.SelectAttrValue("w:h", element.SelectAttrValue("h", "0")))
		return int(float64(widthTwip) * common.EMUPerTwip * float64(heightTwip) * common.EMUPerTwip)
	}
	return 0
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
