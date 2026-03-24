package typedef

import (
	"fmt"
	"my-sub-go/common"
	"path/filepath"
	"strings"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go"
	whisper2 "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type AudioSegment struct {
}

type Transcriber struct {
	Cfg *Config
	//Cvt    *Converter
	model  whisper2.Model
	params whisper.Params
	ctx    whisper2.Context
}

func NewTranscriber(cfg *Config) *Transcriber {
	var transcriber = &Transcriber{}
	_ = transcriber.Init(cfg)
	return transcriber
}

func (t *Transcriber) Init(cfg *Config) error {
	// 	BinaryPath       string `json:"binary_path"` // Whisper可执行文件路径
	//	ModelPath        string `json:"model_path"`  // Whisper模型路径
	//	VADPath          string `json:"vad_path"`
	//	SrcLang          string `json:"src_lang"`           // 源语言
	//	ChunkDurationSec int    `json:"chunk_duration_sec"` // 音频分块时长(秒)
	//	ChunkOverlapSec  int    `json:"chunk_overlap_sec"`  // 音频分块重叠时长(秒)
	t.Cfg = cfg
	err := t.initModel()
	//t.linkConverter(cvt)
	if err != nil {
		return err
	}

	return nil
}

func (t *Transcriber) initModel() error {
	if model, err := whisper2.New(t.Cfg.Whisper.ModelPath); err != nil {
		fmt.Println("[whisper] Error loading model:", err)
		return err
	} else {
		t.model = model
	}
	fmt.Println("[whisper] Model loaded")
	return nil
}

//func (t *Transcriber) linkConverter(cvt *Converter) {
//	t.Cvt = cvt
//}

func (t *Transcriber) initContext() error {
	if ctx, err := t.model.NewContext(); err != nil {
		return err
	} else {
		t.ctx = ctx
	}

	if t.Cfg.Whisper.SrcLang != "" {
		if err := t.ctx.SetLanguage(t.Cfg.Whisper.SrcLang); err != nil {
			return err
		}
	}

	if t.Cfg.Whisper.VADPath != "" {
		t.ctx.SetVAD(true)
		t.ctx.SetVADModelPath(t.Cfg.Whisper.VADPath)
	}

	if t.Cfg.Whisper.Threads > 0 {
		t.ctx.SetThreads(uint(t.Cfg.Whisper.Threads))
	}

	t.ctx.SetMaxContext(233)

	fmt.Println("[whisper] Context loaded")
	return nil
}
func (t *Transcriber) setLang(lang string) error {
	err := t.ctx.SetLanguage(lang)
	if err != nil {
		return err
	}
	return nil
}

func (t *Transcriber) process(path string) (*Subtitle, error) {
	sub := &Subtitle{}
	err := t.initContext()
	if err != nil {
		return sub, err
	}
	data, err := common.DecodeWav(path)
	if err != nil {
		return sub, err
	}
	fmt.Println("[whisper] Processing...")
	if err := t.ctx.Process(data, nil, nil, nil); err != nil {
		return sub, err
	}
	for {
		seg, err := t.ctx.NextSegment()
		if err != nil {
			break // 遍历完成或出错时退出
		}
		sub.AddSegment(seg.Start, seg.End, seg.Text)
	}
	return sub, nil
}

func (t *Transcriber) processLangSpecific(path string, lang string) (*Subtitle, error) {
	sub := &Subtitle{}
	err := t.initContext()
	if err != nil {
		return sub, err
	}
	err = t.setLang(lang)
	if err != nil {
		return sub, err
	}
	data, err := common.DecodeWav(path)
	if err != nil {
		return sub, err
	}
	fmt.Println("[whisper] Processing...")
	if err := t.ctx.Process(data, nil, nil, nil); err != nil {
		return sub, err
	}
	for {
		seg, err := t.ctx.NextSegment()
		if err != nil {
			break // 遍历完成或出错时退出
		}
		sub.AddSegment(seg.Start, seg.End, seg.Text)
	}
	return sub, nil
}

func (t *Transcriber) GetSRTSubtitleFileFromAudio(aPath string, saveSRTDir string, lang string) (*Subtitle, error) {

	sub, err := t.processLangSpecific(aPath, lang)
	if err != nil {
		return nil, err
	}

	err = sub.SaveToFile(filepath.Join(saveSRTDir, strings.TrimSuffix(filepath.Base(aPath), filepath.Ext(aPath))+".srt"))
	if err != nil {
		return nil, err
	}
	return sub, nil
}

func (t *Transcriber) Update(cfg *Config) {
	t.Cfg = cfg
}
