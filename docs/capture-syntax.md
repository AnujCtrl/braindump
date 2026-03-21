# Capture Syntax Guide

## Overview

The default action is capture — no subcommand needed.

```bash
todo fix the server #homelab #deep-focus @minecraft !!urgent !!!important
```

## Token Grammar

```
input     = (token | text)* ["--" text]
token     = tag | source | urgent | important | note
tag       = "#" word
source    = "@" word
urgent    = "!!"         (exactly two bangs, not part of "!!!")
important = "!!!"        (exactly three bangs)
note      = "--note" quoted_string
escape    = "\#"         (literal # in text)
separator = "--"         (everything after is plain text)
```

## Parsing Rules

### Token extraction order
1. Extract `--note "..."` if present
2. Split remaining args on spaces
3. For each token:
   - `!!!` → important=yes
   - `!!` (but not `!!!`) → urgent=yes
   - `#word` → tag (validate against tags.yaml)
   - `@word` → source
   - `\#word` → literal text `#word`
   - `--` → stop parsing, rest is text
   - anything else → text

### Tag resolution
1. Check if tag exists in tags.yaml
2. If not found, compute fuzzy match
3. Prompt: `"#heatlh" not found. Did you mean #health? [Y/n/add as new]`
4. If no tags after parsing → auto-add `#braindump`

### Source resolution
1. If `@source` given, use it
2. If source matches a tag name, auto-add that tag
3. If no source → default based on entry point:
   - CLI → `cli`
   - HTTP API → `api`
   - Minecraft mod → `minecraft`

## Examples

```bash
# Simple capture
todo fix the server
# → text="fix the server", tags=[braindump], source=cli

# With tags
todo fix the server #homelab
# → text="fix the server", tags=[homelab], source=cli

# Multiple tags + urgent
todo fix AE2 #minecraft #deep-focus !!
# → text="fix AE2", tags=[minecraft, deep-focus], urgent=yes

# With note
todo call dentist #health --note "555-1234"
# → text="call dentist", tags=[health], note="555-1234"

# Escaped hash
todo check issue \#42 on github #work
# → text="check issue #42 on github", tags=[work]

# Explicit separator
todo -- dump the old hard drives
# → text="dump the old hard drives", tags=[braindump]

# Source override
todo fix AE2 @minecraft
# → text="fix AE2", source=minecraft, tags=[minecraft] (auto-added)

# Everything
todo fix server !! !!! #homelab @minecraft --note "urgent fix"
# → text="fix server", urgent=yes, important=yes, tags=[homelab, minecraft], source=minecraft
```

## Reserved Subcommands

These words as the first argument trigger subcommands, NOT capture:

`ls`, `done`, `edit`, `delete`, `move`, `dump`, `tag`, `stale`, `looping`

Use `--` to capture text starting with these words:

```bash
todo -- dump the old hard drives    # captures, does NOT enter dump mode
todo -- edit the config file        # captures, does NOT enter edit mode
```
