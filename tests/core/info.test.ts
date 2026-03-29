// tests/core/info.test.ts
//
// Behavioral contract for the info line displayed after every CLI command.
// The info line summarizes system state: unprocessed count, active count,
// looping count. It's the user's primary dashboard at-a-glance.
//
// These tests protect against:
// - Wrong counts (over/under counting statuses)
// - Format string regression (changing separator, labels, or dash style)
// - Empty string not returned for all-zero counts (blank line noise)
// - Missing sections when only some counts are nonzero

import { Database } from 'bun:sqlite';
import { initDb } from '../../src/core/db.js';
import { Store } from '../../src/core/store.js';
import { getInfoLine, formatInfoLine } from '../../src/core/info.js';
import type { Todo } from '../../src/core/models.js';

let db: Database;
let store: Store;

beforeEach(() => {
  db = new Database(':memory:');
  initDb(db);
  store = new Store(db);
});

afterEach(() => {
  db.close();
});

/** Helper to build a valid Todo with sensible defaults. */
function makeTodo(overrides: Partial<Todo> = {}): Todo {
  const now = new Date().toISOString();
  return {
    id: 'info1111',
    linearId: null,
    text: 'Info test',
    source: 'cli',
    status: 'inbox',
    createdAt: now,
    statusChangedAt: now,
    urgent: false,
    important: false,
    staleCount: 0,
    tags: ['braindump'],
    notes: [],
    subtasks: [],
    done: false,
    updatedAt: now,
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// getInfoLine
// ---------------------------------------------------------------------------

describe('getInfoLine', () => {
  it('returns correct counts from store', () => {
    store.create(makeTodo({ id: 'u1', status: 'unprocessed' }));
    store.create(makeTodo({ id: 'u2', status: 'unprocessed' }));
    store.create(makeTodo({ id: 'a1', status: 'active' }));
    store.create(makeTodo({ id: 'loop1', status: 'inbox', staleCount: 3 }));
    store.create(makeTodo({ id: 'done1', status: 'done', done: true }));

    const info = getInfoLine(store);
    expect(info.unprocessed).toBe(2);
    expect(info.active).toBe(1);
    expect(info.looping).toBe(1);
  });

  it('returns all zeros for empty store', () => {
    const info = getInfoLine(store);
    expect(info.unprocessed).toBe(0);
    expect(info.active).toBe(0);
    expect(info.looping).toBe(0);
  });

  it('counts looping correctly -- staleCount >= 2, excludes done', () => {
    store.create(makeTodo({ id: 'sc1', staleCount: 1, status: 'inbox' }));
    store.create(makeTodo({ id: 'sc2', staleCount: 2, status: 'inbox' }));
    store.create(makeTodo({ id: 'sc3', staleCount: 2, status: 'done', done: true }));

    const info = getInfoLine(store);
    // sc1 has staleCount 1, not looping
    // sc2 has staleCount 2, looping
    // sc3 has staleCount 2 but is done, should be excluded
    expect(info.looping).toBe(1);
  });
});

// ---------------------------------------------------------------------------
// formatInfoLine
// ---------------------------------------------------------------------------

describe('formatInfoLine', () => {
  it('returns empty string when all counts are zero', () => {
    const result = formatInfoLine({ unprocessed: 0, active: 0, looping: 0 });
    expect(result).toBe('');
  });

  it('formats all three counts with pipe separators', () => {
    const result = formatInfoLine({ unprocessed: 2, active: 3, looping: 1 });
    expect(result).toBe('-- Unprocessed: 2 | Active: 3 | Looping: 1 --');
  });

  it('shows only active when others are zero', () => {
    const result = formatInfoLine({ unprocessed: 0, active: 3, looping: 0 });
    expect(result).toBe('-- Active: 3 --');
  });

  it('shows only unprocessed when others are zero', () => {
    const result = formatInfoLine({ unprocessed: 5, active: 0, looping: 0 });
    expect(result).toBe('-- Unprocessed: 5 --');
  });

  it('shows only looping when others are zero', () => {
    const result = formatInfoLine({ unprocessed: 0, active: 0, looping: 4 });
    expect(result).toBe('-- Looping: 4 --');
  });

  it('shows two counts with pipe separator when third is zero', () => {
    const result = formatInfoLine({ unprocessed: 1, active: 0, looping: 2 });
    expect(result).toBe('-- Unprocessed: 1 | Looping: 2 --');
  });

  it('preserves order: Unprocessed, Active, Looping', () => {
    // The order in the formatted string should always be:
    // Unprocessed first, Active second, Looping third
    const result = formatInfoLine({ unprocessed: 1, active: 2, looping: 3 });
    const unprocessedIdx = result.indexOf('Unprocessed');
    const activeIdx = result.indexOf('Active');
    const loopingIdx = result.indexOf('Looping');
    expect(unprocessedIdx).toBeLessThan(activeIdx);
    expect(activeIdx).toBeLessThan(loopingIdx);
  });

  it('starts with "-- " and ends with " --"', () => {
    const result = formatInfoLine({ unprocessed: 1, active: 0, looping: 0 });
    expect(result).toMatch(/^-- .+ --$/);
  });
});
