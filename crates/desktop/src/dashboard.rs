//! Dashboard data adapters.
//!
//! The Claude-Design dashboard expects a specific shape (`{id, t, tag, src,
//! age, status}` for todos, daily `{capture, complete}` for history, etc.).
//! This module bridges between v2's domain model (`braindump_core::Todo` +
//! `weekly_assignments` + `events` + `metrics`) and that shape.
//!
//! Why a translation layer here rather than in the frontend: the design
//! statuses (`inbox | this-week | done | rollover | stale`) collapse v2's
//! richer set (`unprocessed | inbox | active | waiting | done | stale`)
//! plus the `weekly_assignments` bucket. Computing it server-side keeps the
//! frontend dumb and consistent.

use braindump_core::{Status, Store, Todo, bi_weekly_report, week_start};
use chrono::{DateTime, Datelike, Duration, NaiveDate, Utc};
use rusqlite::params;
use serde::{Deserialize, Serialize};

/// Per-todo row the dashboard's Pile renders.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DashTodo {
    pub id: String,
    /// Body text. Named `t` to match the design's keys exactly.
    pub t: String,
    /// First tag, or `"untagged"`.
    pub tag: String,
    /// Source (`cli`, `desktop`, `android`).
    pub src: String,
    /// Days since `created_at`.
    pub age: i64,
    /// `inbox | this-week | done | rollover | stale`.
    pub status: String,
}

/// One day in the 28-day history bands.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DashHistoryDay {
    /// ISO date.
    pub date: String,
    /// Human label (`Apr 26`).
    pub label: String,
    /// 0 = Sunday … 6 = Saturday (matches JavaScript's `Date.getDay()`).
    pub dow: u8,
    /// Captures created that day.
    pub capture: i64,
    /// Items transitioned to `Done` that day.
    pub complete: i64,
}

/// Report shape the receipt modal renders. Derived from
/// [`braindump_core::bi_weekly_report`] plus a few synthesized fields the
/// design expects (sparklines, return-rate fraction).
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DashReport {
    pub capture_per_week: i64,
    /// 0..1 — fraction of the last 14 days the dashboard was opened.
    pub return_rate: f64,
    pub inbox_sanity: i64,
    pub skipped_sundays: i64,
    /// Human window label (`apr 13 → apr 26`).
    pub window: String,
    /// 14-day daily capture spark.
    pub spark_capture: Vec<i64>,
    /// 14-day daily complete spark.
    pub spark_complete: Vec<i64>,
}

/// Counts the design overlays use (`{inbox, this-week, done, rollover, stale}`).
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "kebab-case")]
pub struct DashCounts {
    pub inbox: i64,
    pub this_week: i64,
    pub done: i64,
    pub rollover: i64,
    pub stale: i64,
}

pub fn list_todos(store: &Store, now: DateTime<Utc>) -> Result<Vec<DashTodo>, anyhow::Error> {
    // Pull all todos plus weekly assignments. v2's status alone doesn't
    // tell us "this-week" vs plain inbox — weekly_assignments does.
    //
    // Single SELECT, not 6 (per status). Each loop iteration would parse
    // tags+notes JSON for every row in that bucket; over months the `done`
    // bucket dominates and the polling refresh would compound the cost.
    let this_week = week_start(now);
    let next_week = this_week + Duration::days(7);

    let assignments = read_assignments(store, this_week, next_week)?;
    let all: Vec<Todo> = store.list(None, None)?;

    let out = all
        .into_iter()
        .map(|t| {
            let dash_status = derive_status(&t, &assignments);
            DashTodo {
                age: now.signed_duration_since(t.created_at).num_days().max(0),
                tag: t
                    .tags
                    .first()
                    .cloned()
                    .unwrap_or_else(|| "untagged".to_owned()),
                t: t.text,
                src: t.source,
                status: dash_status,
                id: t.id,
            }
        })
        .collect();
    Ok(out)
}

pub fn counts(todos: &[DashTodo]) -> DashCounts {
    let mut c = DashCounts::default();
    for t in todos {
        match t.status.as_str() {
            "inbox" => c.inbox += 1,
            "this-week" => c.this_week += 1,
            "done" => c.done += 1,
            "rollover" => c.rollover += 1,
            "stale" => c.stale += 1,
            _ => {}
        }
    }
    c
}

