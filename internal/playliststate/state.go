package playliststate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
)

type EntryInput struct {
	ID    string
	URL   string
	Title string
}

type EntryState struct {
	ID             string    `json:"id"`
	URL            string    `json:"url"`
	Title          string    `json:"title"`
	Status         string    `json:"status"`
	Error          string    `json:"error,omitempty"`
	Attempts       int       `json:"attempts"`
	Filename       string    `json:"filename,omitempty"`
	Subtitles      []string  `json:"subtitles,omitempty"`
	LastStartedAt  time.Time `json:"last_started_at,omitempty"`
	LastFinishedAt time.Time `json:"last_finished_at,omitempty"`
}

type State struct {
	PlaylistURL   string       `json:"playlist_url"`
	PlaylistTitle string       `json:"playlist_title"`
	PlaylistDir   string       `json:"playlist_dir"`
	UpdatedAt     time.Time    `json:"updated_at"`
	Entries       []EntryState `json:"entries"`
}

type Manager struct {
	path  string
	state State
}

func Open(path, playlistURL, playlistTitle, playlistDir string, entries []EntryInput, reset bool) (*Manager, error) {
	m := &Manager{
		path: path,
		state: State{
			PlaylistURL:   playlistURL,
			PlaylistTitle: playlistTitle,
			PlaylistDir:   playlistDir,
		},
	}

	if !reset {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &m.state)
		}
	}

	m.state.PlaylistURL = playlistURL
	m.state.PlaylistTitle = playlistTitle
	m.state.PlaylistDir = playlistDir
	m.normalizeInterruptedRuns()
	m.mergeEntries(entries)
	if err := m.Save(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) Path() string {
	return m.path
}

func (m *Manager) Entries() []EntryState {
	return m.state.Entries
}

func (m *Manager) MarkRunning(key string) error {
	for i := range m.state.Entries {
		if stateKey(m.state.Entries[i].ID, m.state.Entries[i].URL) != key {
			continue
		}
		m.state.Entries[i].Status = StatusRunning
		m.state.Entries[i].Error = ""
		m.state.Entries[i].Attempts++
		m.state.Entries[i].LastStartedAt = time.Now()
		break
	}
	return m.Save()
}

func (m *Manager) MarkFinished(key, title, filename string, subtitles []string, success bool, errText string) error {
	for i := range m.state.Entries {
		if stateKey(m.state.Entries[i].ID, m.state.Entries[i].URL) != key {
			continue
		}
		if strings.TrimSpace(title) != "" {
			m.state.Entries[i].Title = title
		}
		m.state.Entries[i].Filename = filename
		m.state.Entries[i].Subtitles = subtitles
		m.state.Entries[i].LastFinishedAt = time.Now()
		if success {
			m.state.Entries[i].Status = StatusSucceeded
			m.state.Entries[i].Error = ""
		} else {
			m.state.Entries[i].Status = StatusFailed
			m.state.Entries[i].Error = errText
		}
		break
	}
	return m.Save()
}

func (m *Manager) Save() error {
	m.state.UpdatedAt = time.Now()
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	tempPath := m.path + ".tmp"
	f, err := os.Create(tempPath)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m.state); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, m.path)
}

func StatePath(playlistDir string) string {
	return filepath.Join(playlistDir, ".playlist_state.json")
}

func (m *Manager) normalizeInterruptedRuns() {
	for i := range m.state.Entries {
		if m.state.Entries[i].Status == StatusRunning {
			m.state.Entries[i].Status = StatusFailed
			if strings.TrimSpace(m.state.Entries[i].Error) == "" {
				m.state.Entries[i].Error = "previous run interrupted before completion"
			}
		}
		if m.state.Entries[i].Status == StatusSucceeded && strings.TrimSpace(m.state.Entries[i].Filename) != "" {
			if _, err := os.Stat(m.state.Entries[i].Filename); err != nil {
				m.state.Entries[i].Status = StatusPending
				m.state.Entries[i].Error = "previous output file missing; queued for re-download"
			}
		}
	}
}

func (m *Manager) mergeEntries(entries []EntryInput) {
	index := map[string]EntryState{}
	for _, entry := range m.state.Entries {
		index[stateKey(entry.ID, entry.URL)] = entry
	}

	merged := make([]EntryState, 0, len(entries))
	for _, entry := range entries {
		key := stateKey(entry.ID, entry.URL)
		stateEntry, ok := index[key]
		if !ok {
			stateEntry = EntryState{
				ID:     entry.ID,
				URL:    entry.URL,
				Title:  entry.Title,
				Status: StatusPending,
			}
		}
		if strings.TrimSpace(entry.ID) != "" {
			stateEntry.ID = entry.ID
		}
		if strings.TrimSpace(entry.URL) != "" {
			stateEntry.URL = entry.URL
		}
		if strings.TrimSpace(entry.Title) != "" {
			stateEntry.Title = entry.Title
		}
		if strings.TrimSpace(stateEntry.Status) == "" {
			stateEntry.Status = StatusPending
		}
		merged = append(merged, stateEntry)
		delete(index, key)
	}

	for _, leftover := range index {
		merged = append(merged, leftover)
	}
	m.state.Entries = merged
}

func stateKey(id, url string) string {
	if strings.TrimSpace(id) != "" {
		return id
	}
	return url
}
