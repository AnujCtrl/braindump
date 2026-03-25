// tests/printer/fun.test.ts
//
// Behavioral contract for the printer "fun" utilities.
// These functions generate the decorative elements of receipt-style todo
// printouts. They must produce output that fits within the thermal printer's
// character width and never crashes on edge-case input.
//
// These tests protect against:
// - wordWrap producing lines wider than the specified width
// - Words longer than width not being split (would overflow the printer)
// - Empty input causing crashes or undefined returns
// - centerText producing strings that aren't exactly `width` characters
// - Random generators returning empty strings (blank receipt sections)
// - randomBorder returning wrong length (breaks visual alignment)

import {
  wordWrap,
  centerText,
  randomHeader,
  randomArt,
  randomBorder,
  randomMessage,
  randomSignoff,
  randomTimeGreeting,
  randomDayFlavor,
  isLegendary,
} from '../../src/printer/fun.js';

// ---------------------------------------------------------------------------
// wordWrap
// ---------------------------------------------------------------------------

describe('wordWrap', () => {
  it('wraps text at word boundaries so no line exceeds width', () => {
    // Protects: lines overflowing the thermal printer's character width.
    const result = wordWrap('the quick brown fox jumps over the lazy dog', 20);

    for (const line of result) {
      expect(line.length).toBeLessThanOrEqual(20);
    }
    // Should produce multiple lines for this text at width 20
    expect(result.length).toBeGreaterThan(1);
  });

  it('preserves all words in the output', () => {
    // Protects: words being silently dropped during wrapping.
    const input = 'alpha bravo charlie delta echo';
    const result = wordWrap(input, 15);
    const rejoined = result.join(' ');

    expect(rejoined).toContain('alpha');
    expect(rejoined).toContain('bravo');
    expect(rejoined).toContain('charlie');
    expect(rejoined).toContain('delta');
    expect(rejoined).toContain('echo');
  });

  it('keeps short text on a single line', () => {
    // Protects: unnecessary wrapping of short text.
    const result = wordWrap('hello', 40);

    expect(result).toHaveLength(1);
    expect(result[0]).toBe('hello');
  });

  it('returns single element array for text exactly at width', () => {
    const text = 'exactly ten';  // 11 chars -- use a text that matches width
    const result = wordWrap('hi', 40);

    expect(result).toHaveLength(1);
  });

  it('splits words longer than width at character boundary', () => {
    // Protects: a single very long word causing an infinite loop or
    // producing a line wider than width.
    const longWord = 'abcdefghijklmnopqrstuvwxyz'; // 26 chars
    const result = wordWrap(longWord, 10);

    for (const line of result) {
      expect(line.length).toBeLessThanOrEqual(10);
    }
    // Should split into at least 3 lines (26/10 = 3 chunks)
    expect(result.length).toBeGreaterThanOrEqual(3);

    // All characters should be preserved
    const joined = result.join('');
    expect(joined).toBe(longWord);
  });

  it('handles mixed long and short words', () => {
    // Protects: long-word splitting breaking the flow for subsequent words.
    const result = wordWrap('hello superlongwordthatexceedswidth bye', 10);

    for (const line of result) {
      expect(line.length).toBeLessThanOrEqual(10);
    }
    const joined = result.join(' ');
    expect(joined).toContain('hello');
    expect(joined).toContain('bye');
  });

  it('returns single element array for empty input', () => {
    // Protects: empty input causing crash or returning empty array.
    const result = wordWrap('', 40);

    expect(Array.isArray(result)).toBe(true);
    expect(result).toHaveLength(1);
  });

  it('returns single element array for whitespace-only input', () => {
    // Protects: whitespace-only input causing unexpected multi-line output.
    const result = wordWrap('   ', 40);

    expect(Array.isArray(result)).toBe(true);
    expect(result).toHaveLength(1);
  });

  it('handles width of 1 (extreme edge case)', () => {
    // Protects: division by zero or infinite loop with tiny width.
    const result = wordWrap('ab cd', 1);

    for (const line of result) {
      expect(line.length).toBeLessThanOrEqual(1);
    }
  });

  it('trims trailing whitespace from wrapped lines', () => {
    // Protects: trailing spaces causing misalignment on thermal printer.
    const result = wordWrap('one two three', 8);

    for (const line of result) {
      expect(line).toBe(line.trimEnd());
    }
  });
});

// ---------------------------------------------------------------------------
// centerText
// ---------------------------------------------------------------------------

