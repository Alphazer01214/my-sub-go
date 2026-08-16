package gui

import (
	"context"
	"fmt"
	"image/color"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"my-sub-go/common"
	"my-sub-go/common/logx"
	"my-sub-go/typedef"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// maxLogLines 每个日志模块在 GUI 中保留的最大行数。
const maxLogLines = 1000

// workflowMode 字幕工作流页的三种模式。
type workflowMode string

const (
	modeFull          workflowMode = "transcribe_translate" // 转录+翻译
	modeTranscribeOnly workflowMode = "transcribe_only"      // 仅转录
	modeTranslateOnly  workflowMode = "translate_only"       // 仅翻译
)

// WorkflowUI 字幕工作流页：选择模式后一键处理——转录+翻译 / 仅转录 / 仅翻译。
// 页面上的参数都是任务级副本，不修改全局配置（全局默认值在「设置」页）。
type WorkflowUI struct {
	cm *typedef.ComponentManager
	W  *fyne.Window

	MediaPath    string
	SubtitlePath string
	SubtitleDir  string
	Lang         typedef.Lang
	Mode         workflowMode

	TaskTranscribe typedef.TranscribeOptions
	TaskTranslate  typedef.TranslateOptions

	subtitle *typedef.Subtitle

	btnFull          *widget.Button
	btnTranscribe    *widget.Button
	btnTranslateOnly *widget.Button
	btnCancel        *widget.Button
	btnExport        *widget.Button
	btnExportOrig    *widget.Button
	btnOpenDir       *widget.Button
	progress         *widget.ProgressBar
	status           *widget.Label
	cancelFn         context.CancelFunc
	running          bool

	modeRadio *widget.RadioGroup
	inputForm *fyne.Container

	table       *widget.Table
	editingCell *editCell // 当前正在内联编辑的单元格（用于点击别处时提交）

	logTabs  *container.AppTabs
	logLists map[logx.Module]*widget.List
	logData  map[logx.Module][]logx.Entry
}

func NewWorkflowUI(cm *typedef.ComponentManager, w *fyne.Window) *WorkflowUI {
	st := typedef.LoadState(typedef.StatePath)
	ui := &WorkflowUI{
		cm:        cm,
		W:         w,
		Lang:      typedef.LangAuto,
		Mode:      modeFull,
		logLists:  make(map[logx.Module]*widget.List),
		logData:   make(map[logx.Module][]logx.Entry),
	}
	if st.Workflow.Lang != "" {
		ui.Lang = st.Workflow.Lang
	}
	ui.MediaPath = st.Workflow.MediaPath
	ui.SubtitlePath = st.Workflow.SubtitlePath
	ui.SubtitleDir = st.Workflow.SubtitleDir
	switch workflowMode(st.Workflow.Mode) {
	case modeFull, modeTranscribeOnly, modeTranslateOnly:
		ui.Mode = workflowMode(st.Workflow.Mode)
	}
	// 任务级参数：以全局配置为初值，页面上的修改只影响本次任务。
	cfg := cm.Cfg
	ui.TaskTranscribe = typedef.TranscribeOptions{
		ChunkDurationSec: cfg.Whisper.ChunkDurationSec,
		ChunkOverlapSec:  cfg.Whisper.ChunkOverlapSec,
	}
	ui.TaskTranslate = typedef.TranslateOptions{
		SrcLang:        cfg.LLMAPI.SrcLang,
		TgtLang:        cfg.LLMAPI.TgtLang,
		Provider:       cfg.LLMAPI.Provider,
		ModelName:      cfg.LLMAPI.ModelName,
		PromptTemplate: cfg.LLMAPI.PromptTemplate,
		ProcessWindow:  cfg.LLMAPI.ProcessWindow,
		RefWindow:      cfg.LLMAPI.RefWindow,
	}
	if ui.TaskTranslate.TgtLang == "" || ui.TaskTranslate.TgtLang == typedef.LangAuto {
		ui.TaskTranslate.TgtLang = typedef.LangZh
	}
	for _, m := range logx.Modules {
		ui.logData[m] = []logx.Entry{}
		go ui.consumeLogs(m)
	}
	return ui
}

// saveState 记住本次选择，下次启动自动填入。
func (ui *WorkflowUI) saveState() {
	st := typedef.LoadState(typedef.StatePath)
	st.Workflow.MediaPath = ui.MediaPath
	st.Workflow.SubtitlePath = ui.SubtitlePath
	st.Workflow.SubtitleDir = ui.SubtitleDir
	st.Workflow.Lang = ui.Lang
	st.Workflow.Mode = string(ui.Mode)
	if err := st.Save(typedef.StatePath); err != nil {
		logx.Error(logx.ModuleSystem, "保存界面状态失败: %v", err)
	}
}

// consumeLogs 订阅模块日志并推到 UI 线程渲染。
func (ui *WorkflowUI) consumeLogs(m logx.Module) {
	ch := logx.Listen(m)
	defer logx.Unlisten(m, ch)
	for e := range ch {
		e := e
		fyne.Do(func() {
			d := ui.logData[m]
			if len(d) >= maxLogLines {
				d = d[len(d)-maxLogLines+1:]
			}
			ui.logData[m] = append(d, e)
			if l := ui.logLists[m]; l != nil {
				l.Refresh()
				l.ScrollToBottom()
			}
		})
	}
}

func levelColor(l logx.Level) color.Color {
	switch l {
	case logx.LevelError:
		return color.NRGBA{R: 0xF0, G: 0x60, B: 0x60, A: 0xFF}
	case logx.LevelWarn:
		return color.NRGBA{R: 0xF0, G: 0xA0, B: 0x30, A: 0xFF}
	default:
		return theme.ForegroundColor()
	}
}

func (ui *WorkflowUI) renderField(field *reflect.StructField, value *reflect.Value) fyne.CanvasObject {
	label := field.Tag.Get("label")
	fieldType := field.Tag.Get("type")
	obj := getRealTimeObj(ui.W, fieldType, value, field.Tag.Get("support_ext"), ui.saveState)
	return container.NewVBox(widget.NewLabel(label), obj)
}

// taskIntField 任务级整数输入（每次改动只写 ui 上的任务副本）。
func taskIntField(label string, get func() int, set func(int)) fyne.CanvasObject {
	e := widget.NewEntry()
	e.SetText(strconv.Itoa(get()))
	e.OnChanged = func(s string) {
		if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
			set(v)
		}
	}
	return container.NewVBox(widget.NewLabel(label), e)
}