pub fn history(
    store: &Store,
    now: DateTime<Utc>,
    days: i64,
) -> Result<Vec<DashHistoryDay>, anyhow::Error> {
    let end = now.date_naive();
    let start = end - Duration::days(days - 1);

    // Per-day capture counts: bucket todos.created_at.
    let mut captures: std::collections::BTreeMap<NaiveDate, i64> =
        std::collections::BTreeMap::new();
    {
        let mut stmt = store
            .conn()
            .prepare("SELECT date(created_at), COUNT(*) FROM todos WHERE date(created_at) BETWEEN ? AND ? GROUP BY date(created_at)")?;
        let mut rows = stmt.query(params![start.to_string(), end.to_string()])?;
        while let Some(row) = rows.next()? {
            let day_str: String = row.get(0)?;
            let day = NaiveDate::parse_from_str(&day_str, "%Y-%m-%d")?;
            captures.insert(day, row.get(1)?);
        }
    }

    // Per-day completion counts: status_change events where to=done.
    let mut completes: std::collections::BTreeMap<NaiveDate, i64> =
        std::collections::BTreeMap::new();
    {
        let mut stmt = store.conn().prepare(
            "SELECT date(occurred_at), COUNT(*)
             FROM events
             WHERE kind = 'status_change'
               AND json_extract(payload, '$.to') = 'done'
               AND date(occurred_at) BETWEEN ? AND ?
             GROUP BY date(occurred_at)",
        )?;
        let mut rows = stmt.query(params![start.to_string(), end.to_string()])?;
        while let Some(row) = rows.next()? {
            let day_str: String = row.get(0)?;
            let day = NaiveDate::parse_from_str(&day_str, "%Y-%m-%d")?;
            completes.insert(day, row.get(1)?);
        }
    }

    let mut out = Vec::with_capacity(days as usize);
    let mut day = start;
    while day <= end {
        // chrono Sunday = 6 in num_days_from_monday; design wants
        // JS-style Sunday=0..Saturday=6. Translate.
        let dow = match day.weekday() {
            chrono::Weekday::Sun => 0,
            chrono::Weekday::Mon => 1,
            chrono::Weekday::Tue => 2,
            chrono::Weekday::Wed => 3,
            chrono::Weekday::Thu => 4,
            chrono::Weekday::Fri => 5,
            chrono::Weekday::Sat => 6,
        };
        out.push(DashHistoryDay {
            date: day.to_string(),
            label: day.format("%b %-d").to_string(),
            dow,
            capture: captures.get(&day).copied().unwrap_or(0),
            complete: completes.get(&day).copied().unwrap_or(0),
        });
        day += Duration::days(1);
    }
    Ok(out)
}

pub fn report(
    store: &Store,
    now: DateTime<Utc>,
    history28: &[DashHistoryDay],
) -> Result<DashReport, anyhow::Error> {
    let r = bi_weekly_report(store, now)?;

    // Trailing 14 days of the already-computed 28-day history. Avoids a
    // second pair of date-bucket queries on every dashboard refresh.
    let trailing14: Vec<&DashHistoryDay> = history28
        .iter()
        .skip(history28.len().saturating_sub(14))
        .collect();
    let spark_capture = trailing14.iter().map(|h| h.capture).collect();
    let spark_complete = trailing14.iter().map(|h| h.complete).collect();

    let total_captures: i64 = r.capture_rate.iter().map(|d| d.count).sum();
    let capture_per_week = total_captures / 4;

    let return_rate = if r.return_rate.window_days > 0 {
        r.return_rate.distinct_days_with_opens as f64 / r.return_rate.window_days as f64
    } else {
        0.0
    };

    // True median across the 4-week buckets — not the upper-middle that
    // `Vec::get(len / 2)` returns for even-length input. The receipt
    // labels this "median"; the math has to agree with the label.
    let inbox_sanity = median_i64(
        &r.inbox_sanity
            .iter()
            .map(|w| w.still_unprocessed)
            .collect::<Vec<_>>(),
    );

    let window = format!(
        "{} \u{2192} {}",
        (now - Duration::days(13))
            .format("%b %-d")
            .to_string()
            .to_lowercase(),
        now.format("%b %-d").to_string().to_lowercase(),
    );

    Ok(DashReport {
        capture_per_week,
        return_rate,
        inbox_sanity,
        skipped_sundays: r.sundays_skipped,
        window,
        spark_capture,
        spark_complete,
    })
}

