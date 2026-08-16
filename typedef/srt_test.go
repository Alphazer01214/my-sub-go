package typedef

import (
	"strings"
	"testing"
	"time"
)

const sampleSRT = `1
00:00:01,000 --> 00:00:03,500
Hello world

2
00:00:04,000 --> 00:00:06,000
Second line
Third line

3
00:00:07,200 --> 00:00:09,800
结尾无空行
`

func TestParseSRTBasic(t *testing.T) {
	sub, err := ParseSRT([]byte(sampleSRT))
	if err != nil {
		t.Fatalf("ParseSRT: %v", err)
	}
	if len(sub.Segments) != 3 {
		t.Fatalf("segments = %d, want 3", len(sub.Segments))
	}
	s0 := sub.Segments[0]
	if s0.Index != 1 || s0.StartTime != time.Second || s0.EndTime != 3500*time.Millisecond || s0.Text != "Hello world" {
		t.Fatalf("seg0 = %+v", s0)
	}
	if got := sub.Segments[1].Text; got != "Second line\nThird line" {
		t.Fatalf("multiline text = %q", got)
	}
	if got := sub.Segments[2].Text; got != "结尾无空行" {
		t.Fatalf("last text = %q", got)
	}
}

func TestParseSRTBOMCRLFAndDotMillis(t *testing.T) {
	data := "\uFEFF1\r\n00:00:00.500 --> 00:00:02.750\r\ntext\r\n"
	sub, err := ParseSRT([]byte(data))
	if err != nil {
		t.Fatalf("ParseSRT: %v", err)
	}
	if len(sub.Segments) != 1 {
		t.Fatalf("segments = %d, want 1", len(sub.Segments))
	}
	if sub.Segments[0].StartTime != 500*time.Millisecond || sub.Segments[0].EndTime != 2750*time.Millisecond {
		t.Fatalf("times = %v/%v", sub.Segments[0].StartTime, sub.Segments[0].EndTime)
	}
}

func TestParseSRTWithoutIndexLine(t *testing.T) {
	sub, err := ParseSRT([]byte("00:00:01,000 --> 00:00:02,000\nno index\n"))
	if err != nil {
		t.Fatalf("ParseSRT: %v", err)
	}
	if len(sub.Segments) != 1 || sub.Segments[0].Index != 1 || sub.Segments[0].Text != "no index" {
		t.Fatalf("sub = %+v", sub)
	}
}

func TestParseSRTRenumbersOutOfOrderIndexes(t *testing.T) {
	sub, err := ParseSRT([]byte("9\n00:00:01,000 --> 00:00:02,000\na\n\n1\n00:00:03,000 --> 00:00:04,000\nb\n"))
	if err != nil {
		t.Fatalf("ParseSRT: %v", err)
	}
	if len(sub.Segments) != 2 || sub.Segments[0].Index != 1 || sub.Segments[1].Index != 2 {
		t.Fatalf("indexes = %d,%d, want 1,2", sub.Segments[0].Index, sub.Segments[1].Index)
	}
}

func TestParseSRTBilingualTreatedAsMultilineText(t *testing.T) {
	sub, err := ParseSRT([]byte("1\n00:00:01,000 --> 00:00:02,000\nこんにちは\n你好\n"))
	if err != nil {
		t.Fatalf("ParseSRT: %v", err)
	}
	if got := sub.Segments[0].Text; got != "こんにちは\n你好" {
		t.Fatalf("text = %q", got)
	}
}

func TestParseSRTBadTimeline(t *testing.T) {
	_, err := ParseSRT([]byte("1\n00:00:01 --> 00:00:02\ntext\n"))
	if err == nil || !strings.Contains(err.Error(), "第 1 块") {
		t.Fatalf("err = %v, want block-numbered timeline error", err)
	}
}

func TestParseSRTEmpty(t *testing.T) {
	sub, err := ParseSRT([]byte(""))
	if err != nil || sub == nil || len(sub.Segments) != 0 {
		t.Fatalf("sub = %v, err = %v", sub, err)
	}
}

func TestSRTTimeParseVariants(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"00:00:01,500", 1500 * time.Millisecond},
		{"01:02:03.004", time.Hour + 2*time.Minute + 3*time.Second + 4*time.Millisecond},
		{"00:00:00,5", 500 * time.Millisecond}, // 1 位毫秒按 0.5s 解释
	}
	for _, c := range cases {
		got, _, err := parseTimingLine(c.in + " --> 00:00:00,000")
		if err != nil {
			t.Fatalf("%q: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("%q = %v, want %v", c.in, got, c.want)
		}
	}
}