// taskLangField 任务级语言下拉。
func taskLangField(label string, withAuto bool, get func() typedef.Lang, set func(typedef.Lang)) fyne.CanvasObject {
	options := typedef.LangOptions
	if !withAuto {
		options = typedef.TgtLangOptions
	}
	sel := widget.NewSelect(options, func(s string) {
		set(typedef.Lang(s))
	})
	sel.SetSelected(string(get()))
	return container.NewVBox(widget.NewLabel(label), sel)
}

// taskStringField 任务级文本输入。
func taskStringField(label string, get func() string, set func(string)) fyne.CanvasObject {
	e := widget.NewEntry()
	e.SetText(get())
	e.OnChanged = set
	return container.NewVBox(widget.NewLabel(label), e)
}

// taskTextArea 任务级多行文本（补充提示词）。
func taskTextArea(label string, get func() string, set func(string)) fyne.CanvasObject {
	e := widget.NewEntry()
	e.MultiLine = true
	e.SetText(get())
	e.OnChanged = set
	return container.NewVBox(widget.NewLabel(label), e)
}

// renderAdvancedOptions 工作流页内嵌的任务级选项（折叠面板）。
// 直接操作 ui.Task* 副本，不写 cm.Cfg，下次任务仍以全局配置为初值。
func (ui *WorkflowUI) renderAdvancedOptions() fyne.CanvasObject {
	var items []fyne.CanvasObject
	if ui.Mode == modeFull || ui.Mode == modeTranscribeOnly {
		items = append(items,
			taskIntField("分块时长(秒)", func() int { return ui.TaskTranscribe.ChunkDurationSec },
				func(v int) { ui.TaskTranscribe.ChunkDurationSec = v }),
			taskIntField("分块重叠(秒)", func() int { return ui.TaskTranscribe.ChunkOverlapSec },
				func(v int) { ui.TaskTranscribe.ChunkOverlapSec = v }),
		)
	}
	providers := []string{
		string(typedef.LLMProviderOpenAI), string(typedef.LLMProviderDeepSeek),
		string(typedef.LLMProviderQwen), string(typedef.LLMProviderClaude), string(typedef.LLMProviderOllama),
	}
	providerSel := widget.NewSelect(providers, func(s string) {
		ui.TaskTranslate.Provider = typedef.LLMProvider(s)
	})
	providerSel.SetSelected(string(ui.TaskTranslate.Provider))
	items = append(items,
		container.NewVBox(widget.NewLabel("翻译提供商"), providerSel),
		taskLangField("翻译源语言", true,
			func() typedef.Lang { return ui.TaskTranslate.SrcLang },
			func(l typedef.Lang) { ui.TaskTranslate.SrcLang = l }),
		taskStringField("翻译模型", func() string { return ui.TaskTranslate.ModelName },
			func(s string) { ui.TaskTranslate.ModelName = s }),
		taskTextArea("补充提示词（用户指令）", func() string { return ui.TaskTranslate.PromptTemplate },
			func(s string) { ui.TaskTranslate.PromptTemplate = s }),
		taskIntField("处理窗口(句)", func() int { return ui.TaskTranslate.ProcessWindow },
			func(v int) { ui.TaskTranslate.ProcessWindow = v }),
		taskIntField("参考窗口(句)", func() int { return ui.TaskTranslate.RefWindow },
			func(v int) { ui.TaskTranslate.RefWindow = v }),
	)
	note := widget.NewLabel("以上选项仅对本次任务生效；全局默认值与 API 连接信息在「设置」页配置。")
	note.TextStyle = fyne.TextStyle{Italic: true}
	form := container.NewGridWithColumns(2, items...)
	return widget.NewAccordion(widget.NewAccordionItem("高级选项（任务级）",
		container.NewVBox(form, note)))
}

