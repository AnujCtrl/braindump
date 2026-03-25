// tests/core/parser.test.ts
//
// Behavioral contract for capture syntax parsing.
// The parser is the primary user-facing interface -- every interaction starts
// with text being parsed. Getting this wrong means captured todos have wrong
// text, missing tags, or spurious flags.
//
// Tested grammar:
//   input     = (token | text)* ["--" text]
//   token     = tag | source | urgent | important | note
//   tag       = "#" word
//   source    = "@" word
//   urgent    = "!!"
//   important = "!!!"
//   note      = "--note" quoted_string | "--note" word
//   escape    = "\#" (literal # in text)
//   separator = "--" (everything after is text)

import { tokenize, parseCapture } from '../../src/core/parser.js';

// ---------------------------------------------------------------------------
// tokenize
// ---------------------------------------------------------------------------

describe('tokenize', () => {
  it('splits plain text by whitespace', () => {
    expect(tokenize('hello world')).toEqual(['hello', 'world']);
  });

  it('handles multiple consecutive spaces', () => {
    expect(tokenize('hello   world')).toEqual(['hello', 'world']);
  });

  it('respects double-quoted strings as single tokens', () => {
    expect(tokenize('hello "big world" bye')).toEqual([
      'hello',
      'big world',
      'bye',
    ]);
  });

  it('returns empty array for empty string', () => {
    expect(tokenize('')).toEqual([]);
  });

  it('returns empty array for whitespace-only string', () => {
    expect(tokenize('   ')).toEqual([]);
  });

  it('strips quotes but preserves quoted content', () => {
    // Quotes are delimiters, not part of the token
    expect(tokenize('"hello world"')).toEqual(['hello world']);
  });

  it('handles unclosed quote by treating rest as single token', () => {
    // Unclosed quote: everything from the quote to end is one token
    expect(tokenize('hello "big world')).toEqual(['hello', 'big world']);
  });

  it('handles single word input', () => {
    expect(tokenize('hello')).toEqual(['hello']);
  });

  it('preserves special characters within tokens', () => {
    expect(tokenize('#tag @source !!')).toEqual(['#tag', '@source', '!!']);
  });
});

// ---------------------------------------------------------------------------
// parseCapture
// ---------------------------------------------------------------------------

