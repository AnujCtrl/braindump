//! SQLite-backed storage. Single owner of the schema, sync queue, and all
//! todo/tag CRUD.
//!
//! ## Schema (v2)
//!
//! Mirrors v1's intent minus Linear plumbing:
//! - `todos` — no `linear_id`, no `subtasks` columns.
//! - `tags` — replaces v1's documented-but-unimplemented `tags.yaml`. Tag set
//!   syncs across devices like everything else.
//! - `sync_queue` — same shape as v1: `id, todo_id, action, payload,
//!   created_at, attempts, last_error`.
//! - `events` — append-only log for stale flips, rollovers, sundays
//!   skipped/pulled, and the existing status-change audit trail.
//! - `weekly_assignments` — Phase 2 "this week" bucket; `(todo_id, week_start)`
//!   composite key.
//! - `dashboard_opens` — Phase 5 return-rate metric.
//!
//! Conflict resolution is **last-write-wins** by `updated_at`; see crate-level
//! docs in `lib.rs`.

use crate::model::{Status, Todo};
use chrono::{DateTime, Utc};
use rusqlite::{Connection, OptionalExtension, params};
use std::path::Path;

#[derive(Debug, thiserror::Error)]
pub enum StoreError {
    #[error("sqlite: {0}")]
    Sqlite(#[from] rusqlite::Error),
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),
    #[error("not found: {0}")]
    NotFound(String),
}

pub type Result<T> = std::result::Result<T, StoreError>;

/// One row in the `sync_queue`. `payload` holds the JSON snapshot that the
/// server will replay; for deletes it is the empty object.
#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum SyncAction {
    Create,
    Update,
    Delete,
    StatusChange,
    TagAdd,
}

impl SyncAction {
    fn as_str(&self) -> &'static str {
        match self {
            SyncAction::Create => "create",
            SyncAction::Update => "update",
            SyncAction::Delete => "delete",
            SyncAction::StatusChange => "status_change",
            SyncAction::TagAdd => "tag_add",
        }
    }
}

pub struct Store {
    conn: Connection,
}

impl Store {
    /// Open or create the SQLite database at `path`. Idempotent — runs
    /// migrations on every open.
    ///
    /// # Errors
    ///
    /// Returns [`StoreError::Sqlite`] if the file cannot be opened or the
    /// schema migration fails.
    pub fn open(path: impl AsRef<Path>) -> Result<Self> {
        let conn = Connection::open(path)?;
        Self::init(conn)
    }

    /// Open an in-memory database — for tests and ephemeral CLI invocations.
    ///
    /// # Errors
    ///
    /// Returns [`StoreError::Sqlite`] on schema migration failure.
    pub fn open_in_memory() -> Result<Self> {
        let conn = Connection::open_in_memory()?;
        Self::init(conn)
    }

