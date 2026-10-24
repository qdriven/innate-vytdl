package cmd

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/innate/yt-dl/internal/server"
)

var serveFlags struct {
	port   int
	host   string
	static string
	home   string
	dev    bool
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the vYtDL background service (HTTP API + WebSocket)",
	Long: `serve starts the vYtDL daemon: an HTTP API + WebSocket server that owns
the download queue, SQLite persistence, VTT analysis and optional static
frontend hosting. It is the single backend for the web UI — loaded by the
desktop shell (random loopback port) or reachable directly in the browser,
or deployed to Docker with --host 0.0.0.0.

Examples:
  vytdl serve --port 0 --static ./out          # desktop-shell style launch
  vytdl serve --port 3000 --host 0.0.0.0       # Docker deployment
  vytdl serve --home ~/.vytdl --dev            # local dev with request logs`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().IntVar(&serveFlags.port, "port", envInt("PORT", 0), "listen port; 0 picks a random free port")
	serveCmd.Flags().StringVar(&serveFlags.host, "host", envStr("VYTDL_HOST", "127.0.0.1"), "bind address (use 0.0.0.0 for Docker)")
	serveCmd.Flags().StringVar(&serveFlags.static, "static", envStr("VYTDL_STATIC_DIR", ""), "serve the exported frontend from this directory")
	serveCmd.Flags().StringVar(&serveFlags.home, "home", envStr("VYTDL_HOME", defaultHome()), "daemon data root (vytdl.db lives here)")
	serveCmd.Flags().BoolVar(&serveFlags.dev, "dev", false, "dev mode: request logging")
	rootCmd.AddCommand(serveCmd)
}

// defaultHome matches the desktop's ~/.vytdl convention (agent-sessions
// already live there).
func defaultHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".vytdl"
	}
	return filepath.Join(home, ".vytdl")
}

func envStr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		n := 0
		for _, c := range v {
			if c < '0' || c > '9' {
				return def
			}
			n = n*10 + int(c-'0')
		}
		return n
	}
	return def
}

func runServe(cmd *cobra.Command, args []string) error {
	staticDir := serveFlags.static
	if staticDir != "" {
		abs, err := filepath.Abs(staticDir)
		if err != nil {
			return fmt.Errorf("resolve static dir: %w", err)
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			return fmt.Errorf("static dir not found: %s", abs)
		}
		staticDir = abs
	}

	homeDir := serveFlags.home
	if abs, err := filepath.Abs(homeDir); err == nil {
		homeDir = abs
	}

	outputDir := envStr("VYTDL_OUTPUT_DIR", filepath.Join(homeDir, "downloads"))

	srv, err := server.New(server.Config{
		Host:      serveFlags.host,
		Port:      serveFlags.port,
		StaticDir: staticDir,
		HomeDir:   homeDir,
		OutputDir: outputDir,
		Dev:       serveFlags.dev,
	})
	if err != nil {
		return err
	}
	defer srv.Close()

	ln, err := srv.Listen()
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	fmt.Printf("vYtDL daemon %s listening on %s\n", server.Version, server.Addr(ln))
	fmt.Printf("  data root: %s\n", homeDir)
	fmt.Printf("  downloads: %s\n", outputDir)
	if staticDir != "" {
		fmt.Printf("  static:    %s\n", staticDir)
	}

	// Graceful shutdown on SIGINT/SIGTERM: cancel active downloads, flush state.
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-stop:
		fmt.Println("\nshutting down: cancelling active downloads…")
		srv.Close()
		// small grace period for in-flight HTTP responses
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}
