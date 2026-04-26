//! Hidden CLI sub-binary — dev sanity tool, not a user-facing surface.
//!
//! The "capture-first" UX lives in the desktop app's hotkey-summoned window.
//! This CLI exists so the parser, storage, capture orchestration, status
//! transitions, weekly bucket, stale detector, and metrics can all be
//! exercised end-to-end on any machine without firing up Tauri.

use anyhow::Result;
use braindump_core::{
    Status, Store, bi_weekly_report, capture, list_this_week, pull_into_week,
    record_dashboard_open, rollover, stale, status, sunday_auto_populate,
};
use chrono::Utc;
use clap::{Parser, Subcommand};
use std::path::PathBuf;

#[derive(Parser, Debug)]
#[command(name = "braindump-cli", version, about = "dev sanity sub-binary", long_about = None)]
struct Cli {
    /// Database path. Defaults to `./braindump.db` in the working directory.
    #[arg(long, global = true, default_value = "braindump.db")]
    db: PathBuf,

    #[command(subcommand)]
    cmd: Cmd,
}

#[derive(Subcommand, Debug)]
enum Cmd {
    /// Capture a single todo from the joined positional args.
    Capture {
        /// Capture text — joined with spaces, parsed using v2 grammar.
        #[arg(trailing_var_arg = true, allow_hyphen_values = true)]
        input: Vec<String>,
    },
    /// Capture multiple todos at once. Lines starting with `-` are split
    /// (the v2 dump-mode replacement for v1's interactive `todo dump`).
    Dump {
        #[arg(trailing_var_arg = true, allow_hyphen_values = true)]
        input: Vec<String>,
    },
    /// List todos, optionally filtered by status or tag.
    List {
        #[arg(long)]
        status: Option<Status>,
        #[arg(long)]
        tag: Option<String>,
    },
    /// List items in the current week's bucket.
    Week,
    /// Transition a todo to a new status (validates per the state machine).
    Transition {
        /// Full id or 8-char short prefix.
        id: String,
        status: Status,
    },
    /// Pull a todo into the current week's bucket.
    Pull { id: String },
    /// Run the stale detector. Items idle past their threshold are flipped.
    StaleTick,
    /// Run the weekly rollover. Open prior-week items move to this week.
    Rollover,
    /// Run the Sunday auto-populate (no-op unless today is a Sunday).
    Sunday,
    /// List known tags.
    Tags,
    /// Print the info-line counts the desktop status bar will surface.
    Info,
    /// Record a dashboard open (the return-rate metric counts these).
    RecordOpen,
    /// Emit the bi-weekly report as pretty JSON.
    Report,
}

fn main() -> Result<()> {
    tracing_subscriber::fmt::try_init().ok();
    let cli = Cli::parse();
    let mut store = Store::open(&cli.db)?;
    match cli.cmd {
        Cmd::Capture { input } => cmd_capture(&mut store, &input.join(" ")),
        Cmd::Dump { input } => cmd_dump(&mut store, &input.join(" ")),
        Cmd::List { status: s, tag } => cmd_list(&store, s, tag.as_deref()),
        Cmd::Week => cmd_week(&store),
        Cmd::Transition { id, status: s } => cmd_transition(&mut store, &id, s),
        Cmd::Pull { id } => cmd_pull(&mut store, &id),
        Cmd::StaleTick => cmd_stale_tick(&mut store),
        Cmd::Rollover => cmd_rollover(&mut store),
        Cmd::Sunday => cmd_sunday(&mut store),
        Cmd::Tags => cmd_tags(&store),
        Cmd::Info => cmd_info(&store),
        Cmd::RecordOpen => cmd_record_open(&store),
        Cmd::Report => cmd_report(&store),
    }
}

fn cmd_capture(store: &mut Store, input: &str) -> Result<()> {
    let outcome = capture(store, input, Utc::now(), "cli")?;
    println!(
        "Created: {} [{}]",
        outcome.todo.text,
        outcome.todo.short_id()
    );
    for s in &outcome.suggestions {
        println!(
            "  ? did you mean #{} for #{}? (similarity {:.2})",
            s.suggested, s.captured, s.score
        );
    }
    print_info_line(store)?;
    Ok(())
}

