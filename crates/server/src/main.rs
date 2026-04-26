//! braindump-server: sync hub (relay + canonical replica).
//!
//! Phase 0 stub. Phase 3 introduces the actual axum HTTP server, push/pull
//! endpoints, last-write-wins conflict resolution, and the SQLite replica.

use anyhow::Result;

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt::try_init().ok();
    tracing::info!(
        version = braindump_core::version(),
        "braindump-server bootstrap (Phase 0 stub)"
    );
    Ok(())
}