// refreshInputs 按当前模式重建输入区（模式切换、拖拽导入后调用）。
func (ui *WorkflowUI) refreshInputs() {
	if ui.inputForm == nil {
		return
	}
	transcribe := ui.Mode == modeFull || ui.Mode == modeTranscribeOnly
	var fields []fyne.CanvasObject
	if transcribe {
		mediaField := reflect.StructField{
			Name: "MediaPath",
			Type: reflect.TypeOf(""),
			Tag:  `json:"media_path" label:"媒体文件（音、视频）" type:"file" support_ext:".mp4,.mkv,.avi,.mov,.wmv,.flv,.webm,.mp3,.wav,.m4a,.aac,.ogg,.flac"`,
		}
		mediaValue := reflect.ValueOf(&ui.MediaPath).Elem()
		subField := reflect.StructField{
			Name: "SubtitleDir",
			Type: reflect.TypeOf(""),
			Tag:  `json:"subtitle_dir" label:"字幕保存目录" type:"dir"`,
		}
		subValue := reflect.ValueOf(&ui.SubtitleDir).Elem()
		langField := reflect.StructField{
			Name: "Lang",
			Type: reflect.TypeOf(typedef.Lang("")),
			Tag:  `json:"lang" label:"音频语言" type:"lang"`,
		}
		langValue := reflect.ValueOf(&ui.Lang).Elem()
		fields = append(fields,
			ui.renderField(&mediaField, &mediaValue),
			ui.renderField(&subField, &subValue),
			ui.renderField(&langField, &langValue),
		)
	} else {
		srtField := reflect.StructField{
			Name: "SubtitlePath",
			Type: reflect.TypeOf(""),
			Tag:  `json:"subtitle_path" label:"字幕文件（.srt）" type:"file" support_ext:".srt"`,
		}
		srtValue := reflect.ValueOf(&ui.SubtitlePath).Elem()
		fields = append(fields, ui.renderField(&srtField, &srtValue))
	}
	fields = append(fields,
		taskLangField("目标语言", false,
			func() typedef.Lang { return ui.TaskTranslate.TgtLang },
			func(l typedef.Lang) { ui.TaskTranslate.TgtLang = l }),
	)
	ui.inputForm.Objects = []fyne.CanvasObject{
		container.NewGridWithColumns(2, fields...),
		ui.renderAdvancedOptions(),
	}
	ui.inputForm.Refresh()
}

// syncRadio 让模式单选按钮与 ui.Mode 一致（模式被拖拽等外部途径修改后调用）。
func (ui *WorkflowUI) syncRadio() {
	if ui.modeRadio == nil {
		return
	}
	switch ui.Mode {
	case modeTranscribeOnly:
		ui.modeRadio.SetSelected("仅转录（只出原文字幕）")
	case modeTranslateOnly:
		ui.modeRadio.SetSelected("仅翻译（已有字幕 → 译文）")
	default:
		ui.modeRadio.SetSelected("转录+翻译（全自动）")
	}
}

func (ui *WorkflowUI) renderModeRadio() fyne.CanvasObject {
	ui.modeRadio = widget.NewRadioGroup(
		[]string{"转录+翻译（全自动）", "仅转录（只出原文字幕）", "仅翻译（已有字幕 → 译文）"},
		func(s string) {
			switch {
			case strings.HasPrefix(s, "转录+翻译"):
				ui.Mode = modeFull
			case strings.HasPrefix(s, "仅转录"):
				ui.Mode = modeTranscribeOnly
			default:
				ui.Mode = modeTranslateOnly
			}
			ui.refreshInputs()
			ui.refreshButtons()
			ui.saveState()
		},
	)
	ui.modeRadio.Horizontal = true
	ui.syncRadio()
	return container.NewVBox(widget.NewLabel("模式"), ui.modeRadio)
}

