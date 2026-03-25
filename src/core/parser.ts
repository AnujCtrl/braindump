// src/core/parser.ts
//
// Capture syntax parser: tokenizer and full capture input parser.
//
// Grammar:
//   input     = (token | text)* ["--" text]
//   token     = tag | source | urgent | important | note
//   tag       = "#" word
//   source    = "@" word
//   urgent    = "!!"
//   important = "!!!"
//   note      = "--note" quoted_string | "--note" word
//   escape    = "\#" (literal # in text)
//   separator = "--" (everything after " -- " is plain text)

export interface ParsedCapture {
  text: string;
  tags: string[];
  source: string | null;
  urgent: boolean;
  important: boolean;
  notes: string[];
}

/**
 * tokenize splits a string by whitespace, respecting double-quoted strings.
 * Quotes are delimiters and are not included in the resulting tokens.
 * An unclosed quote causes the rest of the string to be one token.
 * Returns [] for empty or whitespace-only input.
 */
export function tokenize(s: string): string[] {
  s = s.trim();
  if (s === '') return [];

  const tokens: string[] = [];
  let current = '';
  let inQuote = false;

  for (let i = 0; i < s.length; i++) {
    const ch = s[i];
    if (ch === '"') {
      inQuote = !inQuote;
    } else if (ch === ' ' && !inQuote) {
      if (current.length > 0) {
        tokens.push(current);
        current = '';
      }
    } else {
      current += ch;
    }
  }

  if (current.length > 0) {
    tokens.push(current);
  }

  return tokens;
}

/**
 * extractNote finds and removes --note "value" or --note value from the raw string.
 * Returns [remaining, note]. Can be called multiple times for multiple notes.
 */
function extractNote(s: string): [string, string | null] {
  const flag = '--note';
  const idx = s.indexOf(flag);
  if (idx === -1) return [s, null];

  const before = s.slice(0, idx);
  const after = s.slice(idx + flag.length).trimStart();

  if (after === '') {
    // --note at end with no value
    return [before.trimEnd(), null];
  }

  let note: string;
  let rest: string;

  if (after[0] === '"') {
    // Quoted note value — find closing quote
    const closeIdx = after.indexOf('"', 1);
    if (closeIdx === -1) {
      // No closing quote; take the rest (without leading quote)
      note = after.slice(1);
      rest = '';
    } else {
      note = after.slice(1, closeIdx);
      rest = after.slice(closeIdx + 1).trimStart();
    }
  } else {
    // Unquoted — take the next whitespace-delimited token
    const spaceIdx = after.indexOf(' ');
    if (spaceIdx === -1) {
      note = after;
      rest = '';
    } else {
      note = after.slice(0, spaceIdx);
      rest = after.slice(spaceIdx + 1);
    }
  }

  const remaining = (before + ' ' + rest).trim();
  return [remaining, note];
}

/**
 * parseCapture parses a capture input string into its structured components.
 *
 * Processing order (mirrors Go implementation):
 * 1. Extract all --note occurrences from raw string
 * 2. Split on " -- " separator (or leading "-- " at start)
 * 3. Tokenize the pre-separator portion
 * 4. For each token: normalize \! → !, then classify
 * 5. Append plain suffix (post-separator text) to text words
 * 6. Join and trim text words
 */
export function parseCapture(input: string): ParsedCapture {
  const notes: string[] = [];
  const tags: string[] = [];
  let source: string | null = null;
  let urgent = false;
  let important = false;

  // Step 1: Extract all --note occurrences from the raw string
  let raw = input;
  while (raw.includes('--note')) {
    const [remaining, note] = extractNote(raw);
    raw = remaining;
    if (note !== null) {
      notes.push(note);
    } else {
      // --note with no value — stop trying to extract more
      break;
    }
  }

  // Step 2: Split on " -- " separator or handle leading "-- "
  let preSep = raw;
  let plainSuffix = '';

  if (raw.startsWith('-- ')) {
    // Leading separator: everything after "-- " is plain text
    preSep = '';
    plainSuffix = raw.slice(3);
  } else if (raw === '--') {
    // Just "--" alone
    preSep = '';
    plainSuffix = '';
  } else {
    const sepIdx = raw.indexOf(' -- ');
    if (sepIdx !== -1) {
      preSep = raw.slice(0, sepIdx);
      plainSuffix = raw.slice(sepIdx + 4);
    } else if (raw.endsWith(' --')) {
      preSep = raw.slice(0, raw.length - 3);
      plainSuffix = '';
    }
  }

  // Step 3: Tokenize the pre-separator portion
  const tokens = tokenize(preSep);

  // Step 4: Process each token
  const textWords: string[] = [];

  for (const tok of tokens) {
    // Normalize shell-escaped exclamation marks: \! → !
    const normTok = tok.replace(/\\!/g, '!');

    if (normTok === '!!!') {
      important = true;
    } else if (normTok === '!!') {
      urgent = true;
    } else if (tok.startsWith('#')) {
      // Tag token
      const tagName = tok.slice(1);
      if (tagName !== '') {
        tags.push(tagName);
      }
      // Bare "#" with no word → ignored (not added to text)
    } else if (tok.startsWith('\\#')) {
      // Escaped hash → literal # in text
      textWords.push('#' + tok.slice(2));
    } else if (tok.startsWith('@')) {
      // Source token
      const srcName = tok.slice(1);
      if (srcName !== '') {
        source = srcName; // last @source wins
      }
      // Bare "@" with no word → ignored
    } else {
      textWords.push(tok);
    }
  }

  // Step 5: Append plain suffix text
  if (plainSuffix !== '') {
    textWords.push(plainSuffix);
  }

  // Step 6: Join and trim
  const text = textWords.join(' ').trim();

  return { text, tags, source, urgent, important, notes };
}
