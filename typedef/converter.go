package typedef

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"my-sub-go/common/logx"
)

type Converter struct {
	Cfg          *Config
	CvtArgs      *ConverterArgs
	isProcessing bool
}

type ConverterArgs struct {
	AudioCodec string `label:"音频编码" type:"string" options:"pcm_s16le"`
	SampleRate int    `label:"采样率" type:"int" placeholder:"16000"`
	Channels   int    `label:"声道数" type:"int" placeholder:"2"`
}

func NewConverter(cfg *Config) *Converter {
	var cvt = &Converter{
		isProcessing: false,
	}
	_ = cvt.Init(cfg)
	return cvt
}

func (cvt *Converter) Init(cfg *Config) error {
	cvt.Cfg = cfg
	cvt.CvtArgs = &ConverterArgs{
		AudioCodec: cvt.Cfg.FFmpeg.AudioCodec,
		SampleRate: cvt.Cfg.FFmpeg.SampleRate,
		Channels:   2,
	}
	return nil
}

// runFFmpeg 执行 ffmpeg，stderr 进日志；失败时错误信息带 stderr 尾部。
func (cvt *Converter) runFFmpeg(args ...string) error {
	logx.Info(logx.ModuleFFmpeg, "执行: %s %s", cvt.Cfg.FFmpeg.BinaryPath, strings.Join(args, " "))
	var stderr bytes.Buffer
	cmd := exec.Command(cvt.Cfg.FFmpeg.BinaryPath, args...)
	cmd.Stderr = &stderr
	start := time.Now()
	if err := cmd.Run(); err != nil {
		tail := stderr.String()
		logx.Error(logx.ModuleFFmpeg, "ffmpeg 失败 (%.1f 秒): %v\n%s", time.Since(start).Seconds(), err, tailStr(tail, 2000))
		return fmt.Errorf("[FFmpeg] failed: %v\n%s", err, tailStr(tail, 500))
	}
	logx.Info(logx.ModuleFFmpeg, "ffmpeg 完成 (%.1f 秒)", time.Since(start).Seconds())
	return nil
}

// tailStr 截取文本末尾最多 n 个字符。
func tailStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

// GetWavTmpFile 从视频抽取单声道 16k wav 到 targetAudioPath（转录用）。
func (cvt *Converter) GetWavTmpFile(videoPath string, targetAudioPath string) error {
	cvt.isProcessing = true
	defer func() { cvt.isProcessing = false }()

	if err := os.MkdirAll(filepath.Dir(targetAudioPath), 0755); err != nil {
		return fmt.Errorf("[FFmpeg] invalid target dir: %v \n %v", filepath.Dir(targetAudioPath), err)
	}
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("[FFmpeg] invalid video path: %s", videoPath)
	}

	suf := strings.ToLower(filepath.Ext(videoPath))
	if !slices.Contains(VideoType, suf) {
		return fmt.Errorf("[FFmpeg] unsupported video format: %s", suf)
	}

	args := []string{
		"-i", videoPath,
		"-vn",
		"-acodec", cvt.Cfg.FFmpeg.AudioCodec,
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", cvt.Cfg.FFmpeg.SampleRate),
		"-f", "wav",
		"-y",
		targetAudioPath,
	}
	return cvt.runFFmpeg(args...)
}

// GetAudioWavFile 把任意音频文件转成单声道 16k wav（转录用）。
func (cvt *Converter) GetAudioWavFile(audioPath string, targetAudioPath string) error {
	cvt.isProcessing = true
	defer func() { cvt.isProcessing = false }()

	if err := os.MkdirAll(filepath.Dir(targetAudioPath), 0755); err != nil {
		return fmt.Errorf("[FFmpeg] invalid target dir: %v \n %v", filepath.Dir(targetAudioPath), err)
	}
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return fmt.Errorf("[FFmpeg] invalid audio path: %s", audioPath)
	}

	args := []string{
		"-i", audioPath,
		"-vn",
		"-acodec", cvt.Cfg.FFmpeg.AudioCodec,
		"-ac", "1",
		"-ar", fmt.Sprintf("%d", cvt.Cfg.FFmpeg.SampleRate),
		"-f", "wav",
		"-y",
		targetAudioPath,
	}
	return cvt.runFFmpeg(args...)
}

func (cvt *Converter) GetVideoWavFile(videoPath string, targetAudioDir string, args *ConverterArgs) error {
	if err := os.MkdirAll(targetAudioDir, 0755); err != nil {
		return fmt.Errorf("[FFmpeg] invalid target dir: %v \n %v", targetAudioDir, err)
	}
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("[FFmpeg] invalid video path: %s", videoPath)
	}

	ext := strings.ToLower(filepath.Ext(videoPath))
	base := strings.TrimSuffix(filepath.Base(videoPath), ext)

	if !slices.Contains(VideoType, ext) {
		return fmt.Errorf("[FFmpeg] unsupported video format: %s", ext)
	}

	ffArgs := []string{
		"-i", videoPath,
		"-vn",
		"-acodec", args.AudioCodec,
		"-ac", fmt.Sprintf("%d", args.Channels),
		"-ar", fmt.Sprintf("%d", args.SampleRate),
		"-f", "wav",
		"-y",
		filepath.Join(targetAudioDir, base+".wav"),
	}
	return cvt.runFFmpeg(ffArgs...)
}

func (cvt *Converter) Update(cfg *Config) {
	cvt.Cfg = cfg
}

func (cvt *Converter) IsProcessing() bool {
	return cvt.isProcessing
}