    fn init(conn: Connection) -> Result<Self> {
        conn.execute_batch(
            r"
            PRAGMA journal_mode = WAL;
            PRAGMA foreign_keys = ON;

            CREATE TABLE IF NOT EXISTS todos (
                id TEXT PRIMARY KEY,
                text TEXT NOT NULL,
                source TEXT NOT NULL DEFAULT 'cli',
                status TEXT NOT NULL DEFAULT 'inbox',
                created_at TEXT NOT NULL,
                status_changed_at TEXT NOT NULL,
                urgent INTEGER NOT NULL DEFAULT 0,
                important INTEGER NOT NULL DEFAULT 0,
                stale_count INTEGER NOT NULL DEFAULT 0,
                tags TEXT NOT NULL DEFAULT '[]',
                notes TEXT NOT NULL DEFAULT '[]',
                done INTEGER NOT NULL DEFAULT 0,
                updated_at TEXT NOT NULL
            );

            CREATE INDEX IF NOT EXISTS idx_todos_status ON todos(status);
            CREATE INDEX IF NOT EXISTS idx_todos_updated_at ON todos(updated_at);

            CREATE TABLE IF NOT EXISTS tags (
                name TEXT PRIMARY KEY,
                created_at TEXT NOT NULL,
                updated_at TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS sync_queue (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                todo_id TEXT NOT NULL,
                action TEXT NOT NULL,
                payload TEXT NOT NULL DEFAULT '{}',
                created_at TEXT NOT NULL,
                attempts INTEGER NOT NULL DEFAULT 0,
                last_error TEXT
            );

            CREATE TABLE IF NOT EXISTS sync_state (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL
            );

            CREATE TABLE IF NOT EXISTS events (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                kind TEXT NOT NULL,
                todo_id TEXT,
                payload TEXT NOT NULL DEFAULT '{}',
                occurred_at TEXT NOT NULL
            );

            CREATE INDEX IF NOT EXISTS idx_events_kind ON events(kind);
            CREATE INDEX IF NOT EXISTS idx_events_occurred_at ON events(occurred_at);

            CREATE TABLE IF NOT EXISTS weekly_assignments (
                todo_id TEXT NOT NULL,
                week_start TEXT NOT NULL,
                rolled_count INTEGER NOT NULL DEFAULT 0,
                assigned_at TEXT NOT NULL,
                PRIMARY KEY (todo_id, week_start),
                FOREIGN KEY (todo_id) REFERENCES todos(id) ON DELETE CASCADE
            );

            CREATE TABLE IF NOT EXISTS dashboard_opens (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                opened_at TEXT NOT NULL
            );
            ",
        )?;
        Ok(Self { conn })
    }

    pub fn conn(&self) -> &Connection {
        &self.conn
    }

    pub fn conn_mut(&mut self) -> &mut Connection {
        &mut self.conn
    }

    // ---------- todo CRUD ----------

    /// Insert a new todo. Also enqueues a `create` sync action.
    ///
    /// # Errors
    ///
    /// Returns [`StoreError::Sqlite`] on insert failure.
    pub fn insert_todo(&mut self, todo: &Todo) -> Result<()> {
        let tx = self.conn.transaction()?;
        tx.execute(
            "INSERT INTO todos (
                id, text, source, status, created_at, status_changed_at,
                urgent, important, stale_count, tags, notes, done, updated_at
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
            params![
                todo.id,
                todo.text,
                todo.source,
                todo.status.as_str(),
                ts(&todo.created_at),
                ts(&todo.status_changed_at),
                i32::from(todo.urgent),
                i32::from(todo.important),
                todo.stale_count,
                serde_json::to_string(&todo.tags)?,
                serde_json::to_string(&todo.notes)?,
                i32::from(todo.done),
                ts(&todo.updated_at),
            ],
        )?;
        enqueue_in_tx(
            &tx,
            &todo.id,
            SyncAction::Create,
            &serde_json::to_value(todo)?,
            &todo.updated_at,
        )?;
        tx.commit()?;
        Ok(())
    }

    /// Replace a todo wholesale. Bumps `updated_at` is the **caller's**
    /// responsibility — the store writes whatever timestamp is in the struct,
    /// so last-write-wins behavior is deterministic across devices.
    ///
    /// # Errors
    ///
    /// Returns [`StoreError::NotFound`] if no row matches the id.
    pub fn update_todo(&mut self, todo: &Todo) -> Result<()> {
        let tx = self.conn.transaction()?;
        let updated = tx.execute(
            "UPDATE todos SET
                text = ?, source = ?, status = ?, status_changed_at = ?,
                urgent = ?, important = ?, stale_count = ?,
                tags = ?, notes = ?, done = ?, updated_at = ?
             WHERE id = ?",
            params![
                todo.text,
                todo.source,
                todo.status.as_str(),
                ts(&todo.status_changed_at),
                i32::from(todo.urgent),
                i32::from(todo.important),
                todo.stale_count,
                serde_json::to_string(&todo.tags)?,
                serde_json::to_string(&todo.notes)?,
                i32::from(todo.done),
                ts(&todo.updated_at),
                todo.id,
            ],
        )?;
        if updated == 0 {
            return Err(StoreError::NotFound(todo.id.clone()));
        }
        enqueue_in_tx(
            &tx,
            &todo.id,
            SyncAction::Update,
            &serde_json::to_value(todo)?,
            &todo.updated_at,
        )?;
        tx.commit()?;
        Ok(())
    }

    /// Lookup by full id.
    ///
    /// # Errors
    ///
    /// Returns [`StoreError::Sqlite`] on query failure.
    pub fn get(&self, id: &str) -> Result<Option<Todo>> {
        Ok(self
            .conn
            .query_row("SELECT * FROM todos WHERE id = ?", [id], row_to_todo)
            .optional()?)
    }

    /// Lookup by 8-char prefix (CLI ergonomics). Errors on multiple matches.
    ///
    /// # Errors
    ///
    /// Returns [`StoreError::NotFound`] if 0 or >1 rows match.
    pub fn get_by_short_id(&self, short: &str) -> Result<Todo> {
        let pattern = format!("{short}%");
        let mut stmt = self
            .conn
            .prepare("SELECT * FROM todos WHERE id LIKE ? LIMIT 2")?;
        let rows: Vec<Todo> = stmt
            .query_map([&pattern], row_to_todo)?
            .collect::<std::result::Result<_, _>>()?;
        match rows.len() {
            1 => Ok(rows.into_iter().next().unwrap()),
            0 => Err(StoreError::NotFound(short.to_owned())),
            _ => Err(StoreError::NotFound(format!("ambiguous: {short}"))),
        }
    }

    /// Filter by optional status / tag. Sorted by `created_at` ascending.
    ///
    /// # Errors
    ///
    /// Returns [`StoreError::Sqlite`] on query failure.
    pub fn list(&self, status: Option<Status>, tag: Option<&str>) -> Result<Vec<Todo>> {
        let mut sql = String::from("SELECT * FROM todos WHERE 1=1");
        let mut params_vec: Vec<String> = Vec::new();
        if let Some(s) = status {
            sql.push_str(" AND status = ?");
            params_vec.push(s.as_str().to_owned());
        }
        if let Some(t) = tag {
            // tags column is a JSON array; use LIKE on the JSON-encoded form
            // for a no-extra-index match. This is fine at single-user scale.
            sql.push_str(" AND tags LIKE ?");
            params_vec.push(format!("%\"{t}\"%"));
        }
        sql.push_str(" ORDER BY created_at ASC");

        let mut stmt = self.conn.prepare(&sql)?;
        let rows = stmt
            .query_map(rusqlite::params_from_iter(params_vec.iter()), row_to_todo)?
            .collect::<std::result::Result<_, _>>()?;
        Ok(rows)
    }

    pub fn count_by_status(&self, status: Status) -> Result<i64> {
        Ok(self.conn.query_row(
            "SELECT COUNT(*) FROM todos WHERE status = ?",
            [status.as_str()],
            |r| r.get(0),
        )?)
    }

    /// Count items considered "looping" — gone stale 2+ times.
    pub fn count_looping(&self) -> Result<i64> {
        Ok(self.conn.query_row(
            "SELECT COUNT(*) FROM todos WHERE stale_count >= 2",
            [],
            |r| r.get(0),
        )?)
    }

    // ---------- tags ----------

    pub fn list_tags(&self) -> Result<Vec<String>> {
        let mut stmt = self.conn.prepare("SELECT name FROM tags ORDER BY name")?;
        let rows = stmt
            .query_map([], |r| r.get::<_, String>(0))?
            .collect::<std::result::Result<_, _>>()?;
        Ok(rows)
    }

    /// Insert a tag if missing. Idempotent.
    pub fn ensure_tag(&mut self, name: &str, now: DateTime<Utc>) -> Result<()> {
        let tx = self.conn.transaction()?;
        let inserted = tx.execute(
            "INSERT OR IGNORE INTO tags (name, created_at, updated_at) VALUES (?, ?, ?)",
            params![name, ts(&now), ts(&now)],
        )?;
        if inserted > 0 {
            enqueue_in_tx(
                &tx,
                name,
                SyncAction::TagAdd,
                &serde_json::json!({"name": name}),
                &now,
            )?;
        }
        tx.commit()?;
        Ok(())
    }

    pub fn tag_exists(&self, name: &str) -> Result<bool> {
        let count: i64 =
            self.conn
                .query_row("SELECT COUNT(*) FROM tags WHERE name = ?", [name], |r| {
                    r.get(0)
                })?;
        Ok(count > 0)
    }

    // ---------- events log ----------

    pub fn log_event(
        &mut self,
        kind: &str,
        todo_id: Option<&str>,
        payload: &serde_json::Value,
        occurred_at: DateTime<Utc>,
    ) -> Result<()> {
        self.conn.execute(
            "INSERT INTO events (kind, todo_id, payload, occurred_at) VALUES (?, ?, ?, ?)",
            params![kind, todo_id, payload.to_string(), ts(&occurred_at)],
        )?;
        Ok(())
    }
}

pub(crate) fn row_to_todo(row: &rusqlite::Row<'_>) -> rusqlite::Result<Todo> {
    let status_str: String = row.get("status")?;
    let status: Status = status_str
        .parse()
        .map_err(|e: crate::model::UnknownStatus| {
            rusqlite::Error::FromSqlConversionFailure(0, rusqlite::types::Type::Text, Box::new(e))
        })?;
    let tags_json: String = row.get("tags")?;
    let notes_json: String = row.get("notes")?;
    let tags: Vec<String> = serde_json::from_str(&tags_json).map_err(|e| {
        rusqlite::Error::FromSqlConversionFailure(0, rusqlite::types::Type::Text, Box::new(e))
    })?;
    let notes: Vec<String> = serde_json::from_str(&notes_json).map_err(|e| {
        rusqlite::Error::FromSqlConversionFailure(0, rusqlite::types::Type::Text, Box::new(e))
    })?;
    let urgent: i32 = row.get("urgent")?;
    let important: i32 = row.get("important")?;
    let done: i32 = row.get("done")?;
    Ok(Todo {
        id: row.get("id")?,
        text: row.get("text")?,
        source: row.get("source")?,
        status,
        created_at: parse_ts(&row.get::<_, String>("created_at")?)?,
        status_changed_at: parse_ts(&row.get::<_, String>("status_changed_at")?)?,
        urgent: urgent != 0,
        important: important != 0,
        stale_count: row.get("stale_count")?,
        tags,
        notes,
        done: done != 0,
        updated_at: parse_ts(&row.get::<_, String>("updated_at")?)?,
    })
}

fn ts(dt: &DateTime<Utc>) -> String {
    dt.to_rfc3339_opts(chrono::SecondsFormat::Millis, true)
}

fn parse_ts(s: &str) -> rusqlite::Result<DateTime<Utc>> {
    DateTime::parse_from_rfc3339(s)
        .map(|dt| dt.with_timezone(&Utc))
        .map_err(|e| {
            rusqlite::Error::FromSqlConversionFailure(0, rusqlite::types::Type::Text, Box::new(e))
        })
}

fn enqueue_in_tx(
    tx: &rusqlite::Transaction<'_>,
    todo_id: &str,
    action: SyncAction,
    payload: &serde_json::Value,
    now: &DateTime<Utc>,
) -> Result<()> {
    tx.execute(
        "INSERT INTO sync_queue (todo_id, action, payload, created_at) VALUES (?, ?, ?, ?)",
        params![todo_id, action.as_str(), payload.to_string(), ts(now)],
    )?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn now() -> DateTime<Utc> {
        DateTime::parse_from_rfc3339("2026-04-26T12:00:00Z")
            .unwrap()
            .with_timezone(&Utc)
    }

    fn make_todo(id: &str, text: &str) -> Todo {
        let n = now();
        Todo {
            id: id.to_owned(),
            text: text.to_owned(),
            source: "cli".to_owned(),
            status: Status::Inbox,
            created_at: n,
            status_changed_at: n,
            urgent: false,
            important: false,
            stale_count: 0,
            tags: vec!["braindump".to_owned()],
            notes: Vec::new(),
            done: false,
            updated_at: n,
        }
    }

    #[test]
    fn round_trip_todo() {
        let mut store = Store::open_in_memory().unwrap();
        let todo = make_todo("a1b2c3d4-1111-1111-1111-111111111111", "buy groceries");
        store.insert_todo(&todo).unwrap();
        let fetched = store.get(&todo.id).unwrap().unwrap();
        assert_eq!(fetched, todo);
    }

    #[test]
    fn list_filters_by_status() {
        let mut store = Store::open_in_memory().unwrap();
        let mut a = make_todo("aaaa1111-0000-0000-0000-000000000000", "one");
        a.status = Status::Inbox;
        let mut b = make_todo("bbbb2222-0000-0000-0000-000000000000", "two");
        b.status = Status::Done;
        store.insert_todo(&a).unwrap();
        store.insert_todo(&b).unwrap();
        let inbox = store.list(Some(Status::Inbox), None).unwrap();
        assert_eq!(inbox.len(), 1);
        assert_eq!(inbox[0].text, "one");
    }

    #[test]
    fn list_filters_by_tag() {
        let mut store = Store::open_in_memory().unwrap();
        let mut a = make_todo("aaaa1111-0000-0000-0000-000000000000", "fix portal");
        a.tags = vec!["minecraft".to_owned()];
        let mut b = make_todo("bbbb2222-0000-0000-0000-000000000000", "buy bread");
        b.tags = vec!["errands".to_owned()];
        store.insert_todo(&a).unwrap();
        store.insert_todo(&b).unwrap();
        let mc = store.list(None, Some("minecraft")).unwrap();
        assert_eq!(mc.len(), 1);
        assert_eq!(mc[0].text, "fix portal");
    }

    #[test]
    fn short_id_lookup() {
        let mut store = Store::open_in_memory().unwrap();
        let todo = make_todo("abcd1234-0000-0000-0000-000000000000", "x");
        store.insert_todo(&todo).unwrap();
        let found = store.get_by_short_id("abcd1234").unwrap();
        assert_eq!(found.id, todo.id);
    }

    #[test]
    fn short_id_ambiguous_errors() {
        let mut store = Store::open_in_memory().unwrap();
        store
            .insert_todo(&make_todo("aaaa0001-0000-0000-0000-000000000000", "x"))
            .unwrap();
        store
            .insert_todo(&make_todo("aaaa0002-0000-0000-0000-000000000000", "y"))
            .unwrap();
        let err = store.get_by_short_id("aaaa").unwrap_err();
        assert!(matches!(err, StoreError::NotFound(_)));
    }

    #[test]
    fn tag_idempotent() {
        let mut store = Store::open_in_memory().unwrap();
        store.ensure_tag("homelab", now()).unwrap();
        store.ensure_tag("homelab", now()).unwrap();
        let tags = store.list_tags().unwrap();
        assert_eq!(tags, vec!["homelab".to_owned()]);
    }

    #[test]
    fn insert_enqueues_create_action() {
        let mut store = Store::open_in_memory().unwrap();
        let todo = make_todo("aaaa1111-0000-0000-0000-000000000000", "x");
        store.insert_todo(&todo).unwrap();
        let count: i64 = store
            .conn
            .query_row(
                "SELECT COUNT(*) FROM sync_queue WHERE action = 'create'",
                [],
                |r| r.get(0),
            )
            .unwrap();
        assert_eq!(count, 1);
    }

    #[test]
    fn count_looping() {
        let mut store = Store::open_in_memory().unwrap();
        let mut a = make_todo("aaaa0001-0000-0000-0000-000000000000", "x");
        a.stale_count = 3;
        store.insert_todo(&a).unwrap();
        let mut b = make_todo("bbbb0002-0000-0000-0000-000000000000", "y");
        b.stale_count = 1;
        store.insert_todo(&b).unwrap();
        assert_eq!(store.count_looping().unwrap(), 1);
    }
}
