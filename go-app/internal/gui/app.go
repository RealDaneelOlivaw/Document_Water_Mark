package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/lxn/walk"
	"github.com/lxn/walk/declarative"

	"pptwatermark/goapp/internal/config"
	"pptwatermark/goapp/internal/processor"
	"pptwatermark/goapp/internal/watermark"
)

type App struct {
	mainWindow  *walk.MainWindow
	mainSplit   *walk.Splitter
	fileList    *walk.ListBox
	logView     *walk.TextEdit
	status      *walk.Label
	progress    *walk.ProgressBar
	paramScroll *walk.ScrollView

	watermarkText       *walk.TextEdit
	fontNameEdit        *walk.LineEdit
	colorNameEdit       *walk.LineEdit
	positionEdit        *walk.LineEdit
	fontSizeEdit        *walk.LineEdit
	opacityEdit         *walk.LineEdit
	rotationEdit        *walk.LineEdit
	gapXEdit            *walk.LineEdit
	gapYEdit            *walk.LineEdit
	marginXEdit         *walk.LineEdit
	marginYEdit         *walk.LineEdit
	minAreaEdit         *walk.LineEdit
	excludeCheck        *walk.CheckBox
	excludeEdit         *walk.LineEdit
	deleteOriginalCheck *walk.CheckBox

	startButton *walk.PushButton

	files      []string
	processing bool
}

var colorChoices = []struct {
	Label string
	Value string
}{
	{Label: "\u9ed1\u8272", Value: "#000000"},
	{Label: "\u767d\u8272", Value: "#FFFFFF"},
	{Label: "\u7ea2\u8272", Value: "#FF0000"},
	{Label: "\u84dd\u8272", Value: "#1F4E79"},
	{Label: "\u7eff\u8272", Value: "#00B050"},
}

func Run() error {
	app := &App{}
	if err := app.create(); err != nil {
		return err
	}
	app.refreshFileList()
	app.mainWindow.Run()
	return nil
}

