package typedef

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// fakeChatModel 是 chatModel 的测试替身，不发起任何网络请求。
type fakeChatModel struct {
	resp string
	err  error
}

func (f fakeChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &schema.Message{Content: f.resp}, nil
}

// countingErrModel 每次调用都返回 err，并统计调用次数（测试重试是否真正发生）。
type countingErrModel struct {
	err    error
	calls  int
	onCall func()
}

func (m *countingErrModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	m.calls++
	if m.onCall != nil {
		m.onCall()
	}
	return nil, m.err
}

// scriptedModel 按脚本依次返回响应，用于测试多轮调用的窗口逻辑。
type scriptedModel struct {
	responses []string
	prompts   []string
	calls     int
}

func (s *scriptedModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if s.calls >= len(s.responses) {
		return nil, fmt.Errorf("no more scripted responses (call %d)", s.calls+1)
	}
	resp := s.responses[s.calls]
	for _, m := range input {
		if m.Role == schema.User {
			s.prompts = append(s.prompts, m.Content)
		}
	}
	s.calls++
	return &schema.Message{Content: resp}, nil
}

func newTestTranslator(m chatModel) *TranslatorAPI {
	return &TranslatorAPI{
		cfg:       &Config{LLMAPI: LLMAPIConfig{SrcLang: LangEn, TgtLang: LangZh, ProcessWindow: 8}},
		chatModel: m,
	}
}

func makeSegs(n int) []Segment {
	segs := make([]Segment, n)
	for i := range segs {
		segs[i] = Segment{Index: i + 1, Text: fmt.Sprintf("t%d", i+1)}
	}
	return segs
}

// 数量一致但 index 写错：必须按位置对应，不信任模型返回的 index。
func TestTranslateWindowPositionalMapping(t *testing.T) {
	tl := newTestTranslator(fakeChatModel{
		resp: `[{"index":7,"translation":"A"},{"index":7,"translation":"B"},{"index":7,"translation":"C"}]`,
	})
	segs := makeSegs(3)
	got, err := tl.translateWindow(context.Background(), tl.chatModel, segs, nil, LangEn, LangZh, "", 4)
	if err != nil {
		t.Fatalf("translateWindow() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("translateWindow() = %d entries, want 3", len(got))
	}
	for i, want := range []string{"A", "B", "C"} {
		if got[i].Index != i+1 || got[i].Translation != want {
			t.Fatalf("entry %d = %+v, want index %d translation %q", i, got[i], i+1, want)
		}
	}
}

// 模型每轮只返回 1 条且 index 正确：复用已匹配条目、递归补齐缺失。
func TestTranslateWindowReuseAndFill(t *testing.T) {
	model := &scriptedModel{responses: []string{
		`[{"index":1,"translation":"A"}]`, // 窗口 [1,2,3] 只返回 1 条
		`[{"index":2,"translation":"B"}]`, // 缺失 [2,3] 又只返回 1 条
		`[{"index":3,"translation":"C"}]`, // 单句兜底
	}}
	tl := newTestTranslator(model)
	segs := makeSegs(3)
	got, err := tl.translateWindow(context.Background(), tl.chatModel, segs, nil, LangEn, LangZh, "", 4)
	if err != nil {
		t.Fatalf("translateWindow() error = %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("translateWindow() = %d entries, want 3", len(got))
	}
	for i, want := range []string{"A", "B", "C"} {
		if got[i].Index != i+1 || got[i].Translation != want {
			t.Fatalf("entry %d = %+v, want index %d translation %q", i, got[i], i+1, want)
		}
	}
	if model.calls != 3 {
		t.Fatalf("model calls = %d, want 3", model.calls)
	}
}

// 模型返回的 index 全部对不上：拆半递归到单句，每句都有翻译。
func TestTranslateWindowSplitOnUnmatchedIndexes(t *testing.T) {
	model := &scriptedModel{responses: []string{
		`[{"index":99,"translation":"X"}]`, // 无匹配 → 拆半
		`[{"index":99,"translation":"A"}]`, // 单句 [1]
		`[{"index":99,"translation":"B"}]`, // 单句 [2]
	}}
	tl := newTestTranslator(model)
	segs := makeSegs(2)
	got, err := tl.translateWindow(context.Background(), tl.chatModel, segs, nil, LangEn, LangZh, "", 4)
	if err != nil {
		t.Fatalf("translateWindow() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("translateWindow() = %d entries, want 2", len(got))
	}
	for i, want := range []string{"A", "B"} {
		if got[i].Index != i+1 || got[i].Translation != want {
			t.Fatalf("entry %d = %+v, want index %d translation %q", i, got[i], i+1, want)
		}
	}
}

// 单句窗口：无论模型返回什么 index，都取第一条并强制使用目标段 index。
func TestTranslateWindowSingleSegment(t *testing.T) {
	tl := newTestTranslator(fakeChatModel{resp: `[{"index":5,"translation":"你好"}]`})
	segs := makeSegs(1)
	got, err := tl.translateWindow(context.Background(), tl.chatModel, segs, nil, LangEn, LangZh, "", 4)
	if err != nil {
		t.Fatalf("translateWindow() error = %v", err)
	}
	if len(got) != 1 || got[0].Index != 1 || got[0].Translation != "你好" {
		t.Fatalf("translateWindow() = %+v", got)
	}
}

func TestParseTranslation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []TranslateResult
		wantErr bool
	}{
		{"plain json", `[{"index":1,"translation":"hello"}]`, []TranslateResult{{Index: 1, Translation: "hello"}}, false},
		{"fenced json", "```json\n[{\"index\":2,\"translation\":\"你好\"}]\n```", []TranslateResult{{Index: 2, Translation: "你好"}}, false},
		{"empty array", `[]`, []TranslateResult{}, false},
		{"not json", `sorry, I can't do that`, nil, true},
		{"object not array", `{"index":1}`, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTranslation(tt.content)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTranslation(%q) error = %v, wantErr %v", tt.content, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("parseTranslation(%q) = %+v, want %+v", tt.content, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("parseTranslation(%q) = %+v, want %+v", tt.content, got, tt.want)
				}
			}
		})
	}
}

