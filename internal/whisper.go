package internal

import (
	"fmt"
	"my-sub-go/common"
	"my-sub-go/typedef"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go"
	whisper2 "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type AudioSegment struct {
}

type Transcriber struct {
	cfg    common.WhisperConfig
	model  whisper2.Model
	params whisper.Params
	ctx    whisper2.Context
}

func (t *Transcriber) Init(cfg common.WhisperConfig) error {
	// 	BinaryPath       string `json:"binary_path"` // Whisper可执行文件路径
	//	ModelPath        string `json:"model_path"`  // Whisper模型路径
	//	VADPath          string `json:"vad_path"`
	//	SrcLang          string `json:"src_lang"`           // 源语言
	//	ChunkDurationSec int    `json:"chunk_duration_sec"` // 音频分块时长(秒)
	//	ChunkOverlapSec  int    `json:"chunk_overlap_sec"`  // 音频分块重叠时长(秒)
	t.cfg = cfg
	err := t.initModel()
	if err != nil {
		return err
	}

	return nil
}

func (t *Transcriber) initModel() error {
	if model, err := whisper2.New(t.cfg.ModelPath); err != nil {
		fmt.Println("[whisper] Error loading model:", err)
		return err
	} else {
		t.model = model
	}
	fmt.Println("[whisper] Model loaded")
	return nil
}

func (t *Transcriber) initContext() error {
	if ctx, err := t.model.NewContext(); err != nil {
		return err
	} else {
		t.ctx = ctx
	}

	if t.cfg.SrcLang != "" {
		if err := t.ctx.SetLanguage(t.cfg.SrcLang); err != nil {
			return err
		}
	}

	if t.cfg.VADPath != "" {
		t.ctx.SetVAD(true)
		t.ctx.SetVADModelPath(t.cfg.VADPath)
	}

	if t.cfg.Threads > 0 {
		t.ctx.SetThreads(uint(t.cfg.Threads))
	}

	t.ctx.SetMaxContext(233)

	fmt.Println("[whisper] Context loaded")
	return nil
}

func (t *Transcriber) Process(path string) (*typedef.Subtitle, error) {
	sub := &typedef.Subtitle{}
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

func (t *Transcriber) Update(cfg common.WhisperConfig) {
	t.cfg = cfg
}
