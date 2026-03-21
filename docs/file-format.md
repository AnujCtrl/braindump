# File Format Specification

## Todo Files (`/data/YYYY-MM-DD.md`)

Each day has one markdown file. Todos stay in the file of their **creation date** forever.

### Structure

```markdown
# 2026-03-21

- [ ] Todo text {id:a1b2c3} {source:cli} {status:inbox} {created:2026-03-21T14:32:00} {urgent:no} {important:no} {stale_count:0} #tag1 #tag2
  > Note line 1
  > Note line 2
  - [ ] Subtask one
  - [ ] Subtask two
- [x] Completed todo {id:d4e5f6} {source:cli} {status:done} {created:2026-03-21T09:15:00} {urgent:no} {important:yes} {stale_count:0} #errands
```

### Todo line format

```
- [checkbox] text {metadata}... #tags...
```

- **Checkbox**: `[ ]` (not done) or `[x]` (done)
- **Text**: The todo description (plain text, may contain escaped `\#`)
- **Metadata fields** (curly braces, space-separated):
  - `{id:XXXXXX}` — 6-char hex ID
  - `{source:SOURCE}` — origin (cli, minecraft, api)
  - `{status:STATUS}` — current status
  - `{created:TIMESTAMP}` — ISO 8601 creation time
  - `{urgent:yes|no}`
  - `{important:yes|no}`
  - `{stale_count:N}` — number of stale→revive cycles
- **Tags**: `#word` at end of line

### Notes

Lines starting with `>` (indented under the todo):

```markdown
- [ ] Call dentist {id:a1b2c3} ...
  > Phone: 555-1234
  > Ask about cleaning schedule
```

### Subtasks

Indented `- [ ]` lines under the todo. No metadata on subtasks:

```markdown
- [ ] Fix nether portal {id:a1b2c3} ...
  - [ ] Gather obsidian
  - [ ] Find flat area
```

### Empty date files

- Just the date header: `# 2026-03-22`
- Created on any app interaction
- Gaps between usage days are backfilled with empty files
- Empty file = "used system, captured nothing" (meaningful signal)
- Missing file = "didn't use system"

## State Change Log (`/data/YYYY-MM-DD.log`)

Log file lives alongside the **creation date's** `.md` file. All history for a todo is co-located.

### Format

```
TIMESTAMP ID ACTION [DETAILS]
```

### Event types

```
2026-03-21T14:32:00 a1b2c3 created source=cli tags=homelab,deep-focus text="Fix the server"
2026-03-21T15:00:00 a1b2c3 status inbox->today
2026-03-25T15:10:00 a1b2c3 edited field=text old="Fix the server" new="Fix the nether portal"
2026-03-25T17:45:00 a1b2c3 status today->done
2026-03-28T00:00:00 a1b2c3 status inbox->stale
2026-03-28T10:00:00 a1b2c3 revived stale_count=1
2026-03-28T10:00:00 a1b2c3 deleted
```

### Rules

- Log file only created when there's activity
- Actions logged in the **creation date's** log file, not the action date
- All history for a todo is in one place

## Tags File (`/data/tags.yaml`)

```yaml
categories:
  - homelab
  - minecraft
  - work
  - health
  - errands
  - learning
  - finance

energy:
  - quick-win
  - deep-focus
  - low-energy
  - braindump
```

All tags must be predefined. Unknown tags trigger a fuzzy-match prompt.
