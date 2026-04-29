//! Capture-syntax parser — port of v1's [`src/core/parser.ts`](../../src/core/parser.ts).
//!
//! Grammar:
//!
//! ```text
//! input     = (token | text)* [" -- " text | "-- " text | text " --"]
//! token     = tag | source | urgent | important | note
//! tag       = "#" word
//! source    = "@" word
//! urgent    = "^^"
//! important = "^^^"
//! note      = "--note" quoted | "--note" word
//! escape    = "\#word" -> literal "#word" in body text
//! ```
//!
//! Processing order (mirrors v1 byte-for-byte so the parsers can be diffed):
//!
//! 1. Extract all `--note` occurrences from the raw string (multiple supported).
//! 2. Split on `" -- "` separator (or leading `"-- "` / trailing `" --"`).
//! 3. Tokenize the pre-separator portion (whitespace-split, quote-aware).
//! 4. Classify each token: `^^^` / `^^` / `#tag` / `\#text` / `@source` / text.
//! 5. Append the post-separator plain suffix.
//! 6. Join + trim text.

use crate::model::ParsedCapture;

#[derive(Debug, thiserror::Error)]
pub enum ParseError {
    #[error("empty capture")]
    Empty,
}

/// Parse a single capture line into structured form.
///
/// # Errors
///
/// Returns [`ParseError::Empty`] if the input has no body text after stripping
/// tokens, separators, and notes.
pub fn parse(input: &str) -> Result<ParsedCapture, ParseError> {
    let mut notes: Vec<String> = Vec::new();
    let mut tags: Vec<String> = Vec::new();
    let mut source: Option<String> = None;
    let mut urgent = false;
    let mut important = false;

    // Step 1: extract --note occurrences iteratively
    let mut raw = input.to_owned();
    while raw.contains("--note") {
        let (remaining, note) = extract_note(&raw);
        raw = remaining;
        match note {
            Some(n) => notes.push(n),
            None => break,
        }
    }

    // Step 2: split on separator
    let (pre_sep, plain_suffix) = split_separator(&raw);

    // Step 3: tokenize
    let tokens = tokenize(&pre_sep);

    // Step 4: classify
    let mut text_words: Vec<String> = Vec::new();
    for tok in tokens {
        if tok == "^^^" {
            important = true;
        } else if tok == "^^" {
            urgent = true;
        } else if let Some(rest) = tok.strip_prefix("\\#") {
            text_words.push(format!("#{rest}"));
        } else if let Some(name) = tok.strip_prefix('#') {
            if !name.is_empty() {
                tags.push(name.to_owned());
            }
        } else if let Some(name) = tok.strip_prefix('@') {
            if !name.is_empty() {
                source = Some(name.to_owned());
            }
        } else {
            text_words.push(tok);
        }
    }

    // Step 5: append plain suffix
    if !plain_suffix.is_empty() {
        text_words.push(plain_suffix);
    }

    // Step 6: join + trim
    let text = text_words.join(" ").trim().to_owned();

    if text.is_empty() && tags.is_empty() && notes.is_empty() && source.is_none() {
        return Err(ParseError::Empty);
    }

    Ok(ParsedCapture {
        text,
        tags,
        source,
        urgent,
        important,
        notes,
    })
}

/// Whitespace-split, quote-aware tokenizer. Quotes are delimiters, not part
/// of the resulting tokens. Unclosed quote → rest is one token.
pub(crate) fn tokenize(s: &str) -> Vec<String> {
    let s = s.trim();
    if s.is_empty() {
        return Vec::new();
    }

    let mut tokens = Vec::new();
    let mut current = String::new();
    let mut in_quote = false;

    for ch in s.chars() {
        if ch == '"' {
            in_quote = !in_quote;
        } else if ch == ' ' && !in_quote {
            if !current.is_empty() {
                tokens.push(std::mem::take(&mut current));
            }
        } else {
            current.push(ch);
        }
    }

    if !current.is_empty() {
        tokens.push(current);
    }

    tokens
}

