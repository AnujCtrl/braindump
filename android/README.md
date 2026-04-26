# braindump-android

Kotlin-native Android capture app. **Phase 6 of [the v2 plan](../docs/plan-v2.md).** Lives outside the Cargo workspace.

## Status: source committed, unverified

This was authored without an Android toolchain in the loop. Code follows current Android best practices (Compose, Room with KSP, kotlinx-serialization, WorkManager, single-instance translucent capture activity, QS tile service) but has not been compiled. **First build will probably surface 1–3 small fixes** — import nits, version bumps, or resource-path tweaks. Treat the source as ~95% there, not pushed-to-main quality.

## What it does

| Surface | What |
|---------|------|
| Quick-Settings tile | One tap from anywhere → capture activity |
| Capture activity | Compose UI: textarea, voice (`SpeechRecognizer`), save / cancel, settings gear |
| Share-sheet target | Long-press text in any app → share to braindump → lands as a todo |
| Tasker / shortcut | `am start -a com.braindump.android.CAPTURE --es text "..."` |
| Local DB | Room — `todos` (snapshot) + `queue` (outbound) |
| Sync | Periodic WorkManager (~30 min) + immediate one-shot after capture |
| Boot receiver | Re-arms periodic sync on reboot |

Voice ⇒ text via the system `RecognizerIntent` (works offline on most modern phones via Google's on-device speech). One transcript becomes one todo — **no splitting in v2** (per the plan). User splits manually on Sunday.

## Build

You'll need:
- JDK 17
- Android SDK (Platform 35, Build-Tools 34+)
- `ANDROID_HOME` set, or use Android Studio

The Gradle wrapper jar isn't committed. Either install Gradle 8.10+ system-wide and add the wrapper:

```bash
cd android
gradle wrapper --gradle-version 8.10
./gradlew assembleDebug
```

…or open `android/` in Android Studio (Hedgehog or later) and let it sync.

Output: `android/app/build/outputs/apk/debug/app-debug.apk`.

## Install

```bash
adb install -r android/app/build/outputs/apk/debug/app-debug.apk
```

Then on the device:
1. Open the app once. It'll show the capture screen.
2. Tap the **gear** → enter your home server's Tailscale URL (e.g. `http://braindump.tail-scale.ts.net:8181`). Save.
3. Pull down the QS tray → drag the **braindump** tile into your active tiles.
4. Tap the tile from anywhere to capture.

## Configuration

Server URL can be set three ways:

| Path | When |
|------|------|
| Settings gear in capture screen | Normal user flow |
| `adb shell am start -n com.braindump.android.debug/.SettingsActivity` then type | Power-user / scripted setup |
| Hand-edit shared prefs via root | Last resort |

**No auth field.** Tailscale gates access at the network layer and this is single-user. Don't expose the server publicly.

## Why Kotlin native (not Tauri Mobile)

System-level integration (QS tile, voice intents, share-sheet, Tasker) is far better-tooled in native Kotlin than in Tauri Mobile, and the surface shared with the desktop client is small (capture text → enqueue → POST). The win from Tauri Mobile (one parser implementation) is outweighed by the OS-integration loss. See the plan for the full reasoning.

## Wire format

The app POSTs to `/sync/push` with the same JSON shape `crates/server` accepts — `core::sync::SyncPush`. See [`net/BraindumpApi.kt`](app/src/main/kotlin/com/braindump/android/net/BraindumpApi.kt). Field names match `core::Todo` byte-for-byte (snake_case via `@SerialName`) so the same payload that the desktop client sends round-trips here too.

## What's missing (good follow-ups)

- **Pull, not just push.** Currently the Android app only pushes; desktop captures don't appear here. The pull half is mechanical — add a `GET /sync/pull?since=…` to `BraindumpApi`, store a cursor in prefs, apply via Room upserts.
- **Capture-syntax parser.** Android currently treats the input as a single body string and auto-tags `#braindump`. The Rust grammar (`#tag`, `^^`, `@source`, `--note`) would need a Kotlin port for full parity.
- **Mobile dashboard.** Out of scope for v2.
