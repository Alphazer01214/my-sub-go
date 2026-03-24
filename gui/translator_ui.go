package gui

import (
	"my-sub-go/typedef"
	"reflect"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type TranslatorUI struct {
	cm          *typedef.ComponentManager
	W           *fyne.Window
	MediaPath   string
	SubtitleDir string
	Btn         *widget.Button
}

func NewTranslatorUI(cm *typedef.ComponentManager, w *fyne.Window) *TranslatorUI {
	return &TranslatorUI{
		cm:          cm,
		W:           w,
		MediaPath:   "test/test.mp4",
		SubtitleDir: "test/",
	}
}

func (ui *TranslatorUI) renderField(field *reflect.StructField, value *reflect.Value) fyne.CanvasObject {
	// define: `json:"binary_path" label: "name" description: "xxx" type: "file/dir/int/string/bool/lang/textarea" placeholder: "default" options: "a,b,c" support_ext: ".mp4,.wmv"`
	label := field.Tag.Get("label")
	fieldType := field.Tag.Get("type")

	obj := getRealTimeObj(ui.W, fieldType, value)
	//ui.Items[field.Name] = bind
	return container.NewVBox(widget.NewLabel(label), obj)
}

func (ui *TranslatorUI) renderConfig() fyne.CanvasObject {
	tabs := container.NewVBox()

	mediaField := reflect.StructField{
		Name: "MediaPath",
		Type: reflect.TypeOf(""),
		Tag:  `json:"media_path" label:"媒体路径（音、视频）" type:"file" support_ext:".mp4,.wmv"`,
	}
	mediaValue := reflect.ValueOf(&ui.MediaPath).Elem()
	tabs.Add(ui.renderField(&mediaField, &mediaValue))

	subField := reflect.StructField{
		Name: "SubtitleDir",
		Type: reflect.TypeOf(""),
		Tag:  `json:"subtitle_dir" label:"字幕保存目录" type:"dir"`,
	}
	subValue := reflect.ValueOf(&ui.SubtitleDir).Elem()
	tabs.Add(ui.renderField(&subField, &subValue))

	return tabs
}

func (ui *TranslatorUI) RenderTranslatorWindow() fyne.CanvasObject {
	tabs := ui.renderConfig()
	progress := widget.NewProgressBarInfinite()
	progress.Hide()
	ui.Btn = widget.NewButton("execute", func() {
		ui.Btn.Disable()
		progress.Show()
		go func() {
			err := ui.cm.RunTranslatorAPIPipeline(ui.MediaPath, ui.SubtitleDir)
			fyne.Do(func() {
				if err != nil {
					fyne.LogError("RunTranslatorAPIPipeline", err)
					dialog.ShowInformation("Error", err.Error(), *ui.W)
				} else {
					dialog.ShowInformation("Success", "RunTranslatorAPIPipeline success: Subtitle saved at "+ui.SubtitleDir, *ui.W)
				}
				ui.Btn.Enable()
				progress.Hide()
			})
		}()
	})

	return container.NewVBox(tabs, ui.Btn, progress)
}