func (ui *WorkflowUI) renderButtons() fyne.CanvasObject {
	ui.btnFull = widget.NewButton("转录+翻译", ui.onFullPipeline)
	ui.btnFull.Importance = widget.HighImportance
	ui.btnTranscribe = widget.NewButton("仅转录", ui.onTranscribe)
	ui.btnTranslateOnly = widget.NewButton("仅翻译", ui.onTranslateOnly)
	ui.btnCancel = widget.NewButton("取消", func() {
		if ui.cancelFn != nil {
			ui.cancelFn()
		}
	})
	ui.btnCancel.Disable()
	ui.btnExport = widget.NewButton("导出双语 SRT", func() { ui.onExport(true) })
	ui.btnExport.Disable()
	ui.btnExportOrig = widget.NewButton("导出原文 SRT", func() { ui.onExport(false) })
	ui.btnExportOrig.Disable()
	ui.btnOpenDir = widget.NewButton("打开输出目录", ui.onOpenDir)
	return container.NewHBox(
		ui.btnFull, ui.btnTranscribe, ui.btnTranslateOnly, ui.btnCancel,
		ui.btnExportOrig, ui.btnExport, ui.btnOpenDir,
	)
}

// refreshButtons 按模式启用/禁用三个执行按钮（任务运行中由 setRunning 统一禁用）。
// 按钮未创建时直接返回（radio 的 SetSelected 会在 RenderWorkflowWindow 构建按钮之前同步触发回调）。
func (ui *WorkflowUI) refreshButtons() {
	if ui.running || ui.btnFull == nil {
		return
	}
	ui.btnFull.Disable()
	ui.btnTranscribe.Disable()
	ui.btnTranslateOnly.Disable()
	switch ui.Mode {
	case modeFull:
		ui.btnFull.Enable()
	case modeTranscribeOnly:
		ui.btnTranscribe.Enable()
	case modeTranslateOnly:
		ui.btnTranslateOnly.Enable()
	}
}

// buildLogPanel 四个模块标签页，各一个实时日志列表。
func (ui *WorkflowUI) buildLogPanel() fyne.CanvasObject {
	ui.logTabs = container.NewAppTabs()
	ui.logTabs.SetTabLocation(container.TabLocationBottom)
	for _, m := range logx.Modules {
		m := m
		list := widget.NewList(
			func() int { return len(ui.logData[m]) },
			func() fyne.CanvasObject {
				t := canvas.NewText("", theme.ForegroundColor())
				t.TextSize = 12
				t.TextStyle = fyne.TextStyle{Monospace: true}
				return t
			},
			func(id widget.ListItemID, c fyne.CanvasObject) {
				t := c.(*canvas.Text)
				e := ui.logData[m][id]
				t.Text = fmt.Sprintf("%s [%s] %s", e.Time.Format("15:04:05.000"), e.Level, e.Text)
				t.Color = levelColor(e.Level)
			},
		)
		ui.logLists[m] = list
		ui.logTabs.Append(container.NewTabItem(string(m), list))
	}
	return ui.logTabs
}

// buildTable 字幕结果表格：序号 / 开始 / 结束 / 原文 / 译文。
// 原文/译文列支持双击内联编辑；其余列只读。
func (ui *WorkflowUI) buildTable() fyne.CanvasObject {
	headers := []string{"#", "开始", "结束", "原文", "译文"}
	ui.table = widget.NewTable(
		func() (int, int) {
			rows := 1
			if ui.subtitle != nil {
				rows = len(ui.subtitle.Segments) + 1
			}
			return rows, len(headers)
		},
		func() fyne.CanvasObject {
			c := newEditCell(*ui.W, ui.commitCell, nil)
			c.onBeginEdit = func() { ui.editingCell = c }
			c.onEndEdit = func() {
				if ui.editingCell == c {
					ui.editingCell = nil
				}
			}
			return c
		},
		func(id widget.TableCellID, c fyne.CanvasObject) {
			cell := c.(*editCell)
			cell.row = id.Row
			cell.col = id.Col
			cell.onTap = func(row, col int) {
				// 点击其他单元格时提交正在进行的编辑（点击处即确认，无需按 Enter）
				if ui.editingCell != nil && (ui.editingCell.row != row || ui.editingCell.col != col) {
					ui.editingCell.commit()
				}
				ui.table.Select(widget.TableCellID{Row: row, Col: col})
			}
			if id.Row == 0 {
				cell.setEditable(false)
				cell.setLabel(headers[id.Col], true)
				return
			}
			if ui.subtitle == nil || id.Row > len(ui.subtitle.Segments) {
				cell.setEditable(false)
				cell.setLabel("", false)
				return
			}
			seg := ui.subtitle.Segments[id.Row-1]
			switch id.Col {
			case 0:
				cell.setEditable(false)
				cell.setLabel(fmt.Sprintf("%d", id.Row), false)
			case 1:
				cell.setEditable(false)
				cell.setLabel(common.SRTTimeToString(seg.StartTime), false)
			case 2:
				cell.setEditable(false)
				cell.setLabel(common.SRTTimeToString(seg.EndTime), false)
			case 3:
				cell.setEditable(true)
				cell.setLabel(seg.Text, false)
			case 4:
				cell.setEditable(true)
				cell.setLabel(seg.Translation, false)
			}
		},
	)
	ui.table.SetColumnWidth(0, 44)
	ui.table.SetColumnWidth(1, 104)
	ui.table.SetColumnWidth(2, 104)
	ui.table.SetColumnWidth(3, 300)
	ui.table.SetColumnWidth(4, 300)
	return ui.table
}

