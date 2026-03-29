// tests/core/tags.test.ts
//
// Behavioral contract for tag validation and fuzzy matching.
// Tags are the primary organizational mechanism -- invalid tags create
// orphaned items that the user can never filter by. Fuzzy matching prevents
// typos from silently creating wrong tags.
//
// These tests protect against:
// - Case sensitivity bugs (tags should be case-insensitive for validation)
// - Levenshtein distance miscalculation (wrong fuzzy suggestions)
// - addTag not actually making the tag valid for subsequent calls
// - Threshold drift (fuzzy match should require distance <= 3)

import { levenshtein, TagValidator } from '../../src/core/tags.js';

// ---------------------------------------------------------------------------
// levenshtein
// ---------------------------------------------------------------------------

describe('levenshtein', () => {
  it('returns 0 for identical strings', () => {
    expect(levenshtein('hello', 'hello')).toBe(0);
  });

  it('returns length of non-empty string when other is empty', () => {
    expect(levenshtein('', 'hello')).toBe(5);
    expect(levenshtein('hello', '')).toBe(5);
  });

  it('returns 0 for two empty strings', () => {
    expect(levenshtein('', '')).toBe(0);
  });

  it('computes distance 3 for kitten/sitting', () => {
    // Classic example: kitten -> sitten -> sittin -> sitting = 3
    expect(levenshtein('kitten', 'sitting')).toBe(3);
  });

  it('computes distance 1 for single character difference', () => {
    expect(levenshtein('cat', 'bat')).toBe(1);
  });

  it('computes distance for insertion', () => {
    expect(levenshtein('abc', 'abcd')).toBe(1);
  });

  it('computes distance for deletion', () => {
    expect(levenshtein('abcd', 'abc')).toBe(1);
  });

  it('is symmetric -- distance(a,b) === distance(b,a)', () => {
    expect(levenshtein('saturday', 'sunday')).toBe(
      levenshtein('sunday', 'saturday')
    );
  });

  it('handles single character strings', () => {
    expect(levenshtein('a', 'b')).toBe(1);
    expect(levenshtein('a', 'a')).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// TagValidator
// ---------------------------------------------------------------------------

describe('TagValidator', () => {
  describe('constructor', () => {
    it('accepts a string array of known tags', () => {
      const validator = new TagValidator(['homelab', 'work', 'minecraft']);
      // Should not throw; if it does, the constructor API is wrong
      expect(validator).toBeDefined();
    });

    it('accepts an empty array', () => {
      const validator = new TagValidator([]);
      expect(validator).toBeDefined();
    });
  });

  describe('isValid', () => {
    const validator = new TagValidator(['homelab', 'work', 'Minecraft']);

    it('returns true for known tag (exact case)', () => {
      expect(validator.isValid('homelab')).toBe(true);
    });

    it('returns true for known tag (case-insensitive)', () => {
      // Tags are case-insensitive per Go implementation
      expect(validator.isValid('HOMELAB')).toBe(true);
      expect(validator.isValid('Homelab')).toBe(true);
      expect(validator.isValid('minecraft')).toBe(true);
      expect(validator.isValid('MINECRAFT')).toBe(true);
    });

    it('returns false for unknown tag', () => {
      expect(validator.isValid('cooking')).toBe(false);
    });

    it('returns false for empty string', () => {
      expect(validator.isValid('')).toBe(false);
    });
  });

  describe('fuzzyMatch', () => {
    const validator = new TagValidator([
      'homelab',
      'work',
      'minecraft',
      'health',
    ]);

    it('returns closest match within distance 3', () => {
      // "homelib" is distance 1 from "homelab" (i->a)
      expect(validator.fuzzyMatch('homelib')).toBe('homelab');
    });

    it('returns null when no tag is within distance 3', () => {
      // "zzzzzzzzzzz" is far from everything
      expect(validator.fuzzyMatch('zzzzzzzzzzz')).toBeNull();
    });

    it('returns exact match as the best fuzzy match (distance 0)', () => {
      expect(validator.fuzzyMatch('work')).toBe('work');
    });

    it('is case-insensitive in comparison', () => {
      // "HOMELIB" should still fuzzy match "homelab"
      expect(validator.fuzzyMatch('HOMELIB')).toBe('homelab');
    });

    it('returns the closest match when multiple tags are within threshold', () => {
      // Create a validator where "cat" and "car" are both close to "cap"
      const v = new TagValidator(['cat', 'car', 'boat']);
      const match = v.fuzzyMatch('cap');
      // Both "cat" and "car" are distance 1 from "cap"
      // Should return one of them (implementation picks best/first)
      expect(['cat', 'car']).toContain(match);
    });

    it('handles empty input', () => {
      // Empty string has distance = length of each tag, so all are > 3
      // unless there's a 1-3 char tag
      const v = new TagValidator(['abcdef', 'ghijkl']);
      expect(v.fuzzyMatch('')).toBeNull();
    });
  });

  describe('addTag', () => {
    it('makes a newly added tag valid', () => {
      const validator = new TagValidator(['homelab']);
      expect(validator.isValid('cooking')).toBe(false);

      validator.addTag('cooking');

      expect(validator.isValid('cooking')).toBe(true);
    });

    it('does not affect existing tags', () => {
      const validator = new TagValidator(['homelab', 'work']);
      validator.addTag('cooking');
      expect(validator.isValid('homelab')).toBe(true);
      expect(validator.isValid('work')).toBe(true);
    });

    it('handles adding a tag that already exists (no-op)', () => {
      const validator = new TagValidator(['homelab']);
      validator.addTag('homelab');
      // Should not throw or duplicate
      expect(validator.isValid('homelab')).toBe(true);
    });
  });
});
