---
name: bugs_found
description: All bugs discovered during test audits of braindump CLI - braces data loss, subtask parsing, tag shorthand, ID collision risk
type: project
---

Bugs found across test audit sessions:

1. **CRITICAL: Trailing braces in text cause DATA LOSS** - `ParseTodoLine()` in `internal/core/todo.go` strips trailing `{text}` from user text. The metadata extraction loop (lines 70-116) works right-to-left, consuming any `{...}` block at the end of the remaining text. If user text ends with braces (e.g., "check the {config}"), it's silently consumed. Braces in the MIDDLE of text survive because non-brace characters follow them. Even worse: if the braces contain a colon like `{status:broken}`, it overwrites real metadata values.
   - Fix: After stripping known metadata keys, don't continue stripping unrecognized brace blocks. Or use a more robust delimiter between text and metadata.

2. **#tag shorthand in `ls` is silently ignored** - `todo ls #homelab` shows ALL todos. Root cause: `runList()` only reads the `--tag` flag, ignores positional args.

3. **ParseDayFile subtask parsing bug** - Indented subtask lines (`  - [ ] subtask`) are parsed as new top-level todos. Root cause: `TrimSpace` strips indentation, then `HasPrefix(trimmed, "- [")` matches before the continuation check.

4. **ID collision risk** - `GenerateID()` uses only 3 bytes (16M possible values). Birthday paradox gives ~0.3% collision per 10K IDs. The system has a retry loop but the space is small. In testing, collisions were observed at ~7600 IDs.

5. **API doesn't validate tags against tags.yaml** - The API accepts any tag strings without checking `tags.yaml`. The CLI validates tags but the API does not.

6. **AddTag doesn't deduplicate** - Calling `AddTag("work")` when "work" already exists appends a duplicate.

**Why:** Bugs 1-3 affect real user behavior and data integrity.
**How to apply:** Bug 1 is the most critical — any user whose todo text ends with `{word}` silently loses data. Bug 3 means subtasks cannot be reliably used until fixed.
