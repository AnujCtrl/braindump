# braindump-android

Kotlin-native Android capture app. Not part of the Cargo workspace.

**Status:** Phase 0 placeholder. Real implementation lands in **Phase 6** of
[the v2 plan](../docs/plan-v2.md), after the desktop capture loop, sync
server, Mac parity, and metrics layer are validated on the user's machines.

## Why Kotlin instead of Tauri Mobile

Android system-level integration (quick-settings tile, voice intents, Tasker
compatibility) is far better-tooled in native Kotlin than in Tauri Mobile.
Code sharing with the desktop client is not a strong enough reason to give
that up — the surface area shared between phone and desktop capture is small
(local SQLite queue, sync POST). The win from Tauri Mobile (one parser
implementation) is outweighed by the loss in OS integration quality.

## Phase 6 scope

- Quick-settings tile that launches a capture activity.
- Capture activity: text input + voice toggle (`SpeechRecognizer`).
- Voice → text → store as **one** todo. No splitting in v2.
- POSTs to home server via Tailscale.
- Local Room DB queue for offline capture; drains on reconnect.

See [docs/plan-v2.md — Phase 6](../docs/plan-v2.md#phase-6--android-capture)
for the full spec when it's time to build this.
