//! Sync hub: receives writes from clients, persists to the canonical replica,
//! and serves them back to other clients on pull.
//!
//! Endpoints:
//! - `GET  /health` — liveness check, returns the server version.
//! - `POST /sync/push` — `SyncPush` body, last-write-wins merged into the
//!   replica. Returns `SyncPushResponse`.
//! - `GET  /sync/pull?since=<rfc3339>` — returns `SyncPull` with everything
//!   updated strictly after `since`.
//!
//! No auth, no TLS — Tailscale handles transport security and ACLs.
//! No business logic — the server is plumbing only.

use axum::{
    Json, Router,
    extract::{Query, State},
    http::StatusCode,
    response::IntoResponse,
    routing::{get, post},
};
use braindump_core::{
    Store, SyncError, SyncPull, SyncPush, SyncPushResponse, apply_push, build_pull,
};
use chrono::{DateTime, Utc};
use serde::Deserialize;
use std::sync::Arc;
use tokio::sync::Mutex;

/// Shared application state. The `Store` is wrapped in a tokio mutex because
/// rusqlite's `Connection` is not `Sync`, and a single-user server has no
/// real benefit from a connection pool. SQLite WAL mode keeps reads cheap
/// even when one writer holds the lock.
pub type AppState = Arc<Mutex<Store>>;

pub fn router(state: AppState) -> Router {
    Router::new()
        .route("/health", get(health))
        .route("/sync/push", post(push))
        .route("/sync/pull", get(pull))
        .with_state(state)
}

#[derive(Debug, serde::Serialize)]
struct HealthBody {
    status: &'static str,
    version: &'static str,
}

async fn health() -> Json<HealthBody> {
    Json(HealthBody {
        status: "ok",
        version: braindump_core::version(),
    })
}

async fn push(
    State(state): State<AppState>,
    Json(body): Json<SyncPush>,
) -> Result<Json<SyncPushResponse>, SyncFailure> {
    let mut store = state.lock().await;
    let resp = apply_push(&mut store, &body)?;
    tracing::info!(
        applied_todos = resp.applied_todos,
        skipped_todos = resp.skipped_todos,
        applied_tags = resp.applied_tags,
        skipped_tags = resp.skipped_tags,
        "push applied"
    );
    Ok(Json(resp))
}

#[derive(Debug, Deserialize)]
struct PullQuery {
    /// RFC 3339 timestamp. Omit to pull everything.
    since: Option<String>,
}

async fn pull(
    State(state): State<AppState>,
    Query(q): Query<PullQuery>,
) -> Result<Json<SyncPull>, SyncFailure> {
    let since = q
        .since
        .as_deref()
        .map(|s| {
            DateTime::parse_from_rfc3339(s)
                .map(|dt| dt.with_timezone(&Utc))
                .map_err(|_| SyncFailure::BadSince(s.to_owned()))
        })
        .transpose()?
        .unwrap_or_else(|| {
            DateTime::parse_from_rfc3339("1970-01-01T00:00:00Z")
                .unwrap()
                .with_timezone(&Utc)
        });
    let store = state.lock().await;
    let now = Utc::now();
    let pull = build_pull(&store, since, now)?;
    Ok(Json(pull))
}