/// Find and remove the first `--note <value>` occurrence. Returns
/// `(remaining_string, extracted_note)`. If `--note` exists but has no value
/// after it, returns `(remaining, None)` so the caller can break the loop.
fn extract_note(s: &str) -> (String, Option<String>) {
    const FLAG: &str = "--note";
    let Some(idx) = s.find(FLAG) else {
        return (s.to_owned(), None);
    };

    let before = &s[..idx];
    let after = s[idx + FLAG.len()..].trim_start();

    if after.is_empty() {
        return (before.trim_end().to_owned(), None);
    }

    let (note, rest) = if let Some(after_quote) = after.strip_prefix('"') {
        match after_quote.find('"') {
            Some(close) => (
                after_quote[..close].to_owned(),
                after_quote[close + 1..].trim_start().to_owned(),
            ),
            None => (after_quote.to_owned(), String::new()),
        }
    } else {
        match after.find(' ') {
            Some(space) => (after[..space].to_owned(), after[space + 1..].to_owned()),
            None => (after.to_owned(), String::new()),
        }
    };

    let remaining = format!("{before} {rest}").trim().to_owned();
    (remaining, Some(note))
}

/// Returns `(pre_separator_string, plain_suffix_string)`.
fn split_separator(raw: &str) -> (String, String) {
    if let Some(rest) = raw.strip_prefix("-- ") {
        return (String::new(), rest.to_owned());
    }
    if raw == "--" {
        return (String::new(), String::new());
    }
    if let Some(idx) = raw.find(" -- ") {
        return (raw[..idx].to_owned(), raw[idx + 4..].to_owned());
    }
    if let Some(stripped) = raw.strip_suffix(" --") {
        return (stripped.to_owned(), String::new());
    }
    (raw.to_owned(), String::new())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn parsed(text: &str, tags: &[&str]) -> ParsedCapture {
        ParsedCapture {
            text: text.to_owned(),
            tags: tags.iter().map(|t| (*t).to_owned()).collect(),
            source: None,
            urgent: false,
            important: false,
            notes: Vec::new(),
        }
    }

    #[test]
    fn parses_simple_text() {
        let p = parse("buy groceries").unwrap();
        assert_eq!(p, parsed("buy groceries", &[]));
    }

    #[test]
    fn parses_tag() {
        let p = parse("buy groceries #errands").unwrap();
        assert_eq!(p, parsed("buy groceries", &["errands"]));
    }

    #[test]
    fn parses_multiple_tags() {
        let p = parse("fix nether portal #minecraft #deep-focus").unwrap();
        assert_eq!(p, parsed("fix nether portal", &["minecraft", "deep-focus"]));
    }

    #[test]
    fn parses_urgent() {
        let p = parse("call dentist ^^").unwrap();
        assert!(p.urgent);
        assert!(!p.important);
        assert_eq!(p.text, "call dentist");
    }

    #[test]
    fn parses_important() {
        let p = parse("call dentist ^^^").unwrap();
        assert!(p.important);
        assert!(!p.urgent);
        assert_eq!(p.text, "call dentist");
    }

    #[test]
    fn parses_source() {
        let p = parse("test @cli").unwrap();
        assert_eq!(p.source.as_deref(), Some("cli"));
        assert_eq!(p.text, "test");
    }

    #[test]
    fn last_source_wins() {
        let p = parse("test @first @second").unwrap();
        assert_eq!(p.source.as_deref(), Some("second"));
    }

    #[test]
    fn parses_note_quoted() {
        let p = parse("call dentist --note \"555-1234\" #health").unwrap();
        assert_eq!(p.notes, vec!["555-1234".to_owned()]);
        assert_eq!(p.text, "call dentist");
        assert_eq!(p.tags, vec!["health".to_owned()]);
    }

    #[test]
    fn parses_note_unquoted() {
        let p = parse("call dentist --note urgent").unwrap();
        assert_eq!(p.notes, vec!["urgent".to_owned()]);
    }

    #[test]
    fn parses_multiple_notes() {
        let p = parse("x --note one --note two").unwrap();
        assert_eq!(p.notes, vec!["one".to_owned(), "two".to_owned()]);
    }

    #[test]
    fn escaped_hash_becomes_literal() {
        let p = parse("check issue \\#42 on github #work").unwrap();
        assert_eq!(p.text, "check issue #42 on github");
        assert_eq!(p.tags, vec!["work".to_owned()]);
    }

    #[test]
    fn separator_passes_text_through() {
        let p = parse("-- dump the drives").unwrap();
        assert_eq!(p.text, "dump the drives");
        assert!(p.tags.is_empty());
    }

    #[test]
    fn separator_mid_string() {
        let p = parse("text -- #not-a-tag #real-tag").unwrap();
        assert_eq!(p.text, "text #not-a-tag #real-tag");
        assert!(p.tags.is_empty());
    }

    #[test]
    fn bare_hash_is_ignored() {
        let p = parse("text # more").unwrap();
        assert_eq!(p.text, "text more");
        assert!(p.tags.is_empty());
    }

    #[test]
    fn bare_at_is_ignored() {
        let p = parse("text @ more").unwrap();
        assert_eq!(p.text, "text more");
        assert!(p.source.is_none());
    }

    #[test]
    fn empty_input_errors() {
        assert!(matches!(parse(""), Err(ParseError::Empty)));
        assert!(matches!(parse("   "), Err(ParseError::Empty)));
    }

    #[test]
    fn whitespace_around_text_trimmed() {
        let p = parse("  hello world  ").unwrap();
        assert_eq!(p.text, "hello world");
    }

    #[test]
    fn quoted_string_in_text() {
        // Quotes are tokenizer delimiters: the quoted phrase becomes a single
        // token, then text tokens rejoin with single spaces. The quotes
        // themselves are stripped.
        let p = parse("say \"hello world\" loudly").unwrap();
        assert_eq!(p.text, "say hello world loudly");
    }

    #[test]
    fn capture_with_only_tag_no_body_is_valid() {
        let p = parse("#standup").unwrap();
        assert_eq!(p.text, "");
        assert_eq!(p.tags, vec!["standup".to_owned()]);
    }

    #[test]
    fn complex_real_world_capture() {
        let p = parse(
            "fix the nether portal #minecraft #deep-focus @cli ^^ --note \"obsidian needed\"",
        )
        .unwrap();
        assert_eq!(p.text, "fix the nether portal");
        assert_eq!(
            p.tags,
            vec!["minecraft".to_owned(), "deep-focus".to_owned()]
        );
        assert_eq!(p.source.as_deref(), Some("cli"));
        assert!(p.urgent);
        assert!(!p.important);
        assert_eq!(p.notes, vec!["obsidian needed".to_owned()]);
    }
}