func (a *App) create() error {
	labelMin := declarative.Size{Width: 86}
	watermarkMin := declarative.Size{Width: 170, Height: 96}
	selectorTextMin := declarative.Size{Width: 170, Height: 28}
	selectorButtonMin := declarative.Size{Width: 62, Height: 28}
	fieldMin := declarative.Size{Width: 170, Height: 26}

	mw := declarative.MainWindow{
		AssignTo: &a.mainWindow,
		Title:    "\u6587\u6863\u56fe\u7247\u6c34\u5370\u5de5\u5177 V1.7 - Developed by Kejie Zhang, with assistance from Claude.",
		Size:     declarative.Size{Width: 760, Height: 610},
		MinSize:  declarative.Size{Width: 700, Height: 560},
		Visible:  false,
		Layout:   declarative.VBox{Margins: declarative.Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
		Children: []declarative.Widget{
			declarative.Composite{
				Alignment: declarative.AlignHNearVNear,
				Layout:    declarative.HBox{MarginsZero: true, Spacing: 4, Alignment: declarative.AlignHNearVNear},
				Children: []declarative.Widget{
					declarative.Label{
						Text: "\u6587\u6863\u6c34\u5370\u5de5\u5177V1.7",
						Font: declarative.Font{Family: "Microsoft YaHei UI", PointSize: 12, Bold: true},
					},
				},
			},
			declarative.HSplitter{
				AssignTo:    &a.mainSplit,
				HandleWidth: 14,
				Persistent:  false,
				Children: []declarative.Widget{
					declarative.Composite{
						StretchFactor: 2,
						MinSize:       declarative.Size{Width: 120},
						Layout:        declarative.VBox{MarginsZero: true, Spacing: 8},
						Children: []declarative.Widget{
							declarative.GroupBox{
								Title:  "\u6587\u4ef6\u9009\u62e9",
								Layout: declarative.VBox{},
								Children: []declarative.Widget{
									declarative.Label{Text: "Support PPT, Word, Picture"},
									declarative.ListBox{
										AssignTo: &a.fileList,
										Model:    a.files,
										MinSize:  declarative.Size{Height: 210},
									},
									declarative.Composite{
										Layout: declarative.Grid{Columns: 2, Spacing: 6},
										Children: []declarative.Widget{
											declarative.PushButton{Text: "\u6dfb\u52a0\u6587\u4ef6", OnClicked: a.addFiles},
											declarative.PushButton{Text: "\u6dfb\u52a0\u6587\u4ef6\u5939", OnClicked: a.addFolder},
											declarative.PushButton{Text: "\u79fb\u9664\u9009\u4e2d", OnClicked: a.removeSelected},
											declarative.PushButton{Text: "\u6e05\u7a7a\u5217\u8868", OnClicked: a.clearFiles},
										},
									},
								},
							},
							declarative.GroupBox{
								Title:  "\u5904\u7406\u7ed3\u679c",
								Layout: declarative.VBox{},
								Children: []declarative.Widget{
									declarative.TextEdit{
										AssignTo: &a.logView,
										ReadOnly: true,
										VScroll:  true,
										MinSize:  declarative.Size{Height: 145},
									},
								},
							},
						},
					},
					declarative.GroupBox{
						Title:         "\u53c2\u6570\u8bbe\u7f6e",
						StretchFactor: 5,
						MinSize:       declarative.Size{Width: 280},
						Layout:        declarative.VBox{MarginsZero: true},
						Children: []declarative.Widget{
							declarative.ScrollView{
								AssignTo:        &a.paramScroll,
								HorizontalFixed: true,
								VerticalFixed:   false,
								Layout:          declarative.VBox{Margins: declarative.Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}},
								Children: []declarative.Widget{
									declarative.Composite{
										Layout: declarative.Grid{Columns: 2, Spacing: 8},
										Children: []declarative.Widget{
											declarative.Label{Text: "\u6c34\u5370\u6587\u5b57", MinSize: labelMin},
											declarative.TextEdit{
												AssignTo: &a.watermarkText,
												Text:     config.DefaultText,
												MinSize:  watermarkMin,
												MaxSize:  declarative.Size{Height: 108},
												VScroll:  true,
											},

											declarative.Label{Text: "\u5b57\u4f53\u540d\u79f0", MinSize: labelMin},
											declarative.Composite{
												Layout: declarative.HBox{MarginsZero: true, Spacing: 6},
												Children: []declarative.Widget{
													declarative.LineEdit{AssignTo: &a.fontNameEdit, Text: config.DefaultFontName, ReadOnly: true, MinSize: selectorTextMin},
													declarative.PushButton{Text: "\u9009\u62e9", MinSize: selectorButtonMin, OnClicked: a.chooseFont},
												},
											},

											declarative.Label{Text: "\u5b57\u53f7 (pt)", MinSize: labelMin},
											declarative.LineEdit{AssignTo: &a.fontSizeEdit, Text: strconv.Itoa(config.DefaultFontSizePt), MinSize: fieldMin},

											declarative.Label{Text: "\u900f\u660e\u5ea6", MinSize: labelMin},
											declarative.LineEdit{AssignTo: &a.opacityEdit, Text: strconv.Itoa(config.DefaultOpacity), MinSize: fieldMin},

											declarative.Label{Text: "\u989c\u8272", MinSize: labelMin},
											declarative.Composite{
												Layout: declarative.HBox{MarginsZero: true, Spacing: 6},
												Children: []declarative.Widget{
													declarative.LineEdit{AssignTo: &a.colorNameEdit, Text: colorChoices[0].Label, ReadOnly: true, MinSize: selectorTextMin},
													declarative.PushButton{Text: "\u9009\u62e9", MinSize: selectorButtonMin, OnClicked: a.chooseColor},
												},
											},

											declarative.Label{Text: "\u65cb\u8f6c\u89d2\u5ea6", MinSize: labelMin},
											declarative.LineEdit{AssignTo: &a.rotationEdit, Text: fmt.Sprintf("%.0f", config.DefaultRotation), MinSize: fieldMin},

											declarative.Label{Text: "\u4f4d\u7f6e\u6a21\u5f0f", MinSize: labelMin},
											declarative.Composite{
												Layout: declarative.HBox{MarginsZero: true, Spacing: 6},
												Children: []declarative.Widget{
													declarative.LineEdit{AssignTo: &a.positionEdit, Text: config.DefaultPosition, ReadOnly: true, MinSize: selectorTextMin},
													declarative.PushButton{Text: "\u9009\u62e9", MinSize: selectorButtonMin, OnClicked: a.choosePosition},
												},
											},

											declarative.Label{Text: "\u6a2a\u5411\u95f4\u8ddd\u500d\u6570", MinSize: labelMin},
											declarative.LineEdit{AssignTo: &a.gapXEdit, Text: strconv.FormatFloat(config.DefaultGapXRatio, 'f', -1, 64), MinSize: fieldMin},

											declarative.Label{Text: "\u7eb5\u5411\u95f4\u8ddd\u500d\u6570", MinSize: labelMin},
											declarative.LineEdit{AssignTo: &a.gapYEdit, Text: strconv.FormatFloat(config.DefaultGapYRatio, 'f', -1, 64), MinSize: fieldMin},

											declarative.Label{Text: "\u8fb9\u8ddd X (pt)", MinSize: labelMin},
											declarative.LineEdit{AssignTo: &a.marginXEdit, Text: strconv.Itoa(config.DefaultMarginXPt), MinSize: fieldMin},

											declarative.Label{Text: "\u8fb9\u8ddd Y (pt)", MinSize: labelMin},
											declarative.LineEdit{AssignTo: &a.marginYEdit, Text: strconv.Itoa(config.DefaultMarginYPt), MinSize: fieldMin},

											declarative.Label{Text: "\u5c0f\u56fe\u8fc7\u6ee4(%)", MinSize: labelMin},
											declarative.LineEdit{AssignTo: &a.minAreaEdit, Text: strconv.Itoa(config.DefaultMinAreaPct), MinSize: fieldMin},

											declarative.Label{Text: "\u6392\u9664\u9875\u7801", MinSize: labelMin},
											declarative.Composite{
												Layout: declarative.HBox{MarginsZero: true, Spacing: 6},
												Children: []declarative.Widget{
													declarative.CheckBox{
														AssignTo:         &a.excludeCheck,
														Text:             "\u542f\u7528",
														Checked:          false,
														OnCheckedChanged: a.syncExcludePagesControl,
													},
													declarative.LineEdit{
														AssignTo: &a.excludeEdit,
														Text:     "1",
														MinSize:  fieldMin,
													},
												},
											},

											declarative.Label{Text: "\u5220\u9664\u539f\u6587\u4ef6", MinSize: labelMin},
											declarative.CheckBox{
												AssignTo: &a.deleteOriginalCheck,
												Text:     "\u5220\u9664\u539f\u6587\u4ef6\u53ea\u4fdd\u7559\u5e26\u6c34\u5370\u6587\u4ef6",
												Checked:  false,
											},
										},
									},
								},
							},
						},
					},
				},
			},
			declarative.Composite{
				Layout: declarative.Grid{Columns: 3},
				Children: []declarative.Widget{
					declarative.Label{AssignTo: &a.status, Text: "\u8bf7\u9009\u62e9\u8981\u5904\u7406\u7684\u6587\u6863\u3002"},
					declarative.ProgressBar{AssignTo: &a.progress, MaxValue: 100},
					declarative.PushButton{AssignTo: &a.startButton, Text: "\u5f00\u59cb\u5904\u7406", OnClicked: a.startProcessing},
				},
			},
		},
	}

	if err := mw.Create(); err != nil {
		return err
	}
	a.applyWindowIcon()
	a.finalizeLayout()
	a.syncExcludePagesControl()
	a.centerMainWindowIfNeeded()
	a.mainWindow.Show()
	return nil
}

