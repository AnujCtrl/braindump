//! Phase 5 metrics — pure SQL on top of the schema, plus the bi-weekly report
//! envelope. No UI here; the dashboard renders [`BiWeeklyReport`] directly.
//!
//! ## The four metrics
//!
//! - [`capture_rate`] — daily capture count for the last 28 days. Did the
//!   user keep capturing, or did the habit lapse?
//! - [`return_rate`] — distinct days the dashboard was opened in the last 14.
//!   Did the user keep coming back, or is this becoming v1 (built but
//!   never used)?
//! - [`inbox_sanity`] — for each of the last 4 weeks, how many items
//!   captured that week are *still* sitting in `unprocessed` or `inbox`?
//!   A growing oldest-week count = the pile is rotting.
//! - [`sundays_skipped`] — count of `sunday_skipped` events in the last 4
//!   weeks. (Auto-populate logs `sunday_auto_populated`; manual pulls don't
//!   log a skip; this tracks "Sundays where there was nothing eligible.")
//!
//! ## Bi-weekly cadence
//!
//! [`is_report_day`] returns true every other Monday relative to
//! [`REPORT_ANCHOR`]. The dashboard checks this on open and pops the modal.

use crate::storage::{Store, StoreError};
use chrono::{DateTime, Datelike, Duration, NaiveDate, Utc, Weekday};
use rusqlite::params;
use serde::{Deserialize, Serialize};

/// First Monday from which the bi-weekly cadence counts. Picked deliberately
/// far in the past so any future date that's a Monday and has even-weeks
/// distance is a report day.
pub const REPORT_ANCHOR: NaiveDate = match NaiveDate::from_ymd_opt(2026, 1, 5) {
    Some(d) => d,
    None => panic!("REPORT_ANCHOR is invalid"),
};

