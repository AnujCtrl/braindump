// tests/core/id.test.ts
//
// Behavioral contract for ID generation:
// - IDs are 8-character lowercase hex strings (4 random bytes)
// - IDs are statistically unique (no collisions in practical usage)
// - isUniqueId correctly checks against an existing set
//
// These tests protect against:
// - ID format changes that break file parsing (IDs are embedded in markdown metadata)
// - Weak randomness causing collisions
// - Off-by-one in uniqueness checking

import { generateId, isUniqueId } from '../../src/core/id.js';

describe('generateId', () => {
  it('returns an 8-character string', () => {
    const id = generateId();
    expect(id).toHaveLength(8);
  });

  it('returns only lowercase hex characters', () => {
    const id = generateId();
    expect(id).toMatch(/^[0-9a-f]{8}$/);
  });

  it('generates 100 unique IDs with no collisions', () => {
    // With 4 bytes (4.3 billion possibilities), 100 should never collide.
    // If this test fails, the PRNG is broken or seeded incorrectly.
    const ids = new Set<string>();
    for (let i = 0; i < 100; i++) {
      ids.add(generateId());
    }
    expect(ids.size).toBe(100);
  });

  it('generates different IDs on successive calls', () => {
    // Minimal sanity check: two consecutive calls should differ.
    // This catches a constant-return bug.
    const a = generateId();
    const b = generateId();
    expect(a).not.toBe(b);
  });
});

describe('isUniqueId', () => {
  it('returns true when ID is not in the existing set', () => {
    const existing = new Set(['aaa11111', 'bbb22222']);
    expect(isUniqueId('ccc33333', existing)).toBe(true);
  });

  it('returns false when ID is already in the existing set', () => {
    const existing = new Set(['aaa11111', 'bbb22222']);
    expect(isUniqueId('aaa11111', existing)).toBe(false);
  });

  it('returns true for empty existing set', () => {
    expect(isUniqueId('anything', new Set())).toBe(true);
  });

  it('is case-sensitive -- "AAA11111" is unique when "aaa11111" exists', () => {
    // IDs are always lowercase hex, but the uniqueness check should be exact.
    const existing = new Set(['aaa11111']);
    expect(isUniqueId('AAA11111', existing)).toBe(true);
  });
});