func TestWindowContext(t *testing.T) {
	segs := make([]Segment, 10)
	for i := range segs {
		segs[i] = Segment{Index: i + 1}
	}
	indexes := func(segments []Segment) []int {
		out := make([]int, len(segments))
		for i, s := range segments {
			out[i] = s.Index
		}
		return out
	}
	equal := func(a, b []int) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}

	tests := []struct {
		name        string
		i, pw, rw   int
		wantIndexes []int
	}{
		{"start window gets next only", 0, 8, 2, []int{9, 10}},
		{"middle window gets prev and next", 8, 1, 2, []int{7, 8, 10}},
		{"end window gets prev only", 8, 2, 5, []int{4, 5, 6, 7, 8}},
		{"refWindow zero at start, no panic", 0, 8, 0, nil},
		{"refWindow zero mid-window, no panic", 8, 1, 0, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := indexes(windowContext(segs, tt.i, tt.pw, tt.rw))
			if !equal(got, tt.wantIndexes) {
				t.Fatalf("windowContext(segs, %d, %d, %d) = %v, want %v", tt.i, tt.pw, tt.rw, got, tt.wantIndexes)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	segs := []Segment{{Index: 1, Text: "a"}, {Index: 2, Text: "b"}, {Index: 3, Text: "c"}}
	trans := []TranslateResult{{Index: 1, Translation: "A"}, {Index: 3, Translation: "C"}}
	(&TranslatorAPI{}).merge(segs, trans)
	if segs[0].Translation != "A" || segs[1].Translation != "" || segs[2].Translation != "C" {
		t.Fatalf("merge() segments = %+v", segs)
	}
}

func TestBuildPrompt(t *testing.T) {
	tl := &TranslatorAPI{}
	ctx := []Segment{{Index: 1, Text: "one", Translation: "一"}}
	toTranslate := []Segment{{Index: 2, Text: "two"}}
	p := tl.buildPrompt(toTranslate, ctx, LangEn, LangZh, "keep the tone")
	if strings.Contains(p, "{{.") {
		t.Fatalf("buildPrompt: unreplaced placeholders in:\n%s", p)
	}
	for _, want := range []string{"[1] one, 一", `{index: 2, text: "two"}`, "keep the tone", "en", "zh"} {
		if !strings.Contains(p, want) {
			t.Fatalf("buildPrompt: missing %q in:\n%s", want, p)
		}
	}
}

func TestCallParsesFencedJSON(t *testing.T) {
	tl := &TranslatorAPI{}
	fake := fakeChatModel{resp: "```json\n[{\"index\":1,\"translation\":\"你好\"}]\n```"}
	got, err := tl.call(context.Background(), fake, "prompt", 1)
	if err != nil {
		t.Fatalf("call() error = %v", err)
	}
	if len(got) != 1 || got[0].Index != 1 || got[0].Translation != "你好" {
		t.Fatalf("call() = %+v", got)
	}
}

func TestNewChatModelUnknownProvider(t *testing.T) {
	if _, err := newChatModel(context.Background(), &LLMAPIConfig{Provider: "bogus"}); err == nil {
		t.Fatal("newChatModel: expected error for unknown provider")
	}
}

func TestConfigValidate(t *testing.T) {
	base := func() *Config {
		return &Config{
			Whisper: WhisperConfig{SrcLang: LangAuto},
			Llama:   LlamaConfig{SrcLang: LangEn, TgtLang: LangZh},
			LLMAPI:  LLMAPIConfig{SrcLang: LangJa, TgtLang: LangZh},
		}
	}

	tests := []struct {
		name    string
		mutate  func(c *Config)
		wantErr bool
	}{
		{"valid config", func(c *Config) {}, false},
		{"empty langs allowed", func(c *Config) {
			c.LLMAPI.SrcLang = ""
			c.LLMAPI.TgtLang = ""
		}, false},
		{"auto source allowed", func(c *Config) { c.Whisper.SrcLang = LangAuto }, false},
		{"auto target rejected", func(c *Config) { c.LLMAPI.TgtLang = LangAuto }, true},
		{"unknown lang rejected", func(c *Config) { c.Llama.TgtLang = "xx" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := base()
			tt.mutate(c)
			err := c.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// 任务级目标语言覆盖全局配置，且窗口/参考窗口随任务参数生效。
func TestTranslateContextWithOptionsOverrides(t *testing.T) {
	model := &scriptedModel{responses: []string{
		`[{"index":1,"translation":"A"},{"index":2,"translation":"B"}]`, // 窗口 [1,2]（ProcessWindow=2）
		`[{"index":3,"translation":"C"}]`,                              // 窗口 [3]
	}}
	tl := newTestTranslator(model)
	sub := &Subtitle{Segments: makeSegs(3)}
	got, err := tl.TranslateContextWithOptions(context.Background(), sub,
		TranslateOptions{SrcLang: LangJa, TgtLang: LangKo, ProcessWindow: 2, RefWindow: 1}, nil, nil)
	if err != nil {
		t.Fatalf("TranslateContextWithOptions() error = %v", err)
	}
	if len(got.Segments) != 3 {
		t.Fatalf("segments = %d, want 3", len(got.Segments))
	}
	for i, want := range []string{"A", "B", "C"} {
		if got.Segments[i].Translation != want {
			t.Fatalf("seg %d translation = %q, want %q", i, got.Segments[i].Translation, want)
		}
	}
	if model.calls != 2 {
		t.Fatalf("model calls = %d, want 2 (ProcessWindow=2 → 两个窗口)", model.calls)
	}
	if len(model.prompts) != 2 || !strings.Contains(model.prompts[0], "from ja to ko") {
		t.Fatalf("prompt did not carry task-level ja→ko override: %d prompts", len(model.prompts))
	}
}

// checkpoint 每窗口回调一次快照：段数逐步增长，译文只含已完成窗口。
func TestTranslateContextWithOptionsCheckpoint(t *testing.T) {
	model := &scriptedModel{responses: []string{
		`[{"index":1,"translation":"A"},{"index":2,"translation":"B"}]`,
		`[{"index":3,"translation":"C"},{"index":4,"translation":"D"}]`,
	}}
	tl := newTestTranslator(model)
	sub := &Subtitle{Segments: makeSegs(4)}
	var snapshots []*Subtitle
	_, err := tl.TranslateContextWithOptions(context.Background(), sub,
		TranslateOptions{ProcessWindow: 2}, nil, func(s *Subtitle) {
			snapshots = append(snapshots, s)
		})
	if err != nil {
		t.Fatalf("TranslateContextWithOptions() error = %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots = %d, want 2 (每窗口一次)", len(snapshots))
	}
	if len(snapshots[0].Segments) != 4 || len(snapshots[1].Segments) != 4 {
		t.Fatalf("snapshot segment counts = %d/%d, want 4/4（快照始终是全量字幕）",
			len(snapshots[0].Segments), len(snapshots[1].Segments))
	}
	// 第一个快照：只有前两句有译文，后两句为空
	if snapshots[0].Segments[0].Translation != "A" || snapshots[0].Segments[2].Translation != "" {
		t.Fatalf("snapshot[0] translations = %+v", snapshots[0].Segments)
	}
	// 第二个快照：全部译完
	for i := range snapshots[1].Segments {
		if snapshots[1].Segments[i].Translation == "" {
			t.Fatalf("snapshot[1] segment %d translation empty", i)
		}
	}
	// 快照是副本：修改快照[0] 不影响快照[1]
	snapshots[0].Segments[0].Translation = "MUTATED"
	if snapshots[1].Segments[0].Translation != "A" {
		t.Fatal("snapshot should be a copy")
	}
}

// 部分失败：窗口 2 失败后，返回结果保留窗口 1 的译文、窗口 2 留空，且错误非 nil。
func TestTranslateContextWithOptionsPartialFailure(t *testing.T) {
	model := &scriptedModel{responses: []string{
		`[{"index":1,"translation":"A"},{"index":2,"translation":"B"}]`,
		"not json", // 窗口 2 连续两次解析失败
		"not json",
	}}
	tl := newTestTranslator(model)
	sub := &Subtitle{Segments: makeSegs(4)}
	got, err := tl.TranslateContextWithOptions(context.Background(), sub,
		TranslateOptions{ProcessWindow: 2}, nil, nil)
	if err == nil {
		t.Fatal("expected error for failed window")
	}
	if got.Segments[0].Translation != "A" || got.Segments[1].Translation != "B" {
		t.Fatalf("window 1 translations lost: %+v", got.Segments[:2])
	}
	if got.Segments[2].Translation != "" || got.Segments[3].Translation != "" {
		t.Fatalf("failed window should be empty: %+v", got.Segments[2:])
	}
}

// 任务级目标语言不允许为 auto（无论全局配置如何）。
func TestTranslateContextWithOptionsRejectsAutoTarget(t *testing.T) {
	tl := newTestTranslator(fakeChatModel{resp: `[]`})
	sub := &Subtitle{Segments: makeSegs(1)}
	_, err := tl.TranslateContextWithOptions(context.Background(), sub, TranslateOptions{TgtLang: LangAuto}, nil, nil)
	if err == nil {
		t.Fatal("expected error for auto target language")
	}
}

// call 的重试必须真正发生：第一次调用返回超时错误后，等待退避并用独立超时上下文重试第二次。
func TestCallRetriesAfterTimeout(t *testing.T) {
	attempts := 0
	m := &countingErrModel{err: context.DeadlineExceeded, onCall: func() { attempts++ }}
	tl := newTestTranslator(m)
	start := time.Now()
	_, err := tl.call(context.Background(), m, "prompt", 2)
	if err == nil {
		t.Fatal("call() should fail after 2 attempts")
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2（超时后必须真正重试）", attempts)
	}
	if time.Since(start) < 2*time.Second {
		t.Fatalf("retried too fast (%v)：退避等待失效，说明重试被超时上下文短路", time.Since(start))
	}
}

// 任务 ctx 已取消时立即中止，不做无谓退避等待。
func TestCallAbortsImmediatelyWhenCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tl := newTestTranslator(fakeChatModel{err: context.Canceled})
	start := time.Now()
	_, err := tl.call(ctx, fakeChatModel{err: context.Canceled}, "prompt", 2)
	if err == nil {
		t.Fatal("call() should abort")
	}
	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("abort took %v, want immediate", time.Since(start))
	}
}
