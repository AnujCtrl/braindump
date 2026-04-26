//! braindump-desktop: Tauri app shell (capture window + dashboard window).
//!
//! Phase 0 stub. Phase 1 introduces the Tauri app, the global hotkey, and
//! the capture window. Until then, this binary just confirms the workspace
//! builds end-to-end.

use anyhow::Result;

fn main() -> Result<()> {
    tracing_subscriber::fmt::try_init().ok();
    tracing::info!(
        version = braindump_core::version(),
        "braindump-desktop bootstrap (Phase 0 stub)"
    );
    Ok(())
}
