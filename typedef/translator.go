package typedef

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"my-sub-go/common/logx"

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
You MUST respond with a pure JSON array only, containing no additional text, explanations, or formatting. The array must contain EXACTLY {{.Count}} entries, one for each source segment, in the same order as the source content:
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
	cfg         *Config
	ctx         context.Context
	chatModel   chatModel
	callTimeout time.Duration // 单次 LLM 调用的超时时间
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
	t.callTimeout = time.Duration(cfg.LLMAPI.TimeoutSec) * time.Second
	if t.callTimeout <= 0 {
		t.callTimeout = 120 * time.Second
	}
	return nil
}

// logf 输出翻译模块日志。
func (t *TranslatorAPI) logf(format string, args ...any) {
	logx.Info(logx.ModuleTranslate, format, args...)
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

// TranslateOptions 一次翻译任务的参数；零值字段回退到全局配置（cfg.LLMAPI）。
// BaseURL/APIKey 始终取全局配置（连接信息不属于单次任务参数）。
type TranslateOptions struct {
	SrcLang        Lang
	TgtLang        Lang
	Provider       LLMProvider
	ModelName      string
	PromptTemplate string
	ProcessWindow  int
	RefWindow      int
}

func (t *TranslatorAPI) Translate(sub *Subtitle) (Subtitle, error) {
	return t.TranslateContext(context.Background(), sub, nil)
}

// TranslateContext 用全局配置翻译字幕；ctx 取消会中断后续窗口的调用，progress(percent, phase) 可为 nil。
func (t *TranslatorAPI) TranslateContext(ctx context.Context, sub *Subtitle, progress func(int, string)) (Subtitle, error) {
	return t.TranslateContextWithOptions(ctx, sub, TranslateOptions{}, progress, nil)
}

// TranslateContextWithOptions 翻译字幕，任务级参数 opts 覆盖全局配置。
// ctx 取消会中断后续窗口的调用，progress(percent, phase) 可为 nil。
// checkpoint 可为 nil：每完成一个窗口，用"原文 + 迄今已合并的译文"的快照回调一次，
// 供调用方实时落盘（部分失败/取消时已完成的翻译不丢失）。
// 返回的 Subtitle 在 err 非 nil 时仍包含已成功窗口的译文（失败句留空）。
func (t *TranslatorAPI) TranslateContextWithOptions(ctx context.Context, sub *Subtitle, opts TranslateOptions, progress func(int, string), checkpoint func(*Subtitle)) (Subtitle, error) {
	segs := sub.Segments
	t.logf("found %d segments", len(segs))

	srcLang := t.cfg.LLMAPI.SrcLang
	if opts.SrcLang != "" {
		srcLang = opts.SrcLang
	}
	tgtLang := t.cfg.LLMAPI.TgtLang
	if opts.TgtLang != "" {
		tgtLang = opts.TgtLang
	}
	if tgtLang == "" || tgtLang == LangAuto {
		return Subtitle{}, fmt.Errorf("翻译目标语言不能为空或 auto（当前: %q）", tgtLang)
	}
	processWindow := opts.ProcessWindow
	if processWindow <= 0 {
		processWindow = t.cfg.LLMAPI.ProcessWindow
	}
	if processWindow <= 0 {
		processWindow = 8
	}
	refWindow := opts.RefWindow
	if refWindow <= 0 {
		refWindow = t.cfg.LLMAPI.RefWindow
	}
	promptTemplate := opts.PromptTemplate
	if promptTemplate == "" {
		promptTemplate = t.cfg.LLMAPI.PromptTemplate
	}

	model := t.chatModel
	if opts.Provider != "" && (opts.Provider != t.cfg.LLMAPI.Provider || opts.ModelName != t.cfg.LLMAPI.ModelName) {
		// 任务级提供商/模型：用全局 BaseURL/APIKey 重建客户端，不影响全局配置。
		mcfg := t.cfg.LLMAPI
		mcfg.Provider = opts.Provider
		if opts.ModelName != "" {
			mcfg.ModelName = opts.ModelName
		}
		var err error
		if model, err = newChatModel(ctx, &mcfg); err != nil {
			return Subtitle{}, fmt.Errorf("[LLM] 任务级模型初始化失败: %w", err)
		}
		t.logf("使用任务级提供商 %s / 模型 %s", opts.Provider, opts.ModelName)
	}
	t.logf("start translating using %s", t.cfg.LLMAPI.ModelName)

	subRes := Subtitle{}
	var transRes []TranslateResult
	var errs []error
	for i := 0; i < len(segs); i += processWindow {
		if err := ctx.Err(); err != nil {
			return subRes, err
		}
		end := i + processWindow
		if end > len(segs) {
			end = len(segs)
		}
		toTranslate := segs[i:end]
		textCtx := windowContext(segs, i, processWindow, refWindow)
		trans, err := t.translateWindow(ctx, model, toTranslate, textCtx, srcLang, tgtLang, promptTemplate, 4)
		if err != nil {
			if ctx.Err() != nil {
				return subRes, ctx.Err()
			}
			logx.Error(logx.ModuleTranslate, "窗口 [%d:%d] 翻译失败: %v", i+1, end, err)
			errs = append(errs, fmt.Errorf("[LLM] window [%d:%d]: %w", i+1, end, err))
		}
		transRes = append(transRes, trans...)
		if checkpoint != nil {
			// 快照：原文 + 迄今已合并的译文（失败窗口对应句留空）
			snap := &Subtitle{}
			snap.Segments = append([]Segment(nil), segs...)
			t.merge(snap.Segments, transRes)
			checkpoint(snap)
		}
		if progress != nil && len(segs) > 0 {
			progress(int(float64(end)/float64(len(segs))*100), fmt.Sprintf("翻译中 %d/%d 句", end, len(segs)))
		}
	}
	t.merge(segs, transRes)
	subRes.Segments = segs
	// 汇总一次未获得译文的段数（失败窗口留空），不逐条刷日志
	missing := 0
	for _, s := range segs {
		if strings.TrimSpace(s.Text) != "" && strings.TrimSpace(s.Translation) == "" {
			missing++
		}
	}
	if missing > 0 {
		logx.Warn(logx.ModuleTranslate, "%d/%d 句未获得译文（失败窗口留空，可在表格中补译）", missing, len(segs))
	}
	if len(errs) > 0 {
		return subRes, errors.Join(errs...)
	}
	if progress != nil {
		progress(100, fmt.Sprintf("翻译完成 %d 句", len(segs)))
	}
	return subRes, nil
}

// translateWindow 翻译一个窗口，保证"输出条数与输入一致"：
//   - 数量一致 → 按位置对应（不信任模型返回的 index，模型经常写错）；
//   - 数量不符 → 先复用 index 匹配的条目，只对缺失部分递归；无可复用条目则拆半递归；
//   - 单句/深度耗尽 → 单句兜底，保证每条字幕都有翻译或占位。
func (t *TranslatorAPI) translateWindow(ctx context.Context, model chatModel, toTranslate []Segment, textCtx []Segment, srcLang, tgtLang Lang, promptTemplate string, depth int) ([]TranslateResult, error) {
	if len(toTranslate) == 0 {
		return nil, nil
	}
	trans, err := t.callWindow(ctx, model, toTranslate, textCtx, srcLang, tgtLang, promptTemplate)
	if err != nil {
		return appendPlaceholders(nil, toTranslate), err
	}
	if len(trans) == len(toTranslate) {
		out := make([]TranslateResult, len(toTranslate))
		for k, seg := range toTranslate {
			out[k] = TranslateResult{Index: seg.Index, Translation: trans[k].Translation}
		}
		return out, nil
	}
	if len(trans) == 0 {
		return appendPlaceholders(nil, toTranslate), fmt.Errorf("empty translation response")
	}
	if len(toTranslate) == 1 {
		return []TranslateResult{{Index: toTranslate[0].Index, Translation: trans[0].Translation}}, nil
	}
	if depth <= 0 {
		out := mapByIndex(toTranslate, trans)
		return out, fmt.Errorf("translation incomplete: got %d of %d", len(trans), len(toTranslate))
	}

	// 数量不符：复用 index 匹配的条目，只补缺失
	if out, err, ok := t.reuseAndFill(ctx, model, toTranslate, trans, textCtx, srcLang, tgtLang, promptTemplate, depth); ok {
		return out, err
	}

	// 无可复用条目：拆半递归
	logx.Warn(logx.ModuleTranslate, "翻译窗口返回 %d 条 ≠ 期望 %d 条，拆半重试", len(trans), len(toTranslate))
	mid := len(toTranslate) / 2
	left, errL := t.translateWindow(ctx, model, toTranslate[:mid], textCtx, srcLang, tgtLang, promptTemplate, depth-1)
	right, errR := t.translateWindow(ctx, model, toTranslate[mid:], textCtx, srcLang, tgtLang, promptTemplate, depth-1)
	return append(left, right...), errors.Join(errL, errR)
}

// callWindow 构造窗口提示词并调用一次 LLM。
func (t *TranslatorAPI) callWindow(ctx context.Context, model chatModel, toTranslate []Segment, textCtx []Segment, srcLang, tgtLang Lang, promptTemplate string) ([]TranslateResult, error) {
	prompt := t.buildPrompt(toTranslate, textCtx, srcLang, tgtLang, promptTemplate)
	return t.call(ctx, model, prompt, 2)
}

// reuseAndFill 复用已按 index 匹配的翻译，仅对缺失段递归翻译；
// 无可匹配条目时返回 ok=false，由调用方拆半。
func (t *TranslatorAPI) reuseAndFill(ctx context.Context, model chatModel, toTranslate []Segment, trans []TranslateResult, textCtx []Segment, srcLang, tgtLang Lang, promptTemplate string, depth int) ([]TranslateResult, error, bool) {
	byIdx := make(map[int]string, len(trans))
	for _, tr := range trans {
		if tr.Translation != "" {
			byIdx[tr.Index] = tr.Translation
		}
	}
	matched := 0
	var missing []Segment
	for _, seg := range toTranslate {
		if _, ok := byIdx[seg.Index]; ok {
			matched++
		} else {
			missing = append(missing, seg)
		}
	}
	if matched == 0 || len(missing) == 0 {
		return nil, nil, false
	}
	logx.Warn(logx.ModuleTranslate, "复用 %d/%d 条已匹配翻译，补齐缺失 %d 条", matched, len(toTranslate), len(missing))
	sub, err := t.translateWindow(ctx, model, missing, textCtx, srcLang, tgtLang, promptTemplate, depth-1)
	subMap := make(map[int]string, len(sub))
	for _, s := range sub {
		subMap[s.Index] = s.Translation
	}
	out := make([]TranslateResult, 0, len(toTranslate))
	for _, seg := range toTranslate {
		if v, ok := byIdx[seg.Index]; ok {
			out = append(out, TranslateResult{Index: seg.Index, Translation: v})
		} else {
			out = append(out, TranslateResult{Index: seg.Index, Translation: subMap[seg.Index]})
		}
	}
	return out, err, true
}

// mapByIndex 兜底：按 index 匹配，缺失补占位。
func mapByIndex(toTranslate []Segment, trans []TranslateResult) []TranslateResult {
	byIdx := make(map[int]string, len(trans))
	for _, tr := range trans {
		byIdx[tr.Index] = tr.Translation
	}
	out := make([]TranslateResult, 0, len(toTranslate))
	for _, seg := range toTranslate {
		out = append(out, TranslateResult{Index: seg.Index, Translation: byIdx[seg.Index]})
	}
	return out
}

func (t *TranslatorAPI) buildPrompt(toTranslate []Segment, ctx []Segment, src Lang, tgt Lang, up string) string {
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

	prompt := strings.NewReplacer("{{.SrcLang}}", string(src),
		"{{.TgtLang}}", string(tgt),
		"{{.Context}}", ctxBuilder.String(),
		"{{.ToTranslate}}", transBuilder.String(),
		"{{.Count}}", fmt.Sprintf("%d", len(toTranslate)),
		"{{.UserPrompt}}", up).Replace(template)
	return prompt
}

// call 以单一实现覆盖所有 provider：重试 + 解析逻辑只存在一份。
// 每次尝试都有独立的 callTimeout（否则第一次超时后重试会立即"aborted"而形同虚设）；
// 重试间隔可被任务 ctx 取消打断。
func (t *TranslatorAPI) call(ctx context.Context, model chatModel, prompt string, retry int) ([]TranslateResult, error) {
	message := []*schema.Message{
		schema.SystemMessage("You are an expert API, only response valid pure JSON format."),
		schema.UserMessage(prompt),
	}
	t.logf("calling llm (prompt %.200s)", prompt)

	var lastErr error
	for i := 0; i < retry; i++ {
		attemptCtx := ctx
		var cancel context.CancelFunc
		if t.callTimeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, t.callTimeout)
		}
		response, err := model.Generate(attemptCtx, message)
		if cancel != nil {
			cancel()
		}
		if err != nil {
			lastErr = err
			t.logf("llm 调用失败（第 %d/%d 次）: %v", i+1, retry, err)
			if !t.retryBackoff(ctx) {
				return nil, fmt.Errorf("[LLM] call aborted: %w", ctx.Err())
			}
			continue
		}
		t.logf("llm response: %.500s", response.Content)
		res, err := parseTranslation(response.Content)
		if err != nil {
			lastErr = err
			t.logf("llm 响应解析失败（第 %d/%d 次）: %v", i+1, retry, err)
			if !t.retryBackoff(ctx) {
				return nil, fmt.Errorf("[LLM] call aborted: %w", ctx.Err())
			}
			continue
		}
		return res, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no attempts completed")
	}
	return nil, fmt.Errorf("[LLM] call failed after %d retries: %w", retry, lastErr)
}

// retryBackoff 等待 2 秒或直到 ctx 取消；ctx 取消时返回 false。
func (t *TranslatorAPI) retryBackoff(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(2 * time.Second):
		return true
	}
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

// merge 把翻译结果按 index 合并回字幕段；缺失的段保持空译文。
// 注意：本函数被每窗口的 checkpoint 快照频繁调用（此时后面窗口尚未翻译，
// "缺失"是常态而非错误），因此不逐条打日志；整体缺失数由调用方汇总报告。
func (t *TranslatorAPI) merge(segs []Segment, trans []TranslateResult) {
	transMap := make(map[int]string, len(trans))
	for _, tr := range trans {
		transMap[tr.Index] = tr.Translation
	}
	for i := range segs {
		if tr, ok := transMap[segs[i].Index]; ok {
			segs[i].Translation = tr
		}
	}
}
