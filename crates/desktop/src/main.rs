//! braindump-desktop: capture window with a global hotkey.
//!
//! Lifecycle:
//! 1. App launches at login (Hyprland `exec-once` on Linux, Login Item on Mac).
//! 2. Capture window is created hidden.
//! 3. Global shortcut [`DEFAULT_HOTKEY`] toggles the window — show + focus.
//! 4. Frontend (`dist/`) submits via the `submit_capture` Tauri command,
//!    which delegates to [`braindump_core::capture`]. On success it
//!    dismisses the window after a soft tick.
//!
//! ## Open work for Phase 1 validation
//!
//! - `data_dir()` currently uses the OS-standard project dirs; verify on
//!   Linux that the WAL companion files land in the right place.
//! - The dashboard window is not wired up here — it'll be added in Phase 7
//!   when design lands. The capture window is the gated MVP.

use anyhow::Result;
use braindump_core::{Store, capture};
use chrono::Utc;
use serde::Serialize;
use std::path::PathBuf;
use std::sync::Mutex;
use tauri::{AppHandle, Manager, State, WebviewWindow};
use tauri_plugin_global_shortcut::{Code, GlobalShortcutExt, Modifiers, Shortcut, ShortcutState};

/// Default capture hotkey. Can be overridden by an env var (`BRAINDUMP_HOTKEY`)
/// for the user's per-machine preference; respected on first launch.
const DEFAULT_HOTKEY: &str = "ctrl+shift+;";

/// State held in the Tauri app — the SQLite store, behind a sync mutex
/// because Tauri command handlers are sync (rusqlite is also sync).
struct AppState {
    store: Mutex<Store>,
}

#[derive(Debug, Serialize)]
struct CaptureResult {
    count: usize,
    last_id: Option<String>,
}

#[tauri::command]
fn submit_capture(state: State<'_, AppState>, input: String) -> Result<CaptureResult, String> {
    let mut store = state.store.lock().map_err(|e| e.to_string())?;
    let mut count = 0usize;
    let mut last_id: Option<String> = None;
    let now = Utc::now();

    // Dump-mode: lines starting with '-' split into separate todos. Single
    // captures (no '-' prefix) are also handled by this loop because the
    // first iteration sees the entire input.
    let has_dump = input.lines().any(|l| l.trim_start().starts_with('-'));
    if has_dump {
        for line in input.lines() {
            let trimmed = line.trim();
            let item = trimmed
                .strip_prefix('-')
                .map(str::trim_start)
                .unwrap_or(trimmed);
            if item.is_empty() {
                continue;
            }
            match capture(&mut store, item, now, "desktop") {
                Ok(o) => {
                    last_id = Some(o.todo.id);
                    count += 1;
                }
                Err(e) => return Err(e.to_string()),
            }
        }
    } else {
        match capture(&mut store, &input, now, "desktop") {
            Ok(o) => {
                last_id = Some(o.todo.id);
                count = 1;
            }
            Err(e) => return Err(e.to_string()),
        }
    }

    Ok(CaptureResult { count, last_id })
}

fn data_dir() -> Result<PathBuf> {
    if let Ok(custom) = std::env::var("BRAINDUMP_DATA_DIR") {
        return Ok(PathBuf::from(custom));
    }
    let dirs = directories_next::ProjectDirs::from("com", "braindump", "desktop")
        .ok_or_else(|| anyhow::anyhow!("could not resolve project dirs"))?;
    Ok(dirs.data_dir().to_path_buf())
}

fn parse_hotkey(s: &str) -> Result<Shortcut> {
    let mut mods = Modifiers::empty();
    let mut code: Option<Code> = None;
    for part in s.split('+').map(str::trim) {
        match part.to_lowercase().as_str() {
            "ctrl" | "control" => mods |= Modifiers::CONTROL,
            "shift" => mods |= Modifiers::SHIFT,
            "alt" | "option" => mods |= Modifiers::ALT,
            "cmd" | "meta" | "super" => mods |= Modifiers::SUPER,
            ";" | "semicolon" => code = Some(Code::Semicolon),
            "space" => code = Some(Code::Space),
            "tab" => code = Some(Code::Tab),
            other if other.len() == 1 => {
                let c = other.chars().next().unwrap().to_ascii_uppercase();
                if c.is_ascii_alphabetic() {
                    let key_idx = (c as u8) - b'A';
                    let key_codes = [
                        Code::KeyA,
                        Code::KeyB,
                        Code::KeyC,
                        Code::KeyD,
                        Code::KeyE,
                        Code::KeyF,
                        Code::KeyG,
                        Code::KeyH,
                        Code::KeyI,
                        Code::KeyJ,
                        Code::KeyK,
                        Code::KeyL,
                        Code::KeyM,
                        Code::KeyN,
                        Code::KeyO,
                        Code::KeyP,
                        Code::KeyQ,
                        Code::KeyR,
                        Code::KeyS,
                        Code::KeyT,
                        Code::KeyU,
                        Code::KeyV,
                        Code::KeyW,
                        Code::KeyX,
                        Code::KeyY,
                        Code::KeyZ,
                    ];
                    code = Some(key_codes[key_idx as usize]);
                }
            }
            _ => {}
        }
    }
    let code = code.ok_or_else(|| anyhow::anyhow!("hotkey '{s}' has no key code"))?;
    Ok(Shortcut::new(Some(mods), code))
}

fn toggle_capture(app: &AppHandle) {
    if let Some(win) = app.get_webview_window("capture") {
        match win.is_visible() {
            Ok(true) => {
                let _ = win.hide();
            }
            _ => {
                show_and_focus(&win);
            }
        }
    }
}

fn show_and_focus(win: &WebviewWindow) {
    let _ = win.show();
    let _ = win.set_focus();
    let _ = win.center();
}

fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .init();

    let dir = data_dir()?;
    std::fs::create_dir_all(&dir)?;
    let db_path = dir.join("braindump.db");
    let store = Store::open(&db_path)?;
    tracing::info!(?db_path, "opened store");

    let hotkey_str =
        std::env::var("BRAINDUMP_HOTKEY").unwrap_or_else(|_| DEFAULT_HOTKEY.to_owned());
    let hotkey = parse_hotkey(&hotkey_str)?;

    tauri::Builder::default()
        .manage(AppState {
            store: Mutex::new(store),
        })
        .plugin(
            tauri_plugin_global_shortcut::Builder::new()
                .with_handler(move |app, _shortcut, event| {
                    if event.state == ShortcutState::Pressed {
                        toggle_capture(app);
                    }
                })
                .build(),
        )
        .invoke_handler(tauri::generate_handler![submit_capture])
        .setup(move |app| {
            let handle = app.handle().clone();
            handle.global_shortcut().register(hotkey)?;
            Ok(())
        })
        .run(tauri::generate_context!())
        .map_err(|e| anyhow::anyhow!(e))
}
