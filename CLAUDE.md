# Braindump — Todo Capture System

## Build & Test Commands
```bash
go build ./...                    # Build all
go build ./cmd/todo/              # Build CLI
go build ./cmd/server/            # Build HTTP server
go test ./internal/core/...       # Test core library
go test ./...                     # Test everything
go vet ./...                      # Lint
```

## Architecture

**Core library** (`internal/core/`) is the single source of truth. CLI and HTTP API are thin consumers.

```
internal/core/   → Core library (todo struct, store, tags, logger, stale, info)
internal/cli/    → CLI command handlers (cobra)
internal/api/    → HTTP API handlers
cmd/todo/        → CLI entrypoint
cmd/server/      → HTTP server entrypoint
data/            → Runtime data (not committed): .md files, .log files, tags.yaml
```

## Capture Syntax Grammar

```
input     = (token | text)* ["--" text]
token     = tag | source | urgent | important | note
tag       = "#" word                    (must exist in tags.yaml)
source    = "@" word
urgent    = "!!"                        (exactly two bangs)
important = "!!!"                       (exactly three bangs)
note      = "--note" quoted_string
escape    = "\#"                        (literal # in text)
separator = "--"                        (everything after is text)
```

- No tags → auto-tagged `#braindump`
- Source auto-adds matching tag (e.g., `@minecraft` → `#minecraft`)
- Unknown `#tag` → fuzzy suggest + prompt

## File Format

### Todo markdown (`/data/YYYY-MM-DD.md`)
```markdown
# 2026-03-21

- [ ] Todo text {id:a1b2c3} {source:cli} {status:inbox} {created:2026-03-21T14:32:00} {urgent:no} {important:no} {stale_count:0} #tag1 #tag2
  > Note text
  - [ ] Subtask
- [x] Done todo {id:d4e5f6} ...
```

- Todos stay in their **creation date** file forever
- Notes: `>` prefixed lines under the todo
- Subtasks: indented `- [ ]` lines, no metadata

### State change log (`/data/YYYY-MM-DD.log`)
```
2026-03-21T14:32:00 a1b2c3 created source=cli tags=homelab text="Fix server"
2026-03-21T15:00:00 a1b2c3 status inbox->active
```
- Log lives in the **creation date's** log file
- Only created when there's activity

### Empty date files
- Created on any interaction; gaps backfilled
- Empty file = just `# YYYY-MM-DD` header
- Empty file means "used system, no thoughts" vs missing = "didn't use"

## Status Model

```
Regular:  inbox → active (printed) → done
                  active stale after 24h, inbox stale after 7d
                → stale → revive → inbox (stale_count++)
Dump:     unprocessed → inbox → (same flow)
```

Statuses: `unprocessed`, `inbox`, `active`, `waiting`, `done`, `stale`

## Key Design Decisions

- **Capture-first**: Default action is capture, no subcommand needed
- **No smart parsing**: Reserved subcommand words (`ls`, `done`, `edit`, `delete`, `move`, `dump`, `tag`, `stale`, `looping`, `print`) always trigger their function
- **Use `--` separator** to capture text starting with a reserved word: `todo -- dump the drives`
- **Info line** shown after every CLI command (unprocessed count, active count, looping count)
- **tags.yaml** is the source of truth for valid tags
