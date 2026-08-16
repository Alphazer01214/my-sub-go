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

type AudioSegment struct {
}

// Transcriber 基于 whisper.cpp 的转录器。
// 模型按需加载（LoadModel），整段音频按 chunk 分块识别：
// 每块 offset_ms 定位、块间用上一块尾部文本做 initial_prompt、重叠区去重，
// 并带幻觉重复守卫——这是修复"长视频后半段重复同一句字幕"的核心。
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

// TranscribeOptions 一次转录任务的分块参数；全零时回退到全局配置。
type TranscribeOptions struct {
	ChunkDurationSec int
	ChunkOverlapSec  int
}

// chunkConfig 返回分块参数（秒 → 采样点数），并保证合法。
// opts 全零（旧调用方）时使用全局配置；显式给定后按 opts 取值并兜底非法值。
func (t *Transcriber) chunkConfig(opts TranscribeOptions) (chunkSamples, stepSamples int) {
	const sampleRate = 16000
	dur := opts.ChunkDurationSec
	overlap := opts.ChunkOverlapSec
	if dur <= 0 && overlap <= 0 {
		dur = t.Cfg.Whisper.ChunkDurationSec
		overlap = t.Cfg.Whisper.ChunkOverlapSec
	}
	if dur <= 0 {
		dur = 20
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= dur {
		overlap = 0
	}
	return dur * sampleRate, (dur - overlap) * sampleRate
}

// ProcessFile 对 wav 文件做分块转录（使用全局配置的分块参数）。
func (t *Transcriber) ProcessFile(path string, lang Lang, ctx context.Context, progress func(percent int, phase string)) (*Subtitle, error) {
	return t.ProcessFileWithOptions(path, lang, TranscribeOptions{}, ctx, progress, nil)
}

// ProcessFileWithOptions 对 wav 文件做分块转录。
// 块内不做 offset：whisper 的 offset_ms 语义是"跳过本次传入音频的前 N 毫秒"，
// 与"块音频 + 块绝对偏移"叠加会把第 2 块起的音频整段跳过（静默 0 段）。
// 这里让 whisper 每块都从 0 解码，段时间为块内相对时间，再由 mergeChunk 加回绝对偏移。
// progress 回调（可为 nil）：percent 0-100，phase 为人类可读的阶段描述。
// checkpoint 回调（可为 nil）：每完成一块，用当前累计字幕的快照回调一次，供调用方实时落盘。
// ctx 取消后会在当前块结束后停止（块内通过 encoder-begin 回调中止）。
func (t *Transcriber) ProcessFileWithOptions(path string, lang Lang, opts TranscribeOptions, ctx context.Context, progress func(percent int, phase string), checkpoint func(*Subtitle)) (*Subtitle, error) {
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
	chunkSamples, stepSamples := t.chunkConfig(opts)
	total := len(data)
	nChunks := 0
	for start := 0; start < total; start += stepSamples {
		nChunks++
	}
	logx.Info(logx.ModuleASR, "开始转录: %s (时长 %.1f 分钟, %d 块, 每块 %.0fs)",
		filepath.Base(path), float64(total)/sampleRate/60, nChunks, float64(chunkSamples)/sampleRate)

	stepMs := stepSamples * 1000 / sampleRate
	var prevTail string
	zeroStart := -1 // 连续未产出字幕段的起始块（0-based），-1 表示当前没有连续段
	for i := 0; i < nChunks; i++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		start := i * stepSamples
		end := start + chunkSamples
		if end > total {
			end = total
		}
		chunk := data[start:end]
		offsetMs := start * 1000 / sampleRate

		t.ctx.SetInitialPrompt(prevTail)
		t.ctx.ResetTimings()

		chunkIdx := i
		logx.Info(logx.ModuleASR, "转写块 %d/%d (偏移 %s)", i+1, nChunks, fmt.Sprintf("%02d:%02d", offsetMs/60000, offsetMs/1000%60))
		if err := t.ctx.Process(chunk,
			func() bool { // encoder-begin：ctx 取消时中止当前块
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
					overall := int(float64(end) / float64(total) * 100)
					progress(overall, fmt.Sprintf("转写中 %d/%d 块", chunkIdx+1, nChunks))
				}
			},
		); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			logx.Error(logx.ModuleASR, "块 %d/%d 识别失败: %v", i+1, nChunks, err)
			return nil, err
		}

		var raw []rawSegment
		for {
			seg, err := t.ctx.NextSegment()
			if err != nil {
				if err != io.EOF {
					logx.Warn(logx.ModuleASR, "块 %d/%d 读取字幕段失败: %v", i+1, nChunks, err)
				}
				break
			}
			if seg.Text == "" {
				continue
			}
			raw = append(raw, rawSegment{Start: seg.Start, End: seg.End, Text: seg.Text})
		}
		added := mergeChunk(sub, raw, time.Duration(offsetMs)*time.Millisecond, time.Duration(stepMs)*time.Millisecond, i == nChunks-1)
		// 连续未产出告警：合并计数，避免静音/BGM 段逐块刷屏，也让"静默丢字幕"一眼可见。
		if added == 0 {
			if zeroStart < 0 {
				zeroStart = i
			}
		} else if zeroStart >= 0 {
			logx.Warn(logx.ModuleASR, "块 %d-%d 共 %d 块未产出字幕段（疑似静音或识别失败）", zeroStart+1, i, i-zeroStart)
			zeroStart = -1
		}
		// 语言自动检测只做一次：首块成功后固定，后续块沿用（更快且避免逐块误检）。
		if i == 0 && lang == LangAuto {
			if d := t.ctx.DetectedLanguage(); d != "" {
				if err := t.ctx.SetLanguage(d); err == nil {
					logx.Info(logx.ModuleASR, "检测到语言: %s，后续块沿用", d)
				}
			}
		}
		prevTail = tailText(sub, 100)
		if checkpoint != nil {
			checkpoint(sub.Clone())
		}
		logx.Info(logx.ModuleASR, "块 %d/%d 完成: 并入 %d 段, 累计 %d 段", i+1, nChunks, added, len(sub.Segments))
	}
	if zeroStart >= 0 {
		logx.Warn(logx.ModuleASR, "块 %d-%d 共 %d 块未产出字幕段（疑似静音或识别失败）", zeroStart+1, nChunks, nChunks-zeroStart)
	}
	if progress != nil {
		progress(100, fmt.Sprintf("转写完成 %d 段", len(sub.Segments)))
	}
	logx.Info(logx.ModuleASR, "转录完成: 共 %d 段", len(sub.Segments))
	return sub, nil
}

