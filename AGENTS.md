# AGENTS.md — vYtDL CLI (innate-vytdl)

Standalone Go CLI wrapping yt-dlp. Fully independent project — no dependency on
other repositories.

## Quick commands

```bash
go build -o vYtDL .
go test ./...
./vYtDL download --no-tui -o ./downloads "URL"
```

## Skills

Agent skills live in `skills/` (canonical source):

- `skills/vytdl-cli/SKILL.md` — build/run/extend the CLI, multi-site downloads, troubleshooting
- `skills/download-record/SKILL.md` — query the single download catalog (history, subtitle paths, failures)
- `skills/x-video-dl/SKILL.md` — X/Twitter video download via yt-dlp + browser cookies

Read the relevant `SKILL.md` before doing download or CLI-extension tasks. Install
instructions per tool: `docs/skills-guide.md`.

## Rules of thumb

- Prefer changing `internal/downloader` for yt-dlp behavior; `internal/server` is the `serve` daemon (HTTP API + WS + queue + SQLite).
- Download history is one catalog: `$VYTDL_HOME/download_record.json` (default `~/.vytdl/download_record.json`). Subtitle paths are a field on each entry. CLI and `serve` both append to it. Do not add a second mapping file.
- `vytdl.db` is only the live `serve` queue. `.playlist_state.json` is playlist resume state. Callers inspecting past downloads use `vYtDL records` (see `skills/download-record`).
- Do not add a URL domain whitelist — site coverage is yt-dlp's job. Bilibili URL types the current yt-dlp build can download are listed in `docs/bilibili.md`.
- Pass through cookie/proxy/extractor-args rather than reimplementing auth.
- Update `USAGE.md` (and the skill if user-facing) when adding user-facing flags.
- Docs are Chinese-first; keep CLI flags and code identifiers in English.
