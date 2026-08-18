package typedef

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"my-sub-go/common"
	"my-sub-go/common/logx"

	whisper2 "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

const (
	// vadMaxSpeechSec VAD 单个语音段最大时长（秒），超过的会在静音点自动分割
	vadMaxSpeechSec = 30.0
	// vadSamplesOverlapSec VAD 语音段重叠时长（秒），确保语音不被突然截断
	vadSamplesOverlapSec = 0.1
)

// Transcriber 基于 whisper.cpp 的转录器。
// 模型按需加载（LoadModel），依赖 VAD 进行语音活动检测和智能分段。
// VAD 会在静音点自动分割长语音段，避免 Whisper 时间戳漂移。
type Transcriber struct {
	Cfg *Config
	//Cvt    *Converter
	model  whisper2.Model
	ctx    whisper2.Context
	mu     sync.Mutex // 保护 model 加载
	procMu sync.Mutex // 保证同一时间只有一个转录任务
}

func NewTranscriber(cfg *Config) *Transcriber {
	var transcriber = &Transcriber{}
	_ = transcriber.Init(cfg)
	return transcriber
}

func (t *Transcriber) Init(cfg *Config) error {
	// 模型加载延迟到首次转录（LoadModel），让 GUI 秒开。
	t.Cfg = cfg
	return nil
}

// IsModelLoaded 模型是否已加载。
func (t *Transcriber) IsModelLoaded() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.model != nil
}

// LoadModel 加载 whisper 模型（懒加载，线程安全，重复调用无副作用）。
func (t *Transcriber) LoadModel() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.model != nil {
		return nil
	}
	path := t.Cfg.Whisper.ModelPath
	logx.Info(logx.ModuleASR, "开始加载模型: %s", path)
	start := time.Now()
	model, err := whisper2.New(path)
	if err != nil {
		logx.Error(logx.ModuleASR, "模型加载失败: %v", err)
		return err
	}
	t.model = model
	logx.Info(logx.ModuleASR, "模型加载完成 (%.1f 秒)", time.Since(start).Seconds())
	return nil
}

func (t *Transcriber) initContext() error {
	if ctx, err := t.model.NewContext(); err != nil {
		return err
	} else {
		t.ctx = ctx
	}

	if t.Cfg.Whisper.SrcLang != "" {
		if err := t.ctx.SetLanguage(string(t.Cfg.Whisper.SrcLang)); err != nil {
			return err
		}
	}

	if t.Cfg.Whisper.VADPath != "" {
		t.ctx.SetVAD(true)
		t.ctx.SetVADModelPath(t.Cfg.Whisper.VADPath)
		t.ctx.SetVADMaxSpeechSec(vadMaxSpeechSec)
		t.ctx.SetVADSamplesOverlap(vadSamplesOverlapSec)
	}

	if t.Cfg.Whisper.Threads > 0 {
		t.ctx.SetThreads(uint(t.Cfg.Whisper.Threads))
	}
	// 不再设置 SetMaxContext(233)：该魔法数字把 n_max_text_ctx 从默认 16384 压到 233，
	// 是长音频后段上下文失忆的疑点之一。

	return nil
}

func (t *Transcriber) setLang(lang Lang) error {
	err := t.ctx.SetLanguage(string(lang))
	if err != nil {
		return err
	}
	return nil
}



// ProcessFile 对 wav 文件做整段转录，依赖 VAD 进行语音活动检测和智能分段。
// VAD 会在静音点自动分割长语音段，避免 Whisper 时间戳漂移。
func (t *Transcriber) ProcessFile(path string, lang Lang, ctx context.Context, progress func(percent int, phase string)) (*Subtitle, error) {
	t.procMu.Lock()
	defer t.procMu.Unlock()

	if err := t.LoadModel(); err != nil {
		return nil, err
	}
	sub := &Subtitle{}
	if err := t.initContext(); err != nil {
		return nil, err
	}
	if err := t.setLang(lang); err != nil {
		return nil, err
	}

	data, err := common.DecodeWav(path)
	if err != nil {
		logx.Error(logx.ModuleASR, "解码音频失败: %v", err)
		return nil, err
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("音频为空: %s", path)
	}

	const sampleRate = 16000
	total := len(data)
	durationSec := float64(total) / sampleRate
	logx.Info(logx.ModuleASR, "开始转录: %s (时长 %.1f 分钟, VAD 分段, 最大语音段 %ds)",
		filepath.Base(path), durationSec/60, int(vadMaxSpeechSec))

	t.ctx.ResetTimings()

	if err := t.ctx.Process(data,
		func() bool { // encoder-begin：ctx 取消时中止
			select {
			case <-ctx.Done():
				return false
			default:
				return true
			}
		},
		nil,
		func(p int) {
			if progress != nil {
				progress(p, "转写中")
			}
		},
	); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		logx.Error(logx.ModuleASR, "识别失败: %v", err)
		return nil, err
	}

	// 语言自动检测
	if lang == LangAuto {
		if d := t.ctx.DetectedLanguage(); d != "" {
			if err := t.ctx.SetLanguage(d); err == nil {
				logx.Info(logx.ModuleASR, "检测到语言: %s", d)
			}
		}
	}

	// 读取所有字幕段
	for {
		seg, err := t.ctx.NextSegment()
		if err != nil {
			if err != io.EOF {
				logx.Warn(logx.ModuleASR, "读取字幕段失败: %v", err)
			}
			break
		}
		if seg.Text == "" {
			continue
		}
		sub.AddSegment(seg.Start, seg.End, strings.TrimSpace(seg.Text))
	}

	if progress != nil {
		progress(100, fmt.Sprintf("转写完成 %d 段", len(sub.Segments)))
	}
	logx.Info(logx.ModuleASR, "转录完成: 共 %d 段", len(sub.Segments))
	return sub, nil
}



func (t *Transcriber) GetSRTSubtitleFileFromAudio(aPath string, saveSRTDir string, lang Lang) (*Subtitle, error) {
	sub, err := t.ProcessFile(aPath, lang, context.Background(), nil)
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
