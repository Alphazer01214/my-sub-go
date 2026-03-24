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