func (a *App) centerMainWindowIfNeeded() {
	if a.mainWindow == nil {
		return
	}
	app := walk.App()
	if app != nil && app.Settings() != nil {
		state, err := a.mainWindow.ReadState()
		if err == nil && state != "" {
			return
		}
	}
	centerMainWindowOnPrimaryWorkArea(a.mainWindow)
}

func (a *App) applyWindowIcon() {
	if a.mainWindow == nil {
		return
	}

	if exePath, err := os.Executable(); err == nil {
		if icon, err := walk.NewIconExtractedFromFileWithSize(exePath, 0, 256); err == nil {
			_ = a.mainWindow.SetIcon(icon)
			return
		}
	}

	if icon, err := walk.NewIconFromResourceId(7); err == nil {
		_ = a.mainWindow.SetIcon(icon)
	}
}

func (a *App) finalizeLayout() {
	if a.mainSplit == nil {
		return
	}
	a.mainSplit.SetPersistent(false)
	_ = a.mainSplit.SetHandleWidth(18)

	children := a.mainSplit.Children()
	for index := 0; index < children.Len(); index += 2 {
		_ = a.mainSplit.SetFixed(children.At(index), false)
	}

	if children.Len() >= 3 {
		left := children.At(0)
		right := children.At(2)
		if setter, ok := a.mainSplit.Layout().(interface {
			SetStretchFactor(widget walk.Widget, factor int) error
		}); ok {
			_ = setter.SetStretchFactor(left, 38)
			_ = setter.SetStretchFactor(right, 62)
		}
	}
	a.mainSplit.RequestLayout()
}

