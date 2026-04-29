//! Status transitions.
//!
//! v2 keeps v1's transition graph but enforces it in one place rather than
//! letting any caller mutate `todo.status` directly. Every transition writes
//! `status_changed_at`, bumps `updated_at`, and logs a `status_change` event
//! for the audit trail.

use crate::model::{Status, Todo};
use crate::storage::{Store, StoreError};
use chrono::{DateTime, Utc};

#[derive(Debug, thiserror::Error)]
pub enum TransitionError {
    #[error("storage: {0}")]
    Storage(#[from] StoreError),
    #[error("invalid transition: {from} \u{2192} {to}")]
    Invalid { from: Status, to: Status },
    #[error("not found: {0}")]
    NotFound(String),
}

/// Apply `to` to the todo identified by `id`. Validates the transition,
/// writes the updated todo (which enqueues an `update` sync action and the
/// `status_change` event log entry), and returns the new state.
///
/// `now` is injected so tests can be deterministic.
///
/// # Errors
///
/// - [`TransitionError::NotFound`] if no todo matches the id.
/// - [`TransitionError::Invalid`] if the transition isn't allowed.
/// - [`TransitionError::Storage`] for any underlying SQLite failure.
pub fn transition(
    store: &mut Store,
    id: &str,
    to: Status,
    now: DateTime<Utc>,
) -> Result<Todo, TransitionError> {
    let mut todo = store
        .get(id)?
        .ok_or_else(|| TransitionError::NotFound(id.to_owned()))?;
    apply(store, &mut todo, to, now)?;
    Ok(todo)
}

/// Apply `to` to a `Todo` you already have in hand — used by the stale
/// detector and weekly rollover, both of which iterate over many todos and
/// want to avoid re-fetching each one.
///
/// # Errors
///
/// Same as [`transition`].
pub fn apply(
    store: &mut Store,
    todo: &mut Todo,
    to: Status,
    now: DateTime<Utc>,
) -> Result<(), TransitionError> {
    if todo.status == to {
        return Ok(()); // no-op
    }
    if !is_valid(todo.status, to) {
        return Err(TransitionError::Invalid {
            from: todo.status,
            to,
        });
    }

    let from = todo.status;
    todo.status = to;
    todo.status_changed_at = now;
    todo.updated_at = now;
    todo.done = matches!(to, Status::Done);

    // Reviving from stale bumps the stale-count and resets to inbox.
    if from == Status::Stale && to == Status::Inbox {
        todo.stale_count += 1;
    }

    store.update_todo(todo)?;
    store.log_event(
        "status_change",
        Some(&todo.id),
        &serde_json::json!({
            "from": from.as_str(),
            "to": to.as_str(),
        }),
        now,
    )?;
    Ok(())
}

/// Whether `from -> to` is permitted. Mirrors the README's status diagram in
/// `docs/plan-v2.md` plus v1's TS implementation.
#[must_use]
pub fn is_valid(from: Status, to: Status) -> bool {
    use Status::*;
    matches!(
        (from, to),
        // Processing
        (Unprocessed, Inbox)
            | (Unprocessed, Done)
            // Inbox flows
            | (Inbox, Active)
            | (Inbox, Waiting)
            | (Inbox, Stale)
            | (Inbox, Done)
            // Active flows
            | (Active, Inbox)
            | (Active, Done)
            | (Active, Waiting)
            | (Active, Stale)
            // Waiting flows
            | (Waiting, Inbox)
            | (Waiting, Active)
            | (Waiting, Done)
            // Revive a stale todo
            | (Stale, Inbox)
            // Mark a stale item done directly (dashboard ergonomics — saves
            // the user a pointless inbox-bounce when they just want it gone).
            | (Stale, Done)
            // Undo accidental done
            | (Done, Inbox)
            | (Done, Active)
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::storage::Store;

    fn now() -> DateTime<Utc> {
        DateTime::parse_from_rfc3339("2026-04-26T12:00:00Z")
            .unwrap()
            .with_timezone(&Utc)
    }

    fn seed(store: &mut Store, status: Status, stale_count: i64) -> Todo {
        let n = now();
        let todo = Todo {
            id: format!("11111111-1111-1111-1111-{:012x}", stale_count),
            text: "x".to_owned(),
            source: "cli".to_owned(),
            status,
            created_at: n,
            status_changed_at: n,
            urgent: false,
            important: false,
            stale_count,
            tags: vec!["braindump".to_owned()],
            notes: Vec::new(),
            done: matches!(status, Status::Done),
            updated_at: n,
        };
        store.insert_todo(&todo).unwrap();
        todo
    }

    #[test]
    fn happy_path_inbox_active_done() {
        let mut store = Store::open_in_memory().unwrap();
        let todo = seed(&mut store, Status::Inbox, 0);
        let later = now() + chrono::Duration::seconds(60);
        transition(&mut store, &todo.id, Status::Active, later).unwrap();
        let after = store.get(&todo.id).unwrap().unwrap();
        assert_eq!(after.status, Status::Active);
        assert_eq!(after.status_changed_at, later);
    }

    #[test]
    fn invalid_transition_errors() {
        let mut store = Store::open_in_memory().unwrap();
        let todo = seed(&mut store, Status::Done, 0);
        let err = transition(&mut store, &todo.id, Status::Stale, now()).unwrap_err();
        assert!(matches!(err, TransitionError::Invalid { .. }));
    }

    #[test]
    fn stale_revive_increments_count() {
        let mut store = Store::open_in_memory().unwrap();
        let todo = seed(&mut store, Status::Stale, 1);
        transition(&mut store, &todo.id, Status::Inbox, now()).unwrap();
        let after = store.get(&todo.id).unwrap().unwrap();
        assert_eq!(after.status, Status::Inbox);
        assert_eq!(after.stale_count, 2);
    }

    #[test]
    fn no_op_when_unchanged() {
        let mut store = Store::open_in_memory().unwrap();
        let todo = seed(&mut store, Status::Inbox, 0);
        transition(&mut store, &todo.id, Status::Inbox, now()).unwrap();
        let after = store.get(&todo.id).unwrap().unwrap();
        assert_eq!(after.status_changed_at, todo.status_changed_at);
    }

    #[test]
    fn done_marks_done_field_true() {
        let mut store = Store::open_in_memory().unwrap();
        let todo = seed(&mut store, Status::Active, 0);
        transition(&mut store, &todo.id, Status::Done, now()).unwrap();
        let after = store.get(&todo.id).unwrap().unwrap();
        assert!(after.done);
    }
}
