package gui

import (
	"my-sub-go/typedef"
	"reflect"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

type ConverterUI struct {
	//Cvt        *typedef.Converter // 仅使用binary path
	cm         *typedef.ComponentManager
	W          *fyne.Window
	VideoPath  string
	AudioDir   string
	AudioCodec string
	Channels   int
	SampleRate int
	Btn        *widget.Button

	//Items map[string]binding.DataItem
}

func NewConverterUI(cm *typedef.ComponentManager, w *fyne.Window) *ConverterUI {
	return &ConverterUI{
		cm:         cm,
		VideoPath:  "test/test.mp4",
		AudioDir:   ".",
		AudioCodec: "pcm_s16le",
		Channels:   2,
		SampleRate: 16000,
		//Items:      make(map[string]binding.DataItem),
		W: w,
	}
}

func (ui *ConverterUI) renderField(field *reflect.StructField, value *reflect.Value) fyne.CanvasObject {
	// define: `json:"binary_path" label: "name" description: "xxx" type: "file/dir/int/string/bool/lang/textarea" placeholder: "default" options: "a,b,c" support_ext: ".mp4,.wmv"`
	label := field.Tag.Get("label")
	fieldType := field.Tag.Get("type")

	obj := getRealTimeObj(ui.W, fieldType, value)
	//ui.Items[field.Name] = bind
	return container.NewVBox(widget.NewLabel(label), obj)
}

func (ui *ConverterUI) renderConfig() fyne.CanvasObject {
	tabs := container.NewVBox()

	videoField := reflect.StructField{
		Name: "VideoPath",
		Type: reflect.TypeOf(""),
		Tag:  `json:"video_path" label:"视频路径" type:"file" support_ext:".mp4,.wmv"`,
	}
	videoValue := reflect.ValueOf(&ui.VideoPath).Elem()
	tabs.Add(ui.renderField(&videoField, &videoValue))

	audioField := reflect.StructField{
		Name: "AudioDir",
		Type: reflect.TypeOf(""),
		Tag:  `json:"audio_dir" label:"音频保存目录" type:"dir"`,
	}
	audioValue := reflect.ValueOf(&ui.AudioDir).Elem()
	tabs.Add(ui.renderField(&audioField, &audioValue))

	codecField := reflect.StructField{
		Name: "AudioCodec",
		Type: reflect.TypeOf(""),
		Tag:  `json:"audio_codec" label:"音频编码" type:"string" options:"pcm_s16le"`,
	}
	codecValue := reflect.ValueOf(&ui.AudioCodec).Elem()
	tabs.Add(ui.renderField(&codecField, &codecValue))

	channelField := reflect.StructField{
		Name: "Channels",
		Type: reflect.TypeOf(0),
		Tag:  `json:"channels" label:"声道数" type:"int" placeholder:"2"`,
	}
	channelValue := reflect.ValueOf(&ui.Channels).Elem()
	tabs.Add(ui.renderField(&channelField, &channelValue))

	sampleRateField := reflect.StructField{
		Name: "SampleRate",
		Type: reflect.TypeOf(0),
		Tag:  `json:"sample_rate" label:"采样率" type:"int" placeholder:"44100"`,
	}
	sampleRateValue := reflect.ValueOf(&ui.SampleRate).Elem()
	tabs.Add(ui.renderField(&sampleRateField, &sampleRateValue))

	return tabs

}
func (ui *ConverterUI) RenderConverterWindow() fyne.CanvasObject {
	tabs := ui.renderConfig()
	progress := widget.NewProgressBarInfinite()
	progress.Hide()
	ui.Btn = widget.NewButton("execute", func() {
		if ui.cm.Cvt.IsProcessing() {
			return
		}
		ui.Btn.Disable()
		progress.Show()
		go func() {
			err := ui.cm.Cvt.GetVideoWavFile(ui.VideoPath, ui.AudioDir, ui.cm.Cvt.CvtArgs)
			fyne.Do(func() {
				if err != nil {
					fyne.LogError("GetVideoWavFile", err)
					dialog.ShowInformation("Error", err.Error(), *ui.W)
				} else {
					dialog.ShowInformation("Success", "GetVideoWavFile success: Audio saved at "+ui.AudioDir, *ui.W)
				}
				ui.Btn.Enable()
				progress.Hide()
			})
		}()
	})

	return container.NewVBox(tabs, ui.Btn, progress)
}