func (a *App) chooseFont() {
	a.pickTextOption("\u9009\u62e9\u5b57\u4f53", watermark.GUIFontChoices, a.fontNameEdit)
}

func (a *App) chooseColor() {
	a.pickTextOption("\u9009\u62e9\u989c\u8272", colorChoiceLabels(), a.colorNameEdit)
}

func (a *App) choosePosition() {
	a.pickTextOption("\u9009\u62e9\u4f4d\u7f6e\u6a21\u5f0f", config.AllowedPositions, a.positionEdit)
}

func (a *App) pickTextOption(title string, options []string, target *walk.LineEdit) {
	if a.processing || target == nil {
		return
	}
	selected, ok, err := showOptionDialog(a.mainWindow, title, options, target.Text())
	if err != nil {
		walk.MsgBox(a.mainWindow, "\u9519\u8bef", err.Error(), walk.MsgBoxIconError)
		return
	}
	if ok {
		target.SetText(selected)
	}
}

func showOptionDialog(owner walk.Form, title string, options []string, current string) (string, bool, error) {
	if len(options) == 0 {
		return "", false, nil
	}

	var dlg *walk.Dialog
	var listBox *walk.ListBox
	var okButton *walk.PushButton
	var cancelButton *walk.PushButton
	selected := current
	currentIndex := indexOf(options, current)
	if currentIndex < 0 {
		currentIndex = 0
		selected = options[0]
	}

	dialog := declarative.Dialog{
		AssignTo:      &dlg,
		Title:         title,
		MinSize:       declarative.Size{Width: 280, Height: 320},
		Size:          declarative.Size{Width: 300, Height: 340},
		DefaultButton: &okButton,
		CancelButton:  &cancelButton,
		Layout:        declarative.VBox{Margins: declarative.Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 8},
		Children: []declarative.Widget{
			declarative.ListBox{
				AssignTo:     &listBox,
				Model:        options,
				CurrentIndex: currentIndex,
				MinSize:      declarative.Size{Width: 240, Height: 220},
				OnItemActivated: func() {
					if index := listBox.CurrentIndex(); index >= 0 {
						selected = options[index]
						dlg.Accept()
					}
				},
			},
			declarative.Composite{
				Layout: declarative.HBox{MarginsZero: true, Spacing: 8},
				Children: []declarative.Widget{
					declarative.HSpacer{},
					declarative.PushButton{
						AssignTo: &okButton,
						Text:     "\u786e\u5b9a",
						OnClicked: func() {
							if index := listBox.CurrentIndex(); index >= 0 {
								selected = options[index]
								dlg.Accept()
							}
						},
					},
					declarative.PushButton{
						AssignTo: &cancelButton,
						Text:     "\u53d6\u6d88",
						OnClicked: func() {
							dlg.Cancel()
						},
					},
				},
			},
		},
	}

	result, err := dialog.Run(owner)
	if err != nil {
		return "", false, err
	}
	if result != walk.DlgCmdOK {
		return current, false, nil
	}
	return selected, true, nil
}

