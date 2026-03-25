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

  // -- Additional edge cases --

  // Protects: bare "#" with no word after it should not create an empty tag
  // or add "#" to the text output. The token is simply discarded.
  it('bare # as a standalone token is silently ignored', () => {
    const result = parseCapture('# just a hash');
    expect(result.tags).toEqual([]);
    // "just a hash" should remain as text; the bare "#" is dropped
    expect(result.text).toBe('just a hash');
  });

  // Protects: multiple @source tokens -- last one wins per spec.
  // This is important because a user might accidentally type two sources.
  it('last @source wins when multiple are present', () => {
    const result = parseCapture('@discord mentioned this @slack');
    expect(result.source).toBe('slack');
    expect(result.text).toBe('mentioned this');
  });

  // Protects: "--" alone at start of input (no trailing text after separator).
  // Edge case where user types just "--" which means "everything after is plain text"
  // but there IS nothing after.
  it('"--" alone as entire input produces empty text', () => {
    const result = parseCapture('--');
    expect(result.text).toBe('');
    expect(result.tags).toEqual([]);
    expect(result.source).toBeNull();
  });

  // Protects: "-- " at start means everything is plain text, including tokens.
  it('"-- " at start of input treats entire rest as plain text', () => {
    const result = parseCapture('-- #tag @source !! !!!');
    expect(result.text).toBe('#tag @source !! !!!');
    expect(result.tags).toEqual([]);
    expect(result.source).toBeNull();
    expect(result.urgent).toBe(false);
    expect(result.important).toBe(false);
  });

  // Protects: both !! and !!! present as adjacent tokens.
  // Both flags should be set independently.
  it('adjacent !! and !!! both set their respective flags', () => {
    const result = parseCapture('task !! !!!');
    expect(result.urgent).toBe(true);
    expect(result.important).toBe(true);
    expect(result.text).toBe('task');
  });

  // Protects: !!! followed by !! (reverse order) still sets both flags.
  it('!!! before !! still sets both flags', () => {
    const result = parseCapture('task !!! !!');
    expect(result.urgent).toBe(true);
    expect(result.important).toBe(true);
    expect(result.text).toBe('task');
  });

  // Protects: very long input does not crash or truncate.
  // Stress test for the tokenizer and parser.
  it('handles very long input (1000+ characters) without truncation', () => {
    const longWord = 'a'.repeat(500);
    const input = `${longWord} #longtag ${longWord}`;
    const result = parseCapture(input);
    expect(result.text).toBe(`${longWord} ${longWord}`);
    expect(result.tags).toEqual(['longtag']);
    // Verify nothing was truncated
    expect(result.text.length).toBe(1001); // 500 + space + 500
  });

  // Protects: multiple --note extractions in one input.
  it('extracts multiple --note occurrences', () => {
    const result = parseCapture(
      'fix server --note "first note" --note "second note"'
    );
    expect(result.text).toBe('fix server');
    expect(result.notes).toEqual(['first note', 'second note']);
  });

  // Protects: --note with unclosed quote takes the rest of the string.
  it('--note with unclosed quote takes rest as note value', () => {
    const result = parseCapture('fix server --note "this is unclosed');
    expect(result.notes).toEqual(['this is unclosed']);
    expect(result.text).toBe('fix server');
  });

  // Protects: whitespace-only input produces same result as empty input.
  it('whitespace-only input returns empty result', () => {
    const result = parseCapture('   ');
    expect(result.text).toBe('');
    expect(result.tags).toEqual([]);
    expect(result.source).toBeNull();
    expect(result.urgent).toBe(false);
    expect(result.important).toBe(false);
    expect(result.notes).toEqual([]);
  });

  // Protects: bare @ with no word is silently ignored (not added to text).
  it('bare @ is ignored and not included in text', () => {
    const result = parseCapture('send @ email');
    expect(result.source).toBeNull();
    expect(result.text).toBe('send email');
  });

  // Protects: tag with special characters in the word part.
  // "#tag-name" should be captured as tag "tag-name".
  it('tag with hyphen is captured as a single tag', () => {
    const result = parseCapture('fix #my-tag issue');
    expect(result.tags).toEqual(['my-tag']);
    expect(result.text).toBe('fix issue');
  });

  // Protects: "--note" at the very end of input with no value.
  // Should not crash; note is simply not added.
  it('--note at end with no value does not crash', () => {
    const result = parseCapture('text --note');
    expect(result.notes).toEqual([]);
    expect(result.text).toBe('text');
  });
});
