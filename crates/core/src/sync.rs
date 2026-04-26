//! Wire types for client ↔ server sync.
//!
//! Both sides serialize/deserialize this. The server uses the same
//! [`Store`](crate::Store) under the hood, so its conflict resolution is one
//! function: "is the incoming `updated_at` newer than what we have?". The
//! client uses the same types when draining its sync queue.
//!
//! See [`apply_push`] for the canonical last-write-wins logic.

use crate::model::Todo;
use crate::storage::{Store, StoreError};
use chrono::{DateTime, Utc};
use rusqlite::params;
use serde::{Deserialize, Serialize};

/// One synced tag. Tags are tiny so we ship the full row.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct SyncedTag {
    pub name: String,
    #[serde(with = "chrono::serde::ts_milliseconds")]
    pub created_at: DateTime<Utc>,
    #[serde(with = "chrono::serde::ts_milliseconds")]
    pub updated_at: DateTime<Utc>,
}

/// Push payload: a batch of todos and tags from a client. Both arrays may be
/// empty (a no-op heartbeat is allowed).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SyncPush {
    #[serde(default)]
    pub todos: Vec<Todo>,
    #[serde(default)]
    pub tags: Vec<SyncedTag>,
}

/// Server's response to a push: how many incoming rows actually changed
/// state on the server. `applied` = newer than what we had (or new);
/// `skipped` = staler-or-equal.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SyncPushResponse {
    pub applied_todos: usize,
    pub skipped_todos: usize,
    pub applied_tags: usize,
    pub skipped_tags: usize,
}

/// Pull payload: everything updated on the server at-or-after `since`.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct SyncPull {
    pub todos: Vec<Todo>,
    pub tags: Vec<SyncedTag>,
    #[serde(with = "chrono::serde::ts_milliseconds")]
    pub as_of: DateTime<Utc>,
}