func colorChoiceLabels() []string {
	labels := make([]string, 0, len(colorChoices))
	for _, choice := range colorChoices {
		labels = append(labels, choice.Label)
	}
	return labels
}

func indexOf(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}

func (a *App) addFiles() {
	if a.processing {
		return
	}
	dialog := new(walk.FileDialog)
	dialog.Title = "\u9009\u62e9\u8981\u5904\u7406\u7684\u6587\u6863"
	dialog.Filter = "\u652f\u6301\u7684\u6587\u6863 (*.pptx;*.docx;*.jpg;*.jpeg;*.png;*.bmp;*.gif;*.tif;*.tiff)|*.pptx;*.docx;*.jpg;*.jpeg;*.png;*.bmp;*.gif;*.tif;*.tiff"
	if ok, err := dialog.ShowOpenMultiple(a.mainWindow); err != nil {
		walk.MsgBox(a.mainWindow, "\u9519\u8bef", err.Error(), walk.MsgBoxIconError)
		return
	} else if !ok {
		return
	}
	a.appendPaths(dialog.FilePaths)
}

func (a *App) addFolder() {
	if a.processing {
		return
	}
	var allFiles []string
	initialDir := ""

	for {
		root, ok, err := showModernFolderDialog(a.mainWindow, "\u9009\u62e9\u6587\u4ef6\u5939", initialDir)
		if err != nil {
			// Fallback for systems where IFileDialog is unavailable.
			legacyDialog := new(walk.FileDialog)
			legacyDialog.Title = "\u9009\u62e9\u6587\u4ef6\u5939"
			legacyDialog.InitialDirPath = initialDir
			ok, legacyErr := legacyDialog.ShowBrowseFolder(a.mainWindow)
			if legacyErr != nil {
				walk.MsgBox(a.mainWindow, "\u9519\u8bef", err.Error(), walk.MsgBoxIconError)
				return
			}
			if !ok {
				break
			}
			root = legacyDialog.FilePath
		}
		if !ok {
			break
		}

		root = filepath.Clean(root)
		initialDir = root
		files, err := collectSupportedFilesRecursively(root)
		if err != nil {
			walk.MsgBox(a.mainWindow, "\u9519\u8bef", err.Error(), walk.MsgBoxIconError)
			return
		}
		if len(files) == 0 {
			a.appendLog(fmt.Sprintf("%s \u4e0b\u672a\u627e\u5230\u53ef\u5904\u7406\u6587\u4ef6\u3002", root))
		} else {
			a.appendLog(fmt.Sprintf("%s \u4e0b\u627e\u5230 %d \u4e2a\u53ef\u5904\u7406\u6587\u4ef6\u3002", root, len(files)))
		}
		allFiles = append(allFiles, files...)

		choice := walk.MsgBox(
			a.mainWindow,
			"\u7ee7\u7eed\u9009\u62e9",
			"\u662f\u5426\u7ee7\u7eed\u6dfb\u52a0\u5176\u4ed6\u6587\u4ef6\u5939\uff1f",
			walk.MsgBoxYesNo|walk.MsgBoxIconQuestion,
		)
		if choice != walk.DlgCmdYes {
			break
		}
	}

	if len(allFiles) > 0 {
		a.appendPaths(allFiles)
	}
}

func collectSupportedFilesRecursively(root string) ([]string, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("selected path is not a folder: %s", root)
	}

	files := make([]string, 0, 128)
	walkErr := filepath.Walk(root, func(current string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip inaccessible paths so the overall scan can continue.
			return nil
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if isSupportedInputFile(current) {
			files = append(files, current)
		}
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Strings(files)
	return files, nil
}

func isSupportedInputFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".pptx", ".docx", ".jpg", ".jpeg", ".png", ".bmp", ".gif", ".tif", ".tiff":
		return true
	default:
		return false
	}
}