// commitCell 内联编辑提交：把修改写回字幕并刷新表格。
func (ui *WorkflowUI) commitCell(row, col int, text string) {
	if ui.subtitle == nil || row <= 0 || row > len(ui.subtitle.Segments) {
		return
	}
	seg := &ui.subtitle.Segments[row-1]
	switch col {
	case 3:
		seg.Text = text
	case 4:
		seg.Translation = text
	default:
		return
	}
	ui.table.Refresh()
	ui.status.SetText(fmt.Sprintf("已修改第 %d 条字幕", row))
	logx.Info(logx.ModuleSystem, "修改第 %d 条字幕文本", row)
}

func (ui *WorkflowUI) setRunning(r bool) {
	ui.running = r
	if r {
		ui.btnFull.Disable()
		ui.btnTranscribe.Disable()
		ui.btnTranslateOnly.Disable()
		ui.btnExport.Disable()
		ui.btnExportOrig.Disable()
		ui.btnCancel.Enable()
		return
	}
	ui.btnCancel.Disable()
	ui.refreshButtons()
	if ui.subtitle != nil && len(ui.subtitle.Segments) > 0 {
		ui.btnExport.Enable()
		ui.btnExportOrig.Enable()
	}
}

// validateInput 按模式检查输入文件，返回 false 时已弹错误提示。
func (ui *WorkflowUI) validateInput() bool {
	if ui.Mode == modeTranslateOnly {
		if strings.TrimSpace(ui.SubtitlePath) == "" {
			showErrorDialog(*ui.W, fmt.Errorf("请先选择字幕文件"))
			return false
		}
		if _, err := os.Stat(ui.SubtitlePath); err != nil {
			showErrorDialog(*ui.W, fmt.Errorf("字幕文件不存在: %s", ui.SubtitlePath))
			return false
		}
		return true
	}
	if strings.TrimSpace(ui.MediaPath) == "" {
		showErrorDialog(*ui.W, fmt.Errorf("请先选择媒体文件"))
		return false
	}
	if _, err := os.Stat(ui.MediaPath); err != nil {
		showErrorDialog(*ui.W, fmt.Errorf("媒体文件不存在: %s", ui.MediaPath))
		return false
	}
	if strings.TrimSpace(ui.SubtitleDir) == "" {
		ui.SubtitleDir = "."
	}
	return true
}

// baseName 返回媒体/字幕文件的 basename（去扩展名）。
func (ui *WorkflowUI) baseName() string {
	p := ui.MediaPath
	if ui.Mode == modeTranslateOnly {
		p = ui.SubtitlePath
	}
	return strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
}

// showSavedDialog 导出成功弹窗：询问是否打开所在目录（而非无条件打开）。
func (ui *WorkflowUI) showSavedDialog(title, msg, path string) {
	d := dialog.NewConfirm(title, msg+"\n是否打开所在目录？", func(ok bool) {
		if ok {
			ui.openDir(filepath.Dir(path))
		}
	}, *ui.W)
	d.SetConfirmText("打开目录")
	d.SetDismissText("关闭")
	d.Show()
}

// saveCheckpoint 实时落盘字幕快照（在 worker goroutine 内调用）。
func (ui *WorkflowUI) saveCheckpoint(s *typedef.Subtitle, path, what string) {
	if err := s.SaveToFile(path); err != nil {
		logx.Error(logx.ModuleSystem, "实时保存%s失败 (%s): %v", what, path, err)
	} else {
		logx.Info(logx.ModuleSystem, "%s实时保存: %s (%d 段)", what, path, len(s.Segments))
	}
}

// countTranslated 统计已有译文的段数（部分失败时用于汇报进度）。
func countTranslated(sub *typedef.Subtitle) int {
	n := 0
	for _, s := range sub.Segments {
		if strings.TrimSpace(s.Translation) != "" {
			n++
		}
	}
	return n
}

