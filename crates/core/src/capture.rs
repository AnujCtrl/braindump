//! Capture orchestration: parse → resolve tags → write.
//!
//! Sits on top of [`crate::parser`], [`crate::storage`], and [`crate::tags`].
//! The desktop capture window, the CLI sub-binary, and the (Phase 6) Android
//! POST endpoint all funnel through [`capture`] so the auto-add-tag and
//! fuzzy-match behavior stay consistent across surfaces.
//!
//! ## Tag policy
//!
//! - No tags in the input → auto-tag `#braindump` (matches v1).
//! - `@source` token also adds a matching tag if one already exists in the
//!   `tags` table (matches v1's behavior — a `@minecraft` capture gets
//!   `#minecraft` only if the tag exists).
//! - Each captured `#tag`:
//!   - Exact match in `tags` table → use as-is.
//!   - Fuzzy match (≥ [`AUTO_SUGGEST_THRESHOLD`]) → recorded in
//!     [`CaptureOutcome::suggestions`] for the UI to confirm. The original
//!     spelling is **not** auto-substituted; the caller decides.
//!   - No match → auto-added to `tags` table.

use crate::model::{Status, Todo};
use crate::parser::{ParseError, parse};
use crate::storage::{Store, StoreError};
use crate::tags::{AUTO_SUGGEST_THRESHOLD, TagMatch, fuzzy_match};
use chrono::{DateTime, Utc};
use uuid::Uuid;

#[derive(Debug, thiserror::Error)]
pub enum CaptureError {
    #[error("parse: {0}")]
    Parse(#[from] ParseError),
    #[error("storage: {0}")]
    Storage(#[from] StoreError),
}

/// Result of a successful capture. `todo` is what landed in storage;
/// `suggestions` is non-empty when the parser saw a `#tag` that fuzzy-matched
/// an existing tag — the caller should surface "did you mean…?" UI.
#[derive(Debug)]
pub struct CaptureOutcome {
    pub todo: Todo,
    pub suggestions: Vec<TagSuggestion>,
}

#[derive(Debug, Clone)]
pub struct TagSuggestion {
    pub captured: String,
    pub suggested: String,
    pub score: f64,
}

/// Parse `input`, resolve its tags against the `tags` table, persist the new
/// `Todo`, and return what landed.
///
/// `now` is injected for deterministic tests; production callers pass
/// `Utc::now()`.
///
/// # Errors
///
/// - [`CaptureError::Parse`] when `input` is empty or malformed.
/// - [`CaptureError::Storage`] on any underlying SQLite failure.
pub fn capture(
    store: &mut Store,
    input: &str,
    now: DateTime<Utc>,
    default_source: &str,
) -> Result<CaptureOutcome, CaptureError> {
    let parsed = parse(input)?;
    let mut suggestions: Vec<TagSuggestion> = Vec::new();

    // Resolve each captured tag: exact / fuzzy / auto-add.
    let existing: Vec<String> = store.list_tags()?;
    let mut final_tags: Vec<String> = Vec::with_capacity(parsed.tags.len() + 1);

    for raw_tag in &parsed.tags {
        if existing.iter().any(|e| e == raw_tag) {
            final_tags.push(raw_tag.clone());
            continue;
        }
        match fuzzy_match(raw_tag, &existing) {
            Some(TagMatch { candidate, score }) if score >= AUTO_SUGGEST_THRESHOLD => {
                suggestions.push(TagSuggestion {
                    captured: raw_tag.clone(),
                    suggested: candidate,
                    score,
                });
                // Keep the user's spelling in the todo; auto-add it to the
                // tag set so it's a real tag for next time.
                store.ensure_tag(raw_tag, now)?;
                final_tags.push(raw_tag.clone());
            }
            _ => {
                store.ensure_tag(raw_tag, now)?;
                final_tags.push(raw_tag.clone());
            }
        }
    }

    // @source auto-adds a matching tag if it exists (mirrors v1).
    let resolved_source = parsed.source.unwrap_or_else(|| default_source.to_owned());
    if existing.iter().any(|e| e == &resolved_source)
        && !final_tags.iter().any(|t| t == &resolved_source)
    {
        final_tags.push(resolved_source.clone());
    }

    if final_tags.is_empty() {
        // Default tag for tagless captures.
        let braindump = "braindump";
        store.ensure_tag(braindump, now)?;
        final_tags.push(braindump.to_owned());
    }

    let todo = Todo {
        id: Uuid::new_v4().to_string(),
        text: parsed.text,
        source: resolved_source,
        status: Status::Inbox,
        created_at: now,
        status_changed_at: now,
        urgent: parsed.urgent,
        important: parsed.important,
        stale_count: 0,
        tags: final_tags,
        notes: parsed.notes,
        done: false,
        updated_at: now,
    };
    store.insert_todo(&todo)?;
    Ok(CaptureOutcome {
        todo,
        suggestions,
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    fn now() -> DateTime<Utc> {
        DateTime::parse_from_rfc3339("2026-04-26T12:00:00Z")
            .unwrap()
            .with_timezone(&Utc)
    }

    #[test]
    fn captures_with_default_braindump_tag() {
        let mut store = Store::open_in_memory().unwrap();
        let outcome = capture(&mut store, "fix the thing", now(), "cli").unwrap();
        assert_eq!(outcome.todo.tags, vec!["braindump".to_owned()]);
        assert_eq!(outcome.todo.text, "fix the thing");
        assert_eq!(outcome.todo.source, "cli");
    }

    #[test]
    fn auto_adds_unknown_tag() {
        let mut store = Store::open_in_memory().unwrap();
        let outcome = capture(&mut store, "fix portal #minecraft", now(), "cli").unwrap();
        assert_eq!(outcome.todo.tags, vec!["minecraft".to_owned()]);
        assert!(store.tag_exists("minecraft").unwrap());
    }

    #[test]
    fn fuzzy_match_records_suggestion() {
        let mut store = Store::open_in_memory().unwrap();
        store.ensure_tag("minecraft", now()).unwrap();
        let outcome = capture(&mut store, "fix portal #minecaft", now(), "cli").unwrap();
        assert_eq!(outcome.suggestions.len(), 1);
        assert_eq!(outcome.suggestions[0].captured, "minecaft");
        assert_eq!(outcome.suggestions[0].suggested, "minecraft");
    }

    #[test]
    fn source_auto_adds_matching_tag() {
        let mut store = Store::open_in_memory().unwrap();
        store.ensure_tag("minecraft", now()).unwrap();
        let outcome = capture(&mut store, "build farm @minecraft", now(), "cli").unwrap();
        assert_eq!(outcome.todo.source, "minecraft");
        assert_eq!(outcome.todo.tags, vec!["minecraft".to_owned()]);
    }

    #[test]
    fn urgent_and_important_carry_through() {
        let mut store = Store::open_in_memory().unwrap();
        let outcome = capture(&mut store, "deadline ^^^ ^^", now(), "cli").unwrap();
        assert!(outcome.todo.urgent);
        assert!(outcome.todo.important);
    }

    #[test]
    fn empty_input_errors() {
        let mut store = Store::open_in_memory().unwrap();
        assert!(matches!(
            capture(&mut store, "", now(), "cli"),
            Err(CaptureError::Parse(ParseError::Empty))
        ));
    }
}
