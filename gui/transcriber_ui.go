package gui

import (
	"my-sub-go/typedef"
	"reflect"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type TranscriberUI struct {
	//Ts          *typedef.Transcriber
	cm          *typedef.ComponentManager
	W           *fyne.Window
	Path        string
	SubtitleDir string
	Lang        typedef.Lang
	Btn         *widget.Button
}

func NewTranscriberUI(cm *typedef.ComponentManager, w *fyne.Window) *TranscriberUI {
	return &TranscriberUI{
		cm:          cm,
		W:           w,
		Lang:        typedef.LangAuto,
		Path:        "test/test.mp4",
		SubtitleDir: "test/",
	}
}

func (ui *TranscriberUI) renderField(field *reflect.StructField, value *reflect.Value) fyne.CanvasObject {
	// define: `json:"binary_path" label: "name" description: "xxx" type: "file/dir/int/string/bool/lang/textarea" placeholder: "default" options: "a,b,c" support_ext: ".mp4,.wmv"`
	label := field.Tag.Get("label")
	fieldType := field.Tag.Get("type")

	obj := getRealTimeObj(ui.W, fieldType, value)
	//ui.Items[field.Name] = bind
	return container.NewVBox(widget.NewLabel(label), obj)
}

func (ui *TranscriberUI) renderConfig() fyne.CanvasObject {
	tabs := container.NewVBox()

	videoField := reflect.StructField{
		Name: "VideoPath",
		Type: reflect.TypeOf(""),
		Tag:  `json:"video_path" label:"媒体路径（音、视频）" type:"file" support_ext:".mp4,.wmv"`,
	}
	videoValue := reflect.ValueOf(&ui.Path).Elem()
	tabs.Add(ui.renderField(&videoField, &videoValue))

	subField := reflect.StructField{
		Name: "SubtitleDir",
		Type: reflect.TypeOf(""),
		Tag:  `json:"audio_dir" label:"字幕保存目录" type:"dir"`,
	}
	audioValue := reflect.ValueOf(&ui.SubtitleDir).Elem()
	tabs.Add(ui.renderField(&subField, &audioValue))

	langField := reflect.StructField{
		Name: "Lang",
		Type: reflect.TypeOf(typedef.Lang("")),
		Tag:  `json:"audio_codec" label:"音频语言" type:"lang"`,
	}
	codecValue := reflect.ValueOf(&ui.Lang).Elem()
	tabs.Add(ui.renderField(&langField, &codecValue))

	return tabs

}
func (ui *TranscriberUI) RenderTranscriberWindow() fyne.CanvasObject {
	tabs := ui.renderConfig()
	progress := widget.NewProgressBarInfinite()
	progress.Hide()
	ui.Btn = widget.NewButton("execute", func() {
		runInBackground(*ui.W, ui.Btn, progress, "GetSRTSubtitleFile",
			"GetSRTSubtitleFile success: Subtitle saved at "+ui.SubtitleDir,
			func() error {
				_, err := ui.cm.RunVideoTranscriber(ui.Path, ui.SubtitleDir, ui.Lang)
				return err
			})
	})

	return container.NewVBox(tabs, ui.Btn, progress)
}