func (a *App) appendPaths(paths []string) {
	seen := map[string]struct{}{}
	for _, existing := range a.files {
		seen[strings.ToLower(existing)] = struct{}{}
	}
	added := 0
	for _, current := range paths {
		normalized := filepath.Clean(current)
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		a.files = append(a.files, normalized)
		added++
	}
	a.refreshFileList()
	if added > 0 {
		a.appendLog(fmt.Sprintf("\u5df2\u6dfb\u52a0 %d \u4e2a\u6587\u4ef6\u3002", added))
	}
}

func (a *App) removeSelected() {
	if a.processing || a.fileList == nil {
		return
	}
	index := a.fileList.CurrentIndex()
	if index < 0 || index >= len(a.files) {
		return
	}
	a.files = append(a.files[:index], a.files[index+1:]...)
	a.refreshFileList()
}

func (a *App) clearFiles() {
	if a.processing {
		return
	}
	a.files = nil
	a.refreshFileList()
}

func (a *App) refreshFileList() {
	if a.fileList != nil {
		a.fileList.SetModel(a.files)
	}
	if a.status != nil {
		a.status.SetText(fmt.Sprintf("\u5df2\u9009\u62e9 %d \u4e2a\u6587\u4ef6\u3002", len(a.files)))
	}
}

func (a *App) appendLog(message string) {
	if a.logView == nil {
		return
	}
	current := a.logView.Text()
	if current != "" {
		current += "\r\n"
	}
	current += message
	a.logView.SetText(current)
}

func (a *App) startProcessing() {
	if a.processing {
		return
	}
	if len(a.files) == 0 {
		walk.MsgBox(a.mainWindow, "\u63d0\u793a", "\u8bf7\u5148\u6dfb\u52a0\u8981\u5904\u7406\u7684\u6587\u4ef6\u3002", walk.MsgBoxIconInformation)
		return
	}
	cfg, err := a.buildConfig()
	if err != nil {
		walk.MsgBox(a.mainWindow, "\u53c2\u6570\u9519\u8bef", err.Error(), walk.MsgBoxIconWarning)
		return
	}

	a.processing = true
	a.startButton.SetEnabled(false)
	a.progress.SetValue(0)
	files := append([]string(nil), a.files...)

	go func() {
		successCount := 0
		for idx, current := range files {
			progress := int(float64(idx) / float64(len(files)) * 100)
			a.mainWindow.Synchronize(func() {
				a.progress.SetValue(progress)
				a.status.SetText(fmt.Sprintf("\u6b63\u5728\u5904\u7406 %d/%d: %s", idx+1, len(files), filepath.Base(current)))
			})
			result := processor.ProcessFile(current, cfg)
			if result.Success {
				successCount++
			}
			line := formatResultLine(result)
			a.mainWindow.Synchronize(func() {
				a.appendLog(line)
			})
		}
		a.mainWindow.Synchronize(func() {
			a.progress.SetValue(100)
			a.status.SetText(fmt.Sprintf("\u5904\u7406\u5b8c\u6210\u3002\u6210\u529f %d \u4e2a\uff0c\u5931\u8d25 %d \u4e2a\u3002", successCount, len(files)-successCount))
			a.startButton.SetEnabled(true)
			a.processing = false
		})
	}()
}