#[cfg(test)]
mod proptests {
    use super::*;
    use proptest::prelude::*;

    proptest! {
        // Plain ascii text without any token-like punctuation should round-trip
        // identically: no tags, no source, no flags, text == trimmed input.
        #[test]
        fn plain_text_roundtrips(s in "[a-z]{1,30}( [a-z]{1,30}){0,5}") {
            let p = parse(&s).unwrap();
            prop_assert_eq!(p.text, s.trim());
            prop_assert!(p.tags.is_empty());
            prop_assert!(p.source.is_none());
            prop_assert!(!p.urgent);
            prop_assert!(!p.important);
            prop_assert!(p.notes.is_empty());
        }

        // Any number of well-formed tags appended to text are all captured.
        #[test]
        fn tags_are_captured(
            text in "[a-z]{1,20}( [a-z]{1,20}){0,3}",
            tags in proptest::collection::vec("[a-z][a-z0-9-]{0,15}", 1..6)
        ) {
            let mut input = text.clone();
            for t in &tags {
                input.push_str(" #");
                input.push_str(t);
            }
            let p = parse(&input).unwrap();
            prop_assert_eq!(p.text, text);
            prop_assert_eq!(p.tags, tags);
        }

        // Parsing never panics on arbitrary unicode input.
        #[test]
        fn parser_never_panics(s in any::<String>()) {
            let _ = parse(&s);
        }
    }
}
