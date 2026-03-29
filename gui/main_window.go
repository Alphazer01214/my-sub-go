package gui

import (
	"my-sub-go/typedef"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var navItems = []string{"首页", "设置", "字幕提取", "双语字幕提取", "视频音频提取"}

type Instance struct {
	MainWindow    fyne.Window
	cm            *typedef.ComponentManager
	ConfigUI      *ConfigUI
	ConverterUI   *ConverterUI
	TranscriberUI *TranscriberUI
	TranslatorUI  *TranslatorUI

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

	split := container.NewHSplit(ins.leftNav, ins.rightContent)
	split.SetOffset(0.2)
	ins.MainWindow.SetContent(split)
	ins.MainWindow.Resize(fyne.NewSize(1145, 999))
	ins.leftNav.Select(0)
	return nil
}

//func (ins *Instance) MountComponents(comps ...interface{}) {
//	for _, comp := range comps {
//		switch comp.(type) {
//		case *typedef.Converter:
//			ins.ConverterUI = NewConverterUI(comp.(*typedef.Converter), &ins.MainWindow)
//			fmt.Println("[main window] converter mounted")
//
//		case *typedef.Transcriber:
//			ins.TranscriberUI = NewTranscriberUI(comp.(*typedef.Transcriber), &ins.MainWindow)
//			fmt.Println("[main window] transcriber mounted")
//
//		default:
//			fmt.Println("[main window] unknown component")
//		}
//	}
//}

func (ins *Instance) mountComponents(cm *typedef.ComponentManager) {
	ins.cm = cm
	ins.TranscriberUI = NewTranscriberUI(cm, &ins.MainWindow)
	ins.ConverterUI = NewConverterUI(cm, &ins.MainWindow)
	ins.TranslatorUI = NewTranslatorUI(cm, &ins.MainWindow)
}

func (ins *Instance) mountUIs() {
	ins.uis["config"] = ins.ConfigUI.RenderConfigWindow()
	ins.uis["converter"] = ins.ConverterUI.RenderConverterWindow()
	ins.uis["transcriber"] = ins.TranscriberUI.RenderTranscriberWindow()
	ins.uis["translator"] = ins.TranslatorUI.RenderTranslatorWindow()
}

func (ins *Instance) switchContent(id int) {
	var content fyne.CanvasObject

	switch id {
	case 0:
		content = widget.NewLabel("MyGoSubtitle - Bilingual Subtitle Easily.")
	case 1: // Config
		// 使用 ConfigUI 渲染配置界面
		if ui, ok := ins.uis["config"]; ok && ui != nil {
			content = ui
		} else {
			content = widget.NewLabel("配置界面未初始化")
		}
	case 2: // ASR
		if ui, ok := ins.uis["transcriber"]; ok && ui != nil {
			content = ui
		} else {
			content = widget.NewLabel("字幕提取界面未初始化")
		}
	case 3: // Translate
		if ui, ok := ins.uis["translator"]; ok && ui != nil {
			content = ui
		} else {
			content = widget.NewLabel("双语字幕提取界面未初始化")
		}
	case 4:
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
