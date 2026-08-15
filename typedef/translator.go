package typedef

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/ollama/api"
)

const defaultPrompt = ` 
You are an expert subtitle translator specializing in accurate and natural translations. Your task is to translate the following subtitles from {{.SrcLang}} to {{.TgtLang}}.

# Context Information
The following context provides background information and references to help you understand the content better. DO NOT translate this section:
{{.Context}}

# Source Content
Below are the subtitle segments requiring translation. Each segment has a unique index number that must be preserved:
{{.ToTranslate}}

# Translation Guidelines
- Provide simple yet accurate translations that maintain the original meaning and tone
- Ensure translations sound natural in the target language
- Preserve the exact index numbers when mapping translations to source segments
- Adapt culturally specific references appropriately while maintaining clarity
- Keep subtitle length reasonable for reading speed when applicable

# Output Format Requirements
You MUST respond with a pure JSON array only, containing no additional text, explanations, or formatting. The array must follow this exact structure:
[{"index": index, "translation": "translated text"}, ...]

# User Instructions
{{.UserPrompt}}

# Output Example
[{"index": 1, "translation": "Hello world"}, {"index": 2, "translation": "It's MyGO!!!!!"}]
`

// chatModel 是 TranslatorAPI 依赖的最小接口：openai 与 ollama 的 ChatModel 都满足它，
// 测试中也可以注入 fake 实现。
type chatModel interface {
	Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error)
}

type TranslatorAPI struct {
	cfg       *Config
	ctx       context.Context
	chatModel chatModel
}

type TranslateResult struct {
	Index       int    `json:"index"`
	Translation string `json:"translation"`
}

func NewTranslatorAPI(cfg *Config) (*TranslatorAPI, error) {
	t := &TranslatorAPI{}
	if err := t.Init(cfg); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *TranslatorAPI) Init(cfg *Config) error {
	t.cfg = cfg
	t.ctx = context.Background()
	m, err := newChatModel(t.ctx, &cfg.LLMAPI)
	if err != nil {
		return fmt.Errorf("[LLM] can't init model: %w", err)
	}
	t.chatModel = m
	return nil
}

// newChatModel 按 provider 构造对应的 ChatModel；未知 provider 立即报错而不是延迟到调用期。
// qwen/claude 通过 OpenAI 兼容接口接入。
func newChatModel(ctx context.Context, cfg *LLMAPIConfig) (chatModel, error) {
	switch cfg.Provider {
	case LLMProviderOpenAI, LLMProviderDeepSeek, LLMProviderQwen, LLMProviderClaude:
		return openai.NewChatModel(ctx, &openai.ChatModelConfig{
			BaseURL: cfg.BaseURL,
			APIKey:  cfg.APIKey,
			Model:   cfg.ModelName,
		})
	case LLMProviderOllama:
		return ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
			BaseURL:  cfg.BaseURL,
			Model:    cfg.ModelName,
			Thinking: &api.ThinkValue{Value: false},
		})
	default:
		return nil, fmt.Errorf("unsupported provider %q", cfg.Provider)
	}
}

func (t *TranslatorAPI) Translate(sub *Subtitle) (Subtitle, error) {
	segs := sub.Segments
	fmt.Printf("[llm] found %d segments\n", len(segs))
	var transRes []TranslateResult
	subRes := Subtitle{}
	processWindow := t.cfg.LLMAPI.ProcessWindow
	refWindow := t.cfg.LLMAPI.RefWindow
	userPrompt := t.cfg.LLMAPI.PromptTemplate
	srcLang := t.cfg.LLMAPI.SrcLang
	tgtLang := t.cfg.LLMAPI.TgtLang
	fmt.Printf("[llm] start translating using %s \n", t.cfg.LLMAPI.ModelName)

	var errs []error
	for i := 0; i < len(segs); i += processWindow {
		end := i + processWindow
		if end > len(segs) {
			end = len(segs)
		}
		toTranslate := segs[i:end]
		textCtx := windowContext(segs, i, processWindow, refWindow)
		prompt := t.buildPrompt(toTranslate, textCtx, srcLang, tgtLang, userPrompt)
		trans, err := t.call(t.ctx, t.chatModel, prompt, 3)
		if err != nil {
			errs = append(errs, fmt.Errorf("[LLM] window [%d:%d] failed: %w", i, end, err))
			transRes = appendPlaceholders(transRes, toTranslate)
			continue
		}
		if len(trans) == 0 {
			fmt.Printf("[llm] empty response\n")
			errs = append(errs, fmt.Errorf("[LLM] window [%d:%d] returned empty response", i, end))
			transRes = appendPlaceholders(transRes, toTranslate)
			continue
		}
		fmt.Printf("[llm] translating %d segments, example:[%d] %.23s \n", len(trans), trans[0].Index, trans[0].Translation)
		transRes = append(transRes, trans...)
	}
	t.merge(segs, transRes)
	subRes.Segments = segs
	if len(errs) > 0 {
		return subRes, errors.Join(errs...)
	}
	return subRes, nil
}

