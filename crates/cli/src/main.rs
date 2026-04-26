//! Hidden CLI sub-binary — dev sanity tool, not a user-facing surface.
//!
//! The "capture-first" UX lives in the desktop app's hotkey-summoned window.
//! This CLI exists so the parser, storage, and capture orchestration can be
//! exercised end-to-end on any machine without firing up Tauri.

use anyhow::Result;
use braindump_core::{Status, Store, capture};
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
    /// Capture multiple todos at once. Lines starting with `-` are split into
    /// separate todos (the v2 dump-mode replacement for v1's interactive
    /// `todo dump` subcommand).
    Dump {
        /// Multiline string. Each line beginning with `-` is one todo.
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
    /// List known tags (the v2 tag set, source of truth for #tag validation).
    Tags,
    /// Print the info-line counts the desktop status bar will surface.
    Info,
}

fn main() -> Result<()> {
    tracing_subscriber::fmt::try_init().ok();
    let cli = Cli::parse();
    let mut store = Store::open(&cli.db)?;
    match cli.cmd {
        Cmd::Capture { input } => cmd_capture(&mut store, &input.join(" ")),
        Cmd::Dump { input } => cmd_dump(&mut store, &input.join(" ")),
        Cmd::List { status, tag } => cmd_list(&store, status, tag.as_deref()),
        Cmd::Tags => cmd_tags(&store),
        Cmd::Info => cmd_info(&store),
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

fn cmd_list(store: &Store, status: Option<Status>, tag: Option<&str>) -> Result<()> {
    let todos = store.list(status, tag)?;
    if todos.is_empty() {
        println!("(no todos)");
        return Ok(());
    }
    for t in &todos {
        let mark = if t.done { "x" } else { " " };
        let prio = match (t.urgent, t.important) {
            (true, true) => " ^^^^^",
            (true, false) => " ^^",
            (false, true) => " ^^^",
            _ => "",
        };
        let tags: String = t
            .tags
            .iter()
            .map(|x| format!("#{x}"))
            .collect::<Vec<_>>()
            .join(" ");
        println!(
            "- [{}] {} [{}] {}{}{}",
            mark,
            t.text,
            t.short_id(),
            t.status,
            prio,
            if tags.is_empty() {
                String::new()
            } else {
                format!(" {tags}")
            }
        );
    }
    print_info_line(store)?;
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

fn print_info_line(store: &Store) -> Result<()> {
    let unprocessed = store.count_by_status(Status::Unprocessed)?;
    let active = store.count_by_status(Status::Active)?;
    let looping = store.count_looping()?;
    println!("-- Unprocessed: {unprocessed} | Active: {active} | Looping: {looping} --");
    Ok(())
}
