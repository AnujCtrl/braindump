//! "This week" bucket, rollover, and Sunday auto-populate.
//!
//! ## Week boundaries
//!
//! A week starts Monday. `week_start(date)` returns the Monday-anchor for any
//! date. The weekly bucket is keyed by that anchor (`weekly_assignments.week_start`).
//!
//! ## Sunday auto-populate
//!
//! When the user opens the dashboard on Sunday and hasn't pulled anything for
//! the upcoming week yet, [`sunday_auto_populate`] fills the bucket from the
//! priority pyramid:
//!
//! 1. `urgent` AND `important`
//! 2. `urgent` only
//! 3. `important` only
//! 4. *(stop — never pull bare inbox items into the week.)*
//!
//! Cap at 7. Idempotent — if the bucket for the upcoming week already has any
//! assignments, this is a no-op and an `sunday_skipped` event is **not** logged
//! (because the user pulled manually, the more common case).
//!
//! ## Rollover
//!
//! On the first invocation of a new week, [`rollover`] moves any not-`done`
//! items from the just-ended week into this week, incrementing `rolled_count`.
//! When `rolled_count >= 2`, the item is marked stale instead of rolled.
//!
//! Both routines are idempotent: callers can run them every dashboard open.

use crate::model::{Status, Todo};
use crate::status::{TransitionError, apply};
use crate::storage::{Store, StoreError};
use chrono::{DateTime, Datelike, Duration, NaiveDate, Utc, Weekday};
use rusqlite::params;

const WEEKLY_CAP: usize = 7;
const ROLL_THRESHOLD: i64 = 2;