// runTranslate 统一的翻译步骤：在副本上翻译（避免与 UI 线程读表格竞争）。
// checkpoint 每完成一个窗口回调一次（可为 nil）；即使返回 error，结果中也保留已成功窗口的译文。
func (ui *WorkflowUI) runTranslate(ctx context.Context, sub *typedef.Subtitle, prefix string, checkpoint func(*typedef.Subtitle)) (*typedef.Subtitle, error) {
	subCopy := &typedef.Subtitle{}
	subCopy.Segments = append([]typedef.Segment(nil), sub.Segments...)
	translated, err := ui.cm.TlAPI.TranslateContextWithOptions(ctx, subCopy, ui.TaskTranslate, func(p int, phase string) {
		fyne.Do(func() {
			ui.progress.SetValue(float64(p) / 100)
			ui.status.SetText(prefix + phase)
		})
	}, checkpoint)
	return &translated, err
}

// onTranscribe 仅转录：转录并实时保存原文 SRT，不翻译。
// 每完成一块就把当前累计字幕覆写到 原名.srt；取消时已完成的实时产物保留。
func (ui *WorkflowUI) onTranscribe() {
	if ui.running || !ui.validateInput() {
		return
	}
	ui.saveState()

	ctx, cancel := context.WithCancel(context.Background())
	ui.cancelFn = cancel
	ui.setRunning(true)
	ui.progress.SetValue(0)
	ui.status.SetText("仅转录: 转录中…")
	ui.logTabs.SelectIndex(2) // ASR 标签页
	mediaPath := ui.MediaPath
	subtitleDir := ui.SubtitleDir
	lang := ui.Lang
	opts := ui.TaskTranscribe
	srtPath := filepath.Join(subtitleDir, ui.baseName()+".srt")

	go func() {
		sub, err := ui.cm.TranscribeCtxWithOptions(ctx, mediaPath, lang, opts, func(p int, phase string) {
			fyne.Do(func() {
				ui.progress.SetValue(float64(p) / 100)
				ui.status.SetText("仅转录: " + phase)
			})
		}, func(s *typedef.Subtitle) {
			ui.saveCheckpoint(s, srtPath, "原文 SRT")
		})
		if err == nil {
			err = sub.SaveToFile(srtPath)
			logx.Info(logx.ModuleSystem, "原文 SRT 已保存: %s (%d 段)", srtPath, len(sub.Segments))
		}
		fyne.Do(func() {
			ui.setRunning(false)
			if err != nil {
				if ctx.Err() != nil {
					ui.status.SetText("已取消（实时产物保留至当前进度）")
					logx.Warn(logx.ModuleASR, "仅转录已取消，保留实时产物: %s", srtPath)
				} else {
					ui.status.SetText("转录失败")
					showErrorDialog(*ui.W, err)
				}
				return
			}
			ui.subtitle = sub
			ui.table.Refresh()
			ui.status.SetText(fmt.Sprintf("转录完成: %d 段字幕，已保存原文 SRT", len(sub.Segments)))
			ui.showSavedDialog("转录完成", "原文字幕已保存到:\n"+srtPath, srtPath)
		})
	}()
}

// onFullPipeline 转录+翻译：转录（实时写原文 SRT）→ 翻译（实时写双语 SRT）。
// 中间音频（临时 wav）由组件层用后即删；结束后产出原文与双语两份字幕。
// 翻译部分失败时：已翻译句保留、失败句留空，双语 SRT 照常保存并告警；取消时实时产物保留。
func (ui *WorkflowUI) onFullPipeline() {
	if ui.running || !ui.validateInput() {
		return
	}
	ui.saveState()

	ctx, cancel := context.WithCancel(context.Background())
	ui.cancelFn = cancel
	ui.setRunning(true)
	ui.progress.SetValue(0)
	ui.status.SetText("转录+翻译: 转录中…")
	ui.logTabs.SelectIndex(2) // ASR 标签页
	mediaPath := ui.MediaPath
	subtitleDir := ui.SubtitleDir
	lang := ui.Lang
	tOpts := ui.TaskTranscribe
	originalPath := filepath.Join(subtitleDir, ui.baseName()+".srt")
	bilingualPath := filepath.Join(subtitleDir, ui.baseName()+"-bilingual.srt")

	go func() {
		sub, err := ui.cm.TranscribeCtxWithOptions(ctx, mediaPath, lang, tOpts, func(p int, phase string) {
			fyne.Do(func() {
				ui.progress.SetValue(float64(p) / 100)
				ui.status.SetText("转录+翻译: " + phase)
			})
		}, func(s *typedef.Subtitle) {
			ui.saveCheckpoint(s, originalPath, "原文 SRT")
		})
		partialMsg := ""
		if err == nil {
			fyne.Do(func() {
				ui.subtitle = sub
				ui.table.Refresh()
				ui.status.SetText(fmt.Sprintf("转录完成 %d 段，开始翻译…", len(sub.Segments)))
				ui.progress.SetValue(0)
				ui.logTabs.SelectIndex(3) // 翻译标签页
			})
			var translated *typedef.Subtitle
			translated, err = ui.runTranslate(ctx, sub, "转录+翻译: ", func(s *typedef.Subtitle) {
				ui.saveCheckpoint(s, bilingualPath, "双语 SRT")
			})
			if err != nil {
				if ctx.Err() != nil {
					err = ctx.Err() // 取消：实时产物（原文 + 双语部分）保留
				} else {
					// 部分失败：保留已翻译句，失败句留空，双语 SRT 照常保存
					if sErr := translated.SaveToFile(bilingualPath); sErr == nil {
						logx.Info(logx.ModuleSystem, "双语 SRT 已保存（部分）: %s", bilingualPath)
					}
					logx.Error(logx.ModuleTranslate, "翻译部分失败: %v", err)
					partialMsg = fmt.Sprintf("翻译部分失败：%d/%d 句已翻译，失败句已留空；双语字幕已保存",
						countTranslated(translated), len(translated.Segments))
					sub = translated
					err = nil
				}
			} else {
				sub = translated
				err = translated.SaveToFile(bilingualPath)
			}
		}
		fyne.Do(func() {
			ui.setRunning(false)
			if err != nil {
				if ctx.Err() != nil {
					ui.status.SetText("已取消（实时产物保留至当前进度）")
					logx.Warn(logx.ModuleSystem, "转录+翻译已取消，保留实时产物: %s / %s", originalPath, bilingualPath)
				} else {
					ui.status.SetText("转录+翻译失败")
					showErrorDialog(*ui.W, err)
				}
				return
			}
			ui.subtitle = sub
			ui.table.Refresh()
			if partialMsg != "" {
				ui.status.SetText(partialMsg)
				dialog.ShowInformation("翻译部分失败", partialMsg+"\n"+bilingualPath, *ui.W)
				return
			}
			logx.Info(logx.ModuleSystem, "双语 SRT 已保存: %s (%d 段)", bilingualPath, len(sub.Segments))
			ui.status.SetText(fmt.Sprintf("转录+翻译完成: %d 段双语字幕（原文与双语两份 SRT 已保存）", len(sub.Segments)))
			ui.showSavedDialog("转录+翻译完成", "双语字幕已保存到:\n"+bilingualPath, bilingualPath)
		})
	}()
}

