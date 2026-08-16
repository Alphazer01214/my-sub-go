// Package logx 提供按模块分组的应用日志：GUI 订阅展示 + 文件落盘（按天滚动）。
package logx

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Level 日志级别。
type Level int

const (
	LevelInfo Level = iota
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// Module 日志来源模块（见 CONTEXT.md 的"日志模块"术语）。
type Module string

const (
	ModuleSystem    Module = "系统"     // 启动/配置/主流程
	ModuleFFmpeg    Module = "FFmpeg" // 音频提取
	ModuleASR       Module = "ASR"    // 转录
	ModuleTranslate Module = "翻译"     // LLM 翻译
)

// Modules 所有日志模块的固定顺序，GUI 按此渲染标签页。
var Modules = []Module{ModuleSystem, ModuleFFmpeg, ModuleASR, ModuleTranslate}

// Entry 一条日志。
type Entry struct {
	Time   time.Time
	Level  Level
	Module Module
	Text   string
}

// Line 日志文件的单行格式。
func (e Entry) Line() string {
	return fmt.Sprintf("%s [%s] [%s] %s",
		e.Time.Format("2006-01-02 15:04:05.000"), e.Module, e.Level, e.Text)
}

// 订阅缓冲大小；溢出时丢弃最旧日志，保证 GUI 展示不被慢消费者拖死。
const subBuffer = 256

type logger struct {
	mu      sync.Mutex
	dir     string
	file    *os.File
	fileDay string
	subs    map[Module][]chan Entry
}

var log = &logger{subs: make(map[Module][]chan Entry)}

// Init 初始化日志目录（exe 旁的 logs/）、清理过期文件并打开当天的日志文件。
// 返回错误不应阻止程序启动，调用方可忽略。
func Init() error {
	exe, err := os.Executable()
	if err != nil {
		exe = "."
	}
	dir := filepath.Join(filepath.Dir(exe), "logs")
	log.mu.Lock()
	defer log.mu.Unlock()
	log.dir = dir
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	pruneLocked(dir)
	return openFileLocked(time.Now())
}

// Close 关闭日志文件。
func Close() {
	log.mu.Lock()
	defer log.mu.Unlock()
	if log.file != nil {
		_ = log.file.Close()
		log.file = nil
	}
}

func openFileLocked(now time.Time) error {
	if log.file != nil && log.fileDay == now.Format("20060102") {
		return nil
	}
	if log.file != nil {
		_ = log.file.Close()
	}
	name := filepath.Join(log.dir, "my-sub-go-"+now.Format("20060102")+".log")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.file = nil
		return err
	}
	log.file = f
	log.fileDay = now.Format("20060102")
	return nil
}

// pruneLocked 删除 7 天前的日志文件。
func pruneLocked(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "my-sub-go-") {
			continue
		}
		info, err := e.Info()
		if err == nil && info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func (l *logger) emit(module Module, level Level, format string, args ...any) {
	entry := Entry{
		Time:   time.Now(),
		Level:  level,
		Module: module,
		Text:   fmt.Sprintf(format, args...),
	}
	l.mu.Lock()
	if l.file != nil || l.dir != "" {
		if err := openFileLocked(entry.Time); err == nil && l.file != nil {
			_, _ = l.file.WriteString(entry.Line() + "\n")
		}
	}
	subs := append([]chan Entry(nil), l.subs[module]...)
	l.mu.Unlock()

	// 调试构建（有控制台）时同步输出，windowsgui 构建下写入是安全的空操作。
	switch level {
	case LevelError:
		fmt.Fprintln(os.Stderr, entry.Line())
	case LevelWarn:
		fmt.Fprintln(os.Stderr, entry.Line())
	default:
		fmt.Fprintln(os.Stdout, entry.Line())
	}

	for _, ch := range subs {
		select {
		case ch <- entry:
		default:
			// 订阅者来不及消费：丢弃本条，避免阻塞日志生产者。
		}
	}
}

// Info 记录一条信息日志。
func Info(module Module, format string, args ...any) {
	log.emit(module, LevelInfo, format, args...)
}

// Warn 记录一条警告日志。
func Warn(module Module, format string, args ...any) {
	log.emit(module, LevelWarn, format, args...)
}

// Error 记录一条错误日志。
func Error(module Module, format string, args ...any) {
	log.emit(module, LevelError, format, args...)
}

// Listen 订阅某模块的新日志，返回接收通道；用 Unlisten 退订。
func Listen(module Module) <-chan Entry {
	ch := make(chan Entry, subBuffer)
	log.mu.Lock()
	log.subs[module] = append(log.subs[module], ch)
	log.mu.Unlock()
	return ch
}

// Unlisten 退订模块日志。
func Unlisten(module Module, ch <-chan Entry) {
	log.mu.Lock()
	defer log.mu.Unlock()
	subs := log.subs[module]
	for i, c := range subs {
		if c == ch {
			log.subs[module] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}