func (t *TranslatorAPI) buildPrompt(toTranslate []Segment, ctx []Segment, src string, tgt string, up string) string {
	template := defaultPrompt
	var ctxBuilder strings.Builder
	for _, segment := range ctx {
		if segment.Translation != "" {
			ctxBuilder.WriteString(fmt.Sprintf("[%d] %s, %s\n", segment.Index, segment.Text, segment.Translation))
		} else {
			ctxBuilder.WriteString(fmt.Sprintf("[%d] %s\n", segment.Index, segment.Text))
		}
	}

	var transBuilder strings.Builder
	for _, segment := range toTranslate {
		transBuilder.WriteString(fmt.Sprintf("{index: %d, text: \"%s\"}\n", segment.Index, segment.Text))
	}

	prompt := strings.NewReplacer("{{.SrcLang}}", src,
		"{{.TgtLang}}", tgt,
		"{{.Context}}", ctxBuilder.String(),
		"{{.ToTranslate}}", transBuilder.String(),
		"{{.UserPrompt}}", up).Replace(template)
	return prompt
}

// call 以单一实现覆盖所有 provider：重试 + 解析逻辑只存在一份。
func (t *TranslatorAPI) call(ctx context.Context, model chatModel, prompt string, retry int) ([]TranslateResult, error) {
	message := []*schema.Message{
		schema.SystemMessage("You are an expert API, only response valid pure JSON format."),
		schema.UserMessage(prompt),
	}
	fmt.Printf("[llm] calling llm: %s\n", prompt)

	var lastErr error
	for i := 0; i < retry; i++ {
		response, err := model.Generate(ctx, message)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		fmt.Printf("[llm] llm response: %s", response.Content)
		res, err := parseTranslation(response.Content)
		if err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		return res, nil
	}
	return nil, fmt.Errorf("[LLM] call failed after %d retries: %w", retry, lastErr)
}

// parseTranslation 把 LLM 返回内容解析为翻译结果，容忍 ```json 代码围栏。
func parseTranslation(content string) ([]TranslateResult, error) {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimSuffix(strings.TrimPrefix(content, "```json"), "```")
		content = strings.TrimSpace(content)
	}
	var res []TranslateResult
	if err := json.Unmarshal([]byte(content), &res); err != nil {
		return nil, err
	}
	return res, nil
}

// windowContext 收集窗口 i 的上下文：前 refWindow 句 + 后 refWindow 句；refWindow <= 0 时为空。
func windowContext(segs []Segment, i, processWindow, refWindow int) []Segment {
	if refWindow <= 0 {
		return nil
	}
	var ctx []Segment
	start := i - refWindow
	if start < 0 {
		start = 0
	}
	ctx = append(ctx, segs[start:i]...)
	if nextStart := i + processWindow; nextStart < len(segs) {
		nextEnd := nextStart + refWindow
		if nextEnd > len(segs) {
			nextEnd = len(segs)
		}
		ctx = append(ctx, segs[nextStart:nextEnd]...)
	}
	return ctx
}

// appendPlaceholders 为失败的窗口补上索引占位，保证输出与输入片段一一对应。
func appendPlaceholders(res []TranslateResult, segs []Segment) []TranslateResult {
	for _, segment := range segs {
		res = append(res, TranslateResult{Index: segment.Index, Translation: ""})
	}
	return res
}

func (t *TranslatorAPI) merge(segs []Segment, trans []TranslateResult) {
	transMap := make(map[int]string, len(trans))
	for _, tr := range trans {
		transMap[tr.Index] = tr.Translation
	}
	for i := range segs {
		if tr, ok := transMap[segs[i].Index]; ok {
			segs[i].Translation = tr
		} else {
			fmt.Printf("[LLM] can't find translation for segment %d, content: %s", segs[i].Index, segs[i].Text)
		}
	}
}
