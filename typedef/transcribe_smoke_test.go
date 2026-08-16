package typedef

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// TestSmokeTranscribeChunks 真实媒体冒烟测试：用 ffmpeg 截取 test/1.mp4 前 60 秒，
// 以 15s/5s 分块转录，验证多块都有产出、时间戳跨块推进。
//
// 这是"字幕只剩第一句"缺陷的回归测试：修复前第 2 块起被 whisper 的
// offset_ms 语义静默跳过（"input is too short"），只能产出第 1 块前 15 秒的内容，
// 无法满足"存在起点 >= 20 秒的段"的断言。
//
// 默认跳过（需要 whisper 模型 + ffmpeg）；设置 MYSUBGO_SMOKE=1 启用：
//
//	$env:MYSUBGO_SMOKE='1'; go test ./typedef/ -run TestSmokeTranscribeChunks -v
func TestSmokeTranscribeChunks(t *testing.T) {
	if os.Getenv("MYSUBGO_SMOKE") != "1" {
		t.Skip("冒烟测试默认跳过：设置 MYSUBGO_SMOKE=1 启用")
	}
	root, err := filepath.Abs(filepath.Join(".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	cfgMgr := NewConfigManager(filepath.Join(root, ConfigPath))
	if err := cfgMgr.Init(); err != nil {
		t.Skipf("加载配置失败: %v", err)
	}
	cfg := cfgMgr.Cfg
	cfg.FFmpeg.BinaryPath = filepath.Join(root, cfg.FFmpeg.BinaryPath)
	cfg.Whisper.ModelPath = filepath.Join(root, cfg.Whisper.ModelPath)
	if cfg.Whisper.VADPath != "" {
		cfg.Whisper.VADPath = filepath.Join(root, cfg.Whisper.VADPath)
	}

	src := filepath.Join(root, "test", "1.mp4")
	if _, err := os.Stat(src); err != nil {
		t.Skipf("测试媒体缺失: %v", err)
	}
	if _, err := os.Stat(cfg.Whisper.ModelPath); err != nil {
		t.Skipf("whisper 模型缺失: %v", err)
	}

	tmp, err := os.CreateTemp("", "smoke-*.wav")
	if err != nil {
		t.Fatalf("create temp wav: %v", err)
	}
	tmp.Close()
	defer os.Remove(tmp.Name())

	ff := exec.Command(cfg.FFmpeg.BinaryPath, "-y", "-i", src, "-t", "120",
		"-ar", "16000", "-ac", "1", "-c:a", "pcm_s16le", tmp.Name())
	if out, err := ff.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg 抽音频失败: %v\n%s", err, out)
	}

	ts := NewTranscriber(cfg)
	var checkpoints []int
	sub, err := ts.ProcessFileWithOptions(tmp.Name(), LangAuto,
		TranscribeOptions{ChunkDurationSec: 15, ChunkOverlapSec: 5},
		context.Background(), nil,
		func(s *Subtitle) {
			checkpoints = append(checkpoints, len(s.Segments))
		})
	if err != nil {
		t.Fatalf("转录失败: %v", err)
	}
	if len(checkpoints) == 0 {
		t.Fatal("checkpoint 一次都没触发（实时保存回调失效）")
	}
	if checkpoints[len(checkpoints)-1] != len(sub.Segments) {
		t.Fatalf("最后一个 checkpoint %d 段 ≠ 最终 %d 段", checkpoints[len(checkpoints)-1], len(sub.Segments))
	}
	if len(sub.Segments) < 3 {
		t.Fatalf("转录只产出 %d 段，期望 >= 3 段（多块均有产出）", len(sub.Segments))
	}
	var maxStart time.Duration
	for i, seg := range sub.Segments {
		if seg.StartTime < 0 || seg.EndTime <= seg.StartTime {
			t.Fatalf("段 %d 时间轴无效: %v --> %v", i+1, seg.StartTime, seg.EndTime)
		}
		if seg.StartTime > maxStart {
			maxStart = seg.StartTime
		}
	}
	if maxStart < 20*time.Second {
		t.Fatalf("所有段都落在 20 秒内（最大起点 %v），疑似只有第 1 块被转录", maxStart)
	}
	t.Logf("转录 %d 段，最大起点 %v（跨块正常）", len(sub.Segments), maxStart)
}
