# braindump

A capture-first todo system for ADHD brains. You think it, you press the hotkey, you type, it's gone — the thought is captured before it can vanish. No app to open, no project to find, no friction.

> **v2 is in active development.** This README documents the v2 architecture. v1 (Go and TypeScript) is still on `main` and used as reference material while v2 is built. See [docs/plan-v2.md](docs/plan-v2.md) for the full multi-phase plan.

## Architecture (v2)

**Local-first, device-autonomous.** Each client has its own SQLite. The home server is a dumb sync hub — relay + canonical replica — reachable only via Tailscale. Clients work fully offline; the server can die without affecting capture or the dashboard. Logic lives on the clients, not the server.

```
┌─────────────────────┐         ┌─────────────────────┐
│ Linux desktop       │         │ Mac desktop         │
│ ┌─────────────────┐ │         │ ┌─────────────────┐ │
│ │ braindump-      │ │         │ │ braindump-      │ │
│ │   desktop       │ │         │ │   desktop       │ │
│ │  (Tauri)        │ │         │ │  (Tauri)        │ │
│ │   capture win   │ │         │ │   capture win   │ │
│ │   dashboard win │ │         │ │   dashboard win │ │
│ └────────┬────────┘ │         │ └────────┬────────┘ │
│          │          │         │          │          │
│   local SQLite      │         │   local SQLite      │
│   sync_queue        │         │   sync_queue        │
└──────────┼──────────┘         └──────────┼──────────┘
           │                               │
           │      Tailscale (no auth,      │
           │      no public endpoint)      │
           │                               │
           └──────────┬────────────────────┘
                      ▼
           ┌─────────────────────┐
           │ home server         │
           │ ┌─────────────────┐ │
           │ │ braindump-server│ │
           │ │  (axum)         │ │
           │ │  push / pull    │ │
           │ └────────┬────────┘ │
           │   canonical SQLite  │
           │   (replica only)    │
           └──────────┼──────────┘
                      ▲
                      │
           ┌──────────┴──────────┐
           │ Android (Kotlin)    │
           │   quick-settings    │
           │   tile + voice      │
           │   Room queue        │
           └─────────────────────┘
```

### Conflict resolution: last-write-wins

If two devices edit the same todo while offline, the one with the later `updated_at` timestamp wins when both sync. This is a deliberate rule, not a bug. With a single user across 2–3 devices, conflicts are rare and the simpler model beats CRDTs for this scale.

## Crates

| Crate | Purpose |
|-------|---------|
| [`crates/core`](crates/core) | Shared logic: parser, status state machine, storage, sync types. |
| [`crates/desktop`](crates/desktop) | Tauri app — registers global hotkey, capture window, dashboard window. |
| [`crates/server`](crates/server) | Sync hub on the home lab. axum HTTP server, canonical SQLite replica, no business logic. |
| [`android/`](android) | Kotlin-native Android capture app. Not part of the Cargo workspace. |

## Build

```bash
cargo build --workspace            # all Rust crates
cargo tauri dev --config crates/desktop/tauri.conf.json   # desktop app
cargo run -p braindump-server      # sync server
cargo test --workspace             # all tests
cargo clippy --workspace -- -D warnings
cargo fmt --all
```

CI runs fmt, clippy, tests, and a launch smoke test for the desktop binary — see [`.github/workflows/rust.yml`](.github/workflows/rust.yml).

## Capture hotkey

The desktop app registers `Ctrl+Shift+;` by default (override with `BRAINDUMP_HOTKEY=...`).

**macOS** uses Tauri's native global shortcut. First run, grant Accessibility permission when prompted.

**Linux/Hyprland (Wayland)** can't use a native global shortcut — Wayland's security model refuses system-wide key registration. Use the compositor binding instead:

```hyprland
# ~/.config/hypr/hyprland.conf
exec-once = braindump-desktop
bind = CTRL SHIFT, semicolon, exec, braindump-desktop --toggle
```

`--toggle` connects to the running instance over a Unix socket (`$XDG_RUNTIME_DIR/braindump.sock`) and asks it to show/hide. Same pattern works on any tiling WM with `bind`/`exec` directives.

## Phase status

The v2 build is staged in 7 phases. **Phase 1 is a hard validation gate** — if capture isn't being used daily after Phase 1, the rest of the plan is wasted effort.

| Phase | Status | Description |
|-------|--------|-------------|
| 0 — Repo bootstrap | **in progress** | Cargo workspace, CI, architecture diagram |
| 1 — Linux capture loop | not started | Tauri shell, global hotkey, parser, local SQLite |
| 2 — Logic layer | not started | Stale, rollover, "this week", Sunday auto-populate |
| 3 — Sync server | not started | axum, push/pull, last-write-wins |
| 4 — Mac parity | not started | Cross-platform capture + sync |
| 5 — Metrics | not started | Bi-weekly report data structure |
| 6 — Android capture | not started | Kotlin app, quick-settings tile, voice |
| 7 — Dashboard wiring | blocked | Waiting on Claude Design output |

See [docs/plan-v2.md](docs/plan-v2.md) for full per-phase definitions of done and verification.

## v1 (legacy reference)

v1's TypeScript code lives under `src/`, `tests/`, and the older Go code under `cmd/` + `internal/`. **Don't extend either.** They remain in-tree only as reference material — see ["v1 reference points" in the plan](docs/plan-v2.md#v1-reference-points-do-not-migrate-just-consult) for the specific files to consult while building Rust equivalents. Both will be removed once v2 reaches feature parity.

v1's `data/` directory is **not** migrated — the user accepts losing the small amount of v1 state in exchange for a clean v2 schema.

## License

MIT
