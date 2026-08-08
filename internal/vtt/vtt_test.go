package vtt

import (
	"strings"
	"testing"
	"time"
)

func TestParseSimpleVTT(t *testing.T) {
	// Type A: simple manual captions (Chinese)
	input := `WEBVTT
Kind: captions
Language: zh

00:00:25.333 --> 00:00:26.800
最近我一直在想一个问题

00:00:26.933 --> 00:00:29.800
为什么很多人学 AI 越学越焦虑

00:00:29.900 --> 00:00:31.400
而且他们的水平其实
`
	tr, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tr.Kind != "captions" {
		t.Errorf("Kind = %q, want captions", tr.Kind)
	}
	if tr.Language != "zh" {
		t.Errorf("Language = %q, want zh", tr.Language)
	}
	if len(tr.Cues) != 3 {
		t.Fatalf("len(Cues) = %d, want 3", len(tr.Cues))
	}
	if tr.Cues[0].Text != "最近我一直在想一个问题" {
		t.Errorf("cue[0].Text = %q", tr.Cues[0].Text)
	}
	if tr.Cues[2].Text != "而且他们的水平其实" {
		t.Errorf("cue[2].Text = %q", tr.Cues[2].Text)
	}
}

func TestParseYouTubeAutoVTT(t *testing.T) {
	// Type B: YouTube auto-generated captions with <c> tags.
	// The parser deduplicates 10ms snapshot cues that are contained
	// in adjacent word-timed cues.
	input := `WEBVTT
Kind: captions
Language: en

00:00:02.399 --> 00:00:07.990 align:start position:0%
 
So<00:00:03.200><c> Trump</c><00:00:04.160><c> is</c><00:00:04.640><c> in</c><00:00:05.520><c> China.</c>

00:00:07.990 --> 00:00:08.000 align:start position:0%
So Trump is in China. This happened
 

00:00:08.000 --> 00:00:11.430 align:start position:0%
So Trump is in China. This happened
about<00:00:08.960><c> um</c><00:00:09.440><c> 30</c><00:00:09.679><c> minutes</c><00:00:09.920><c> ago,</c>

00:00:11.430 --> 00:00:11.440 align:start position:0%
about um 30 minutes ago,
 
`
	tr, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if tr.Kind != "captions" {
		t.Errorf("Kind = %q, want captions", tr.Kind)
	}
	if tr.Language != "en" {
		t.Errorf("Language = %q, want en", tr.Language)
	}

	// After dedup, at least 1 cue; exact count depends on subset relations.
	if len(tr.Cues) < 1 {
		t.Fatalf("len(Cues) = %d, want >= 1", len(tr.Cues))
	}

	// First cue should have cleaned text — no <c> tags
	cue0 := tr.Cues[0].Text
	if strings.Contains(cue0, "<c>") || strings.Contains(cue0, "</c>") {
		t.Errorf("cue[0] contains <c> tags: %q", cue0)
	}
	if !strings.Contains(cue0, "Trump") {
		t.Errorf("cue[0] missing Trump: %q", cue0)
	}

	// There should be at least one cue mentioning "about"
	foundAbout := false
	for _, c := range tr.Cues {
		if strings.Contains(c.Text, "about") {
			foundAbout = true
			break
		}
	}
	if !foundAbout {
		t.Errorf("no cue contains 'about'")
	}

	// Timestamp check: first kept cue should have a valid start > 0
	if tr.Cues[0].Start <= 0 {
		t.Errorf("cue[0].Start = %v, want > 0", tr.Cues[0].Start)
	}
	// Verify timestamps are in ascending order
	for i := 1; i < len(tr.Cues); i++ {
		if tr.Cues[i].Start < tr.Cues[i-1].Start {
			t.Errorf("cues not in order: cue[%d].Start=%v < cue[%d].Start=%v",
				i, tr.Cues[i].Start, i-1, tr.Cues[i-1].Start)
		}
	}
}

func TestPlainText(t *testing.T) {
	input := `WEBVTT
Kind: captions
Language: zh

00:00:01.000 --> 00:00:02.000
你好世界

00:00:02.000 --> 00:00:03.000
这是测试
`
	tr, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	got := tr.PlainText()
	want := "你好世界 这是测试"
	if got != want {
		t.Errorf("PlainText = %q, want %q", got, want)
	}
}

func TestSegmentByDuration(t *testing.T) {
	input := `WEBVTT
Kind: captions
Language: zh

00:00:01.000 --> 00:00:02.000
第一句

00:00:03.000 --> 00:00:04.000
第二句

00:00:05.000 --> 00:00:06.000
第三句

00:00:07.000 --> 00:00:08.000
第四句
`
	tr, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	// 3-second windows → 4 cues should split into 3 segments
	segs := tr.SegmentByDuration(3 * time.Second)
	if len(segs) != 3 {
		t.Fatalf("len(segs) = %d, want 3", len(segs))
	}
	if segs[0].Text != "第一句 第二句" {
		t.Errorf("seg[0].Text = %q", segs[0].Text)
	}
	if segs[1].Text != "第三句" {
		t.Errorf("seg[1].Text = %q", segs[1].Text)
	}
	if segs[2].Text != "第四句" {
		t.Errorf("seg[2].Text = %q", segs[2].Text)
	}
}

func TestParseEmpty(t *testing.T) {
	tr, err := Parse(strings.NewReader("WEBVTT\n\n"))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(tr.Cues) != 0 {
		t.Errorf("expected 0 cues, got %d", len(tr.Cues))
	}
}

func TestParseNoBlankLineEnd(t *testing.T) {
	input := `WEBVTT
Kind: captions
Language: zh

00:00:01.000 --> 00:00:02.000
最后一句话`
	tr, err := Parse(strings.NewReader(input))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(tr.Cues) != 1 {
		t.Fatalf("len(Cues) = %d, want 1", len(tr.Cues))
	}
	if tr.Cues[0].Text != "最后一句话" {
		t.Errorf("Text = %q", tr.Cues[0].Text)
	}
}

func TestCleanTextStripsTags(t *testing.T) {
	raw := "So<00:00:03.200><c> Trump</c><00:00:04.160><c> is</c> in China."
	got := cleanText(raw)
	want := "So Trump is in China."
	if got != want {
		t.Errorf("cleanText = %q, want %q", got, want)
	}
}