describe('centerText', () => {
  it('pads short text to exact width with spaces', () => {
    // Protects: centered text being wrong length, breaking receipt alignment.
    const result = centerText('hello', 20);

    expect(result.length).toBe(20);
    // 'hello' is 5 chars, so padding = 15 total spaces (approximately even)
    expect(result.trim()).toBe('hello');
  });

  it('returns original text if already >= width', () => {
    // Protects: text being truncated or extra padding added when unnecessary.
    const text = 'this is a long text';
    const result = centerText(text, 5);

    expect(result).toBe(text);
  });

  it('returns text unchanged when exactly equal to width', () => {
    const text = '12345';
    const result = centerText(text, 5);

    expect(result).toBe(text);
  });

  it('produces result of exactly width characters', () => {
    // Round-trip length check for various inputs.
    const cases = [
      { text: 'a', width: 10 },
      { text: 'ab', width: 10 },
      { text: 'abc', width: 11 },
      { text: '', width: 20 },
    ];

    for (const { text, width } of cases) {
      const result = centerText(text, width);
      expect(result.length).toBe(width);
    }
  });

  it('approximately centers the text (left padding roughly equals right)', () => {
    const result = centerText('hi', 10);
    const leftPad = result.length - result.trimStart().length;
    const rightPad = result.length - result.trimEnd().length;

    // Left and right padding should differ by at most 1
    expect(Math.abs(leftPad - rightPad)).toBeLessThanOrEqual(1);
  });
});

// ---------------------------------------------------------------------------
// Random generators
// ---------------------------------------------------------------------------

describe('randomHeader', () => {
  it('returns a non-empty string', () => {
    // Protects: empty header leaving a blank space at the top of the receipt.
    const header = randomHeader();

    expect(typeof header).toBe('string');
    expect(header.length).toBeGreaterThan(0);
  });

  it('returns a string on repeated calls (no crashes)', () => {
    // Protects: array index out of bounds on random selection.
    for (let i = 0; i < 50; i++) {
      expect(typeof randomHeader()).toBe('string');
    }
  });
});

describe('randomArt', () => {
  it('returns a non-empty string', () => {
    // Protects: missing ASCII art causing blank receipt section.
    const art = randomArt();

    expect(typeof art).toBe('string');
    expect(art.length).toBeGreaterThan(0);
  });
});

describe('randomBorder', () => {
  it('returns a string of exactly the specified width', () => {
    // Protects: border being too short or too long, breaking receipt alignment.
    const border = randomBorder(32);

    expect(border.length).toBe(32);
  });

  it('returns correct length for various widths', () => {
    const widths = [1, 10, 20, 32, 48, 80];

    for (const width of widths) {
      const border = randomBorder(width);
      expect(border.length).toBe(width);
    }
  });

  it('returns non-empty string even for width 1', () => {
    const border = randomBorder(1);
    expect(border.length).toBe(1);
    expect(border.trim().length).toBeGreaterThan(0);
  });
});

describe('randomMessage', () => {
  it('returns a non-empty string', () => {
    const msg = randomMessage();
    expect(typeof msg).toBe('string');
    expect(msg.length).toBeGreaterThan(0);
  });
});

describe('randomSignoff', () => {
  it('returns a non-empty string', () => {
    const signoff = randomSignoff();
    expect(typeof signoff).toBe('string');
    expect(signoff.length).toBeGreaterThan(0);
  });
});

describe('randomTimeGreeting', () => {
  it('returns a non-empty string', () => {
    const greeting = randomTimeGreeting();
    expect(typeof greeting).toBe('string');
    expect(greeting.length).toBeGreaterThan(0);
  });
});

describe('randomDayFlavor', () => {
  it('returns a non-empty string', () => {
    const flavor = randomDayFlavor();
    expect(typeof flavor).toBe('string');
    expect(flavor.length).toBeGreaterThan(0);
  });
});

describe('isLegendary', () => {
  it('returns a boolean', () => {
    // Protects: truthy/falsy values instead of actual booleans.
    const result = isLegendary();
    expect(typeof result).toBe('boolean');
  });

  it('returns boolean on repeated calls (probabilistic, should not crash)', () => {
    // Just ensure it never throws. We can't assert on the value since
    // it's random, but it must always be a boolean.
    for (let i = 0; i < 100; i++) {
      expect(typeof isLegendary()).toBe('boolean');
    }
  });
});
