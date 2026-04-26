//! braindump-core: shared logic for capture, parsing, storage, and sync.
//!
//! Phase 0 stub. Real modules land in Phase 1 (parser, storage) and Phase 2
//! (status state machine, stale detection, rollover).

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
