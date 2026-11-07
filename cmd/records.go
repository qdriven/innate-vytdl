package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/innate/yt-dl/internal/record"
)

var (
	recordsFile   string
	recordsFormat string
	recordsFailed bool
	recordsJSON   bool
	recordsLimit  int
)

func init() {
	recordsCmd.Flags().StringVar(&recordsFormat, "log-format", "json",
		"Download log format: json or csv")
	recordsCmd.Flags().StringVar(&recordsFile, "record-file", "",
		"Download log path (default: $VYTDL_HOME/download_record.json, usually ~/.vytdl/download_record.json)")
	recordsCmd.Flags().BoolVar(&recordsFailed, "failed", false,
		"Show only failed attempts")
	recordsCmd.Flags().BoolVar(&recordsJSON, "json", false,
		"Print the selected records as JSON")
	recordsCmd.Flags().IntVar(&recordsLimit, "limit", 0,
		"Show only the most recent N records (0 = all)")
	rootCmd.AddCommand(recordsCmd)
}

var recordsCmd = &cobra.Command{
	Use:   "records",
	Short: "Show the shared download log",
	Long: `records prints the download catalog shared by the CLI and serve.

The default file is $VYTDL_HOME/download_record.json (usually ~/.vytdl/download_record.json).
Each entry includes subtitle paths. Legacy download_record.* and subtitle_mapping.*
files in the current directory are imported into that catalog and then removed.

Playlist resume state stays in <output>/<title>/.playlist_state.json.
The serve daemon still keeps the live queue in vytdl.db.`,
	Args: cobra.NoArgs,
	RunE: runRecords,
}

func runRecords(cmd *cobra.Command, args []string) error {
	format := strings.ToLower(strings.TrimSpace(recordsFormat))
	mgr, err := record.NewManager(format, record.ResolvePath(format, recordsFile))
	if err != nil {
		return err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	imported, err := importLegacyRecords(mgr, cwd)
	if err != nil {
		return err
	}
	if imported > 0 {
		fmt.Fprintf(os.Stderr, "Imported %d legacy record(s) into %s\n", imported, mgr.Path())
	}

	entries, err := mgr.List()
	if err != nil {
		return err
	}
	selected := selectRecords(entries, recordsFailed, recordsLimit)
	fmt.Fprintf(os.Stderr, "Download log: %s\n", mgr.Path())
	if recordsJSON {
		payload := record.Catalog{Version: 1, Records: selected}
		if payload.Records == nil {
			payload.Records = []record.DownloadRecord{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)
	}
	if len(selected) == 0 {
		fmt.Println("No download records.")
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STARTED\tOK\tVIDEO\tTITLE\tFILE\tSUBS")
	for i := len(selected) - 1; i >= 0; i-- {
		r := selected[i]
		ok := "no"
		if r.Success {
			ok = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n",
			r.StartedAt.Format("2006-01-02 15:04:05"),
			ok,
			r.VideoID,
			oneLine(r.Title),
			filepath.Base(r.Filename),
			len(r.Subtitles),
		)
	}
	return w.Flush()
}

func selectRecords(entries []record.DownloadRecord, failedOnly bool, limit int) []record.DownloadRecord {
	if failedOnly {
		filtered := make([]record.DownloadRecord, 0, len(entries))
		for _, r := range entries {
			if !r.Success {
				filtered = append(filtered, r)
			}
		}
		entries = filtered
	}
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	return entries
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

func importLegacyRecords(mgr *record.Manager, dirs ...string) (int, error) {
	seen := map[string]struct{}{}
	total := 0
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		abs, err := filepath.Abs(dir)
		if err != nil {
			abs = dir
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		n, err := mgr.ImportLegacyDir(dir)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}
