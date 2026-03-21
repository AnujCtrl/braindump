# CLI Reference

## Capture (default)

```bash
todo <text> [#tags] [@source] [!!] [!!!] [--note "..."]
```

Default action — no subcommand needed. Creates a todo with status `inbox`.

### Inline syntax

| Token | Meaning | Example |
|-------|---------|---------|
| plain text | todo description | `fix the server` |
| `#word` | tag (must be in tags.yaml) | `#homelab` |
| `@word` | source override | `@minecraft` |
| `!!` | urgent=yes | `!!` |
| `!!!` | important=yes | `!!!` |
| `\#` | escaped literal `#` in text | `check issue \#42` |
| `--` | everything after is plain text | `todo -- dump the drives` |
| `--note "text"` | attach a note | `--note "555-1234"` |

### Tag resolution
- Unknown `#word` → prompt: `"#heatlh" not found. Did you mean #health? [Y/n/add as new]`
- No tags → auto-tagged `#braindump`
- Source auto-adds matching tag (e.g., `@minecraft` → `#minecraft`)
- Source defaults to `cli`

### Examples

```bash
todo fix the server                              # inbox, #braindump, source:cli
todo fix the server #homelab                     # inbox, #homelab, source:cli
todo fix AE2 #minecraft #deep-focus !!           # inbox, urgent, #minecraft #deep-focus
todo call dentist #health --note "555-1234"      # inbox, #health, with note
todo check issue \#42 on github #work            # text has literal "#42", tagged #work
todo -- dump the old hard drives                 # captures "dump the old hard drives"
```

## Dump Mode

```bash
todo dump [--tag tagname]
```

Interactive line-by-line brain dump. Each line is a separate todo.

- Inline syntax works per line
- Lines without tags → `#braindump`
- Empty line exits
- All items get status: `unprocessed`
- `--tag` applies a tag to all lines (per-line tags override)

```
$ todo dump
> fix the server #homelab
> buy groceries #errands !!
> check SSL cert
> that thing about DNS
>
Created 4 todos. (2 #braindump, 1 #homelab, 1 #errands)
```

## List

```bash
todo ls                           # today's file
todo ls --all                     # all files
todo ls --tag homelab             # filter by tag
todo ls --date 2026-03-20         # specific date
todo ls --looping                 # items with stale_count >= 2
```

## Complete

```bash
todo done                         # fuzzy picker from today's file
todo done <id>                    # complete by ID
```

## Edit

```bash
todo edit <id> <new text> [#tags] [!! !!!]
```

## Delete

```bash
todo delete <id>
```

## Move (Status Change)

```bash
todo move <id> <status>
```

Valid statuses: `unprocessed`, `inbox`, `today`, `waiting`, `done`, `stale`

## Stale

```bash
todo stale                        # show stale items
todo stale revive <id>            # move back to inbox, stale_count++
```

## Tags

```bash
todo tag list                     # list all tags
todo tag add <name>               # add a new tag
```

## Discovery Shortcuts

```bash
todo '#'                          # list all tags (quote the #)
todo '@'                          # list all sources
```

## Info Line

Shown after every command:

```
✓ Captured (a1b2c3)
── 📬 Unprocessed: 12 │ 🔁 Looping: 2 ──
```