#[derive(Debug, thiserror::Error)]
pub enum SyncError {
    #[error("storage: {0}")]
    Storage(#[from] StoreError),
    #[error("sqlite: {0}")]
    Sqlite(#[from] rusqlite::Error),
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),
}

/// Apply an incoming push to the store, last-write-wins by `updated_at`.
///
/// **Critical invariant:** the incoming `updated_at` is treated as truth.
/// Identical timestamps are skipped (not re-applied) so two clients
/// independently writing the same value don't ping-pong each other.
///
/// # Errors
///
/// Returns [`SyncError::Sqlite`] / [`SyncError::Json`] / [`SyncError::Storage`]
/// on failure. The whole batch runs in one transaction; either everything
/// applies or nothing does.
pub fn apply_push(store: &mut Store, push: &SyncPush) -> Result<SyncPushResponse, SyncError> {
    let mut resp = SyncPushResponse::default();

    let tx = store.conn_mut().transaction()?;
    for incoming in &push.todos {
        let existing_ts: Option<String> = tx
            .query_row(
                "SELECT updated_at FROM todos WHERE id = ?",
                [&incoming.id],
                |r| r.get(0),
            )
            .ok();

        let should_apply = match existing_ts {
            None => true,
            Some(s) => {
                let cur = DateTime::parse_from_rfc3339(&s)
                    .map(|dt| dt.with_timezone(&Utc))
                    .map_err(|e| {
                        rusqlite::Error::FromSqlConversionFailure(
                            0,
                            rusqlite::types::Type::Text,
                            Box::new(e),
                        )
                    })?;
                incoming.updated_at > cur
            }
        };

        if should_apply {
            apply_todo_raw(&tx, incoming)?;
            resp.applied_todos += 1;
        } else {
            resp.skipped_todos += 1;
        }
    }

    for tag in &push.tags {
        let existing_ts: Option<String> = tx
            .query_row(
                "SELECT updated_at FROM tags WHERE name = ?",
                [&tag.name],
                |r| r.get(0),
            )
            .ok();
        let should_apply = match existing_ts {
            None => true,
            Some(s) => {
                let cur = DateTime::parse_from_rfc3339(&s)
                    .map(|dt| dt.with_timezone(&Utc))
                    .map_err(|e| {
                        rusqlite::Error::FromSqlConversionFailure(
                            0,
                            rusqlite::types::Type::Text,
                            Box::new(e),
                        )
                    })?;
                tag.updated_at > cur
            }
        };
        if should_apply {
            tx.execute(
                "INSERT OR REPLACE INTO tags (name, created_at, updated_at) VALUES (?, ?, ?)",
                params![
                    tag.name,
                    tag.created_at
                        .to_rfc3339_opts(chrono::SecondsFormat::Millis, true),
                    tag.updated_at
                        .to_rfc3339_opts(chrono::SecondsFormat::Millis, true),
                ],
            )?;
            resp.applied_tags += 1;
        } else {
            resp.skipped_tags += 1;
        }
    }

    tx.commit()?;
    Ok(resp)
}

/// Build a pull response containing everything updated at-or-after `since`.
///
/// `since` is exclusive — pulling with `as_of` from a previous response will
/// not re-pull the same rows.
///
/// # Errors
///
/// Returns [`SyncError::Sqlite`] / [`SyncError::Json`] on failure.
pub fn build_pull(
    store: &Store,
    since: DateTime<Utc>,
    now: DateTime<Utc>,
) -> Result<SyncPull, SyncError> {
    let since_str = since.to_rfc3339_opts(chrono::SecondsFormat::Millis, true);
    let mut todos: Vec<Todo> = Vec::new();
    let mut stmt = store
        .conn()
        .prepare("SELECT * FROM todos WHERE updated_at > ? ORDER BY updated_at ASC")?;
    let mut rows = stmt.query([&since_str])?;
    while let Some(row) = rows.next()? {
        todos.push(crate::storage::row_to_todo(row)?);
    }
    drop(rows);
    drop(stmt);

    let mut tags: Vec<SyncedTag> = Vec::new();
    let mut stmt = store.conn().prepare(
        "SELECT name, created_at, updated_at FROM tags WHERE updated_at > ? ORDER BY updated_at ASC",
    )?;
    let mut rows = stmt.query([&since_str])?;
    while let Some(row) = rows.next()? {
        let created: String = row.get(1)?;
        let updated: String = row.get(2)?;
        tags.push(SyncedTag {
            name: row.get(0)?,
            created_at: DateTime::parse_from_rfc3339(&created)
                .map(|dt| dt.with_timezone(&Utc))
                .map_err(|e| {
                    rusqlite::Error::FromSqlConversionFailure(
                        0,
                        rusqlite::types::Type::Text,
                        Box::new(e),
                    )
                })?,
            updated_at: DateTime::parse_from_rfc3339(&updated)
                .map(|dt| dt.with_timezone(&Utc))
                .map_err(|e| {
                    rusqlite::Error::FromSqlConversionFailure(
                        0,
                        rusqlite::types::Type::Text,
                        Box::new(e),
                    )
                })?,
        });
    }

    Ok(SyncPull {
        todos,
        tags,
        as_of: now,
    })
}

fn apply_todo_raw(tx: &rusqlite::Transaction<'_>, todo: &Todo) -> Result<(), SyncError> {
    tx.execute(
        "INSERT OR REPLACE INTO todos (
            id, text, source, status, created_at, status_changed_at,
            urgent, important, stale_count, tags, notes, done, updated_at
         ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
        params![
            todo.id,
            todo.text,
            todo.source,
            todo.status.as_str(),
            todo.created_at
                .to_rfc3339_opts(chrono::SecondsFormat::Millis, true),
            todo.status_changed_at
                .to_rfc3339_opts(chrono::SecondsFormat::Millis, true),
            i32::from(todo.urgent),
            i32::from(todo.important),
            todo.stale_count,
            serde_json::to_string(&todo.tags)?,
            serde_json::to_string(&todo.notes)?,
            i32::from(todo.done),
            todo.updated_at
                .to_rfc3339_opts(chrono::SecondsFormat::Millis, true),
        ],
    )?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::{Status, Todo};

    fn at(s: &str) -> DateTime<Utc> {
        DateTime::parse_from_rfc3339(s).unwrap().with_timezone(&Utc)
    }

    fn todo_at(id: &str, updated_at: DateTime<Utc>) -> Todo {
        Todo {
            id: id.to_owned(),
            text: id.to_owned(),
            source: "cli".to_owned(),
            status: Status::Inbox,
            created_at: updated_at,
            status_changed_at: updated_at,
            urgent: false,
            important: false,
            stale_count: 0,
            tags: vec!["braindump".to_owned()],
            notes: Vec::new(),
            done: false,
            updated_at,
        }
    }

    #[test]
    fn push_inserts_new_todo() {
        let mut store = Store::open_in_memory().unwrap();
        let push = SyncPush {
            todos: vec![todo_at("11111111", at("2026-04-26T10:00:00Z"))],
            tags: Vec::new(),
        };
        let resp = apply_push(&mut store, &push).unwrap();
        assert_eq!(resp.applied_todos, 1);
        assert_eq!(resp.skipped_todos, 0);
        assert!(store.get("11111111").unwrap().is_some());
    }

    #[test]
    fn push_skips_older_update() {
        let mut store = Store::open_in_memory().unwrap();
        // Server has a recent version
        let recent = todo_at("aaaa", at("2026-04-26T12:00:00Z"));
        store.insert_todo(&recent).unwrap();
        // Client pushes an OLDER version
        let push = SyncPush {
            todos: vec![todo_at("aaaa", at("2026-04-26T10:00:00Z"))],
            tags: Vec::new(),
        };
        let resp = apply_push(&mut store, &push).unwrap();
        assert_eq!(resp.skipped_todos, 1);
        assert_eq!(resp.applied_todos, 0);
        let stored = store.get("aaaa").unwrap().unwrap();
        assert_eq!(stored.updated_at, recent.updated_at);
    }

    #[test]
    fn push_applies_newer_update() {
        let mut store = Store::open_in_memory().unwrap();
        let older = todo_at("aaaa", at("2026-04-26T08:00:00Z"));
        store.insert_todo(&older).unwrap();
        let mut newer = todo_at("aaaa", at("2026-04-26T12:00:00Z"));
        newer.text = "edited".to_owned();
        let push = SyncPush {
            todos: vec![newer.clone()],
            tags: Vec::new(),
        };
        let resp = apply_push(&mut store, &push).unwrap();
        assert_eq!(resp.applied_todos, 1);
        let stored = store.get("aaaa").unwrap().unwrap();
        assert_eq!(stored.text, "edited");
    }

    #[test]
    fn push_equal_timestamp_is_skipped() {
        let mut store = Store::open_in_memory().unwrap();
        let same = todo_at("aaaa", at("2026-04-26T12:00:00Z"));
        store.insert_todo(&same).unwrap();
        let push = SyncPush {
            todos: vec![same],
            tags: Vec::new(),
        };
        let resp = apply_push(&mut store, &push).unwrap();
        assert_eq!(resp.skipped_todos, 1);
    }

    #[test]
    fn pull_returns_only_after_since() {
        let mut store = Store::open_in_memory().unwrap();
        let early = todo_at("11111111", at("2026-04-26T08:00:00Z"));
        let late = todo_at("22222222", at("2026-04-26T16:00:00Z"));
        store.insert_todo(&early).unwrap();
        store.insert_todo(&late).unwrap();

        let pull = build_pull(
            &store,
            at("2026-04-26T12:00:00Z"),
            at("2026-04-26T18:00:00Z"),
        )
        .unwrap();
        assert_eq!(pull.todos.len(), 1);
        assert_eq!(pull.todos[0].id, "22222222");
    }

    #[test]
    fn tag_lww_works_too() {
        let mut store = Store::open_in_memory().unwrap();
        let push = SyncPush {
            todos: Vec::new(),
            tags: vec![SyncedTag {
                name: "homelab".to_owned(),
                created_at: at("2026-04-26T08:00:00Z"),
                updated_at: at("2026-04-26T08:00:00Z"),
            }],
        };
        let r1 = apply_push(&mut store, &push).unwrap();
        assert_eq!(r1.applied_tags, 1);
        let r2 = apply_push(&mut store, &push).unwrap();
        assert_eq!(r2.skipped_tags, 1);
    }
}
