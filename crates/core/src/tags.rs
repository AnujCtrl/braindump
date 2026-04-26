//! Fuzzy matcher for unknown `#tag` tokens.
//!
//! Strategy: case-insensitive Damerau–Levenshtein distance, normalized to a
//! similarity ratio in `[0, 1]`. Matches above [`AUTO_SUGGEST_THRESHOLD`] are
//! treated as "did you mean…?"; below it, callers may auto-add the tag (since
//! v2's tag set lives in the database, the cost of a stray tag is low).

use strsim::normalized_damerau_levenshtein;

/// Similarity ratio at or above which a candidate is suggested instead of
/// silently auto-added. Empirically fine for short tag names.
pub const AUTO_SUGGEST_THRESHOLD: f64 = 0.7;

#[derive(Debug, Clone, PartialEq)]
pub struct TagMatch {
    pub candidate: String,
    pub score: f64,
}

/// Find the closest existing tag to `query`. Returns `None` if `existing` is
/// empty or the best candidate scores below [`AUTO_SUGGEST_THRESHOLD`].
#[must_use]
pub fn fuzzy_match<I, S>(query: &str, existing: I) -> Option<TagMatch>
where
    I: IntoIterator<Item = S>,
    S: AsRef<str>,
{
    let q = query.to_lowercase();
    let mut best: Option<TagMatch> = None;
    for tag in existing {
        let t = tag.as_ref();
        let score = normalized_damerau_levenshtein(&q, &t.to_lowercase());
        if score >= AUTO_SUGGEST_THRESHOLD
            && best.as_ref().is_none_or(|b| score > b.score)
        {
            best = Some(TagMatch {
                candidate: t.to_owned(),
                score,
            });
        }
    }
    best
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn finds_close_match() {
        let m = fuzzy_match("minecaft", ["minecraft", "homelab", "errands"]).unwrap();
        assert_eq!(m.candidate, "minecraft");
        assert!(m.score > 0.8, "score was {}", m.score);
    }

    #[test]
    fn ignores_unrelated_tags() {
        assert!(fuzzy_match("xylophone", ["minecraft", "errands"]).is_none());
    }

    #[test]
    fn case_insensitive() {
        let m = fuzzy_match("MineCraft", ["minecraft"]).unwrap();
        assert_eq!(m.candidate, "minecraft");
        assert!((m.score - 1.0).abs() < 1e-9);
    }

    #[test]
    fn empty_existing_returns_none() {
        let empty: Vec<&str> = Vec::new();
        assert!(fuzzy_match("anything", empty).is_none());
    }
}
