package typedef

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"my-sub-go/common/logx"
)

type ComponentManager struct {
	Cfg   *Config
	Cvt   *Converter
	Ts    *Transcriber
	TlAPI *TranslatorAPI
}

func NewComponentManager(cfg *Config, comps ...interface{}) (*ComponentManager, error) {
	var cm = &ComponentManager{
		Cfg: cfg,
	}
	if err := cm.Init(cfg, comps...); err != nil {
		return nil, err
	}
	return cm, nil
}

func (cm *ComponentManager) Init(cfg *Config, comps ...interface{}) error {
	for _, comp := range comps {
		switch comp.(type) {
		case *Converter:
			cm.Cvt = NewConverter(cfg)
			logx.Info(logx.ModuleSystem, "converter 组件已挂载")

		case *Transcriber:
			cm.Ts = NewTranscriber(cfg)
			logx.Info(logx.ModuleSystem, "transcriber 组件已挂载（模型将在首次转录时加载）")

		case *TranslatorAPI:
			tl, err := NewTranslatorAPI(cfg)
			if err != nil {
				return fmt.Errorf("translator init failed: %w", err)
			}
			cm.TlAPI = tl
			logx.Info(logx.ModuleSystem, "translator 组件已挂载")

		default:
			logx.Warn(logx.ModuleSystem, "未知组件类型: %T", comp)
		}
	}
	return nil
}

// mediaKind 判断媒体文件类型：视频（需 FFmpeg 抽音频）还是音频（直接转写）。
func mediaKind(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if slices.Contains(VideoType, ext) {
		return "video"
	}
	if slices.Contains(AudioType, ext) {
		return "audio"
	}
	return ""
}

// TranscribeCtx 转录媒体文件（视频先抽音频、非 wav 音频先转码），使用全局分块参数，不落盘任何字幕。
func (cm *ComponentManager) TranscribeCtx(ctx context.Context, vPath string, lang Lang, progress func(int, string)) (*Subtitle, error) {
	return cm.TranscribeCtxWithOptions(ctx, vPath, lang, TranscribeOptions{}, progress, nil)
}

// TranscribeCtxWithOptions 同上，但分块参数来自 opts（全零时回退全局配置），
// checkpoint 每块回调一次（可为 nil，供调用方实时落盘字幕快照）。
// 中间音频使用隐藏临时名（.{base}.tmp.wav），转写完成后立即删除，不留中间产物。
// ctx 用于取消；progress(percent 0-100, phase) 可为 nil。
func (cm *ComponentManager) TranscribeCtxWithOptions(ctx context.Context, vPath string, lang Lang, opts TranscribeOptions, progress func(int, string), checkpoint func(*Subtitle)) (*Subtitle, error) {
	kind := mediaKind(vPath)
	if kind == "" {
		return nil, fmt.Errorf("不支持的媒体格式: %s", filepath.Ext(vPath))
	}

	base := strings.TrimSuffix(filepath.Base(vPath), filepath.Ext(vPath))
	dir := filepath.Dir(vPath)
	wavPath := ""
	removeWav := false

	if kind == "video" {
		wavPath = filepath.Join(dir, "."+base+".tmp.wav")
		logx.Info(logx.ModuleSystem, "开始提取音频: %s", vPath)
		if err := cm.Cvt.GetWavTmpFile(vPath, wavPath); err != nil {
			return nil, err
		}
		removeWav = true
	} else if strings.ToLower(filepath.Ext(vPath)) == ".wav" {
		wavPath = vPath
	} else {
		// 非 wav 音频先转成 16k 单声道 wav
		wavPath = filepath.Join(dir, "."+base+".tmp.wav")
		logx.Info(logx.ModuleSystem, "转换音频格式: %s -> %s", vPath, wavPath)
		if err := cm.Cvt.GetAudioWavFile(vPath, wavPath); err != nil {
			return nil, err
		}
		removeWav = true
	}
	if removeWav {
		defer func() {
			if err := os.Remove(wavPath); err != nil {
				logx.Warn(logx.ModuleFFmpeg, "清理临时音频失败: %v", err)
			} else {
				logx.Info(logx.ModuleFFmpeg, "临时音频已删除: %s", wavPath)
			}
		}()
	}

	sub, err := cm.Ts.ProcessFileWithOptions(wavPath, lang, opts, ctx, progress, checkpoint)
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// RunVideoTranscriber 转写并保存原文 SRT（兼容旧调用：后台上下文、无进度回调）。
func (cm *ComponentManager) RunVideoTranscriber(vPath string, saveSRTDir string, lang Lang) (*Subtitle, error) {
	sub, err := cm.TranscribeCtx(context.Background(), vPath, lang, nil)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(filepath.Base(vPath), filepath.Ext(vPath))
	srtPath := filepath.Join(saveSRTDir, base+".srt")
	if err := sub.SaveToFile(srtPath); err != nil {
		return nil, err
	}
	logx.Info(logx.ModuleSystem, "字幕已保存: %s (%d 段)", srtPath, len(sub.Segments))
	return sub, nil
}

// RunAudioTranscriber 兼容旧调用：直接转写 wav 并保存 SRT。
func (cm *ComponentManager) RunAudioTranscriber(aPath string, saveSRTDir string, lang Lang) (*Subtitle, error) {
	sub, err := cm.Ts.ProcessFile(aPath, lang, context.Background(), nil)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(filepath.Base(aPath), filepath.Ext(aPath))
	srtPath := filepath.Join(saveSRTDir, base+".srt")
	if err := sub.SaveToFile(srtPath); err != nil {
		return nil, err
	}
	logx.Info(logx.ModuleSystem, "字幕已保存: %s (%d 段)", srtPath, len(sub.Segments))
	return sub, nil
}

func (cm *ComponentManager) RunTranslatorAPIPipeline(mediaPath string, saveSRTDir string) error {
	// Pipeline
	ext := filepath.Ext(mediaPath)
	base := filepath.Base(mediaPath)
	if slices.Contains(AudioType, ext) {
		sub, err := cm.RunAudioTranscriber(mediaPath, saveSRTDir, cm.Cfg.Whisper.SrcLang)
		if err != nil {
			return err
		}
		res, err := cm.TlAPI.Translate(sub)
		if err != nil {
			return err
		}

		return res.SaveToFile(filepath.Join(saveSRTDir, strings.TrimSuffix(base, ext)+"-bilingual.srt"))
	}

	if slices.Contains(VideoType, ext) {
		sub, err := cm.RunVideoTranscriber(mediaPath, saveSRTDir, cm.Cfg.Whisper.SrcLang)
		if err != nil {
			return err
		}
		res, err := cm.TlAPI.Translate(sub)
		if err != nil {
			return err
		}
		return res.SaveToFile(filepath.Join(saveSRTDir, base+"-bilingual.srt"))
	}
	return fmt.Errorf("不支持的媒体类型 %s", ext)
}