func (a *App) buildConfig() (config.WatermarkConfig, error) {
	cfg := config.Default()
	cfg.Text = strings.TrimSpace(a.watermarkText.Text())
	cfg.FontName = strings.TrimSpace(a.fontNameEdit.Text())
	cfg.Color = resolveColor(a.colorNameEdit.Text())

	var err error
	if cfg.FontSizePt, err = strconv.Atoi(strings.TrimSpace(a.fontSizeEdit.Text())); err != nil {
		return cfg, fmt.Errorf("\u5b57\u53f7\u5fc5\u987b\u662f\u6574\u6570")
	}
	if cfg.Opacity, err = strconv.Atoi(strings.TrimSpace(a.opacityEdit.Text())); err != nil {
		return cfg, fmt.Errorf("\u900f\u660e\u5ea6\u5fc5\u987b\u662f\u6574\u6570")
	}
	if cfg.Rotation, err = strconv.ParseFloat(strings.TrimSpace(a.rotationEdit.Text()), 64); err != nil {
		return cfg, fmt.Errorf("\u65cb\u8f6c\u89d2\u5ea6\u5fc5\u987b\u662f\u6570\u5b57")
	}
	if cfg.GapXRatio, err = strconv.ParseFloat(strings.TrimSpace(a.gapXEdit.Text()), 64); err != nil {
		return cfg, fmt.Errorf("\u6a2a\u5411\u95f4\u8ddd\u500d\u6570\u5fc5\u987b\u662f\u6570\u5b57")
	}
	if cfg.GapYRatio, err = strconv.ParseFloat(strings.TrimSpace(a.gapYEdit.Text()), 64); err != nil {
		return cfg, fmt.Errorf("\u7eb5\u5411\u95f4\u8ddd\u500d\u6570\u5fc5\u987b\u662f\u6570\u5b57")
	}
	if cfg.MarginXPt, err = strconv.Atoi(strings.TrimSpace(a.marginXEdit.Text())); err != nil {
		return cfg, fmt.Errorf("\u8fb9\u8ddd X \u5fc5\u987b\u662f\u6574\u6570")
	}
	if cfg.MarginYPt, err = strconv.Atoi(strings.TrimSpace(a.marginYEdit.Text())); err != nil {
		return cfg, fmt.Errorf("\u8fb9\u8ddd Y \u5fc5\u987b\u662f\u6574\u6570")
	}
	if cfg.MinAreaPct, err = strconv.Atoi(strings.TrimSpace(a.minAreaEdit.Text())); err != nil {
		return cfg, fmt.Errorf("\u5c0f\u56fe\u8fc7\u6ee4\u5fc5\u987b\u662f\u6574\u6570")
	}
	cfg.ExcludePagesEnabled = a.excludeCheck != nil && a.excludeCheck.Checked()
	if cfg.ExcludePagesEnabled {
		pages, parseErr := config.ParseExcludePages(strings.TrimSpace(a.excludeEdit.Text()))
		if parseErr != nil {
			return cfg, fmt.Errorf("\u6392\u9664\u9875\u7801\u683c\u5f0f\u9519\u8bef: %w", parseErr)
		}
		cfg.ExcludePages = pages
	}
	cfg.DeleteOriginalEnabled = a.deleteOriginalCheck != nil && a.deleteOriginalCheck.Checked()
	cfg.Position = strings.TrimSpace(a.positionEdit.Text())
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (a *App) syncExcludePagesControl() {
	if a.excludeEdit == nil {
		return
	}
	enabled := a.excludeCheck != nil && a.excludeCheck.Checked()
	a.excludeEdit.SetEnabled(enabled)
}

func resolveColor(label string) string {
	for _, choice := range colorChoices {
		if choice.Label == label {
			return choice.Value
		}
	}
	return config.DefaultColor
}

func formatResultLine(result processor.Result) string {
	if !result.Success {
		return fmt.Sprintf("\u5931\u8d25: %s -> %v", filepath.Base(result.InputPath), result.Err)
	}
	line := fmt.Sprintf(
		"\u6210\u529f: %s -> %s (\u56fe\u7247 %d, \u6c34\u5370 %d, \u8df3\u8fc7 %d)",
		filepath.Base(result.InputPath),
		filepath.Base(result.OutputPath),
		result.Stats.TotalImages,
		result.Stats.WatermarkedImages,
		result.Stats.SkippedImages,
	)
	if strings.TrimSpace(result.Warning) != "" {
		line += fmt.Sprintf(" [\u8b66\u544a: %s]", result.Warning)
	}
	return line
}
