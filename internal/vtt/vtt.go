// Package vtt parses WebVTT subtitle files, including YouTube's
// auto-generated caption format with <c> word-timing tags.
package vtt

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Cue represents a single subtitle cue with a time range and cleaned text.
type Cue struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

// Segment groups cues that fall within a time window.
type Segment struct {
	Start time.Duration
	End   time.Duration
	Cues  []Cue
	Text  string
}

// Transcript holds the parsed content of a WebVTT file.
type Transcript struct {
	Language string
	Kind     string
	Cues     []Cue
}

var timeRe = regexp.MustCompile(`^(\d{2}:\d{2}:\d{2}\.\d{3})\s*-->\s*(\d{2}:\d{2}:\d{2}\.\d{3})`)
var tagRe = regexp.MustCompile(`<\d{2}:\d{2}:\d{2}\.\d{3}>|</?c>`)

func parseTimestamp(s string) (time.Duration, error) {
	var h, m, sec, ms int
	_, err := fmt.Sscanf(s, "%d:%d:%d.%d", &h, &m, &sec, &ms)
	if err != nil {
		return 0, fmt.Errorf("invalid timestamp %q: %w", s, err)
	}
	return time.Duration(h)*time.Hour +
		time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second +
		time.Duration(ms)*time.Millisecond, nil
}

func cleanText(raw string) string {
	s := tagRe.ReplaceAllString(raw, "")
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

// Parse reads a WebVTT document and returns a Transcript.
// Handles both simple manual captions and YouTube auto-generated captions
// with <c> word-timing tags and duplicate snapshots.
func Parse(r io.Reader) (*Transcript, error) {
	sc := bufio.NewScanner(r)
	t := &Transcript{}
	var cues []Cue
	state := 0 // 0=expect timestamp, 1=collecting text
	var cur *Cue
	lineNo := 0

	for sc.Scan() {
		lineNo++
		line := sc.Text()

		if lineNo == 1 && strings.HasPrefix(line, "WEBVTT") {
			continue
		}
		if strings.HasPrefix(line, "Kind:") {
			t.Kind = strings.TrimSpace(strings.TrimPrefix(line, "Kind:"))
			continue
		}
		if strings.HasPrefix(line, "Language:") {
			t.Language = strings.TrimSpace(strings.TrimPrefix(line, "Language:"))
			continue
		}

		trimmed := strings.TrimSpace(line)

		// Blank line → end current cue
		if trimmed == "" {
			if cur != nil {
				cleaned := cleanText(cur.Text)
				if cleaned != "" {
					cur.Text = cleaned
					cues = append(cues, *cur)
				}
				cur = nil
			}
			state = 0
			continue
		}

		// Timestamp line
		if m := timeRe.FindStringSubmatch(trimmed); m != nil {
			start, err := parseTimestamp(m[1])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			end, err := parseTimestamp(m[2])
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", lineNo, err)
			}
			if cur != nil {
				cleaned := cleanText(cur.Text)
				if cleaned != "" {
					cur.Text = cleaned
					cues = append(cues, *cur)
				}
			}
			cur = &Cue{Start: start, End: end}
			state = 1
			continue
		}

		// Collect text for current cue
		if state == 1 && cur != nil {
			if cur.Text != "" {
				cur.Text += " "
			}
			cur.Text += trimmed
		}
	}

	// Flush last cue
	if cur != nil {
		cleaned := cleanText(cur.Text)
		if cleaned != "" {
			cur.Text = cleaned
			cues = append(cues, *cur)
		}
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}
	t.Cues = deduplicateCues(cues)
	return t, nil
}

// deduplicateCues removes YouTube's 10ms "sentence snapshot" cues that
// duplicate content from adjacent word-timed cues.
//
// YouTube auto-generated VTT produces pairs:
//
//	[word-timed cue: 5s, with <c> tags]  → cleaned text
//	[snapshot cue:  10ms, no tags]       → same or overlapping text
//
// We keep word-timed cues and drop snapshots whose text is contained
// within an adjacent cue.
func deduplicateCues(cues []Cue) []Cue {
	const snapshotThreshold = 50 * time.Millisecond

	if len(cues) < 2 {
		return cues
	}

	out := make([]Cue, 0, len(cues))
	for i, c := range cues {
		dur := c.End - c.Start

		// Keep all cues with meaningful duration
		if dur >= snapshotThreshold {
			out = append(out, c)
			continue
		}

		// Short cue: check if it's redundant with previous or next cue
		redundant := false
		if i > 0 && strings.Contains(out[len(out)-1].Text, c.Text) {
			redundant = true
		}
		if i+1 < len(cues) && strings.Contains(cues[i+1].Text, c.Text) {
			redundant = true
		}
		if !redundant {
			out = append(out, c)
		}
	}
	return out
}

// PlainText returns all cue texts joined into a single string.
func (t *Transcript) PlainText() string {
	var parts []string
	for _, c := range t.Cues {
		if c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, " ")
}

// ToMarkdown renders the transcript as a Markdown document with
// timestamped headings for each cue group.
func (t *Transcript) ToMarkdown() string {
	if len(t.Cues) == 0 {
		return ""
	}
	var sb strings.Builder
	if t.Language != "" {
		sb.WriteString("<!-- language: ")
		sb.WriteString(t.Language)
		sb.WriteString(" -->\n\n")
	}
	sb.WriteString("# Transcript\n\n")
	prevHour := int64(-1)
	for _, c := range t.Cues {
		h := int64(c.Start.Hours())
		if h != prevHour {
			sb.WriteString("## ")
			sb.WriteString(formatTimestamp(c.Start))
			sb.WriteString("\n\n")
			prevHour = h
		}
		sb.WriteString(formatTimestamp(c.Start))
		sb.WriteString("  ")
		sb.WriteString(c.Text)
		sb.WriteString("\n\n")
	}
	return sb.String()
}

func formatTimestamp(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// SegmentByDuration groups cues into time windows of length d.
func (t *Transcript) SegmentByDuration(d time.Duration) []Segment {
	if len(t.Cues) == 0 || d <= 0 {
		return nil
	}
	var segs []Segment
	start := t.Cues[0].Start
	end := start + d
	var window []Cue
	for _, c := range t.Cues {
		for c.Start >= end && len(window) > 0 {
			segs = append(segs, buildSegment(window, start, end))
			window = nil
			start = end
			end = start + d
		}
		window = append(window, c)
	}
	if len(window) > 0 {
		segs = append(segs, buildSegment(window, start, end))
	}
	return segs
}

func buildSegment(cues []Cue, start, end time.Duration) Segment {
	var texts []string
	for _, c := range cues {
		if c.Text != "" {
			texts = append(texts, c.Text)
		}
	}
	return Segment{Start: start, End: end, Cues: cues, Text: strings.Join(texts, " ")}
}
