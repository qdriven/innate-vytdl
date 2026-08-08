package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/innate/yt-dl/internal/vtt"
	"github.com/spf13/cobra"
)

var (
	flagAnalyzeMode    string
	flagAnalyzeOutput  string
	flagAnalyzeSegment string
)

func init() {
	an := analyzeCmd

	an.Flags().StringVarP(&flagAnalyzeMode, "mode", "m", "text",
		"Analysis mode: text")
	an.Flags().StringVarP(&flagAnalyzeOutput, "output", "o", "",
		"Write output to file (default: stdout)")
	an.Flags().StringVarP(&flagAnalyzeSegment, "segment", "s", "",
		"Segment duration for modes that support it (e.g. 3m, 5m)")

	rootCmd.AddCommand(an)
}

var analyzeCmd = &cobra.Command{
	Use:     "analyze [flags] <file.vtt>",
	Aliases: []string{"an", "ana"},
	Short:   "Analyze a VTT subtitle file",
	Long: `Analyze a WebVTT subtitle file and extract text, summaries, or key points.

Modes:
  text        Extract plain text without timestamps (default)
  markdown    Output as Markdown with timestamped headings

Examples:
  yt-dl analyze video.zh.vtt
  yt-dl analyze --mode text --output transcript.txt video.en.vtt
  yt-dl analyze --mode markdown --output transcript.md video.zh.vtt
  yt-dl analyze < video.vtt`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAnalyze,
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	mode := strings.ToLower(strings.TrimSpace(flagAnalyzeMode))

	var input *os.File
	if len(args) > 0 {
		f, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("cannot open %q: %w", args[0], err)
		}
		defer f.Close()
		input = f
	} else {
		input = os.Stdin
	}

	tr, err := vtt.Parse(input)
	if err != nil {
		return fmt.Errorf("parse VTT: %w", err)
	}

	out := os.Stdout
	if flagAnalyzeOutput != "" {
		f, err := os.Create(flagAnalyzeOutput)
		if err != nil {
			return fmt.Errorf("cannot create output %q: %w", flagAnalyzeOutput, err)
		}
		defer f.Close()
		out = f
	}

	switch mode {
	case "text":
		_, err := fmt.Fprintln(out, tr.PlainText())
		return err
	case "markdown":
		_, err := fmt.Fprint(out, tr.ToMarkdown())
		return err
	default:
		return fmt.Errorf("unknown mode %q; supported: text, markdown", mode)
	}
}

// parseSegmentFlag parses a duration string like "3m" or "5m30s".
func parseSegmentFlag(s string) (time.Duration, error) {
	return time.ParseDuration(strings.TrimSpace(s))
}
