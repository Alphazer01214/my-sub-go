package typedef

import (
	"strings"
	"time"
)

// rawSegment 一块音频中 whisper 返回的段（块内相对时间，不含块偏移）。
type rawSegment struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

// mergeChunk 把一块的相对时间段并入累计字幕 sub：
//   - 给时间段加上块偏移 offset，得到绝对时间；
//   - 非最后一块只保留推进区（起点 < offset+step）内的段，重叠区交给下一块重新识别；
//   - 对绝对时间做幻觉重复守卫；
//
// 返回本块实际并入的段数。纯函数，便于单元测试分块拼接逻辑。
func mergeChunk(sub *Subtitle, raw []rawSegment, offset, step time.Duration, isLast bool) int {
	added := 0
	for _, seg := range raw {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}
		absStart := seg.Start + offset
		absEnd := seg.End + offset
		if !isLast && absStart >= offset+step {
			continue // 落在重叠区，交给下一块
		}
		cand := Segment{StartTime: absStart, EndTime: absEnd, Text: text}
		if isHallucination(sub, cand) {
			continue
		}
		sub.AddSegment(absStart, absEnd, text)
		added++
	}
	return added
}
