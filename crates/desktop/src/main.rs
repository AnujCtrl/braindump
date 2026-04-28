//! braindump-desktop: capture window with a global hotkey + IPC toggle.
//!
//! Lifecycle:
//! 1. App launches at login (Hyprland `exec-once` on Linux, Login Item on Mac).
//! 2. Capture window is created hidden.
//! 3. Global shortcut [`DEFAULT_HOTKEY`] toggles the window — show + focus.
//! 4. **Or** another invocation with `--toggle` connects to the running
//!    instance over a Unix socket and asks it to toggle. This is the
//!    Hyprland/Wayland path: the compositor binds the hotkey and execs
//!    `braindump-desktop --toggle`, since Wayland's security model
//!    refuses to expose system-wide key registration to apps.
//! 5. Frontend (`dist/`) submits via the `submit_capture` Tauri command,
//!    which delegates to [`braindump_core::capture`]. On success it
//!    dismisses the window after a soft tick.
//!
//! ## Hyprland / Wayland setup
//!
//! ```text
//! # ~/.config/hypr/hyprland.conf
//! exec-once = braindump-desktop
//! bind = CTRL SHIFT, semicolon, exec, braindump-desktop --toggle
//! ```
//!
//! ## Dashboard
//!
//! A second window (`label: "dashboard"`, `url: dashboard.html`) hosts the
//! Phase 7 dashboard. Open it via the [`DEFAULT_DASHBOARD_HOTKEY`] (default
//! `Ctrl+Shift+D`) or `braindump-desktop --dashboard` from the compositor.
//! Data flows through the `dashboard_load` / `dashboard_action` commands
//! (see [`dashboard`] module).

mod dashboard;
mod sync_client;

use anyhow::{Context, Result};
use braindump_core::{
    Status, Store, capture, pull_into_week, record_dashboard_open, status::transition,
};
use chrono::Utc;
use serde::Serialize;
use std::io::Write as _;
use std::os::unix::net::{UnixListener as StdUnixListener, UnixStream as StdUnixStream};
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::Duration;
use sync_client::{SharedStore, server_url_from_env};
use tauri::{AppHandle, Manager, State, WebviewWindow};
use tauri_plugin_autostart::{MacosLauncher, ManagerExt as _};
use tauri_plugin_global_shortcut::{Code, GlobalShortcutExt, Modifiers, Shortcut, ShortcutState};
use tokio::io::AsyncReadExt as _;
use tokio::net::UnixListener;
use tokio::sync::Mutex as AsyncMutex;

/// Default capture hotkey. Override with `BRAINDUMP_HOTKEY` per-machine.
const DEFAULT_HOTKEY: &str = "ctrl+shift+;";

/// Default dashboard hotkey. Override with `BRAINDUMP_DASHBOARD_HOTKEY`.
const DEFAULT_DASHBOARD_HOTKEY: &str = "ctrl+shift+d";

/// State held in the Tauri app. The store is an async mutex because the
/// background sync drainer holds it across `.await` (HTTP I/O); command
/// handlers acquire the same lock asynchronously. Single-user single-machine
/// — contention is a non-issue.
struct AppState {
    store: SharedStore,
    server_url: Option<url::Url>,
}

#[derive(Debug, Serialize)]
struct CaptureResult {
    count: usize,
    last_id: Option<String>,
}

#[tauri::command]
async fn submit_capture(
    state: State<'_, AppState>,
    input: String,
) -> Result<CaptureResult, String> {
    let mut store = state.store.lock().await;
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

    drop(store); // release the lock before nudging the drainer
    sync_client::nudge(state.store.clone(), state.server_url.clone());

    Ok(CaptureResult { count, last_id })
}

