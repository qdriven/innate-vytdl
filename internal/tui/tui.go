package tui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/innate/yt-dl/internal/downloader"
)

// ---- Styles ----

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	barFill      = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	barEmpty     = lipgloss.NewStyle().Foreground(lipgloss.Color("237"))
)

const barWidth = 30

func renderBar(pct float64) string {
	filled := int(pct / 100 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled
	return barFill.Render(strings.Repeat("█", filled)) +
		barEmpty.Render(strings.Repeat("░", empty))
}

// ---- Model ----

// itemState tracks progress for one video.
type itemState struct {
	id      string
	title   string
	percent float64
	speed   string
	eta     string
	status  string // downloading / merging / done / error
	errMsg  string
}

type progressMsg downloader.ProgressUpdate
type doneMsg struct{}
type tickMsg time.Time

// Model is the bubbletea model for the TUI.
type Model struct {
	mu       sync.Mutex
	items    map[string]*itemState // keyed by VideoID (or URL if ID unknown)
	order    []string              // insertion order
	done     bool
	quitting bool
	// channel from which the model receives updates
	updates <-chan downloader.ProgressUpdate
}

// New creates a new TUI model that reads from updates.
func New(updates <-chan downloader.ProgressUpdate) *Model {
	return &Model{
		items:   make(map[string]*itemState),
		updates: updates,
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.waitForUpdate(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *Model) waitForUpdate() tea.Cmd {
	return func() tea.Msg {
		update, ok := <-m.updates
		if !ok {
			return doneMsg{}
		}
		return progressMsg(update)
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.quitting = true
			return m, tea.Quit
		}

	case progressMsg:
		m.mu.Lock()
		key := msg.Key
		if key == "" {
			key = msg.VideoID
		}
		if key == "" {
			key = msg.Title
		}
		if key == "" {
			key = fmt.Sprintf("item-%d", len(m.items))
		}
		item, exists := m.items[key]
		if !exists {
			item = &itemState{id: msg.VideoID, title: msg.Title}
			m.items[key] = item
			m.order = append(m.order, key)
		}
		if msg.Title != "" {
			item.title = msg.Title
		}
		item.percent = msg.Percent
		item.speed = msg.Speed
		item.eta = msg.ETA
		item.status = msg.Status
		item.errMsg = msg.Error
		m.mu.Unlock()
		return m, m.waitForUpdate()

	case doneMsg:
		m.done = true
		return m, tea.Quit

	case tickMsg:
		return m, tickCmd()
	}

	return m, nil
}

func (m *Model) View() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("yt-dl — Video Downloader") + "\n")
	sb.WriteString(dimStyle.Render("Press q / Ctrl+C to cancel\n\n"))

	for _, key := range m.order {
		item := m.items[key]
		title := item.title
		if title == "" {
			title = key
		}
		if len(title) > 55 {
			title = title[:52] + "..."
		}

		switch item.status {
		case "done":
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				successStyle.Render("✓"),
				title,
			))
		case "skipped":
			sb.WriteString(fmt.Sprintf("  %s  %s\n",
				dimStyle.Render("•"),
				title+" (already downloaded)",
			))
		case "error":
			sb.WriteString(fmt.Sprintf("  %s  %s\n     %s\n",
				errorStyle.Render("✗"),
				title,
				errorStyle.Render(item.errMsg),
			))
		default:
			bar := renderBar(item.percent)
			info := ""
			if item.speed != "" {
				info = fmt.Sprintf(" %s  ETA %s", item.speed, item.eta)
			}
			sb.WriteString(fmt.Sprintf("  %s %s %5.1f%%%s\n     %s\n",
				bar,
				dimStyle.Render(item.status),
				item.percent,
				dimStyle.Render(info),
				title,
			))
		}
	}

	if m.done {
		sb.WriteString("\n" + successStyle.Render("All downloads completed.") + "\n")
	}

	return sb.String()
}

// Run starts the TUI and blocks until it exits.
func Run(updates <-chan downloader.ProgressUpdate) error {
	m := New(updates)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
