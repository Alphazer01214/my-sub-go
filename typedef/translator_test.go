package typedef

import (
	"context"
	"strings"
	"testing"

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
	p := tl.buildPrompt(toTranslate, ctx, "en", "zh", "keep the tone")
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
