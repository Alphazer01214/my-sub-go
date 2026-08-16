package gui

import (
	"my-sub-go/common/logx"
	"my-sub-go/typedef"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var navItems = []string{"首页", "字幕工作流", "设置", "视频音频提取"}

type Instance struct {
	MainWindow  fyne.Window
	cm          *typedef.ComponentManager
	ConfigUI    *ConfigUI
	ConverterUI *ConverterUI
	WorkflowUI  *WorkflowUI

	leftNav         *widget.List // Navigation
	leftNavItemName []string
	rightContent    *fyne.Container //  Content
	uis             map[string]fyne.CanvasObject
}

func NewInstance(a fyne.App) *Instance {
	w := a.NewWindow("MyGoSubtitle")

	return &Instance{
		MainWindow: w,
		uis:        make(map[string]fyne.CanvasObject),
	}
}

func (ins *Instance) Init(cm *typedef.ConfigManager, cpm *typedef.ComponentManager) error {
	// load config and setup UI
	ins.ConfigUI = NewConfigUI(cm, &ins.MainWindow)
	ins.rightContent = container.NewStack(widget.NewLabel("Hello MyGoSubtitle"))
	ins.leftNav = widget.NewList(
		func() int {
			return len(navItems)
		},
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			item.(*widget.Label).SetText(navItems[id])
		},
	)
	ins.leftNav.OnSelected = func(id widget.ListItemID) {
		ins.switchContent(id)
	}
	ins.mountComponents(cpm)
	ins.mountUIs()

	// 拖拽文件到窗口：按扩展名切换到对应工作流模式并填入路径
	ins.MainWindow.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		if len(uris) == 0 {
			return
		}
		ins.WorkflowUI.HandleDropped(uris[0].Path())
	})

	split := container.NewHSplit(ins.leftNav, ins.rightContent)
	split.SetOffset(0.2)
	ins.MainWindow.SetContent(split)
	ins.MainWindow.Resize(fyne.NewSize(1145, 999))
	// 默认进入字幕工作流页
	ins.leftNav.Select(1)
	logx.Info(logx.ModuleSystem, "主窗口已初始化")
	return nil
}

func (ins *Instance) mountComponents(cm *typedef.ComponentManager) {
	ins.cm = cm
	ins.WorkflowUI = NewWorkflowUI(cm, &ins.MainWindow)
	ins.ConverterUI = NewConverterUI(cm, &ins.MainWindow)
}

func (ins *Instance) mountUIs() {
	ins.uis["config"] = ins.ConfigUI.RenderConfigWindow()
	ins.uis["converter"] = ins.ConverterUI.RenderConverterWindow()
	ins.uis["workflow"] = ins.WorkflowUI.RenderWorkflowWindow()
}

func (ins *Instance) switchContent(id int) {
	var content fyne.CanvasObject

	switch id {
	case 0:
		content = widget.NewLabel("MyGoSubtitle - Bilingual Subtitle Easily.")
	case 1: // 字幕工作流
		if ui, ok := ins.uis["workflow"]; ok && ui != nil {
			content = ui
		} else {
			content = widget.NewLabel("字幕工作流界面未初始化")
		}
	case 2: // Config
		// 使用 ConfigUI 渲染配置界面
		if ui, ok := ins.uis["config"]; ok && ui != nil {
			content = ui
		} else {
			content = widget.NewLabel("配置界面未初始化")
		}
	case 3: // 视频音频提取
		if ui, ok := ins.uis["converter"]; ok && ui != nil {
			content = ui
		} else {
			content = widget.NewLabel("视频音频提取界面未初始化")
		}
	default:
		content = widget.NewLabel("未知页面")
	}

	if ins.rightContent != nil {
		ins.rightContent.Objects = []fyne.CanvasObject{content}
		ins.rightContent.Refresh()
	}
}

func (ins *Instance) Run() {
	ins.MainWindow.ShowAndRun()
}