/// Median of a small i64 slice. Returns 0 for empty input — on the receipt
/// "0 (median)" reads correctly as "no data yet".
fn median_i64(xs: &[i64]) -> i64 {
    if xs.is_empty() {
        return 0;
    }
    let mut v = xs.to_vec();
    v.sort_unstable();
    let mid = v.len() / 2;
    if v.len().is_multiple_of(2) {
        (v[mid - 1] + v[mid]) / 2
    } else {
        v[mid]
    }
}

/// Park a todo into next week's bucket — the design's "rollover" action.
pub fn park_for_next_week(
    store: &mut Store,
    todo_id: &str,
    now: DateTime<Utc>,
) -> Result<(), anyhow::Error> {
    let next_monday = week_start(now) + Duration::days(7);
    let assigned_at = now.to_rfc3339_opts(chrono::SecondsFormat::Millis, true);
    store.conn().execute(
        "INSERT OR IGNORE INTO weekly_assignments (todo_id, week_start, rolled_count, assigned_at)
         VALUES (?, ?, 0, ?)",
        params![todo_id, next_monday.to_string(), assigned_at],
    )?;
    store.log_event(
        "park_next_week",
        Some(todo_id),
        &serde_json::json!({"week_start": next_monday.to_string()}),
        now,
    )?;
    Ok(())
}

/// Read the todo IDs assigned to this-week and next-week buckets, used for
/// status derivation.
struct Assignments {
    this_week: std::collections::HashSet<String>,
    next_week: std::collections::HashSet<String>,
}

fn read_assignments(
    store: &Store,
    this_week: NaiveDate,
    next_week: NaiveDate,
) -> Result<Assignments, anyhow::Error> {
    let mut a = Assignments {
        this_week: std::collections::HashSet::new(),
        next_week: std::collections::HashSet::new(),
    };
    let mut stmt = store
        .conn()
        .prepare("SELECT todo_id, week_start FROM weekly_assignments WHERE week_start IN (?, ?)")?;
    let mut rows = stmt.query(params![this_week.to_string(), next_week.to_string()])?;
    while let Some(row) = rows.next()? {
        let id: String = row.get(0)?;
        let week: String = row.get(1)?;
        if week == this_week.to_string() {
            a.this_week.insert(id);
        } else {
            a.next_week.insert(id);
        }
    }
    Ok(a)
}

