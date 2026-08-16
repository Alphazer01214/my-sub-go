package typedef

import (
	"testing"
	"time"
)

func TestMergeChunkAddsAbsoluteOffset(t *testing.T) {
	sub := &Subtitle{}
	raw := []rawSegment{
		{Start: 2 * time.Second, End: 4 * time.Second, Text: "hello"},
	}
	added := mergeChunk(sub, raw, 15*time.Second, 15*time.Second, false)
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	seg := sub.Segments[0]
	if seg.StartTime != 17*time.Second || seg.EndTime != 19*time.Second {
		t.Fatalf("absolute times = %v/%v, want 17s/19s", seg.StartTime, seg.EndTime)
	}
}

func TestMergeChunkDropsOverlapTailForNonLast(t *testing.T) {
	sub := &Subtitle{}
	raw := []rawSegment{
		{Start: 14 * time.Second, End: 15 * time.Second, Text: "keep"},
		{Start: 15 * time.Second, End: 16 * time.Second, Text: "drop at boundary"},
		{Start: 17 * time.Second, End: 18 * time.Second, Text: "drop"},
	}
	// offset 0s，推进区 15s：起点 >= 15s 的段留给下一块
	added := mergeChunk(sub, raw, 0, 15*time.Second, false)
	if added != 1 || len(sub.Segments) != 1 {
		t.Fatalf("added = %d, len = %d, want 1/1", added, len(sub.Segments))
	}
	if sub.Segments[0].Text != "keep" {
		t.Fatalf("kept segment = %q, want keep", sub.Segments[0].Text)
	}
}

func TestMergeChunkLastKeepsEverything(t *testing.T) {
	sub := &Subtitle{}
	raw := []rawSegment{
		{Start: 16 * time.Second, End: 18 * time.Second, Text: "tail"},
	}
	added := mergeChunk(sub, raw, 30*time.Second, 15*time.Second, true)
	if added != 1 {
		t.Fatalf("added = %d, want 1", added)
	}
	if sub.Segments[0].StartTime != 46*time.Second {
		t.Fatalf("start = %v, want 46s", sub.Segments[0].StartTime)
	}
}

func TestMergeChunkDedupAcrossChunkBoundary(t *testing.T) {
	sub := &Subtitle{}
	// 块 0 已经有一条 13.5s-16s 的段（推进区内的部分被保留）
	sub.AddSegment(13500*time.Millisecond, 16000*time.Millisecond, "same text")
	// 块 1（offset 15s）在重叠区重新识别出同一句：相对 0s-1s → 绝对 15s-16s，应被幻觉守卫丢弃
	raw := []rawSegment{
		{Start: 0, End: 1 * time.Second, Text: "same text"},
		{Start: 2 * time.Second, End: 4 * time.Second, Text: "new sentence"},
	}
	added := mergeChunk(sub, raw, 15*time.Second, 15*time.Second, false)
	if added != 1 || len(sub.Segments) != 2 {
		t.Fatalf("added = %d, len = %d, want 1/2", added, len(sub.Segments))
	}
	if sub.Segments[1].Text != "new sentence" || sub.Segments[1].StartTime != 17*time.Second {
		t.Fatalf("second segment = %q @ %v, want new sentence @ 17s", sub.Segments[1].Text, sub.Segments[1].StartTime)
	}
}

func TestMergeChunkSkipsEmptyText(t *testing.T) {
	sub := &Subtitle{}
	added := mergeChunk(sub, []rawSegment{
		{Start: 0, End: time.Second, Text: "   "},
	}, 0, 15*time.Second, true)
	if added != 0 || len(sub.Segments) != 0 {
		t.Fatalf("added = %d, len = %d, want 0/0", added, len(sub.Segments))
	}
}

func TestIsHallucinationAbPattern(t *testing.T) {
	sub := &Subtitle{}
	sub.AddSegment(0, 1500*time.Millisecond, "A")
	sub.AddSegment(1600*time.Millisecond, 3*time.Second, "B")
	if !isHallucination(sub, Segment{StartTime: 3100 * time.Millisecond, EndTime: 4 * time.Second, Text: "A"}) {
		t.Fatal("A-B-A pattern not detected")
	}
	if isHallucination(sub, Segment{StartTime: 3100 * time.Millisecond, EndTime: 4 * time.Second, Text: "C"}) {
		t.Fatal("new text wrongly flagged as hallucination")
	}
}