// onTranslateOnly 仅翻译：读入已有 SRT → 翻译 → 实时保存双语 SRT（与字幕文件同目录）。
// 部分失败/取消时已完成的翻译保留在磁盘上。
func (ui *WorkflowUI) onTranslateOnly() {
	if ui.running || !ui.validateInput() {
		return
	}
	ui.saveState()

	srcPath := ui.SubtitlePath
	sub, err := typedef.ParseSRTFile(srcPath)
	if err != nil {
		showErrorDialog(*ui.W, fmt.Errorf("解析字幕失败: %v", err))
		return
	}
	if len(sub.Segments) == 0 {
		showErrorDialog(*ui.W, fmt.Errorf("字幕文件没有内容: %s", srcPath))
		return
	}
	ui.subtitle = sub
	ui.table.Refresh()
	ui.btnExport.Enable()
	ui.btnExportOrig.Enable()

	ctx, cancel := context.WithCancel(context.Background())
	ui.cancelFn = cancel
	ui.setRunning(true)
	ui.progress.SetValue(0)
	ui.status.SetText(fmt.Sprintf("仅翻译: 已载入 %d 段，翻译中…", len(sub.Segments)))
	ui.logTabs.SelectIndex(3) // 翻译标签页
	bilingualPath := filepath.Join(filepath.Dir(srcPath), ui.baseName()+"-bilingual.srt")

	go func() {
		translated, tErr := ui.runTranslate(ctx, sub, "仅翻译: ", func(s *typedef.Subtitle) {
			ui.saveCheckpoint(s, bilingualPath, "双语 SRT")
		})
		partialMsg := ""
		if tErr != nil {
			if ctx.Err() != nil {
				// 取消：实时产物保留
			} else {
				// 部分失败：保留已翻译句，失败句留空，双语 SRT 照常保存
				if sErr := translated.SaveToFile(bilingualPath); sErr == nil {
					logx.Info(logx.ModuleSystem, "双语 SRT 已保存（部分）: %s", bilingualPath)
				}
				logx.Error(logx.ModuleTranslate, "翻译部分失败: %v", tErr)
				partialMsg = fmt.Sprintf("翻译部分失败：%d/%d 句已翻译，失败句已留空；双语字幕已保存",
					countTranslated(translated), len(translated.Segments))
				tErr = nil
			}
		} else {
			tErr = translated.SaveToFile(bilingualPath)
		}
		fyne.Do(func() {
			ui.setRunning(false)
			if tErr != nil {
				if ctx.Err() != nil {
					ui.status.SetText("已取消（实时产物保留至当前进度）")
					logx.Warn(logx.ModuleSystem, "仅翻译已取消，保留实时产物: %s", bilingualPath)
				} else {
					ui.status.SetText("仅翻译失败")
					showErrorDialog(*ui.W, tErr)
				}
				return
			}
			ui.subtitle = translated
			ui.table.Refresh()
			if partialMsg != "" {
				ui.status.SetText(partialMsg)
				dialog.ShowInformation("翻译部分失败", partialMsg+"\n"+bilingualPath, *ui.W)
				return
			}
			logx.Info(logx.ModuleSystem, "双语 SRT 已保存: %s (%d 段)", bilingualPath, len(translated.Segments))
			ui.status.SetText(fmt.Sprintf("仅翻译完成: %d 段双语字幕", len(translated.Segments)))
			ui.showSavedDialog("仅翻译完成", "双语字幕已保存到:\n"+bilingualPath, bilingualPath)
		})
	}()
}

