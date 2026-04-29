//! Stale detection — flips eligible todos to `Status::Stale`.
//!
//! Constants mirror v1's [`src/core/stale.ts:15-31`](../../../src/core/stale.ts):
//!
//! - `Inbox` items go stale after 7 days of inactivity.
//! - `Active` items go stale after 24 hours of inactivity.
//!
//! "Inactivity" is measured against `status_changed_at` — the moment the todo
//! entered its current status. Editing text, tags, or notes does not reset
//! the stale clock; the user explicitly chose this status, and the goal is
//! to surface forgotten items, not actively-tweaked ones.

use crate::model::{Status, Todo};
use crate::status::{TransitionError, apply};
use crate::storage::{Store, StoreError};
use chrono::{DateTime, Duration, Utc};

pub const INBOX_STALE_AFTER: Duration = Duration::days(7);
pub const ACTIVE_STALE_AFTER: Duration = Duration::hours(24);

#[derive(Debug, thiserror::Error)]
pub enum StaleError {
    #[error("storage: {0}")]
    Storage(#[from] StoreError),
    #[error("transition: {0}")]
    Transition(#[from] TransitionError),
}

/// Run one pass of stale detection. Returns the number of todos transitioned.
///
/// `now` is injected for deterministic time-travel tests; production callers
/// pass `Utc::now()`.
///
/// # Errors
///
/// Returns [`StaleError::Storage`] or [`StaleError::Transition`] on any
/// underlying failure. Stops on the first error rather than half-applying.
pub fn run(store: &mut Store, now: DateTime<Utc>) -> Result<usize, StaleError> {
    let mut count = 0;
    let mut candidates: Vec<Todo> = Vec::new();
    candidates.extend(store.list(Some(Status::Inbox), None)?);
    candidates.extend(store.list(Some(Status::Active), None)?);

    for mut todo in candidates {
        let threshold = match todo.status {
            Status::Inbox => INBOX_STALE_AFTER,
            Status::Active => ACTIVE_STALE_AFTER,
            _ => continue,
        };
        let age = now.signed_duration_since(todo.status_changed_at);
        if age >= threshold {
            apply(store, &mut todo, Status::Stale, now)?;
            store.log_event(
                "stale",
                Some(&todo.id),
                &serde_json::json!({
                    "from_status": match age >= INBOX_STALE_AFTER && todo.stale_count == 0 {
                        true => "inbox",
                        false => "active",
                    },
                    "age_seconds": age.num_seconds(),
                }),
                now,
            )?;
            count += 1;
        }
    }
    Ok(count)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::Todo;

    fn at(s: &str) -> DateTime<Utc> {
        DateTime::parse_from_rfc3339(s).unwrap().with_timezone(&Utc)
    }

    fn seed(store: &mut Store, id: &str, status: Status, status_changed_at: DateTime<Utc>) {
        let todo = Todo {
            id: id.to_owned(),
            text: id.to_owned(),
            source: "cli".to_owned(),
            status,
            created_at: status_changed_at,
            status_changed_at,
            urgent: false,
            important: false,
            stale_count: 0,
            tags: vec!["braindump".to_owned()],
            notes: Vec::new(),
            done: false,
            updated_at: status_changed_at,
        };
        store.insert_todo(&todo).unwrap();
    }

    #[test]
    fn inbox_goes_stale_after_seven_days() {
        let mut store = Store::open_in_memory().unwrap();
        seed(
            &mut store,
            "00000000-0000-0000-0000-000000000001",
            Status::Inbox,
            at("2026-04-15T12:00:00Z"),
        );
        let now = at("2026-04-22T12:00:01Z");
        let n = run(&mut store, now).unwrap();
        assert_eq!(n, 1);
        let after = store
            .get("00000000-0000-0000-0000-000000000001")
            .unwrap()
            .unwrap();
        assert_eq!(after.status, Status::Stale);
    }

    #[test]
    fn fresh_inbox_unchanged() {
        let mut store = Store::open_in_memory().unwrap();
        seed(
            &mut store,
            "00000000-0000-0000-0000-000000000002",
            Status::Inbox,
            at("2026-04-25T12:00:00Z"),
        );
        let now = at("2026-04-26T12:00:00Z");
        let n = run(&mut store, now).unwrap();
        assert_eq!(n, 0);
    }

    #[test]
    fn active_goes_stale_after_24h() {
        let mut store = Store::open_in_memory().unwrap();
        seed(
            &mut store,
            "00000000-0000-0000-0000-000000000003",
            Status::Active,
            at("2026-04-25T11:00:00Z"),
        );
        let now = at("2026-04-26T12:00:00Z");
        let n = run(&mut store, now).unwrap();
        assert_eq!(n, 1);
    }

    #[test]
    fn fresh_active_unchanged() {
        let mut store = Store::open_in_memory().unwrap();
        seed(
            &mut store,
            "00000000-0000-0000-0000-000000000004",
            Status::Active,
            at("2026-04-26T11:00:00Z"),
        );
        let now = at("2026-04-26T12:00:00Z");
        let n = run(&mut store, now).unwrap();
        assert_eq!(n, 0);
    }

    #[test]
    fn done_items_skipped() {
        let mut store = Store::open_in_memory().unwrap();
        seed(
            &mut store,
            "00000000-0000-0000-0000-000000000005",
            Status::Done,
            at("2026-01-01T12:00:00Z"),
        );
        let now = at("2026-04-26T12:00:00Z");
        let n = run(&mut store, now).unwrap();
        assert_eq!(n, 0);
    }
}
