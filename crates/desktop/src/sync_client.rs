//! Background sync drainer.
//!
//! When `BRAINDUMP_SERVER_URL` is set (e.g.
//! `http://braindump.tail-scale.ts.net:8181`), this module spawns a tokio
//! task that, every [`TICK`], does:
//!
//! 1. Collect every entry in `sync_queue` into a `SyncPush` (deduped by
//!    todo id — multiple updates collapse to the latest snapshot).
//! 2. POST `/sync/push`. On 2xx, delete those queue rows.
//! 3. GET `/sync/pull?since=<last_pull_cursor>`. Apply the response via
//!    `apply_push` (last-write-wins by `updated_at`).
//! 4. Persist the new cursor.
//!
//! Failures are logged with backoff. The capture path never blocks on this —
//! it always writes to local SQLite + queue and returns immediately.

use braindump_core::{
    Store, SyncPull, apply_push, clear_queue_rows, last_pull_cursor, pending_push, set_pull_cursor,
};
use chrono::Utc;
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::Mutex as AsyncMutex;
use url::Url;

const TICK: Duration = Duration::from_secs(30);
const BACKOFF_MAX: Duration = Duration::from_secs(300);
const BACKOFF_INIT: Duration = Duration::from_secs(5);

/// State the drainer needs. The store is wrapped in an async mutex so the
/// drainer can hold it across the awaited HTTP call without blocking the
/// Tauri command path; commands acquire the same mutex when they touch
/// SQLite. Single-user, single-machine — contention is negligible.
pub type SharedStore = Arc<AsyncMutex<Store>>;

/// Spawn the drainer. Returns immediately; the loop runs forever on the
/// Tauri runtime. If `server_url` is `None`, this is a no-op (sync disabled).
pub fn spawn(store: SharedStore, server_url: Option<Url>) {
    let Some(server_url) = server_url else {
        tracing::info!("BRAINDUMP_SERVER_URL unset — sync disabled, local-only mode");
        return;
    };
    tracing::info!(%server_url, every = ?TICK, "sync drainer spawning");

    tauri::async_runtime::spawn(async move {
        // reqwest::Client is meant to be reused — it pools connections.
        let client = match reqwest::Client::builder()
            .timeout(Duration::from_secs(15))
            .build()
        {
            Ok(c) => c,
            Err(e) => {
                tracing::error!(?e, "failed to build reqwest client; sync disabled");
                return;
            }
        };

        let mut backoff = BACKOFF_INIT;
        loop {
            match tick_once(&client, &server_url, &store).await {
                Ok(report) => {
                    if report.had_work {
                        tracing::info!(?report, "sync tick");
                    }
                    backoff = BACKOFF_INIT;
                    tokio::time::sleep(TICK).await;
                }
                Err(e) => {
                    tracing::warn!(error = %e, ?backoff, "sync tick failed; backing off");
                    tokio::time::sleep(backoff).await;
                    backoff = (backoff * 2).min(BACKOFF_MAX);
                }
            }
        }
    });
}

/// Trigger a sync drain immediately — call from app-resume or after a
/// capture so the user's writes hit the server without waiting for the next
/// tick. Errors are logged but never propagated.
pub fn nudge(store: SharedStore, server_url: Option<Url>) {
    let Some(server_url) = server_url else {
        return;
    };
    tauri::async_runtime::spawn(async move {
        let Ok(client) = reqwest::Client::builder()
            .timeout(Duration::from_secs(15))
            .build()
        else {
            return;
        };
        if let Err(e) = tick_once(&client, &server_url, &store).await {
            tracing::warn!(error = %e, "nudge sync failed");
        }
    });
}

#[derive(Debug, Default)]
pub struct TickReport {
    pub pushed_todos: usize,
    pub pushed_tags: usize,
    pub pulled_todos: usize,
    pub pulled_tags: usize,
    pub had_work: bool,
}

async fn tick_once(
    client: &reqwest::Client,
    server: &Url,
    store: &SharedStore,
) -> anyhow::Result<TickReport> {
    let mut report = TickReport::default();

    // ---- Push ----
    let (push, queue_ids) = {
        let s = store.lock().await;
        pending_push(&s)?
    };
    if !push.todos.is_empty() || !push.tags.is_empty() {
        let push_url = server.join("sync/push")?;
        let resp = client.post(push_url).json(&push).send().await?;
        if !resp.status().is_success() {
            anyhow::bail!("push failed: {}", resp.status());
        }
        let mut s = store.lock().await;
        clear_queue_rows(&mut s, &queue_ids)?;
        report.pushed_todos = push.todos.len();
        report.pushed_tags = push.tags.len();
        report.had_work = true;
    }

    // ---- Pull ----
    let cursor = {
        let s = store.lock().await;
        last_pull_cursor(&s)?
    };
    let mut pull_url = server.join("sync/pull")?;
    if let Some(c) = cursor {
        pull_url.query_pairs_mut().append_pair(
            "since",
            &c.to_rfc3339_opts(chrono::SecondsFormat::Millis, true),
        );
    }
    let resp = client.get(pull_url).send().await?;
    if !resp.status().is_success() {
        anyhow::bail!("pull failed: {}", resp.status());
    }
    let pull: SyncPull = resp.json().await?;
    if !pull.todos.is_empty() || !pull.tags.is_empty() {
        let mut s = store.lock().await;
        // Apply server data to local store via the same LWW path the server
        // uses for incoming pushes. Symmetry: server-as-truth on pull,
        // client-as-truth on push, both arbitrated by updated_at.
        apply_push(
            &mut s,
            &braindump_core::SyncPush {
                todos: pull.todos.clone(),
                tags: pull.tags.clone(),
            },
        )?;
        report.pulled_todos = pull.todos.len();
        report.pulled_tags = pull.tags.len();
        report.had_work = true;
    }
    {
        let s = store.lock().await;
        // Always advance cursor to the server's as_of, even on empty pulls,
        // so we don't re-scan the whole table next tick.
        set_pull_cursor(&s, pull.as_of)?;
    }

    Ok(report)
}

/// Parse the server URL from `BRAINDUMP_SERVER_URL`. None if unset.
pub fn server_url_from_env() -> Option<Url> {
    let raw = std::env::var("BRAINDUMP_SERVER_URL").ok()?;
    match Url::parse(&raw) {
        Ok(mut u) => {
            // Make trailing-slash handling forgiving: `Url::join("sync/push")`
            // requires a trailing slash on the base. Normalize once here.
            if !u.path().ends_with('/') {
                let new_path = format!("{}/", u.path());
                u.set_path(&new_path);
            }
            Some(u)
        }
        Err(e) => {
            tracing::error!(raw, ?e, "BRAINDUMP_SERVER_URL is not a valid URL — ignored");
            None
        }
    }
}

/// Today's date in `Utc::now`-flavored form. Trivial helper, but factoring
/// it out keeps tests deterministic.
#[allow(dead_code)]
fn now() -> chrono::DateTime<Utc> {
    Utc::now()
}
