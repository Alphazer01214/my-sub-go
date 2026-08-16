package typedef

import (
	"fmt"
	"my-sub-go/common"
	"os"
	"strings"
	"time"
)

type Segment struct {
	Index       int
	StartTime   time.Duration
	EndTime     time.Duration
	Text        string
	Translation string
}

type Subtitle struct {
	Segments []Segment
}

func (s *Segment) SRTSegment() string {
	idx := s.Index
	duration := common.SRTTimeToString(s.StartTime) + " --> " + common.SRTTimeToString(s.EndTime)
	text := s.Text
	if s.Translation != "" {
		text += "\n" + s.Translation
	}

	return fmt.Sprintf("%d\n%s\n%s\n\n", idx, duration, text)
}

func (s *Subtitle) AddSegment(start time.Duration, end time.Duration, text string) {
	seg := Segment{
		Index:     len(s.Segments) + 1,
		StartTime: start,
		EndTime:   end,
		Text:      strings.TrimSpace(text),
	}

	s.Segments = append(s.Segments, seg)
}

// Clone 返回字幕快照（段切片浅拷贝，段字段均按不可变语义使用）。
// 供实时保存（checkpoint）回调使用，避免与转录/翻译工作 goroutine 竞争同一份数据。
func (s *Subtitle) Clone() *Subtitle {
	out := &Subtitle{Segments: make([]Segment, len(s.Segments))}
	copy(out.Segments, s.Segments)
	return out
}

func (s *Subtitle) SRT() string {
	var result strings.Builder

	for _, segment := range s.Segments {
		result.WriteString(segment.SRTSegment())
	}

	return result.String()
}

func (s *Subtitle) SaveToFile(path string) error {
	content := s.SRT()
	return os.WriteFile(path, []byte(content), 0644)
}