/// One round-trip command that hands the dashboard everything it needs to
/// render. Saves the frontend from juggling 4 separate invokes on every
/// open/refresh.
///
/// Does **not** log a `dashboard_open` event — call [`dashboard_record_open`]
/// once per session for that. Otherwise the 15s polling refresh would
/// pollute the return-rate metric (technically harmless because it dedupes
/// by day, but visually noisy in the events table).
#[tauri::command]
async fn dashboard_load(state: State<'_, AppState>) -> Result<DashboardSnapshot, String> {
    let store = state.store.lock().await;
    let now = Utc::now();
    let todos = dashboard::list_todos(&store, now).map_err(|e| e.to_string())?;
    let counts = dashboard::counts(&todos);
    let history = dashboard::history(&store, now, 28).map_err(|e| e.to_string())?;
    let report = dashboard::report(&store, now).map_err(|e| e.to_string())?;
    Ok(DashboardSnapshot {
        todos,
        counts,
        history,
        report,
    })
}

/// Log one dashboard-open event. Frontend calls this once on mount.
#[tauri::command]
async fn dashboard_record_open(state: State<'_, AppState>) -> Result<(), String> {
    let store = state.store.lock().await;
    record_dashboard_open(&store, Utc::now()).map_err(|e| e.to_string())
}

#[derive(Debug, Serialize)]
struct DashboardSnapshot {
    todos: Vec<dashboard::DashTodo>,
    counts: dashboard::DashCounts,
    history: Vec<dashboard::DashHistoryDay>,
    report: dashboard::DashReport,
}

/// Pile-row action: `done | this-week | park | revive`.
#[tauri::command]
async fn dashboard_action(
    state: State<'_, AppState>,
    id: String,
    action: String,
) -> Result<(), String> {
    let mut store = state.store.lock().await;
    let now = Utc::now();
    let result: Result<(), anyhow::Error> = (|| -> Result<(), anyhow::Error> {
        match action.as_str() {
            "done" => {
                transition(&mut store, &id, Status::Done, now)?;
            }
            "this-week" => {
                pull_into_week(&mut store, &id, now)?;
            }
            "park" => {
                dashboard::park_for_next_week(&mut store, &id, now)?;
            }
            "revive" => {
                transition(&mut store, &id, Status::Inbox, now)?;
            }
            other => anyhow::bail!("unknown dashboard action: {other}"),
        }
        Ok(())
    })();
    result.map_err(|e| e.to_string())?;
    drop(store);
    sync_client::nudge(state.store.clone(), state.server_url.clone());
    Ok(())
}

#[tauri::command]
fn open_dashboard(app: AppHandle) {
    show_dashboard(&app);
}

/// Summon the capture window from the dashboard "+ capture" button.
#[tauri::command]
fn open_capture(app: AppHandle) {
    if let Some(win) = app.get_webview_window("capture") {
        show_and_focus(&win);
    }
}

fn data_dir() -> Result<PathBuf> {
    if let Ok(custom) = std::env::var("BRAINDUMP_DATA_DIR") {
        return Ok(PathBuf::from(custom));
    }
    let dirs = directories_next::ProjectDirs::from("com", "braindump", "desktop")
        .ok_or_else(|| anyhow::anyhow!("could not resolve project dirs"))?;
    Ok(dirs.data_dir().to_path_buf())
}

/// Per-user IPC socket path. Prefers `$XDG_RUNTIME_DIR/braindump.sock` when
/// set (Linux), otherwise `/tmp/braindump-$USER.sock` (Mac fallback).
/// Unix domain sockets have a ~104-byte path limit on macOS, so we keep this
/// short rather than burying it under `~/Library/Application Support`.
fn socket_path() -> PathBuf {
    if let Ok(rt) = std::env::var("XDG_RUNTIME_DIR") {
        return PathBuf::from(rt).join("braindump.sock");
    }
    let user = std::env::var("USER").unwrap_or_else(|_| "default".to_owned());
    PathBuf::from("/tmp").join(format!("braindump-{user}.sock"))
}