#[derive(Debug, thiserror::Error)]
pub enum SyncFailure {
    #[error("bad since: {0}")]
    BadSince(String),
    #[error("sync: {0}")]
    Sync(#[from] SyncError),
}

impl IntoResponse for SyncFailure {
    fn into_response(self) -> axum::response::Response {
        let (status, message) = match &self {
            SyncFailure::BadSince(_) => (StatusCode::BAD_REQUEST, self.to_string()),
            SyncFailure::Sync(_) => (StatusCode::INTERNAL_SERVER_ERROR, self.to_string()),
        };
        tracing::error!(error = %self, "sync failure");
        (status, Json(serde_json::json!({"error": message}))).into_response()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use axum::{
        body::Body,
        http::{Request, StatusCode},
    };
    use braindump_core::{Status, Todo};
    use http_body_util::BodyExt;
    use tower::ServiceExt;

    fn at(s: &str) -> DateTime<Utc> {
        DateTime::parse_from_rfc3339(s).unwrap().with_timezone(&Utc)
    }

    fn fresh_state() -> AppState {
        Arc::new(Mutex::new(Store::open_in_memory().unwrap()))
    }

    fn todo_at(id: &str, ts: DateTime<Utc>) -> Todo {
        Todo {
            id: id.to_owned(),
            text: id.to_owned(),
            source: "cli".to_owned(),
            status: Status::Inbox,
            created_at: ts,
            status_changed_at: ts,
            urgent: false,
            important: false,
            stale_count: 0,
            tags: vec!["braindump".to_owned()],
            notes: Vec::new(),
            done: false,
            updated_at: ts,
        }
    }

    #[tokio::test]
    async fn health_returns_ok() {
        let app = router(fresh_state());
        let resp = app
            .oneshot(Request::get("/health").body(Body::empty()).unwrap())
            .await
            .unwrap();
        assert_eq!(resp.status(), StatusCode::OK);
        let body = resp.into_body().collect().await.unwrap().to_bytes();
        let json: serde_json::Value = serde_json::from_slice(&body).unwrap();
        assert_eq!(json["status"], "ok");
    }

    #[tokio::test]
    async fn push_then_pull_round_trips() {
        let state = fresh_state();
        let app = router(state.clone());

        // Push two todos
        let push = SyncPush {
            todos: vec![
                todo_at("11111111", at("2026-04-26T08:00:00Z")),
                todo_at("22222222", at("2026-04-26T16:00:00Z")),
            ],
            tags: Vec::new(),
        };
        let req = Request::post("/sync/push")
            .header("content-type", "application/json")
            .body(Body::from(serde_json::to_vec(&push).unwrap()))
            .unwrap();
        let resp = app.clone().oneshot(req).await.unwrap();
        assert_eq!(resp.status(), StatusCode::OK);

        // Pull with a since-cursor between the two
        let req = Request::get("/sync/pull?since=2026-04-26T12:00:00Z")
            .body(Body::empty())
            .unwrap();
        let resp = app.oneshot(req).await.unwrap();
        assert_eq!(resp.status(), StatusCode::OK);
        let body = resp.into_body().collect().await.unwrap().to_bytes();
        let pull: SyncPull = serde_json::from_slice(&body).unwrap();
        assert_eq!(pull.todos.len(), 1);
        assert_eq!(pull.todos[0].id, "22222222");
    }

    #[tokio::test]
    async fn push_lww_skips_older() {
        let state = fresh_state();
        let app = router(state.clone());

        // Pre-populate with a recent version
        {
            let mut store = state.lock().await;
            store
                .insert_todo(&todo_at("aaaa", at("2026-04-26T16:00:00Z")))
                .unwrap();
        }

        // Push an OLDER version
        let push = SyncPush {
            todos: vec![todo_at("aaaa", at("2026-04-26T08:00:00Z"))],
            tags: Vec::new(),
        };
        let req = Request::post("/sync/push")
            .header("content-type", "application/json")
            .body(Body::from(serde_json::to_vec(&push).unwrap()))
            .unwrap();
        let resp = app.oneshot(req).await.unwrap();
        let body = resp.into_body().collect().await.unwrap().to_bytes();
        let r: SyncPushResponse = serde_json::from_slice(&body).unwrap();
        assert_eq!(r.skipped_todos, 1);
        assert_eq!(r.applied_todos, 0);
    }

    #[tokio::test]
    async fn pull_with_bad_since_400s() {
        let app = router(fresh_state());
        let resp = app
            .oneshot(
                Request::get("/sync/pull?since=not-a-date")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        assert_eq!(resp.status(), StatusCode::BAD_REQUEST);
    }
}
