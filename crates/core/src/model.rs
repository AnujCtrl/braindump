//! Domain types: `Todo`, `Status`, `ParsedCapture`, `SyncQueueEntry`.
//!
//! The shape mirrors v1 (`src/core/models.ts`) minus `linear_id` and
//! `subtasks` — both intentionally dropped. Field naming follows Rust
//! convention (`snake_case`) and serializes to camelCase for the wire
//! format that desktop, server, and android clients all share.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::fmt;
use std::str::FromStr;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Status {
    Unprocessed,
    Inbox,
    Active,
    Waiting,
    Done,
    Stale,
}

impl Status {
    pub const ALL: &'static [Status] = &[
        Status::Unprocessed,
        Status::Inbox,
        Status::Active,
        Status::Waiting,
        Status::Done,
        Status::Stale,
    ];

    #[must_use]
    pub fn as_str(&self) -> &'static str {
        match self {
            Status::Unprocessed => "unprocessed",
            Status::Inbox => "inbox",
            Status::Active => "active",
            Status::Waiting => "waiting",
            Status::Done => "done",
            Status::Stale => "stale",
        }
    }
}

impl fmt::Display for Status {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

#[derive(Debug, thiserror::Error)]
#[error("unknown status: {0}")]
pub struct UnknownStatus(pub String);

impl FromStr for Status {
    type Err = UnknownStatus;
    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "unprocessed" => Ok(Status::Unprocessed),
            "inbox" => Ok(Status::Inbox),
            "active" => Ok(Status::Active),
            "waiting" => Ok(Status::Waiting),
            "done" => Ok(Status::Done),
            "stale" => Ok(Status::Stale),
            other => Err(UnknownStatus(other.to_owned())),
        }
    }
}

/// Output of `parser::parse` — the structured form of a single capture line
/// before it touches storage. No id, no timestamps yet.
#[derive(Debug, Clone, PartialEq, Eq, Default, Serialize, Deserialize)]
pub struct ParsedCapture {
    pub text: String,
    pub tags: Vec<String>,
    pub source: Option<String>,
    pub urgent: bool,
    pub important: bool,
    pub notes: Vec<String>,
}

/// Convenience alias — `ParsedNote` is a freeform string in v1, kept the same
/// here so future structured notes (timestamps, kinds) can replace it without
/// rippling through call sites.
pub type ParsedNote = String;

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct Todo {
    pub id: String,
    pub text: String,
    pub source: String,
    pub status: Status,
    pub created_at: DateTime<Utc>,
    pub status_changed_at: DateTime<Utc>,
    pub urgent: bool,
    pub important: bool,
    pub stale_count: i64,
    pub tags: Vec<String>,
    pub notes: Vec<String>,
    pub done: bool,
    pub updated_at: DateTime<Utc>,
}

impl Todo {
    /// Short 8-char ID for CLI display.
    #[must_use]
    pub fn short_id(&self) -> &str {
        &self.id[..self.id.len().min(8)]
    }
}

/// Info line stats — what the CLI prints after every command.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub struct InfoLine {
    pub unprocessed: i64,
    pub active: i64,
    pub looping: i64,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn status_round_trip() {
        for s in Status::ALL {
            let parsed: Status = s.as_str().parse().unwrap();
            assert_eq!(parsed, *s);
        }
    }

    #[test]
    fn status_unknown_errors() {
        let err = "today".parse::<Status>().unwrap_err();
        assert_eq!(err.0, "today");
    }

    #[test]
    fn status_serializes_lowercase() {
        let json = serde_json::to_string(&Status::Active).unwrap();
        assert_eq!(json, "\"active\"");
    }
}