fn cmd_dump(store: &mut Store, input: &str) -> Result<()> {
    let mut count = 0;
    for line in input.lines() {
        let trimmed = line.trim();
        let item = trimmed
            .strip_prefix('-')
            .map(str::trim_start)
            .unwrap_or(trimmed);
        if item.is_empty() {
            continue;
        }
        let outcome = capture(store, item, Utc::now(), "cli")?;
        println!("  + {} [{}]", outcome.todo.text, outcome.todo.short_id());
        count += 1;
    }
    println!("Created {count} todos.");
    print_info_line(store)?;
    Ok(())
}

fn cmd_list(store: &Store, status_filter: Option<Status>, tag: Option<&str>) -> Result<()> {
    let todos = store.list(status_filter, tag)?;
    if todos.is_empty() {
        println!("(no todos)");
        return Ok(());
    }
    for t in &todos {
        println!(
            "- [{}] {} [{}] {}{}{}",
            if t.done { "x" } else { " " },
            t.text,
            t.short_id(),
            t.status,
            priority_str(t.urgent, t.important),
            tags_str(&t.tags)
        );
    }
    print_info_line(store)?;
    Ok(())
}

fn cmd_week(store: &Store) -> Result<()> {
    let items = list_this_week(store, Utc::now())?;
    if items.is_empty() {
        println!("(nothing pulled into this week yet)");
        return Ok(());
    }
    for t in &items {
        println!(
            "* {} [{}] {}{}",
            t.text,
            t.short_id(),
            t.status,
            priority_str(t.urgent, t.important)
        );
    }
    Ok(())
}

fn cmd_transition(store: &mut Store, id: &str, to: Status) -> Result<()> {
    let resolved_id = if id.len() == 36 {
        id.to_owned()
    } else {
        store.get_by_short_id(id)?.id
    };
    let after = status::transition(store, &resolved_id, to, Utc::now())?;
    println!("Moved [{}] to {}", after.short_id(), after.status);
    Ok(())
}

fn cmd_pull(store: &mut Store, id: &str) -> Result<()> {
    let resolved_id = if id.len() == 36 {
        id.to_owned()
    } else {
        store.get_by_short_id(id)?.id
    };
    pull_into_week(store, &resolved_id, Utc::now())?;
    println!("Pulled [{}] into this week", &resolved_id[..8]);
    Ok(())
}

fn cmd_stale_tick(store: &mut Store) -> Result<()> {
    let n = stale::run(store, Utc::now())?;
    println!("Marked {n} items stale.");
    Ok(())
}

fn cmd_rollover(store: &mut Store) -> Result<()> {
    let r = rollover(store, Utc::now())?;
    println!("Rolled: {} | Staled: {}", r.rolled, r.staled);
    Ok(())
}

fn cmd_sunday(store: &mut Store) -> Result<()> {
    let outcome = sunday_auto_populate(store, Utc::now())?;
    println!("{outcome:?}");
    Ok(())
}

fn cmd_tags(store: &Store) -> Result<()> {
    for t in store.list_tags()? {
        println!("#{t}");
    }
    Ok(())
}

fn cmd_info(store: &Store) -> Result<()> {
    print_info_line(store)
}

fn cmd_record_open(store: &Store) -> Result<()> {
    record_dashboard_open(store, Utc::now())?;
    println!("Recorded dashboard open.");
    Ok(())
}

fn cmd_report(store: &Store) -> Result<()> {
    let r = bi_weekly_report(store, Utc::now())?;
    println!("{}", serde_json::to_string_pretty(&r)?);
    Ok(())
}

fn print_info_line(store: &Store) -> Result<()> {
    let unprocessed = store.count_by_status(Status::Unprocessed)?;
    let active = store.count_by_status(Status::Active)?;
    let looping = store.count_looping()?;
    println!("-- Unprocessed: {unprocessed} | Active: {active} | Looping: {looping} --");
    Ok(())
}

fn priority_str(urgent: bool, important: bool) -> &'static str {
    match (urgent, important) {
        (true, true) => " ^^/^^^",
        (true, false) => " ^^",
        (false, true) => " ^^^",
        _ => "",
    }
}

fn tags_str(tags: &[String]) -> String {
    if tags.is_empty() {
        String::new()
    } else {
        let joined = tags
            .iter()
            .map(|x| format!("#{x}"))
            .collect::<Vec<_>>()
            .join(" ");
        format!(" {joined}")
    }
}