fn derive_status(todo: &Todo, asg: &Assignments) -> String {
    match todo.status {
        Status::Done => "done".into(),
        Status::Stale => "stale".into(),
        _ if asg.this_week.contains(&todo.id) => "this-week".into(),
        _ if asg.next_week.contains(&todo.id) => "rollover".into(),
        _ => "inbox".into(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use braindump_core::pull_into_week;

    fn at(s: &str) -> DateTime<Utc> {
        DateTime::parse_from_rfc3339(s).unwrap().with_timezone(&Utc)
    }

    fn seed(
        store: &mut Store,
        id: &str,
        text: &str,
        status: Status,
        created_at: DateTime<Utc>,
    ) -> Todo {
        let t = Todo {
            id: id.to_owned(),
            text: text.to_owned(),
            source: "cli".to_owned(),
            status,
            created_at,
            status_changed_at: created_at,
            urgent: false,
            important: false,
            stale_count: 0,
            tags: vec!["work".to_owned()],
            notes: Vec::new(),
            done: matches!(status, Status::Done),
            updated_at: created_at,
        };
        store.insert_todo(&t).unwrap();
        t
    }

    #[test]
    fn counts_serialize_with_kebab_case_keys() {
        // The frontend reads `c['this-week']`, not `c.this_week`. If serde
        // produces snake_case here, every band breaks silently.
        let c = DashCounts {
            inbox: 3,
            this_week: 7,
            done: 1,
            rollover: 2,
            stale: 0,
        };
        let v = serde_json::to_value(&c).unwrap();
        assert_eq!(v["this-week"], 7);
        assert!(v.get("this_week").is_none(), "should not emit snake_case");
    }

    #[test]
    fn report_serializes_camel_case() {
        // The receipt modal reads metrics.capturePerWeek etc.
        let r = DashReport {
            capture_per_week: 12,
            return_rate: 0.5,
            inbox_sanity: 4,
            skipped_sundays: 0,
            window: "apr 13 \u{2192} apr 26".into(),
            spark_capture: vec![1, 2, 3],
            spark_complete: vec![0, 1, 2],
        };
        let v = serde_json::to_value(&r).unwrap();
        assert_eq!(v["capturePerWeek"], 12);
        assert_eq!(v["sparkCapture"], serde_json::json!([1, 2, 3]));
    }

    #[test]
    fn list_todos_status_mapping() {
        let mut store = Store::open_in_memory().unwrap();
        let now = at("2026-04-22T12:00:00Z"); // Wednesday, week of Apr 20
        seed(
            &mut store,
            "11111111-0000-0000-0000-000000000000",
            "plain inbox",
            Status::Inbox,
            now,
        );
        let pulled = seed(
            &mut store,
            "22222222-0000-0000-0000-000000000000",
            "this week",
            Status::Inbox,
            now,
        );
        seed(
            &mut store,
            "33333333-0000-0000-0000-000000000000",
            "done",
            Status::Done,
            now,
        );
        seed(
            &mut store,
            "44444444-0000-0000-0000-000000000000",
            "stale",
            Status::Stale,
            now,
        );
        let parked = seed(
            &mut store,
            "55555555-0000-0000-0000-000000000000",
            "next week",
            Status::Inbox,
            now,
        );
        pull_into_week(&mut store, &pulled.id, now).unwrap();
        park_for_next_week(&mut store, &parked.id, now).unwrap();

        let todos = list_todos(&store, now).unwrap();
        let by_text: std::collections::HashMap<_, _> = todos
            .iter()
            .map(|t| (t.t.clone(), t.status.clone()))
            .collect();
        assert_eq!(by_text["plain inbox"], "inbox");
        assert_eq!(by_text["this week"], "this-week");
        assert_eq!(by_text["done"], "done");
        assert_eq!(by_text["stale"], "stale");
        assert_eq!(by_text["next week"], "rollover");
    }

    #[test]
    fn median_handles_even_and_odd_lengths() {
        // Empty
        assert_eq!(median_i64(&[]), 0);
        // Odd
        assert_eq!(median_i64(&[7]), 7);
        assert_eq!(median_i64(&[3, 1, 2]), 2);
        // Even — must be average of the two middle values, not the upper.
        // The pre-fix code returned `v[len/2]` which would be 5 here.
        assert_eq!(median_i64(&[1, 3, 5, 7]), 4);
        // 4-bucket inbox-sanity shape (the realistic input).
        assert_eq!(median_i64(&[2, 8, 4, 6]), 5);
    }

    #[test]
    fn park_for_next_week_marks_rollover() {
        let mut store = Store::open_in_memory().unwrap();
        let now = at("2026-04-22T12:00:00Z");
        let id = "66666666-0000-0000-0000-000000000000";
        seed(&mut store, id, "park me", Status::Inbox, now);
        park_for_next_week(&mut store, id, now).unwrap();
        // Idempotent — second call no-ops, doesn't bump status off rollover.
        park_for_next_week(&mut store, id, now).unwrap();

        let todos = list_todos(&store, now).unwrap();
        let parked = todos.iter().find(|t| t.t == "park me").unwrap();
        assert_eq!(parked.status, "rollover");
    }

    #[test]
    fn dash_todo_serializes_with_design_keys() {
        // The pile reads todo.t / .tag / .src / .age / .status — not
        // .text / .tags / .source. Lock this in.
        let t = DashTodo {
            id: "abc".into(),
            t: "buy milk".into(),
            tag: "errand".into(),
            src: "android".into(),
            age: 3,
            status: "inbox".into(),
        };
        let v = serde_json::to_value(&t).unwrap();
        assert_eq!(v["t"], "buy milk");
        assert_eq!(v["tag"], "errand");
        assert_eq!(v["src"], "android");
        assert!(v.get("text").is_none());
    }

    #[test]
    fn history_uses_js_dow() {
        let store = Store::open_in_memory().unwrap();
        let h = history(&store, at("2026-04-26T12:00:00Z"), 7).unwrap();
        assert_eq!(h.len(), 7);
        // 2026-04-26 is a Sunday → JS dow = 0.
        assert_eq!(h.last().unwrap().dow, 0);
        // 2026-04-20 Mon → JS dow = 1.
        assert_eq!(h.first().unwrap().dow, 1);
    }
}