#[derive(Debug, thiserror::Error)]
pub enum MetricsError {
    #[error("storage: {0}")]
    Storage(#[from] StoreError),
    #[error("sqlite: {0}")]
    Sqlite(#[from] rusqlite::Error),
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct DailyCount {
    pub date: NaiveDate,
    pub count: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct WeeklyInbox {
    /// Monday-anchor for the week.
    pub week_start: NaiveDate,
    /// Items captured that week still in `unprocessed` or `inbox`.
    pub still_unprocessed: i64,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
pub struct ReturnRate {
    pub distinct_days_with_opens: i64,
    pub window_days: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct BiWeeklyReport {
    pub generated_at: DateTime<Utc>,
    pub capture_rate: Vec<DailyCount>,
    pub return_rate: ReturnRate,
    pub inbox_sanity: Vec<WeeklyInbox>,
    pub sundays_skipped: i64,
}

/// Append a row to `dashboard_opens`. Call this every time the dashboard
/// window becomes visible (not on every render — once per session is enough,
/// the metric is "did the user come back today?").
///
/// # Errors
///
/// Returns [`MetricsError::Sqlite`] on insert failure.
pub fn record_dashboard_open(store: &Store, now: DateTime<Utc>) -> Result<(), MetricsError> {
    store.conn().execute(
        "INSERT INTO dashboard_opens (opened_at) VALUES (?)",
        params![now.to_rfc3339_opts(chrono::SecondsFormat::Millis, true)],
    )?;
    Ok(())
}

/// Daily capture counts for the 28-day window ending on `now`'s date,
/// inclusive. Days with zero captures are present with `count = 0`.
///
/// # Errors
///
/// Returns [`MetricsError::Sqlite`] on query failure.
pub fn capture_rate(store: &Store, now: DateTime<Utc>) -> Result<Vec<DailyCount>, MetricsError> {
    let end = now.date_naive();
    let start = end - Duration::days(27);

    let mut stmt = store.conn().prepare(
        "SELECT date(created_at), COUNT(*)
         FROM todos
         WHERE date(created_at) BETWEEN ? AND ?
         GROUP BY date(created_at)",
    )?;
    let mut rows = stmt.query(params![start.to_string(), end.to_string()])?;
    let mut bucket: std::collections::BTreeMap<NaiveDate, i64> = std::collections::BTreeMap::new();
    while let Some(row) = rows.next()? {
        let day_str: String = row.get(0)?;
        let day = NaiveDate::parse_from_str(&day_str, "%Y-%m-%d").map_err(|e| {
            rusqlite::Error::FromSqlConversionFailure(0, rusqlite::types::Type::Text, Box::new(e))
        })?;
        bucket.insert(day, row.get(1)?);
    }

    let mut out = Vec::with_capacity(28);
    let mut day = start;
    while day <= end {
        out.push(DailyCount {
            date: day,
            count: bucket.get(&day).copied().unwrap_or(0),
        });
        day += Duration::days(1);
    }
    Ok(out)
}

/// Distinct days in the last 14 where the dashboard was opened at least once.
///
/// # Errors
///
/// Returns [`MetricsError::Sqlite`] on query failure.
pub fn return_rate(store: &Store, now: DateTime<Utc>) -> Result<ReturnRate, MetricsError> {
    let end = now.date_naive();
    let start = end - Duration::days(13);
    let count: i64 = store.conn().query_row(
        "SELECT COUNT(DISTINCT date(opened_at))
         FROM dashboard_opens
         WHERE date(opened_at) BETWEEN ? AND ?",
        params![start.to_string(), end.to_string()],
        |r| r.get(0),
    )?;
    Ok(ReturnRate {
        distinct_days_with_opens: count,
        window_days: 14,
    })
}

/// For each of the last 4 ISO weeks (Monday-anchored), how many items
/// captured that week are still in `unprocessed` or `inbox`. Includes the
/// current week.
///
/// # Errors
///
/// Returns [`MetricsError::Sqlite`] on query failure.
pub fn inbox_sanity(store: &Store, now: DateTime<Utc>) -> Result<Vec<WeeklyInbox>, MetricsError> {
    let mut out = Vec::with_capacity(4);
    let this_monday = monday_of(now.date_naive());
    for offset_weeks in 0..4 {
        let week_start = this_monday - Duration::weeks(offset_weeks);
        let week_end = week_start + Duration::days(6);
        let count: i64 = store.conn().query_row(
            "SELECT COUNT(*) FROM todos
             WHERE status IN ('unprocessed', 'inbox')
               AND date(created_at) BETWEEN ? AND ?",
            params![week_start.to_string(), week_end.to_string()],
            |r| r.get(0),
        )?;
        out.push(WeeklyInbox {
            week_start,
            still_unprocessed: count,
        });
    }
    out.reverse(); // Oldest first, matches the dashboard's left-to-right reading order.
    Ok(out)
}

/// Count of `sunday_skipped` events in the last 4 weeks.
///
/// # Errors
///
/// Returns [`MetricsError::Sqlite`] on query failure.
pub fn sundays_skipped(store: &Store, now: DateTime<Utc>) -> Result<i64, MetricsError> {
    let end = now.date_naive();
    let start = end - Duration::weeks(4);
    let count: i64 = store.conn().query_row(
        "SELECT COUNT(*) FROM events
         WHERE kind = 'sunday_skipped'
           AND date(occurred_at) BETWEEN ? AND ?",
        params![start.to_string(), end.to_string()],
        |r| r.get(0),
    )?;
    Ok(count)
}

/// Whether `now` is a bi-weekly report day — every other Monday relative to
/// [`REPORT_ANCHOR`].
#[must_use]
pub fn is_report_day(now: DateTime<Utc>) -> bool {
    let date = now.date_naive();
    if date.weekday() != Weekday::Mon {
        return false;
    }
    let weeks = (date - REPORT_ANCHOR).num_weeks();
    weeks >= 0 && weeks % 2 == 0
}

/// One-shot snapshot of all four metrics + the report-day flag, suitable for
/// JSON-encoding to the dashboard.
///
/// # Errors
///
/// Returns [`MetricsError::Sqlite`] / [`MetricsError::Storage`] on failure.
pub fn bi_weekly_report(store: &Store, now: DateTime<Utc>) -> Result<BiWeeklyReport, MetricsError> {
    Ok(BiWeeklyReport {
        generated_at: now,
        capture_rate: capture_rate(store, now)?,
        return_rate: return_rate(store, now)?,
        inbox_sanity: inbox_sanity(store, now)?,
        sundays_skipped: sundays_skipped(store, now)?,
    })
}

fn monday_of(date: NaiveDate) -> NaiveDate {
    let dow = date.weekday().num_days_from_monday();
    date - Duration::days(i64::from(dow))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::{Status, Todo};

    fn at(s: &str) -> DateTime<Utc> {
        DateTime::parse_from_rfc3339(s).unwrap().with_timezone(&Utc)
    }

    fn seed_todo(store: &mut Store, id: &str, status: Status, created_at: DateTime<Utc>) {
        let t = Todo {
            id: id.to_owned(),
            text: id.to_owned(),
            source: "cli".to_owned(),
            status,
            created_at,
            status_changed_at: created_at,
            urgent: false,
            important: false,
            stale_count: 0,
            tags: vec!["braindump".to_owned()],
            notes: Vec::new(),
            done: matches!(status, Status::Done),
            updated_at: created_at,
        };
        store.insert_todo(&t).unwrap();
    }

    #[test]
    fn capture_rate_returns_28_days() {
        let store = Store::open_in_memory().unwrap();
        let now = at("2026-04-26T12:00:00Z");
        let r = capture_rate(&store, now).unwrap();
        assert_eq!(r.len(), 28);
        assert_eq!(
            r.first().unwrap().date,
            NaiveDate::from_ymd_opt(2026, 3, 30).unwrap()
        );
        assert_eq!(
            r.last().unwrap().date,
            NaiveDate::from_ymd_opt(2026, 4, 26).unwrap()
        );
        assert!(r.iter().all(|d| d.count == 0));
    }

    #[test]
    fn capture_rate_counts_today_correctly() {
        let mut store = Store::open_in_memory().unwrap();
        seed_todo(
            &mut store,
            "11111111",
            Status::Inbox,
            at("2026-04-26T08:00:00Z"),
        );
        seed_todo(
            &mut store,
            "22222222",
            Status::Inbox,
            at("2026-04-26T18:00:00Z"),
        );
        let r = capture_rate(&store, at("2026-04-26T20:00:00Z")).unwrap();
        let today = r
            .iter()
            .find(|d| d.date == NaiveDate::from_ymd_opt(2026, 4, 26).unwrap())
            .unwrap();
        assert_eq!(today.count, 2);
    }

    #[test]
    fn return_rate_dedupes_same_day() {
        let store = Store::open_in_memory().unwrap();
        record_dashboard_open(&store, at("2026-04-26T08:00:00Z")).unwrap();
        record_dashboard_open(&store, at("2026-04-26T18:00:00Z")).unwrap();
        record_dashboard_open(&store, at("2026-04-25T12:00:00Z")).unwrap();
        let r = return_rate(&store, at("2026-04-26T20:00:00Z")).unwrap();
        assert_eq!(r.distinct_days_with_opens, 2);
        assert_eq!(r.window_days, 14);
    }

    #[test]
    fn return_rate_excludes_outside_window() {
        let store = Store::open_in_memory().unwrap();
        record_dashboard_open(&store, at("2026-03-01T08:00:00Z")).unwrap(); // outside 14-day window
        record_dashboard_open(&store, at("2026-04-26T08:00:00Z")).unwrap();
        let r = return_rate(&store, at("2026-04-26T20:00:00Z")).unwrap();
        assert_eq!(r.distinct_days_with_opens, 1);
    }

    #[test]
    fn inbox_sanity_returns_4_weeks() {
        let mut store = Store::open_in_memory().unwrap();
        // Monday of "now": 2026-04-20.
        // Place captures in each of the last 4 weeks, all still inbox.
        seed_todo(
            &mut store,
            "thisweek-1111-0000-0000-000000000000",
            Status::Inbox,
            at("2026-04-22T10:00:00Z"),
        );
        seed_todo(
            &mut store,
            "lastweek-1111-0000-0000-000000000000",
            Status::Inbox,
            at("2026-04-15T10:00:00Z"),
        );
        seed_todo(
            &mut store,
            "twoback1-1111-0000-0000-000000000000",
            Status::Inbox,
            at("2026-04-08T10:00:00Z"),
        );
        seed_todo(
            &mut store,
            "threeback-111-0000-0000-000000000000",
            Status::Inbox,
            at("2026-04-01T10:00:00Z"),
        );

        let r = inbox_sanity(&store, at("2026-04-26T12:00:00Z")).unwrap();
        assert_eq!(r.len(), 4);
        // Oldest first.
        assert_eq!(
            r[0].week_start,
            NaiveDate::from_ymd_opt(2026, 3, 30).unwrap()
        );
        assert_eq!(
            r[3].week_start,
            NaiveDate::from_ymd_opt(2026, 4, 20).unwrap()
        );
        for w in &r {
            assert_eq!(w.still_unprocessed, 1, "{} should have 1", w.week_start);
        }
    }

    #[test]
    fn inbox_sanity_excludes_processed_items() {
        let mut store = Store::open_in_memory().unwrap();
        seed_todo(
            &mut store,
            "done0000-1111-0000-0000-000000000000",
            Status::Done,
            at("2026-04-22T10:00:00Z"),
        );
        let r = inbox_sanity(&store, at("2026-04-26T12:00:00Z")).unwrap();
        assert!(r.iter().all(|w| w.still_unprocessed == 0));
    }

    #[test]
    fn is_report_day_only_alternating_mondays() {
        // 2026-01-05 is the anchor (Monday).
        assert!(is_report_day(at("2026-01-05T08:00:00Z")));
        // +1 week → Monday 2026-01-12 → odd week → not a report day.
        assert!(!is_report_day(at("2026-01-12T08:00:00Z")));
        // +2 weeks → Monday 2026-01-19 → even week → report day.
        assert!(is_report_day(at("2026-01-19T08:00:00Z")));
        // Any non-Monday is never a report day.
        assert!(!is_report_day(at("2026-01-20T08:00:00Z"))); // Tuesday
    }

    #[test]
    fn report_assembles_all_pieces() {
        let mut store = Store::open_in_memory().unwrap();
        seed_todo(
            &mut store,
            "11111111",
            Status::Inbox,
            at("2026-04-26T08:00:00Z"),
        );
        record_dashboard_open(&store, at("2026-04-26T08:30:00Z")).unwrap();
        store
            .log_event(
                "sunday_skipped",
                None,
                &serde_json::json!({}),
                at("2026-04-19T08:00:00Z"),
            )
            .unwrap();
        let r = bi_weekly_report(&store, at("2026-04-26T12:00:00Z")).unwrap();
        assert_eq!(r.capture_rate.len(), 28);
        assert_eq!(r.return_rate.distinct_days_with_opens, 1);
        assert_eq!(r.inbox_sanity.len(), 4);
        assert_eq!(r.sundays_skipped, 1);
    }
}
