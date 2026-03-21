# braindump

A fast, frictionless todo capture system that gets out of your way. Built for ADHD brains that need to dump thoughts before they vanish — not another project management tool.

## The problem

You're mid-build and remember you need to renew that SSL cert. By the time you open a todo app, unlock it, find the right project, and type it out — the thought is gone. Or worse, you captured it but the friction means you'll never do it again.

Braindump is capture-first. The default action is adding a todo. No subcommands, no menus, no friction.

## How it works

```bash
# capture mid-thought
$ todo fix the nether portal #minecraft #deep-focus
Created: fix the nether portal [a1b2c3d4]
-- Unprocessed: 0 | Looping: 0 --

# urgent stuff
$ todo buy groceries #errands !!
Created: buy groceries [e5f6a7b8]
-- Unprocessed: 0 | Looping: 0 --

# brain dump session — rapid-fire capture
$ todo dump
> fix AE2 autocrafting #minecraft
> buy groceries #errands !!
> check SSL cert #homelab
> that DNS thing
>
Created 4 todos.
-- Unprocessed: 4 | Looping: 0 --

# see what's on your plate
$ todo ls #minecraft
- [ ] fix the nether portal [a1b2c3d4] #minecraft #deep-focus
- [ ] fix AE2 autocrafting [c9d0e1f2] #minecraft

# move to today, mark done
$ todo move a1b2c3d4 today
$ todo done a1b2c3d4

# things you forgot about come back
$ todo stale
- [ ] that DNS thing [f3a4b5c6] (stale 12 days) #braindump
$ todo stale revive f3a4b5c6
```

## Install

### Go install

```bash
go install github.com/anujp/braindump/cmd/todo@latest
go install github.com/anujp/braindump/cmd/server@latest
```

### Docker

```yaml
# docker-compose.yaml
services:
  todo:
    image: ghcr.io/anujctrl/braindump:latest
    ports:
      - "8080:8080"
    volumes:
      - ./data:/data
    environment:
      - TODO_DATA_DIR=/data
```

The server runs stale checks automatically at startup and daily at midnight.

### Build from source

```bash
git clone https://github.com/AnujCtrl/braindump.git
cd braindump
go build ./cmd/todo/        # CLI binary
go build ./cmd/server/      # HTTP server
```

## Capture syntax

```
todo <text> [#tag] [@source] [!!] [!!!] [--note "text"] [-- literal text]
```

| Token | What it does |
|-------|-------------|
| `#tag` | Adds a tag (must exist in tags.yaml, or fuzzy-matched) |
| `@source` | Sets the source (auto-adds matching tag if one exists) |
| `!!` | Marks as urgent |
| `!!!` | Marks as important |
| `--note "text"` | Attaches a note |
| `\#` | Literal `#` in text (escaped) |
| `--` | Everything after is plain text, no parsing |

No tags? Auto-tagged `#braindump`. Unknown tag? Fuzzy-matched and suggested — or add it on the fly.

```bash
todo check issue \#42 on github #work
todo -- dump the old hard drives        # "dump" is a reserved word, so use --
todo call dentist #health --note "555-1234"
```

## Commands

| Command | Description |
|---------|-------------|
| `todo <text>` | Capture a todo (default action) |
| `todo dump` | Brain dump mode — multi-line rapid capture |
| `todo ls [#tag] [@source]` | List todos, optionally filtered |
| `todo done <id>` | Mark a todo as done |
| `todo edit <id>` | Edit text, tags, or priority |
| `todo delete <id>` | Permanently delete a todo |
| `todo move <id> <status>` | Move to a different status |
| `todo stale` | Show stale todos |
| `todo stale revive <id>` | Revive a stale todo back to inbox |
| `todo looping` | Show todos that have gone stale 2+ times |
| `todo tag list` | List all tags |
| `todo tag add <name>` | Add a new tag |
| `todo '#'` | Shortcut: list all tags |
| `todo '@'` | Shortcut: list all sources |

Every command prints an info line: `-- Unprocessed: N | Looping: M --`

## Status flow

```
                    ┌─────────┐
  dump ──────────► unprocessed ──► inbox ──► today ──► done
                                    │  ▲
                                    │  │ revive (stale_count++)
                                    ▼  │
                                   stale
                                 (7 days untouched)

                                 inbox ──► waiting
```

Todos that go stale twice or more are "looping" — they show up in `todo looping` so you can decide to actually do them or let them go.

## HTTP API

The server exposes a REST API on port 8080 for integration with scripts, shortcuts, and other tools.

```bash
# capture from a script
curl -X POST http://localhost:8080/api/todo \
  -H 'Content-Type: application/json' \
  -d '{"text":"test from script","tags":["homelab"],"source":"curl"}'

# list today's todos
curl http://localhost:8080/api/todo?date=2026-03-22

# system info
curl http://localhost:8080/api/info
```

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/todo` | `POST` | Create a todo |
| `/api/todo` | `GET` | List todos (query: `date`, `tag`, `status`, `source`) |
| `/api/todo/{id}` | `PUT` | Edit a todo |
| `/api/todo/{id}` | `DELETE` | Delete a todo |
| `/api/todo/{id}/status` | `PATCH` | Change status |
| `/api/dump` | `POST` | Bulk create todos |
| `/api/tags` | `GET` | List all tags |
| `/api/info` | `GET` | Unprocessed and looping counts |
| `/api/health` | `GET` | Health check |

## Minecraft integration

There's a companion Forge mod that lets you `/todo` from inside Minecraft without alt-tabbing. Captures are tagged with `source:minecraft` automatically.

**[braindump-mc](https://github.com/AnujCtrl/braindump-mc)** — requires this server running at `localhost:8080`.

```
/todo fix AE2 autocrafting #minecraft
/todo ls
```

## File format

Everything is stored as plain markdown and log files in the `data/` directory. Human-readable, git-friendly, no database.

```
data/
  2026-03-21.md    # todos created on this date
  2026-03-21.log   # state changes for that day's todos
  2026-03-22.md
  tags.yaml        # valid tags
```

Todos live in their creation date's file forever — status changes are tracked but the todo doesn't move files. Empty date files are created for days you used the system but had no thoughts (presence tracking, not just todo tracking).

```markdown
# 2026-03-21

- [ ] fix the nether portal {id:a1b2c3d4} {source:cli} {status:today} {created:2026-03-21T14:32:00} {urgent:no} {important:no} {stale_count:0} #minecraft #deep-focus
  > probably need more obsidian
- [x] buy groceries {id:e5f6a7b8} {source:cli} {status:done} {created:2026-03-21T14:33:00} {urgent:yes} {important:no} {stale_count:0} #errands
```

## Contributing

PRs welcome. This is a personal tool that scratches a specific itch, but if it's useful to you, I'd love contributions.

The architecture is intentionally simple: `internal/core/` is the single source of truth — a pure Go library that handles todos, tags, storage, stale detection, and logging. The CLI (`internal/cli/`) and HTTP API (`internal/api/`) are thin consumers of that core library. If you're adding a feature, it probably belongs in core.

Things that would be particularly useful:
- New integrations (more sources beyond CLI, API, and Minecraft)
- Better stale/looping heuristics
- TUI for processing unprocessed dumps

```bash
go test ./...    # run tests before submitting
go vet ./...     # lint
```

## License

MIT
