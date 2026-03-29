package gui

import (
	"my-sub-go/typedef"
	"reflect"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type ConfigUI struct {
	Cm       *typedef.ConfigManager
	Items    map[string]binding.DataItem
	W        *fyne.Window
	SaveBtn  *widget.Button
	ResetBtn *widget.Button
}

func NewConfigUI(cm *typedef.ConfigManager, w *fyne.Window) *ConfigUI {

	return &ConfigUI{
		Cm:    cm,
		Items: make(map[string]binding.DataItem),
		W:     w,
	}
}

func (ui *ConfigUI) renderField(field reflect.StructField, value *reflect.Value) fyne.CanvasObject {
	// define: `json:"binary_path" label: "name" description: "xxx" type: "file/dir/int/string/bool/lang/textarea" placeholder: "default" options: "a,b,c" support_ext: ".mp4,.wmv"`
	label := field.Tag.Get("label")
	fieldType := field.Tag.Get("type")

	var bind binding.DataItem
	var obj fyne.CanvasObject

	bind, _ = bindObj(fieldType, value, bind, obj)
	obj = getRealTimeObj(ui.W, fieldType, value)
	//ui.Items[field.Name] = bind
	return container.NewVBox(widget.NewLabel(label), obj)
}

func (ui *ConfigUI) renderTab() fyne.CanvasObject {
	// tabs
	tabs := container.NewAppTabs()
	v := reflect.ValueOf(ui.Cm.Cfg).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		f := t.Field(i)
		group := f.Tag.Get("group")
		if group == "" {
			continue
		}
		groupContainer := container.NewVBox()
		fieldVal := v.Field(i)
		if fieldVal.Kind() == reflect.Struct {
			// If: FFmpeg.BinaryPath:
			for j := 0; j < fieldVal.NumField(); j++ {
				field := fieldVal.Type().Field(j)
				fieldVal := fieldVal.Field(j)
				field.Name = group + "." + field.Name
				groupContainer.Add(ui.renderField(field, &fieldVal))
				field.Name = strings.TrimPrefix(field.Name, group+".")
			}
		} else {
			groupContainer.Add(ui.renderField(f, &fieldVal))
		}

		tabs.Append(container.NewTabItem(group, groupContainer))
	}

	return tabs
}

//func (ui *ConfigUI) reload() error {
//	// 将 UI binding 的值同步到 Config 结构体
//	v := reflect.ValueOf(ui.Cm.Cfg).Elem()
//	t := v.Type()
//
//	for i := 0; i < v.NumField(); i++ {
//		f := t.Field(i)
//		fieldVal := v.Field(i)
//
//		if fieldVal.Kind() == reflect.Struct {
//			// 嵌套结构体（如 FFmpegConfig, WhisperConfig 等）
//			for j := 0; j < fieldVal.NumField(); j++ {
//				field := fieldVal.Type().Field(j)
//				fieldValue := fieldVal.Field(j)
//				err := ui.syncFieldToConfig(field, fieldValue)
//				if err != nil {
//					return err
//				}
//			}
//		} else {
//			// 普通字段（如 CUDA, DefaultOutputDir）
//			err := ui.syncFieldToConfig(f, fieldVal)
//			if err != nil {
//				return err
//			}
//		}
//	}
//	return nil
//}
//
//func (ui *ConfigUI) syncFieldToConfig(field reflect.StructField, fieldValue reflect.Value) error {
//	fieldName := field.Name
//	bind, ok := ui.Items[fieldName]
//	if !ok {
//		return fmt.Errorf("error")
//	}
//
//	switch field.Tag.Get("type") {
//	case "bool":
//		if b, ok := bind.(binding.Bool); ok {
//			if val, err := b.Get(); err == nil {
//				fieldValue.SetBool(val)
//			}
//		}
//	case "int":
//		if s, ok := bind.(binding.String); ok {
//			if val, err := s.Get(); err == nil {
//				if intVal, err := strconv.Atoi(val); err == nil {
//					fieldValue.SetInt(int64(intVal))
//				}
//			}
//		}
//	default:
//		// string, file, dir, lang, textarea
//		if s, ok := bind.(binding.String); ok {
//			if val, err := s.Get(); err == nil {
//				fieldValue.SetString(val)
//			}
//		}
//	}
//	return nil
//}
//
//func (ui *ConfigUI) syncConfigToUI() error {
//	// 将 Config 的值同步到 UI binding
//	v := reflect.ValueOf(ui.Cm.Cfg).Elem()
//	t := v.Type()
//
//	for i := 0; i < v.NumField(); i++ {
//		f := t.Field(i)
//		fieldVal := v.Field(i)
//
//		if fieldVal.Kind() == reflect.Struct {
//			// 嵌套结构体
//			for j := 0; j < fieldVal.NumField(); j++ {
//				field := fieldVal.Type().Field(j)
//				fieldValue := fieldVal.Field(j)
//				err := ui.syncConfigToField(field, fieldValue)
//				if err != nil {
//					return err
//				}
//			}
//		} else {
//			// 普通字段
//			err := ui.syncConfigToField(f, fieldVal)
//			if err != nil {
//				return err
//			}
//		}
//	}
//	return nil
//}
//
//func (ui *ConfigUI) syncConfigToField(field reflect.StructField, fieldValue reflect.Value) error {
//	fieldName := field.Name
//	bind, ok := ui.Items[fieldName]
//	if !ok {
//		return fmt.Errorf("error")
//	}
//
//	switch field.Tag.Get("type") {
//	case "bool":
//		if b, ok := bind.(binding.Bool); ok {
//			err := b.Set(fieldValue.Bool())
//			if err != nil {
//				return err
//			}
//		}
//	case "int":
//		if s, ok := bind.(binding.String); ok {
//			err := s.Set(strconv.Itoa(int(fieldValue.Int())))
//			if err != nil {
//				return err
//			}
//		}
//	default:
//		// string, file, dir, lang, textarea
//		if s, ok := bind.(binding.String); ok {
//			err := s.Set(fieldValue.String())
//			if err != nil {
//				return err
//			}
//		}
//	}
//	return nil
//}

func (ui *ConfigUI) RenderConfigWindow() fyne.CanvasObject {
	tabs := ui.renderTab()
	ui.SaveBtn = widget.NewButton("Save", func() {
		// 显示确认对话框
		d := dialog.NewConfirm("保存配置", "是否保存配置并重启程序？",
			func(yes bool) {
				if yes {
					// 保存配置
					if err := ui.Cm.Save(); err != nil {
						dialog.ShowError(err, *ui.W)
						return
					}
					// 重启程序
					restartApp()
				}
			}, *ui.W)
		d.SetConfirmImportance(widget.DangerImportance)
		d.Show()
	})
	ui.ResetBtn = widget.NewButton("Reset", func() {
		if err := ui.Cm.Reset(); err != nil {
			dialog.ShowError(err, *ui.W)
			return
		}
		dialog.ShowInformation("Success", "Reset config success", *ui.W)
	})

	return container.NewVBox(tabs, container.NewHBox(ui.SaveBtn, ui.ResetBtn))

}
