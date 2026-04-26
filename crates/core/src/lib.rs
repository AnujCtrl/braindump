//! braindump-core: shared logic for capture, parsing, storage, and sync.
//!
//! ## Conflict resolution
//!
//! Across devices, conflicts are resolved by **last-write-wins** on the
//! `updated_at` timestamp. This is intentional: with one user across 2-3
//! devices, conflicts are rare and the simpler model beats CRDTs at this
//! scale. Two offline edits to the same todo will produce a deterministic
//! winner — the device whose write has the later timestamp.

pub mod capture;
pub mod model;
pub mod parser;
pub mod storage;
pub mod tags;

pub use capture::{CaptureError, CaptureOutcome, capture};
pub use model::{ParsedCapture, ParsedNote, Status, Todo};
pub use parser::{ParseError, parse};
pub use storage::{Store, StoreError, SyncAction};
pub use tags::{TagMatch, fuzzy_match};

#[must_use]
pub fn version() -> &'static str {
    env!("CARGO_PKG_VERSION")
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn version_is_set() {
        assert!(!version().is_empty());
    }
}
