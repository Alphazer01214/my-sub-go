package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"my-sub-go/common"
	"my-sub-go/typedef"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
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

type TranslatorAPI struct {
	cfg       common.LLMAPIConfig
	ctx       context.Context
	chatModel *openai.ChatModel
}

type TranslateResult struct {
	Index       int    `json:"index"`
	Translation string `json:"translation"`
}

func (t *TranslatorAPI) Init(cfg common.LLMAPIConfig) error {
	var err error
	t.cfg = cfg
	t.ctx = context.Background()
	t.chatModel, err = openai.NewChatModel(t.ctx, &openai.ChatModelConfig{
		BaseURL: t.cfg.BaseURL,
		APIKey:  t.cfg.APIKey,
		Model:   t.cfg.ModelName,
	})
	if err != nil {
		return fmt.Errorf("[LLM] can't init model: %v", err)
	}
	return nil
}

func (t *TranslatorAPI) Translate(sub typedef.Subtitle) (typedef.Subtitle, error) {
	// 	BaseURL        string `json:"base_url"`        // API基础URL
	//	ModelName      string `json:"model_name"`      // 模型名称
	//	APIKey         string `json:"api_key"`         // API密钥
	//	SrcLang        string `json:"src_lang"`        // 源语言
	//	TgtLang        string `json:"tgt_lang"`        // 目标语言
	//	PromptTemplate string `json:"prompt_template"` // 提示词模板
	//	RefWindow      int    `json:"ref_window"`      // 参考上下文大小
	//	ProcessWindow  int    `json:"process_window"`  // 一次翻译几句
	segs := sub.Segments
	fmt.Printf("[llm] found %d segments\n", len(segs))
	var transRes []TranslateResult
	subRes := typedef.Subtitle{}
	processWindow := t.cfg.ProcessWindow
	refWindow := t.cfg.RefWindow
	userPrompt := t.cfg.PromptTemplate
	srcLang := t.cfg.SrcLang
	tgtLang := t.cfg.TgtLang
	// model response:
	// { {index: xxx, translation: "xxx"}, ... }
	fmt.Printf("[llm] start translating using %s \n", t.cfg.ModelName)
	for i := 0; i < len(segs); i += processWindow {
		end := i + processWindow
		if end > len(segs) {
			end = len(segs)
		}
		toTranslate := segs[i:end]
		var textCtx []typedef.Segment
		if i > refWindow {
			prev := segs[i-refWindow : i-1]
			textCtx = append(textCtx, prev...)
		}
		if i < len(segs)-refWindow-processWindow {
			nxt := segs[i+processWindow : i+processWindow+refWindow]
			textCtx = append(textCtx, nxt...)
		}
		prompt := t.buildPrompt(toTranslate, textCtx, srcLang, tgtLang, userPrompt)
		trans, err := t.call(t.ctx, t.chatModel, prompt, 3)
		if err != nil {
			return subRes, err
		}
		if len(trans) == 0 {
			fmt.Printf("[llm] empty response\n")
			continue
		}
		fmt.Printf("[llm] translating %d segments, example:[%d] %.23s \n", len(trans), trans[0].Index, trans[0].Translation)
		transRes = append(transRes, trans...)
	}
	if err := t.merge(segs, transRes); err != nil {
		return subRes, err
	}
	subRes.Segments = segs

	return subRes, nil
}

func (t *TranslatorAPI) buildPrompt(toTranslate []typedef.Segment, ctx []typedef.Segment, src string, tgt string, up string) string {
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

func (t *TranslatorAPI) call(ctx context.Context, model *openai.ChatModel, prompt string, retry int) ([]TranslateResult, error) {
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

		var res []TranslateResult
		content := strings.TrimSpace(response.Content)
		fmt.Printf("[llm] llm response: %s", content)
		if strings.HasPrefix(content, "```json") {
			content = strings.TrimPrefix(content, "```json")
			content = strings.TrimSuffix(content, "```")
			content = strings.TrimSpace(content)
		}
		if err := json.Unmarshal([]byte(content), &res); err != nil {
			lastErr = err
			time.Sleep(2 * time.Second)
			continue
		}
		return res, nil
	}
	return nil, fmt.Errorf("[LLM] call failed after %d retries: %w", retry, lastErr)
}

func (t *TranslatorAPI) merge(segs []typedef.Segment, trans []TranslateResult) error {
	transMap := make(map[int]string)
	for _, t := range trans {
		transMap[t.Index] = t.Translation
	}
	for i, seg := range segs {
		if trans, ok := transMap[seg.Index]; ok {
			segs[i].Translation = trans
		} else {
			fmt.Printf("[LLM] can't find translation for segment %d, content: %s", seg.Index, seg.Text)
			continue
		}
	}
	return nil
}
