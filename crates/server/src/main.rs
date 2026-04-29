//! Sync hub binary. Bind to a Tailscale-only address and serve.
//!
//! Defaults to `127.0.0.1:8181` so accidentally running this without
//! Tailscale doesn't expose anything to the LAN. Use `--bind 0.0.0.0:8181`
//! when running on the home server (Tailscale will gate access).

use anyhow::Result;
use braindump_server::router;
use clap::Parser;
use std::net::SocketAddr;
use std::path::PathBuf;
use std::sync::Arc;
use tokio::sync::Mutex;
use tower_http::trace::TraceLayer;

#[derive(Parser, Debug)]
#[command(name = "braindump-server", version, about = "v2 sync hub", long_about = None)]
struct Args {
    /// SQLite path for the canonical replica.
    #[arg(long, default_value = "braindump-server.db")]
    db: PathBuf,
    /// Bind address. Defaults to localhost-only for safety.
    #[arg(long, default_value = "127.0.0.1:8181")]
    bind: SocketAddr,
}

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info,tower_http=info")),
        )
        .init();

    let args = Args::parse();
    let store = braindump_core::Store::open(&args.db)?;
    let state = Arc::new(Mutex::new(store));

    let app = router(state).layer(TraceLayer::new_for_http());

    tracing::info!(?args.bind, ?args.db, "braindump-server listening");
    let listener = tokio::net::TcpListener::bind(args.bind).await?;
    axum::serve(listener, app).await?;
    Ok(())
}