func (ui *WorkflowUI) onExport(bilingual bool) {
	if ui.subtitle == nil || len(ui.subtitle.Segments) == 0 {
		return
	}
	ui.saveState()
	sub := ui.subtitle
	if !bilingual {
		// 原文导出：不带译文
		sub = &typedef.Subtitle{}
		for _, s := range ui.subtitle.Segments {
			sub.AddSegment(s.StartTime, s.EndTime, s.Text)
		}
	}
	name := ui.baseName() + ".srt"
	if bilingual {
		name = ui.baseName() + "-bilingual.srt"
	}
	var dir string
	if ui.Mode == modeTranslateOnly {
		dir = filepath.Dir(ui.SubtitlePath)
	} else {
		dir = ui.SubtitleDir
	}
	path := filepath.Join(dir, name)
	if err := sub.SaveToFile(path); err != nil {
		logx.Error(logx.ModuleSystem, "保存字幕失败: %v", err)
		showErrorDialog(*ui.W, err)
		return
	}
	logx.Info(logx.ModuleSystem, "字幕已导出: %s (%d 段)", path, len(sub.Segments))
	ui.status.SetText(fmt.Sprintf("已导出: %s", path))
	ui.showSavedDialog("导出成功", "字幕已保存到:\n"+path, path)
}

// openDir 用系统资源管理器打开目录。
func (ui *WorkflowUI) openDir(dir string) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	u, err := url.Parse("file://" + filepath.ToSlash(abs))
	if err != nil {
		logx.Error(logx.ModuleSystem, "解析输出目录失败: %v", err)
		showErrorDialog(*ui.W, err)
		return
	}
	if err := fyne.CurrentApp().OpenURL(u); err != nil {
		logx.Error(logx.ModuleSystem, "打开输出目录失败: %v", err)
		showErrorDialog(*ui.W, err)
	}
}

func (ui *WorkflowUI) onOpenDir() {
	if ui.Mode == modeTranslateOnly {
		ui.openDir(filepath.Dir(ui.SubtitlePath))
		return
	}
	ui.openDir(ui.SubtitleDir)
}

// HandleDropped 处理拖拽到窗口的文件：按扩展名切到对应模式并填入路径。
func (ui *WorkflowUI) HandleDropped(p string) {
	ext := strings.ToLower(filepath.Ext(p))
	if ext == ".srt" {
		ui.SubtitlePath = p
		ui.Mode = modeTranslateOnly
		ui.syncRadio()
		ui.refreshInputs()
		ui.refreshButtons()
		ui.saveState()
		ui.status.SetText("已导入字幕文件: " + p)
		return
	}
	if slices.Contains(typedef.VideoType, ext) || slices.Contains(typedef.AudioType, ext) {
		ui.MediaPath = p
		if ui.Mode == modeTranslateOnly {
			ui.Mode = modeFull
			ui.syncRadio()
		}
		ui.refreshInputs()
		ui.refreshButtons()
		ui.saveState()
		ui.status.SetText("已导入媒体文件: " + p)
		return
	}
	showErrorDialog(*ui.W, fmt.Errorf("不支持的文件类型: %s", ext))
}

func (ui *WorkflowUI) RenderWorkflowWindow() fyne.CanvasObject {
	ui.progress = widget.NewProgressBar()
	ui.status = widget.NewLabel("选择模式：转录+翻译（全自动） / 仅转录（只出原文字幕） / 仅翻译（已有字幕 → 译文）")
	ui.inputForm = container.NewVBox()

	top := container.NewVBox(
		ui.renderModeRadio(),
		ui.inputForm,
		ui.renderButtons(),
		container.NewGridWithColumns(2, ui.progress, ui.status),
	)

	tableArea := container.NewBorder(nil, nil, nil, nil, ui.buildTable())
	split := container.NewVSplit(tableArea, ui.buildLogPanel())
	split.SetOffset(0.62)

	ui.refreshInputs()
	ui.refreshButtons()
	return container.NewBorder(top, nil, nil, nil, split)
}