#[derive(Debug, thiserror::Error)]
pub enum WeeklyError {
    #[error("storage: {0}")]
    Storage(#[from] StoreError),
    #[error("transition: {0}")]
    Transition(#[from] TransitionError),
    #[error("sqlite: {0}")]
    Sqlite(#[from] rusqlite::Error),
    #[error("json: {0}")]
    Json(#[from] serde_json::Error),
}

/// Monday-of-week for the given UTC datetime, as a `NaiveDate`.
#[must_use]
pub fn week_start(dt: DateTime<Utc>) -> NaiveDate {
    let date = dt.date_naive();
    let dow = date.weekday().num_days_from_monday();
    date - Duration::days(i64::from(dow))
}

fn iso(date: NaiveDate) -> String {
    date.format("%Y-%m-%d").to_string()
}

/// Manually pull `todo_id` into the bucket for the week containing `now`.
/// Idempotent — re-pulling an existing assignment leaves it alone.
///
/// # Errors
///
/// Returns [`WeeklyError::Sqlite`] or [`WeeklyError::Storage`] on failure.
pub fn pull_into_week(
    store: &mut Store,
    todo_id: &str,
    now: DateTime<Utc>,
) -> Result<(), WeeklyError> {
    let week = iso(week_start(now));
    store.conn().execute(
        "INSERT OR IGNORE INTO weekly_assignments
            (todo_id, week_start, rolled_count, assigned_at)
         VALUES (?, ?, 0, ?)",
        params![
            todo_id,
            week,
            now.to_rfc3339_opts(chrono::SecondsFormat::Millis, true)
        ],
    )?;
    store.log_event(
        "weekly_pulled",
        Some(todo_id),
        &serde_json::json!({"week_start": week, "manual": true}),
        now,
    )?;
    Ok(())
}

/// Items currently assigned to the week containing `now`, in `created_at`
/// order. Joined-and-fetched in one shot.
///
/// # Errors
///
/// Returns [`WeeklyError::Sqlite`] on query failure.
pub fn list_this_week(store: &Store, now: DateTime<Utc>) -> Result<Vec<Todo>, WeeklyError> {
    let week = iso(week_start(now));
    let mut stmt = store.conn().prepare(
        "SELECT t.* FROM todos t
         INNER JOIN weekly_assignments w ON t.id = w.todo_id
         WHERE w.week_start = ?
         ORDER BY t.created_at ASC",
    )?;
    let mut rows = stmt.query([week])?;
    let mut out = Vec::new();
    while let Some(row) = rows.next()? {
        out.push(crate::storage::row_to_todo(row)?);
    }
    Ok(out)
}

/// Outcome of [`sunday_auto_populate`] — exposed so the desktop dashboard can
/// surface "we pulled N items for you" or "you already pulled, nothing to do".
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SundayOutcome {
    AlreadyPulled,
    AutoPulled(usize),
    NothingEligible,
}

/// Run the Sunday auto-populate logic for the upcoming week (the week
/// **starting tomorrow**, not the current week).
///
/// Idempotent. Should be called whenever the dashboard opens — if today is
/// not Sunday, it returns [`SundayOutcome::AlreadyPulled`] without doing
/// anything (callers can also gate by `now.weekday() == Sunday` to skip the
/// roundtrip entirely).
///
/// # Errors
///
/// Returns [`WeeklyError::Sqlite`] / [`WeeklyError::Storage`] on failure.
pub fn sunday_auto_populate(
    store: &mut Store,
    now: DateTime<Utc>,
) -> Result<SundayOutcome, WeeklyError> {
    if now.weekday() != Weekday::Sun {
        return Ok(SundayOutcome::AlreadyPulled);
    }

    // Upcoming week's Monday.
    let upcoming = week_start(now) + Duration::days(7);
    let week = iso(upcoming);

    // Already-populated check: any row in weekly_assignments for that week.
    let already: i64 = store.conn().query_row(
        "SELECT COUNT(*) FROM weekly_assignments WHERE week_start = ?",
        [&week],
        |r| r.get(0),
    )?;
    if already > 0 {
        return Ok(SundayOutcome::AlreadyPulled);
    }

    // Priority pyramid — do not include bare inbox items (level 4 is "stop").
    let candidates: Vec<Todo> = {
        let mut stmt = store.conn().prepare(
            "SELECT * FROM todos
             WHERE status IN ('inbox', 'active') AND done = 0 AND (urgent = 1 OR important = 1)
             ORDER BY (urgent + important) DESC, urgent DESC, important DESC, created_at ASC
             LIMIT ?",
        )?;
        let mut rows = stmt.query([WEEKLY_CAP as i64])?;
        let mut v: Vec<Todo> = Vec::new();
        while let Some(row) = rows.next()? {
            v.push(crate::storage::row_to_todo(row)?);
        }
        v
    };

    if candidates.is_empty() {
        store.log_event(
            "sunday_skipped",
            None,
            &serde_json::json!({"week_start": week, "reason": "nothing_eligible"}),
            now,
        )?;
        return Ok(SundayOutcome::NothingEligible);
    }

    let pulled = candidates.len();
    let assigned_at = now.to_rfc3339_opts(chrono::SecondsFormat::Millis, true);
    let tx = store.conn_mut().transaction()?;
    for c in &candidates {
        tx.execute(
            "INSERT OR IGNORE INTO weekly_assignments
                (todo_id, week_start, rolled_count, assigned_at)
             VALUES (?, ?, 0, ?)",
            params![c.id, &week, &assigned_at],
        )?;
    }
    tx.commit()?;

    store.log_event(
        "sunday_auto_populated",
        None,
        &serde_json::json!({"week_start": week, "count": pulled}),
        now,
    )?;
    Ok(SundayOutcome::AutoPulled(pulled))
}

/// Roll any open items from the just-ended week into this week. Items rolled
/// twice without action get marked stale instead.
///
/// Idempotent. Safe to call on every dashboard open — only the first call in
/// a new week actually rolls anything, because rolled items get reassigned
/// to the new week and the old week's bucket empties.
///
/// # Errors
///
/// Returns [`WeeklyError::Sqlite`] / [`WeeklyError::Transition`] on failure.
pub fn rollover(store: &mut Store, now: DateTime<Utc>) -> Result<RolloverOutcome, WeeklyError> {
    let this_week = iso(week_start(now));
    let last_week = iso(week_start(now) - Duration::days(7));

    let prior: Vec<(String, i64)> = {
        let mut stmt = store.conn().prepare(
            "SELECT w.todo_id, w.rolled_count
             FROM weekly_assignments w
             INNER JOIN todos t ON t.id = w.todo_id
             WHERE w.week_start = ? AND t.done = 0",
        )?;
        let mut rows = stmt.query([&last_week])?;
        let mut v: Vec<(String, i64)> = Vec::new();
        while let Some(row) = rows.next()? {
            v.push((row.get(0)?, row.get(1)?));
        }
        v
    };

    let mut rolled = 0;
    let mut staled = 0;
    for (id, count) in prior {
        if count + 1 >= ROLL_THRESHOLD {
            // Drop from last week's bucket and stale the todo.
            store.conn().execute(
                "DELETE FROM weekly_assignments WHERE todo_id = ? AND week_start = ?",
                params![id, &last_week],
            )?;
            if let Some(mut todo) = store.get(&id)?
                && !matches!(todo.status, Status::Stale | Status::Done)
            {
                apply(store, &mut todo, Status::Stale, now)?;
                staled += 1;
            }
        } else {
            // Move into this week with rolled_count + 1.
            store.conn().execute(
                "DELETE FROM weekly_assignments WHERE todo_id = ? AND week_start = ?",
                params![id, &last_week],
            )?;
            store.conn().execute(
                "INSERT INTO weekly_assignments
                    (todo_id, week_start, rolled_count, assigned_at)
                 VALUES (?, ?, ?, ?)",
                params![
                    id,
                    &this_week,
                    count + 1,
                    now.to_rfc3339_opts(chrono::SecondsFormat::Millis, true)
                ],
            )?;
            store.log_event(
                "rollover",
                Some(&id),
                &serde_json::json!({"from_week": last_week, "to_week": this_week, "rolled_count": count + 1}),
                now,
            )?;
            rolled += 1;
        }
    }

    Ok(RolloverOutcome { rolled, staled })
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct RolloverOutcome {
    pub rolled: usize,
    pub staled: usize,
}

#[cfg(test)]
mod tests {
    use super::*;

    fn at(s: &str) -> DateTime<Utc> {
        DateTime::parse_from_rfc3339(s).unwrap().with_timezone(&Utc)
    }

    fn seed(
        store: &mut Store,
        id: &str,
        status: Status,
        urgent: bool,
        important: bool,
        created_at: DateTime<Utc>,
    ) {
        let todo = Todo {
            id: id.to_owned(),
            text: id.to_owned(),
            source: "cli".to_owned(),
            status,
            created_at,
            status_changed_at: created_at,
            urgent,
            important,
            stale_count: 0,
            tags: vec!["braindump".to_owned()],
            notes: Vec::new(),
            done: matches!(status, Status::Done),
            updated_at: created_at,
        };
        store.insert_todo(&todo).unwrap();
    }

    #[test]
    fn week_start_is_monday() {
        // 2026-04-26 is a Sunday.
        let sunday = at("2026-04-26T12:00:00Z");
        assert_eq!(
            week_start(sunday),
            NaiveDate::from_ymd_opt(2026, 4, 20).unwrap()
        );
        // 2026-04-20 is a Monday.
        let monday = at("2026-04-20T00:00:00Z");
        assert_eq!(
            week_start(monday),
            NaiveDate::from_ymd_opt(2026, 4, 20).unwrap()
        );
        // Mid-week.
        let wed = at("2026-04-22T15:00:00Z");
        assert_eq!(
            week_start(wed),
            NaiveDate::from_ymd_opt(2026, 4, 20).unwrap()
        );
    }

    #[test]
    fn manual_pull_then_list() {
        let mut store = Store::open_in_memory().unwrap();
        let now = at("2026-04-22T10:00:00Z");
        seed(
            &mut store,
            "00000000-0000-0000-0000-000000000001",
            Status::Inbox,
            false,
            false,
            now,
        );
        pull_into_week(&mut store, "00000000-0000-0000-0000-000000000001", now).unwrap();
        let week = list_this_week(&store, now).unwrap();
        assert_eq!(week.len(), 1);
    }

    #[test]
    fn pull_idempotent() {
        let mut store = Store::open_in_memory().unwrap();
        let now = at("2026-04-22T10:00:00Z");
        seed(
            &mut store,
            "00000000-0000-0000-0000-000000000001",
            Status::Inbox,
            false,
            false,
            now,
        );
        pull_into_week(&mut store, "00000000-0000-0000-0000-000000000001", now).unwrap();
        pull_into_week(&mut store, "00000000-0000-0000-0000-000000000001", now).unwrap();
        let week = list_this_week(&store, now).unwrap();
        assert_eq!(week.len(), 1);
    }

    #[test]
    fn sunday_auto_populates_priority_order() {
        let mut store = Store::open_in_memory().unwrap();
        let mid_week = at("2026-04-22T10:00:00Z");
        // Three urgent+important, two urgent-only, one important-only, then
        // bare inbox items that should NOT be pulled.
        for i in 0..3 {
            seed(
                &mut store,
                &format!("aaaa{:04}-0000-0000-0000-000000000000", i),
                Status::Inbox,
                true,
                true,
                mid_week,
            );
        }
        for i in 0..2 {
            seed(
                &mut store,
                &format!("bbbb{:04}-0000-0000-0000-000000000000", i),
                Status::Inbox,
                true,
                false,
                mid_week,
            );
        }
        seed(
            &mut store,
            "cccc0000-0000-0000-0000-000000000000",
            Status::Inbox,
            false,
            true,
            mid_week,
        );
        for i in 0..5 {
            seed(
                &mut store,
                &format!("dddd{:04}-0000-0000-0000-000000000000", i),
                Status::Inbox,
                false,
                false,
                mid_week,
            );
        }

        let sunday = at("2026-04-26T12:00:00Z");
        let outcome = sunday_auto_populate(&mut store, sunday).unwrap();
        assert_eq!(outcome, SundayOutcome::AutoPulled(6));

        // Items live in the *upcoming* week's bucket.
        let next_monday = at("2026-04-27T12:00:00Z");
        let pulled = list_this_week(&store, next_monday).unwrap();
        assert_eq!(pulled.len(), 6);

        // None of the bare-inbox 'dddd' items should be in the bucket.
        for t in pulled {
            assert!(
                t.urgent || t.important,
                "non-priority item leaked: {}",
                t.id
            );
        }
    }

    #[test]
    fn sunday_auto_populate_caps_at_seven() {
        let mut store = Store::open_in_memory().unwrap();
        let mid_week = at("2026-04-22T10:00:00Z");
        for i in 0..15 {
            seed(
                &mut store,
                &format!("aaaa{:04}-0000-0000-0000-000000000000", i),
                Status::Inbox,
                true,
                true,
                mid_week,
            );
        }
        let sunday = at("2026-04-26T12:00:00Z");
        let outcome = sunday_auto_populate(&mut store, sunday).unwrap();
        assert_eq!(outcome, SundayOutcome::AutoPulled(7));
    }

    #[test]
    fn sunday_auto_populate_idempotent_after_manual_pull() {
        let mut store = Store::open_in_memory().unwrap();
        let mid_week = at("2026-04-22T10:00:00Z");
        seed(
            &mut store,
            "aaaa0000-0000-0000-0000-000000000000",
            Status::Inbox,
            true,
            true,
            mid_week,
        );

        // Pull manually for the upcoming week.
        let upcoming_week_day = at("2026-04-28T10:00:00Z");
        pull_into_week(
            &mut store,
            "aaaa0000-0000-0000-0000-000000000000",
            upcoming_week_day,
        )
        .unwrap();

        let sunday = at("2026-04-26T12:00:00Z");
        let outcome = sunday_auto_populate(&mut store, sunday).unwrap();
        assert_eq!(outcome, SundayOutcome::AlreadyPulled);
    }

    #[test]
    fn sunday_only_runs_on_sundays() {
        let mut store = Store::open_in_memory().unwrap();
        let monday = at("2026-04-27T12:00:00Z");
        let outcome = sunday_auto_populate(&mut store, monday).unwrap();
        assert_eq!(outcome, SundayOutcome::AlreadyPulled);
    }

    #[test]
    fn rollover_moves_open_items_into_this_week() {
        let mut store = Store::open_in_memory().unwrap();
        let last_mon = at("2026-04-13T10:00:00Z");
        seed(
            &mut store,
            "aaaa0000-0000-0000-0000-000000000000",
            Status::Inbox,
            false,
            false,
            last_mon,
        );
        pull_into_week(&mut store, "aaaa0000-0000-0000-0000-000000000000", last_mon).unwrap();

        let this_mon = at("2026-04-20T10:00:00Z");
        let r = rollover(&mut store, this_mon).unwrap();
        assert_eq!(r.rolled, 1);
        assert_eq!(r.staled, 0);

        let week = list_this_week(&store, this_mon).unwrap();
        assert_eq!(week.len(), 1);
    }

    #[test]
    fn rollover_marks_twice_rolled_stale() {
        let mut store = Store::open_in_memory().unwrap();
        let three_weeks_ago = at("2026-04-06T10:00:00Z");
        seed(
            &mut store,
            "aaaa0000-0000-0000-0000-000000000000",
            Status::Inbox,
            false,
            false,
            three_weeks_ago,
        );
        // Manually set rolled_count to 1 by simulating a prior rollover:
        // Pull into week of the 6th, then rollover into the 13th.
        pull_into_week(
            &mut store,
            "aaaa0000-0000-0000-0000-000000000000",
            three_weeks_ago,
        )
        .unwrap();
        let two_weeks_ago = at("2026-04-13T10:00:00Z");
        let r1 = rollover(&mut store, two_weeks_ago).unwrap();
        assert_eq!(r1.rolled, 1);

        // Second rollover one week later — rolled_count + 1 == 2 → stale.
        let one_week_ago = at("2026-04-20T10:00:00Z");
        let r2 = rollover(&mut store, one_week_ago).unwrap();
        assert_eq!(r2.staled, 1);
        assert_eq!(r2.rolled, 0);

        let after = store
            .get("aaaa0000-0000-0000-0000-000000000000")
            .unwrap()
            .unwrap();
        assert_eq!(after.status, Status::Stale);
    }

    #[test]
    fn rollover_skips_done_items() {
        let mut store = Store::open_in_memory().unwrap();
        let last_mon = at("2026-04-13T10:00:00Z");
        seed(
            &mut store,
            "aaaa0000-0000-0000-0000-000000000000",
            Status::Done,
            false,
            false,
            last_mon,
        );
        pull_into_week(&mut store, "aaaa0000-0000-0000-0000-000000000000", last_mon).unwrap();

        let this_mon = at("2026-04-20T10:00:00Z");
        let r = rollover(&mut store, this_mon).unwrap();
        assert_eq!(r.rolled, 0);
        assert_eq!(r.staled, 0);
    }
}