describe('parseCapture', () => {
  // -- Plain text only --

  it('returns text with no tokens when input has no special syntax', () => {
    const result = parseCapture('buy some groceries');
    expect(result.text).toBe('buy some groceries');
    expect(result.tags).toEqual([]);
    expect(result.source).toBeNull();
    expect(result.urgent).toBe(false);
    expect(result.important).toBe(false);
    expect(result.notes).toEqual([]);
  });

  it('trims leading and trailing whitespace from text', () => {
    const result = parseCapture('  buy groceries  ');
    expect(result.text).toBe('buy groceries');
  });

  // -- Tags --

  it('extracts a single #tag', () => {
    const result = parseCapture('fix server #homelab');
    expect(result.text).toBe('fix server');
    expect(result.tags).toEqual(['homelab']);
  });

  it('extracts multiple #tags preserving order', () => {
    const result = parseCapture('#work fix the bug #urgent');
    expect(result.text).toBe('fix the bug');
    expect(result.tags).toEqual(['work', 'urgent']);
  });

  it('ignores bare # with no word after it', () => {
    // A lone "#" should not create an empty tag
    const result = parseCapture('fix # server');
    expect(result.tags).toEqual([]);
    expect(result.text).toContain('fix');
    expect(result.text).toContain('server');
  });

  // -- Source --

  it('extracts @source', () => {
    const result = parseCapture('play bedwars @minecraft');
    expect(result.text).toBe('play bedwars');
    expect(result.source).toBe('minecraft');
  });

  it('uses last @source when multiple are given', () => {
    // Ambiguous input, but parser should not crash. Last one wins.
    const result = parseCapture('@first text @second');
    expect(result.source).toBe('second');
  });

  it('ignores bare @ with no word after it', () => {
    const result = parseCapture('email @ someone');
    expect(result.source).toBeNull();
  });

  // -- Urgent / Important --

  it('sets urgent=true for !! (exactly two bangs)', () => {
    const result = parseCapture('fix server !!');
    expect(result.urgent).toBe(true);
    expect(result.important).toBe(false);
    expect(result.text).toBe('fix server');
  });

  it('sets important=true for !!! (exactly three bangs)', () => {
    const result = parseCapture('fix server !!!');
    expect(result.important).toBe(true);
    expect(result.urgent).toBe(false);
    expect(result.text).toBe('fix server');
  });

  it('sets both urgent and important when both are present', () => {
    const result = parseCapture('fix server !! !!!');
    expect(result.urgent).toBe(true);
    expect(result.important).toBe(true);
  });

  it('treats single ! as plain text, not a flag', () => {
    const result = parseCapture('wow! that is great');
    expect(result.urgent).toBe(false);
    expect(result.important).toBe(false);
    expect(result.text).toContain('wow!');
  });

  it('treats !!!! (four bangs) as plain text, not urgent or important', () => {
    // Only exactly 2 or exactly 3 bangs are special tokens
    const result = parseCapture('!!!!');
    expect(result.urgent).toBe(false);
    expect(result.important).toBe(false);
    expect(result.text).toContain('!!!!');
  });

  // -- Escaped hash --

  it('converts escaped \\# to literal # in text', () => {
    const result = parseCapture('fix issue \\#42');
    expect(result.text).toBe('fix issue #42');
    expect(result.tags).toEqual([]);
  });

  it('handles multiple escaped hashes', () => {
    const result = parseCapture('colors \\#ff0000 and \\#00ff00');
    expect(result.text).toBe('colors #ff0000 and #00ff00');
    expect(result.tags).toEqual([]);
  });

  // -- Double-dash separator --

  it('treats everything after " -- " as plain text', () => {
    const result = parseCapture('capture this -- #not-a-tag !! @not-source');
    expect(result.text).toBe('capture this #not-a-tag !! @not-source');
    expect(result.tags).toEqual([]);
    expect(result.urgent).toBe(false);
    expect(result.source).toBeNull();
  });

  it('handles -- at end of input (trailing separator)', () => {
    const result = parseCapture('just text --');
    expect(result.text).toBe('just text');
  });

  it('handles -- at start of input to capture reserved words', () => {
    // "-- dump the drives" should capture "dump the drives" as text
    const result = parseCapture('-- dump the drives');
    expect(result.text).toBe('dump the drives');
  });

  // -- Notes --

  it('extracts --note with quoted string', () => {
    const result = parseCapture('fix server --note "check logs first"');
    expect(result.text).toBe('fix server');
    expect(result.notes).toEqual(['check logs first']);
  });

  it('extracts --note with unquoted single word', () => {
    const result = parseCapture('fix server --note important');
    expect(result.text).toBe('fix server');
    expect(result.notes).toEqual(['important']);
  });

  it('handles --note with no value after it', () => {
    const result = parseCapture('fix server --note');
    expect(result.text).toBe('fix server');
    // Should not crash; note is empty or omitted
    expect(result.notes).toEqual([]);
  });

  // -- Shell-escaped bangs --

  it('handles shell-escaped bangs \\!\\! as urgent', () => {
    // When bash escapes !, the parser receives \!\!
    const result = parseCapture('fix server \\!\\!');
    expect(result.urgent).toBe(true);
    expect(result.text).toBe('fix server');
  });

  it('handles shell-escaped bangs \\!\\!\\! as important', () => {
    const result = parseCapture('fix server \\!\\!\\!');
    expect(result.important).toBe(true);
    expect(result.text).toBe('fix server');
  });

  // -- Combined tokens --

  it('parses all token types in a single input', () => {
    const result = parseCapture(
      '#homelab fix the DNS @minecraft !! --note "urgent fix" !!!'
    );
    expect(result.text).toBe('fix the DNS');
    expect(result.tags).toEqual(['homelab']);
    expect(result.source).toBe('minecraft');
    expect(result.urgent).toBe(true);
    expect(result.important).toBe(true);
    expect(result.notes).toEqual(['urgent fix']);
  });

  it('handles tags mixed into middle of text', () => {
    const result = parseCapture('before #tag after');
    expect(result.text).toBe('before after');
    expect(result.tags).toEqual(['tag']);
  });

  // -- Edge cases --

  it('returns empty text for input with only tokens', () => {
    const result = parseCapture('#work @cli !!');
    expect(result.text).toBe('');
    expect(result.tags).toEqual(['work']);
    expect(result.source).toBe('cli');
    expect(result.urgent).toBe(true);
  });

  it('handles empty input', () => {
    const result = parseCapture('');
    expect(result.text).toBe('');
    expect(result.tags).toEqual([]);
    expect(result.source).toBeNull();
    expect(result.urgent).toBe(false);
    expect(result.important).toBe(false);
    expect(result.notes).toEqual([]);
  });

  it('preserves text that contains the word "note" without the -- prefix', () => {
    const result = parseCapture('take note of this');
    expect(result.text).toBe('take note of this');
    expect(result.notes).toEqual([]);
  });

  it('handles double-dash separator combined with --note before it', () => {
    const result = parseCapture(
      '--note "check logs" fix server -- dump the cache'
    );
    expect(result.notes).toEqual(['check logs']);
    expect(result.text).toContain('fix server');
    expect(result.text).toContain('dump the cache');
  });
});
