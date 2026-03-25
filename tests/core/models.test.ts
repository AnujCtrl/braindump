// tests/core/models.test.ts
//
// Behavioral contract for core model utilities:
// - Priority mapping from urgent/important flags to Linear priority integers
// - Meta comment serialization/deserialization for Linear issue descriptions
// - VALID_STATUSES exhaustive list
//
// These tests protect against:
// - Priority mapping regressions (wrong priority sent to Linear API)
// - Meta comment format drift (sync engine can't find local metadata in Linear issues)
// - Status enum drift (new status added to code but not to the constant)

import {
  type Todo,
  type TodoStatus,
  VALID_STATUSES,
  toLinearPriority,
  buildMetaComment,
  parseMetaComment,
} from '../../src/core/models.js';

// ---------------------------------------------------------------------------
// toLinearPriority
// ---------------------------------------------------------------------------

describe('toLinearPriority', () => {
  // The priority mapping is a critical contract with Linear's API.
  // Getting this wrong means issues appear with incorrect priority in Linear.

  it('returns 1 (Urgent) when urgent is true and important is false', () => {
    expect(toLinearPriority(true, false)).toBe(1);
  });

  it('returns 2 (High) when important is true and urgent is false', () => {
    expect(toLinearPriority(false, true)).toBe(2);
  });

  it('returns 1 (Urgent) when both urgent and important are true -- urgent wins', () => {
    // Spec says: both flags => Urgent (1). This is a deliberate design choice.
    expect(toLinearPriority(true, true)).toBe(1);
  });

  it('returns 0 (No priority) when neither flag is set', () => {
    expect(toLinearPriority(false, false)).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// buildMetaComment / parseMetaComment
// ---------------------------------------------------------------------------

describe('buildMetaComment', () => {
  // The meta comment is embedded in Linear issue descriptions so the sync
  // engine can map Linear issues back to local todos. If the format breaks,
  // sync silently fails to find local items.

  it('produces a valid HTML comment with JSON payload', () => {
    const result = buildMetaComment({
      source: 'cli',
      staleCount: 0,
      localId: 'a1b2c3d4',
    });

    // Must be an HTML comment so Linear renders it invisibly
    expect(result).toMatch(/^<!--\s*braindump:/);
    expect(result).toMatch(/-->$/);

    // Must contain valid JSON between the markers
    const jsonMatch = result.match(/braindump:({.*})/);
    expect(jsonMatch).not.toBeNull();
    const parsed = JSON.parse(jsonMatch![1]);
    expect(parsed.source).toBe('cli');
    expect(parsed.staleCount).toBe(0);
    expect(parsed.localId).toBe('a1b2c3d4');
  });

  it('encodes non-zero staleCount correctly', () => {
    const result = buildMetaComment({
      source: 'api',
      staleCount: 3,
      localId: 'ff001122',
    });
    const jsonMatch = result.match(/braindump:({.*})/);
    expect(jsonMatch).not.toBeNull();
    const parsed = JSON.parse(jsonMatch![1]);
    expect(parsed.staleCount).toBe(3);
    expect(parsed.source).toBe('api');
  });

  it('does not include extra fields beyond source, staleCount, localId', () => {
    const result = buildMetaComment({
      source: 'cli',
      staleCount: 0,
      localId: 'abcd1234',
    });
    const jsonMatch = result.match(/braindump:({.*})/);
    const parsed = JSON.parse(jsonMatch![1]);
    const keys = Object.keys(parsed).sort();
    expect(keys).toEqual(['localId', 'source', 'staleCount']);
  });
});

describe('parseMetaComment', () => {
  it('extracts metadata from a description containing a braindump comment', () => {
    const description =
      'Fix the server timeout issue\n<!-- braindump:{"source":"cli","staleCount":2,"localId":"a1b2c3d4"} -->';
    const meta = parseMetaComment(description);
    expect(meta).not.toBeNull();
    expect(meta!.source).toBe('cli');
    expect(meta!.staleCount).toBe(2);
    expect(meta!.localId).toBe('a1b2c3d4');
  });

  it('returns null when description has no braindump comment', () => {
    const description = 'Just a regular Linear issue description';
    expect(parseMetaComment(description)).toBeNull();
  });

  it('returns null for empty string', () => {
    expect(parseMetaComment('')).toBeNull();
  });

  it('returns null for null/undefined description', () => {
    // Linear API can return null descriptions
    expect(parseMetaComment(null as unknown as string)).toBeNull();
    expect(parseMetaComment(undefined as unknown as string)).toBeNull();
  });

  it('extracts metadata even when comment is in the middle of text', () => {
    const description =
      'Some preamble\n<!-- braindump:{"source":"api","staleCount":0,"localId":"deadbeef"} -->\nSome epilogue';
    const meta = parseMetaComment(description);
    expect(meta).not.toBeNull();
    expect(meta!.localId).toBe('deadbeef');
  });

  it('handles comment built by buildMetaComment (round-trip)', () => {
    const input = { source: 'cli', staleCount: 1, localId: 'cafebabe' };
    const comment = buildMetaComment(input);
    // Wrap in a typical description
    const description = `Some task description\n${comment}`;
    const parsed = parseMetaComment(description);
    expect(parsed).not.toBeNull();
    expect(parsed!.source).toBe(input.source);
    expect(parsed!.staleCount).toBe(input.staleCount);
    expect(parsed!.localId).toBe(input.localId);
  });
});

// ---------------------------------------------------------------------------
// VALID_STATUSES
// ---------------------------------------------------------------------------

describe('VALID_STATUSES', () => {
  it('contains all 6 statuses from the spec', () => {
    const expected: TodoStatus[] = [
      'unprocessed',
      'inbox',
      'active',
      'waiting',
      'done',
      'stale',
    ];
    for (const status of expected) {
      expect(VALID_STATUSES).toContain(status);
    }
    expect(VALID_STATUSES).toHaveLength(6);
  });

  it('does not contain the legacy "today" status', () => {
    // The Go codebase normalized "today" -> "active". The TS rewrite
    // should not include "today" as a valid status at all.
    expect(VALID_STATUSES).not.toContain('today');
  });
});
