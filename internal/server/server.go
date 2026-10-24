package server

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
)

// Version is the daemon version reported by /api/health.
// Matches the standalone CLI's user-facing version.
const Version = "1.0.0"

// Config configures the daemon.
type Config struct {
	// Host binds the listener ("127.0.0.1" by default; "0.0.0.0" for Docker).
	Host string
	// Port to listen on; 0 picks a random free port (desktop shell launch).
	Port int
	// StaticDir serves the exported Next.js frontend; empty disables static
	// hosting (API-only / dev mode).
	StaticDir string
	// HomeDir is the daemon data root: vytdl.db and logs live here.
	HomeDir string
	// OutputDir is the default download destination.
	OutputDir string
	// Dev enables request logging.
	Dev bool
}

// Server is the vYtDL background service.
type Server struct {
	cfg              Config
	db               *Database
	hub              *Hub
	queue            *QueueManager
	defaultOutputDir string
}

// New wires the server: opens the database, seeds default settings, resumes
// interrupted downloads and prepares the queue.
func New(cfg Config) (*Server, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.HomeDir == "" {
		return nil, fmt.Errorf("home dir is required")
	}
	if err := os.MkdirAll(cfg.HomeDir, 0o755); err != nil {
		return nil, fmt.Errorf("create home dir: %w", err)
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "downloads"
	}
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}
	if p, err := filepath.Abs(cfg.OutputDir); err == nil {
		cfg.OutputDir = p
	}

	db, err := OpenDatabase(dbPath(cfg.HomeDir))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	hub := NewHub()
	s := &Server{
		cfg:              cfg,
		db:               db,
		hub:              hub,
		defaultOutputDir: cfg.OutputDir,
	}
	s.queue = NewQueueManager(db, hub, cfg.OutputDir)

	if err := s.seedDefaults(); err != nil {
		db.Close()
		return nil, fmt.Errorf("seed settings: %w", err)
	}
	s.queue.LoadSettings()
	if err := s.queue.ResumeOnStartup(); err != nil {
		db.Close()
		return nil, fmt.Errorf("resume downloads: %w", err)
	}
	return s, nil
}

// Close shuts the daemon down: cancels active downloads, then the database.
func (s *Server) Close() {
	s.queue.Shutdown()
	_ = s.db.Close()
}

// dbPath is where the SQLite database lives inside the data root.
func dbPath(homeDir string) string { return homeDir + string(os.PathSeparator) + "vytdl.db" }

// Handler builds the HTTP handler: CORS, API routes, optional static hosting.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	if s.cfg.StaticDir != "" {
		mux.HandleFunc("/", s.staticHandler())
	}
	var h http.Handler = mux
	if s.cfg.Dev {
		h = s.logRequests(h)
	}
	return corsMiddleware(h)
}

// Listen binds the configured address and returns the listener (call Serve).
func (s *Server) Listen() (net.Listener, error) {
	return net.Listen("tcp", net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port)))
}

// Serve accepts connections on ln until the process exits.
func (s *Server) Serve(ln net.Listener) error { return http.Serve(ln, s.Handler()) }

// Addr renders the listener address for banner output.
func Addr(ln net.Listener) string { return "http://" + ln.Addr().String() }

func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("[dev] %s %s\n", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func isDarwin() bool  { return runtime.GOOS == "darwin" }
func isWindows() bool { return runtime.GOOS == "windows" }

// spawnedCommand builds a detached OS command (no shell).
func spawnedCommand(bin string, args ...string) *exec.Cmd {
	cmd := exec.Command(bin, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd
}
