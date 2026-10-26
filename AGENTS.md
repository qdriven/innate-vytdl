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
- `skills/x-video-dl/SKILL.md` — X/Twitter video download via yt-dlp + browser cookies

Read the relevant `SKILL.md` before doing download or CLI-extension tasks. Install
instructions per tool: `docs/skills-guide.md`.

## Rules of thumb

- Prefer changing `internal/downloader` for yt-dlp behavior; `internal/server` is the `serve` daemon (HTTP API + WS + queue + SQLite).
- Do not add a URL domain whitelist — site coverage is yt-dlp's job.
- Pass through cookie/proxy/extractor-args rather than reimplementing auth.
- Update `USAGE.md` (and the skill if user-facing) when adding user-facing flags.
- Docs are Chinese-first; keep CLI flags and code identifiers in English.
