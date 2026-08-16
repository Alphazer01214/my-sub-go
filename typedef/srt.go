package typedef

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"my-sub-go/common"
)

// ParseSRTFile 读取并解析 SRT 文件。
func ParseSRTFile(path string) (*Subtitle, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseSRT(data)
}

// ParseSRT 解析标准 SRT 文本为字幕（序号按出现顺序重编，1 起）。
// 兼容：UTF-8 BOM、CRLF/LF 换行、块序号行可有可无、毫秒分隔符 "," 或 "."。
// 一条字幕的正文可多行，按换行合并为原文；对"原文+译文"两行的双语 SRT
// 不做猜测识别——按标准 SRT 规范它们都是正文。
func ParseSRT(data []byte) (*Subtitle, error) {
	content := strings.TrimPrefix(string(data), "\uFEFF")
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	content = strings.TrimSpace(content)
	if content == "" {
		return &Subtitle{}, nil
	}

	sub := &Subtitle{}
	for bi, block := range strings.Split(content, "\n\n") {
		lines := splitNonEmptyLines(block)
		if len(lines) == 0 {
			continue
		}
		i := 0
		if _, err := strconv.Atoi(strings.TrimSpace(lines[0])); err == nil {
			i++ // 跳过可选序号行
		}
		if i >= len(lines) {
			return nil, fmt.Errorf("SRT 第 %d 块缺少时间轴", bi+1)
		}
		start, end, err := parseTimingLine(strings.TrimSpace(lines[i]))
		if err != nil {
			return nil, fmt.Errorf("SRT 第 %d 块时间轴无效 %q: %w", bi+1, strings.TrimSpace(lines[i]), err)
		}
		i++
		var text string
		if i < len(lines) {
			text = strings.Join(lines[i:], "\n")
		}
		sub.AddSegment(start, end, text)
	}
	return sub, nil
}

// parseTimingLine 解析 "HH:MM:SS,mmm --> HH:MM:SS,mmm"。
func parseTimingLine(line string) (time.Duration, time.Duration, error) {
	parts := strings.Split(line, "-->")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("缺少 --> 分隔符")
	}
	start, err := common.SRTTimeParse(parts[0])
	if err != nil {
		return 0, 0, err
	}
	end, err := common.SRTTimeParse(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

// splitNonEmptyLines 按行切分并去掉空行、行首尾空白。
func splitNonEmptyLines(s string) []string {
	raw := strings.Split(s, "\n")
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}
