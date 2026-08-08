# AGENTS.md — vYtDL CLI

Standalone Go CLI wrapping yt-dlp. See `.agents/skills/vytdl-cli/SKILL.md` for agent instructions.

## Quick commands

```bash
go build -o vYtDL .
go test ./...
./vYtDL download --no-tui -o ./downloads "URL"
```

## Rules of thumb

- Prefer changing `internal/downloader` for yt-dlp behavior; keep Desktop/Web out of this repo.
- Do not add a URL domain whitelist — site coverage is yt-dlp’s job.
- Pass through cookie/proxy/extractor-args rather than reimplementing auth.
- Update `USAGE.md` when adding user-facing flags.