/// Connect to a running instance and send a one-line IPC command (`toggle`,
/// `dashboard`). Returns an actionable error if no instance is listening —
/// the user almost certainly forgot to start the app at login.
fn send_ipc(path: &Path, command: &str) -> Result<()> {
    let mut sock = StdUnixStream::connect(path).with_context(|| {
        format!(
            "no running instance at {} — is braindump-desktop running?",
            path.display()
        )
    })?;
    sock.set_write_timeout(Some(Duration::from_secs(2)))?;
    sock.write_all(command.as_bytes())?;
    sock.write_all(b"\n")?;
    Ok(())
}

/// Spawn the IPC listener on Tauri's runtime. Each connection sends one
/// command line and disconnects. Currently we only handle `toggle`; future
/// commands (`hide`, `focus`, `submit <text>`) can extend the dispatch.
///
/// The socket is bound synchronously via `std` before the async task starts,
/// so any bind error (path collision, permission, etc.) surfaces here at
/// startup rather than as a silent task failure later.
fn spawn_socket_listener(app: AppHandle, path: PathBuf) -> Result<()> {
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent).ok();
    }
    let _ = std::fs::remove_file(&path);
    let std_listener = StdUnixListener::bind(&path)
        .with_context(|| format!("failed to bind toggle socket at {}", path.display()))?;
    std_listener.set_nonblocking(true)?;
    tracing::info!(?path, "toggle socket bound");

    tauri::async_runtime::spawn(async move {
        let listener = match UnixListener::from_std(std_listener) {
            Ok(l) => l,
            Err(e) => {
                tracing::error!(?e, "failed to register socket listener with tokio");
                return;
            }
        };
        loop {
            match listener.accept().await {
                Ok((mut stream, _)) => {
                    let mut buf = [0u8; 64];
                    let n = match stream.read(&mut buf).await {
                        Ok(n) => n,
                        Err(e) => {
                            tracing::warn!(?e, "socket read failed");
                            continue;
                        }
                    };
                    let cmd = std::str::from_utf8(&buf[..n]).unwrap_or("").trim();
                    if cmd.is_empty() {
                        // Singleton-check probes connect-and-close without writing;
                        // ignore those silently rather than logging "unknown".
                        continue;
                    }
                    tracing::info!(cmd, "ipc command received");
                    match cmd {
                        "toggle" => {
                            let handle = app.clone();
                            let _ = app.run_on_main_thread(move || toggle_capture(&handle));
                        }
                        "dashboard" => {
                            let handle = app.clone();
                            let _ = app.run_on_main_thread(move || show_dashboard(&handle));
                        }
                        other => tracing::warn!(other, "unknown ipc command"),
                    }
                }
                Err(e) => tracing::warn!(?e, "socket accept failed"),
            }
        }
    });
    Ok(())
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

/// Show the dashboard window (always show — never hide on second hotkey).
/// Dashboards have different ergonomics than capture: you toggle capture
/// many times a day, but you read the dashboard for minutes at a stretch.
/// Hiding-on-toggle would feel like a glitch.
fn show_dashboard(app: &AppHandle) {
    if let Some(win) = app.get_webview_window("dashboard") {
        let _ = win.show();
        let _ = win.set_focus();
    }
}

fn show_and_focus(win: &WebviewWindow) {
    let _ = win.show();
    let _ = win.set_focus();
    let _ = win.center();
}

/// One-shot login-item enable. Reads/writes a marker file in the data dir so
/// we only do this once — if the user later disables autostart via the OS,
/// we respect that decision rather than re-enabling on each launch.
fn enable_autostart_on_first_run(app: &AppHandle) {
    let Ok(dir) = data_dir() else { return };
    let marker = dir.join(".autostart-initialized");
    if marker.exists() {
        return;
    }
    let manager = app.autolaunch();
    match manager.enable() {
        Ok(()) => {
            tracing::info!("autostart enabled (first-run)");
            let _ = std::fs::write(&marker, b"1");
        }
        Err(e) => tracing::warn!(error = %e, "could not enable autostart"),
    }
}

fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .init();

    let socket = socket_path();

    // IPC commands forward to a running instance and exit. Order matters:
    // a fresh launch hits the singleton check below.
    if std::env::args().any(|a| a == "--toggle") {
        return send_ipc(&socket, "toggle");
    }
    if std::env::args().any(|a| a == "--dashboard") {
        return send_ipc(&socket, "dashboard");
    }

    // Singleton check: if connect succeeds, another instance is alive.
    if StdUnixStream::connect(&socket).is_ok() {
        anyhow::bail!(
            "braindump-desktop is already running (socket: {}). Use `--toggle` to summon the capture window.",
            socket.display()
        );
    }

    let dir = data_dir()?;
    std::fs::create_dir_all(&dir)?;
    let db_path = dir.join("braindump.db");
    let store = Store::open(&db_path)?;
    tracing::info!(?db_path, "opened store");
    let store: SharedStore = Arc::new(AsyncMutex::new(store));

    let hotkey_str =
        std::env::var("BRAINDUMP_HOTKEY").unwrap_or_else(|_| DEFAULT_HOTKEY.to_owned());
    let hotkey = parse_hotkey(&hotkey_str)?;
    let dash_hotkey_str = std::env::var("BRAINDUMP_DASHBOARD_HOTKEY")
        .unwrap_or_else(|_| DEFAULT_DASHBOARD_HOTKEY.to_owned());
    let dash_hotkey = parse_hotkey(&dash_hotkey_str)?;
    let server_url = server_url_from_env();

    let store_for_state = store.clone();
    let server_for_state = server_url.clone();

    tauri::Builder::default()
        .manage(AppState {
            store: store_for_state,
            server_url: server_for_state,
        })
        .plugin(tauri_plugin_autostart::init(
            // LaunchAgent on Mac, registry on Windows, .desktop on Linux.
            // Args are passed when the OS launches us at login.
            MacosLauncher::LaunchAgent,
            None,
        ))
        .plugin({
            let capture_hotkey = hotkey;
            let dashboard_hotkey = dash_hotkey;
            tauri_plugin_global_shortcut::Builder::new()
                .with_handler(move |app, shortcut, event| {
                    if event.state != ShortcutState::Pressed {
                        return;
                    }
                    if shortcut == &capture_hotkey {
                        toggle_capture(app);
                    } else if shortcut == &dashboard_hotkey {
                        show_dashboard(app);
                    }
                })
                .build()
        })
        .invoke_handler(tauri::generate_handler![
            submit_capture,
            dashboard_load,
            dashboard_record_open,
            dashboard_action,
            open_dashboard,
            open_capture,
        ])
        .setup(move |app| {
            let handle = app.handle().clone();
            // Wayland note: register() returns Err on Hyprland and most
            // Wayland compositors. We log + ignore so the app still starts;
            // the user is expected to configure compositor binds →
            // `braindump-desktop --toggle` / `--dashboard`.
            if let Err(e) = handle.global_shortcut().register(hotkey) {
                tracing::warn!(error = %e, "capture global shortcut registration failed (expected on Wayland — use --toggle)");
            }
            if let Err(e) = handle.global_shortcut().register(dash_hotkey) {
                tracing::warn!(error = %e, "dashboard global shortcut registration failed (expected on Wayland — use --dashboard)");
            }
            spawn_socket_listener(handle.clone(), socket.clone())?;

            // Best-effort: enable login-item registration on first run. The
            // user can disable via the OS UI if they want; we don't fight
            // that decision on subsequent launches.
            enable_autostart_on_first_run(&handle);

            // Background sync drainer — no-op if BRAINDUMP_SERVER_URL is unset.
            sync_client::spawn(store.clone(), server_url.clone());
            Ok(())
        })
        .run(tauri::generate_context!())
        .map_err(|e| anyhow::anyhow!(e))
}