// isHallucination 幻觉重复守卫：识别 whisper 在 BGM/静音段陷入的
// "同一句反复出现 / 两句交替出现"模式，以及跨块边界重识别产生的重复。
// 规则保守，只拦时间上紧贴/重叠的重复。入参为绝对时间。
func isHallucination(sub *Subtitle, cand Segment) bool {
	n := len(sub.Segments)
	if n == 0 {
		return false
	}
	segDur := cand.EndTime - cand.StartTime
	last := sub.Segments[n-1]
	gap := cand.StartTime - last.EndTime

	// 跨块边界重识别重复：与上一段同文本且时间重叠过半
	overlapStart := cand.StartTime
	if last.StartTime > overlapStart {
		overlapStart = last.StartTime
	}
	overlapEnd := cand.EndTime
	if last.EndTime < overlapEnd {
		overlapEnd = last.EndTime
	}
	if cand.Text == last.Text && overlapEnd > overlapStart && 2*(overlapEnd-overlapStart) > segDur {
		return true
	}

	// 与上一段文本相同且紧贴、时长很短 → 重复幻觉
	if cand.Text == last.Text && segDur < 3*time.Second && gap < 1*time.Second {
		return true
	}
	// A B A B 交替模式（如示例: メイサちゃんの / お店に）
	if n >= 2 {
		prev := sub.Segments[n-2]
		lastDur := last.EndTime - last.StartTime
		if cand.Text == prev.Text && cand.Text != last.Text &&
			segDur < 2*time.Second && lastDur < 2*time.Second && gap < 500*time.Millisecond {
			return true
		}
	}
	return false
}

// tailText 取当前字幕末尾最多 maxChars 个字符，作为下一块的 initial_prompt。
func tailText(sub *Subtitle, maxChars int) string {
	var b strings.Builder
	for i := len(sub.Segments) - 1; i >= 0 && b.Len() < maxChars; i-- {
		b.WriteString(sub.Segments[i].Text)
	}
	s := b.String()
	runes := []rune(s)
	if len(runes) > maxChars {
		s = string(runes[len(runes)-maxChars:])
	}
	return s
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
