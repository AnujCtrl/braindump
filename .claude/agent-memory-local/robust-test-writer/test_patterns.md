---
name: test_patterns
description: Test patterns and gotchas for the braindump CLI test suite
type: feedback
---

Key testing patterns for this codebase:

1. **Cobra global state leakage**: The CLI uses global `var RootCmd` and global `store`/`logger`/`tagStore`. Tests MUST reset cobra flag state between `executeCmd()` calls using `pflag.Flag.Value.Set(f.DefValue)`. The `resetFlags()` helper handles this.

2. **os.Setenv for TODO_DATA_DIR**: Tests set env vars with `os.Setenv` which is process-global. Each test's `setupTestEnv` creates a fresh temp dir and sets the env, but cleanup only runs at test end.

3. **ParseDayFile vs ParseTodoBlock**: `ParseTodoBlock` handles subtasks correctly. `ParseDayFile` has a bug where subtasks are misidentified as new todos. Use `ParseTodoBlock` for unit testing individual todo serialization, but test through `ParseDayFile` for integration.

4. **Round-trip tests through disk**: Always test Store.Add -> Store.Read cycles to verify file format fidelity.

5. **Braces in text**: Trailing `{word}` in user text gets silently consumed by the parser. Middle braces survive. Always test round-trip with braces at different positions (start, middle, end) when touching the parser.

6. **Cobra `--` handling**: Cobra consumes `--` as end-of-flags marker before the app sees it. `todo -- #work text` makes `#work` a positional arg parsed as tag. The braindump `--` separator only works mid-sentence: `todo text -- more text`.

7. **GenerateID collision**: 3-byte IDs (16M values) collide around 5000-10000 IDs via birthday paradox. Keep uniqueness tests under 1000 to avoid flaky failures.

**Why:** These patterns caused real test failures during audits.
**How to apply:** Follow these patterns when adding any new tests.
