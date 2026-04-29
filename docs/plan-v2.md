# Braindump v2 — Plan (everything except the dashboard frontend)

## Context

Total rewrite of the existing TypeScript+Bun+SQLite codebase to **Rust + Tauri + rusqlite**. Linear is removed entirely (the free-tier 150-ticket cap would fill quickly under honest use, and v1's coupling is deep — see "Linear removal scope" below). Architecture is **local-first**: each client is fully autonomous, the home server is a dumb sync hub over Tailscale. Single user; no auth.

The user has explicitly accepted that the build is multi-month with AI codegen. The Phase 1 validation gate is the structural defense against this becoming v3 by abandonment.

## Architecture

- **Monorepo (Cargo workspaces).** Packages: `core` (logic, parser, models, storage), `desktop` (Tauri app — capture window + dashboard window in one binary), `server` (sync hub), `android` (capture, Kotlin native).
- **Local-first.** Every client has its own SQLite. Writes go to local DB and a sync queue. Server maintains a canonical replica and relays between devices.
- **Conflict resolution: last-write-wins** by `updated_at` timestamp. Documented as a rule, not a bug. Visible in README and in code comments.
- **Capture is part of the desktop app.** The Tauri binary registers the OS-level global hotkey, summons a frameless window, dismisses on submit. Same binary opens the dashboard window via a separate hotkey or tray icon.
- **No Linear, no Raycast.** Tauri's `tauri-plugin-global-shortcut` handles Mac and Linux uniformly.
- **Logic lives on each client**, not on the server. The server is plumbing only — receives writes, distributes to other clients, persists for durability. Clients work fully offline; the server can die without affecting capture or the dashboard.

## v1 reference points (do not migrate, just consult)

The existing TS code is **reference material for logic shape**, not something to port. Read these files when implementing the corresponding Rust pieces:

- Parser grammar: `src/core/parser.ts` — confirms tokens are `^^` (urgent), `^^^` (important), `#tag`, `@source`, `--note "..."`, `\#` escape, `--` literal-rest separator.
- Status enum: `src/core/models.ts:1-7` — `unprocessed | inbox | active | waiting | done | stale`. (v1 README incorrectly says `today` in places — ignore; the code says `active`.)
- Stale rules: `src/core/stale.ts:15-31` — inbox stale after 7 days, active stale after 24 hours.
- Sync queue shape: `src/core/db.ts:30-38` — fields `id, todo_id, action, payload, created_at, attempts, last_error`.
- Schema: `src/core/models.ts:18-34` — Rust schema below drops `linearId` and `subtasks`; everything else carries forward.

Don't preserve TS code. Don't migrate v1 user data — minimal v1 state.

## Linear removal scope (informational; affects Phase 0 mental model)

v1's Linear coupling is more than a column: `linear_id` UNIQUE column on `todos`, plus full `labels` and `workflow_states` tables, plus `LinearBridge` calls in `src/core/sync.ts`. The Rust schema simply omits all of it from the start — there's no migration to write, just a fresh schema without those concepts.

---

## Phase 0 — Repo bootstrap

**Goal:** monorepo skeleton with CI.

**Work:**
- Initialize Cargo workspace with `core`, `desktop`, `server`, `android` packages. (Android is Kotlin — included in the monorepo for colocated docs/issues, but not a Cargo crate.)
- Set up CI (GitHub Actions): `cargo fmt --check`, `cargo clippy -- -D warnings`, `cargo test`.
- Architecture diagram in README, including the last-write-wins conflict rule.

**Reasoning:** Foundation prevents future restructure. CI catches Rust mistakes early — the user is reading code, not writing it, so machine review is the only review.

**Definition of done:** `cargo build` succeeds across all Rust packages; CI green on a no-op PR.

---

## Phase 1 — Linux capture loop (THE validation gate)

**Goal:** capture works on the user's Linux machine, end-to-end, locally. **STOP HERE BEFORE CONTINUING.**

**Work:**
- Tauri app shell, runs on login (Hyprland `exec-once`).
- Global hotkey via `tauri-plugin-global-shortcut`. Default `Ctrl+Shift+;`. Configurable.
- Frameless, centered, focused capture window. Multi-line input. Soft single-tick (~80ms) on Enter. On submit: dismiss, return focus to previous window.
- Local SQLite via `rusqlite` with WAL mode. Schema:
  - `todos` (id, text, status, source, urgent, important, stale_count, tags JSON, notes JSON, created_at, status_changed_at, updated_at, done). **No `linear_id`. No `subtasks`** — v2 drops them; if a captured item is multi-step, user splits via dump mode.
  - `tags` (name PK, created_at) — replaces v1's documented-but-unimplemented `tags.yaml`. Tag set lives in SQLite and syncs across devices like everything else.
  - `sync_queue` (mirror of v1's shape: id, todo_id, action, payload, created_at, attempts, last_error).
- **Rust parser** (highest-bug-density component — test heavily):
  - Tokens: `#tag`, `^^` (urgent), `^^^` (important), `@source`, `--note "..."`, `\#` escape, `--` literal-rest separator.
  - **Dump mode**: lines beginning with `-` inside the capture window are split into separate todos. Replaces v1's interactive `todo dump` subcommand (which can't translate to a popup window UX).
  - **Match the existing TS grammar exactly** for the per-todo tokens. Cross-check against `src/core/parser.ts`.
  - Property-based tests via `proptest` for the parser specifically — disproportionate test effort here is justified by the bug density.
- Fuzzy match unknown `#tag` against the `tags` table; suggest existing close matches OR auto-add when no close match exists. Prompt is in-line in the capture window (no second dialog).
- Hidden CLI sub-binary for `list` / `dump` — dev sanity only, not a user-facing surface.

**Reasoning:** This is the early-warning system. If after Phase 1 the user is not capturing daily, v2 is failing the same way v1 failed and the rest of the build is wasted effort. The validation gate exists precisely because v1 was never installed — Phase 1 must be installed and used before any further work.

**Definition of done:** binary installed on Linux machine, hotkey captures into local SQLite, soft tick fires, capture window dismisses cleanly. **User has captured ≥3 todos/day for 5 of 7 days before Phase 2 starts.** Non-negotiable.

---

## Phase 2 — Logic layer (stale, rollover, "this week")

**Goal:** the data the dashboard will eventually read is correct and rich.

**Work:**
- Status state machine: `unprocessed | inbox | active | waiting | done | stale`. (v1 README says `today`; ignore — actual enum uses `active`. v2 uses `active`.)
- Stale detection background tick (configurable interval, default daily at midnight local). Constants: inbox → 7 days, active → 24 hours (mirrors `src/core/stale.ts:15-31`).
- "This week" assignment: items can be pulled into a weekly bucket via API and (later) the dashboard.
- **Sunday auto-populate** (when user skipped pulling): pick items in priority order:
  1. `urgent` AND `important`
  2. `urgent` only
  3. `important` only
  4. Stop.
  Cap at 7. Idempotent — won't re-pull if user has already pulled manually.
- **Rollover engine:** end-of-week, items in "this week" not marked `done` roll to next week. Items rolled twice without action → stale.
- Skipped-Sunday tracking — append to an event log table.
- Status-change log preserved (audit trail for analytics).

**Reasoning:** Logic must be in place *before* the dashboard is built, because the dashboard reads this state. Building it now means real data is there to render against when design lands.

**Definition of done:** unit tests for stale detection, rollover, Sunday auto-populate, "this week" assignment. Manual test: artificially advance time (test fixture), observe correct state transitions.

---

## Phase 3 — Sync server + multi-device plumbing

**Goal:** invisible sync between devices.

**Work:**
- Rust sync server using `axum`. Runs on home lab, exposed via Tailscale only.
- Sync API: push (client → server), pull (server → client). Conflict resolution: last-write-wins by `updated_at`.
- Server maintains its own SQLite as canonical replica.
- Client sync queue: writes go to local DB **and** queue; queue drains to server periodically and on app open. Pattern mirrors v1's `sync_queue` table.
- Tailscale-only, no auth. Hardcode the server's tailnet hostname in client config.
- On Tailscale-flaky: backoff and retry. Log to console — never silent failure.
- No public endpoint, no codesigning, no TLS (Tailscale handles transport security).
- Server has **no business logic**. Relay + replica only. Reject malformed payloads; persist + broadcast valid ones.
- **Tag table syncs too** — adding a new tag on one device propagates to others.

**Reasoning:** Invisible plumbing that doesn't expand UX yet. Build it before adding device 2 so device 2 isn't an emergency port.

**Definition of done:** server runs on home lab. Linux client syncs both directions. Manual test: kill server, capture offline, restart server, verify queue drains correctly.

---

## Phase 4 — Mac parity

**Goal:** identical capture UX on Mac, sync working between Linux and Mac.

**Work:**
- Same Tauri binary builds for Mac (`cargo tauri build --target universal-apple-darwin` or per-arch).
- Verify global hotkey works on Mac. May require Accessibility permission flow on first run — handle this gracefully with a one-time dialog.
- Frameless window matches Linux behavior exactly.
- Login item registered on first run.
- Test: capture on Mac while Linux is online → todo appears on Linux within sync interval.

**Reasoning:** Tests cross-platform AND cross-device sync simultaneously. The Linux↔Mac path is where bugs hide.

**Definition of done:** user has captured from both machines in one day, sees todos appear on both within ~30s. No UX delta between machines.

---

## Phase 5 — Metrics computation + bi-weekly report data

**Goal:** all four metrics are computable from local SQLite. Report data structure is generatable.

**Work:**
- Metric queries:
  - **Capture rate** — rolling 28-day count, day-by-day.
  - **Return rate** — requires app-open event log. Add a `dashboard_opens` table; log on every dashboard window open.
  - **Inbox sanity** — 4-week unprocessed pile size shape.
  - **Sundays skipped** — already tracked in Phase 2.
- Bi-weekly trigger logic: "is today a report day?" — every other Monday from a fixed anchor date.
- Report data structure (Rust struct serializable to JSON) that the dashboard's modal will eventually render.
- **No UI work here.** Just Rust computation and the data shape.

**Reasoning:** Compute lives in Rust, render lives in the (forthcoming) dashboard. Doing the compute now means the dashboard can render immediately when design lands.

**Definition of done:** running a CLI command outputs a JSON dump of the current bi-weekly report against real captured data.

---

## Phase 6 — Android capture

**Goal:** phone capture works for the 50% of moments that aren't at a desk.

**Work:**
- **Decision: Kotlin native, not Tauri Mobile**, unless Tauri Mobile has matured significantly since planning. Android system-level integration (quick-settings tile, voice intents, Tasker compatibility) is far better-tooled in native Kotlin.
- Quick-settings tile that launches a capture activity. (Confirm gesture-vs-tile preference with user before building — they expressed interest in a gesture but tile is more reliable.)
- Capture activity: text input + voice toggle button. Voice via `SpeechRecognizer` → text → store as **one** todo. No splitting in v2 — user splits manually on Sunday.
- POSTs to home server via Tailscale.
- Local Room DB queue for offline capture.
- Drain queue when network returns.

**Reasoning:** Mobile capture is 50% of the user's capture surface. Skipping it means v2 fails at the same friction point v1 did. Reels, dinner, meetings, games — none of these are at a desk.

**Definition of done:** APK installed on user's Android. Captures sync to Linux/Mac dashboard within ~30s when Tailscale is up. Offline captures drain on reconnect.

---

## Phase 7 — Wait for design, then wire up the dashboard

**Goal:** receive Claude Design output, integrate into the desktop app's WebView.

**Work:** out of scope for this plan. Triggered when the design brief produces deliverables.

**Definition of done:** dashboard renders in the desktop app, reads from local SQLite via Tauri commands, displays bi-weekly report modal on report day.

---

## Critical files / paths to consult while building

- v1 parser (grammar reference): `src/core/parser.ts`
- v1 schema (field reference): `src/core/db.ts`, `src/core/models.ts`
- v1 stale logic (constants): `src/core/stale.ts`
- v1 sync action shape: `src/core/sync.ts` (note: strip out all `LinearBridge` calls — that's exactly what's removed)
- v1 CLAUDE.md (capture-syntax grammar spec): the source of truth for the parser spec; v2 inherits this verbatim except `!!`/`!!!` are not present in v1 (use `^^`/`^^^`) and dump becomes `-` prefix.

## Verification

**Per-phase verification is in each phase's "Definition of done" section.** Cross-cutting verification:

- **Parser:** `cargo test -p core parser::` — including `proptest`-based property tests. Round-trip a corpus of real captures from the user's v1 data (if any) through the Rust parser and confirm output matches the TS parser's output.
- **Sync:** Two-machine integration test by hand. Capture on A while B is offline, then B comes online; capture on B while A is offline; both online simultaneously.
- **Stale/rollover:** Time-travel test fixture. Inject a fake `now()` and walk the state machine through 14 days of activity; assert expected end state.
- **Hotkey:** Cannot be unit-tested. Phase 1's "user has captured ≥3 todos/day for 5 of 7 days" is the verification — if the hotkey is broken, this metric won't pass.

## Risks called out

- **Rust rewrite cost:** the parser is the highest-density bug area. Spend disproportionate testing effort there. Property-based tests (`proptest`) are worth it for the parser specifically.
- **Time estimate:** the user has accepted multi-month. The Phase 1 gate is the only structural defense against v2 going the way of v1.
- **Single-user durability:** with Linear gone, off-device backups of the home server's SQLite are the only DR. Set up a `cron` snapshot to a separate disk. Confirm it's running before Phase 6.
- **Conflict resolution surprise:** last-write-wins is fine but document it visibly in the README and in code comments. If two devices edit the same todo offline, the later `updated_at` wins. Don't surprise the user.
- **Tauri Mobile vs Kotlin (Phase 6):** if Tauri Mobile is chosen for code-sharing reasons, expect Android capture parity work to expand significantly. Default Kotlin unless there's a strong reason.
- **The validation gate at Phase 1 is the most important risk control in this entire plan.** Skipping it (or treating it as soft) is how v2 fails the same way v1 did.

## What this plan does not include

- The dashboard frontend (HTML/CSS/JS components inside the WebView). Blocked on design.
- Mobile dashboard view (deferred to post-v2).
- Onboarding, settings UI, theme switcher, account UI — single user, no auth, no settings beyond hotkey and tag set.
- Linear sync (intentional removal).
- Public endpoint, TLS, auth — Tailscale handles all of this.
- Subtasks (dropped from v2 — capture multi-step items via dump mode instead).
