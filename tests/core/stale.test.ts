// tests/core/stale.test.ts
//
// Behavioral contract for stale detection, marking, and revival.
// Stale detection is time-sensitive and uses different thresholds for inbox
// (7 calendar days) vs active (24 hours). Getting this wrong means items
// either go stale too early (annoying) or never go stale (system dies).
//
// These tests protect against:
// - Off-by-one in day/hour calculations
// - Wrong reference time (createdAt vs statusChangedAt for active items)
// - Done/waiting/stale items incorrectly being returned as stale candidates
// - Stale count not incrementing on revive
// - Sync queue not being populated on stale/revive operations

import { Database } from 'bun:sqlite';
import { initDb } from '../../src/core/db.js';
import { Store } from '../../src/core/store.js';
import {
  findStaleItems,
  findLoopingItems,
  markStale,
  reviveTodo,
  runStaleCheck,
} from '../../src/core/stale.js';
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

/** Helper to build a Todo with specific timestamps for stale testing. */
function makeTodo(overrides: Partial<Todo> = {}): Todo {
  const now = new Date().toISOString();
  return {
    id: 'stale111',
    linearId: null,
    text: 'Stale test',
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

/** Returns an ISO string for N days ago from now. */
function daysAgo(n: number): string {
  const d = new Date();
  d.setDate(d.getDate() - n);
  return d.toISOString();
}

/** Returns an ISO string for N hours ago from now. */
function hoursAgo(n: number): string {
  const d = new Date();
  d.setTime(d.getTime() - n * 60 * 60 * 1000);
  return d.toISOString();
}

// ---------------------------------------------------------------------------
// findStaleItems
// ---------------------------------------------------------------------------

describe('findStaleItems', () => {
  it('returns inbox item older than 7 days', () => {
    const old = makeTodo({
      id: 'old-inbox',
      status: 'inbox',
      createdAt: daysAgo(8),
      statusChangedAt: daysAgo(8),
    });
    store.create(old);

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(1);
    expect(stale[0].id).toBe('old-inbox');
  });

  it('does NOT return inbox item younger than 7 days', () => {
    const recent = makeTodo({
      id: 'recent-inbox',
      status: 'inbox',
      createdAt: daysAgo(3),
      statusChangedAt: daysAgo(3),
    });
    store.create(recent);

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(0);
  });

  it('returns inbox item at exactly 7 days boundary as stale', () => {
    // Edge case: exactly 7 days should be considered stale
    // (the Go implementation uses "before cutoff" which is start-of-day 7 days ago)
    const boundary = makeTodo({
      id: 'boundary-inbox',
      status: 'inbox',
      createdAt: daysAgo(7),
      statusChangedAt: daysAgo(7),
    });
    store.create(boundary);

    // Items at exactly 7 days should be stale (created date is before the 7-day cutoff)
    const stale = findStaleItems(store);
    expect(stale).toHaveLength(1);
  });

  it('returns active item with statusChangedAt older than 24 hours', () => {
    const staleActive = makeTodo({
      id: 'old-active',
      status: 'active',
      createdAt: daysAgo(2),
      statusChangedAt: hoursAgo(25),
    });
    store.create(staleActive);

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(1);
    expect(stale[0].id).toBe('old-active');
  });

  it('does NOT return active item with recent statusChangedAt', () => {
    const recentActive = makeTodo({
      id: 'recent-active',
      status: 'active',
      createdAt: daysAgo(5),
      statusChangedAt: hoursAgo(12),
    });
    store.create(recentActive);

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(0);
  });

  it('uses statusChangedAt (not createdAt) for active items', () => {
    // Active item created 10 days ago but status changed 1 hour ago -- NOT stale
    const activeRecent = makeTodo({
      id: 'active-recent-change',
      status: 'active',
      createdAt: daysAgo(10),
      statusChangedAt: hoursAgo(1),
    });
    store.create(activeRecent);

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(0);
  });

  it('does NOT return done items regardless of age', () => {
    store.create(
      makeTodo({
        id: 'old-done',
        status: 'done',
        done: true,
        createdAt: daysAgo(30),
        statusChangedAt: daysAgo(30),
      })
    );

    expect(findStaleItems(store)).toHaveLength(0);
  });

  it('does NOT return waiting items regardless of age', () => {
    store.create(
      makeTodo({
        id: 'old-waiting',
        status: 'waiting',
        createdAt: daysAgo(30),
        statusChangedAt: daysAgo(30),
      })
    );

    expect(findStaleItems(store)).toHaveLength(0);
  });

  it('does NOT return items already marked stale', () => {
    store.create(
      makeTodo({
        id: 'already-stale',
        status: 'stale',
        createdAt: daysAgo(30),
        statusChangedAt: daysAgo(30),
      })
    );

    expect(findStaleItems(store)).toHaveLength(0);
  });

  it('returns multiple stale items from mixed statuses', () => {
    store.create(
      makeTodo({
        id: 'stale-inbox',
        status: 'inbox',
        createdAt: daysAgo(10),
        statusChangedAt: daysAgo(10),
      })
    );
    store.create(
      makeTodo({
        id: 'stale-active',
        status: 'active',
        createdAt: daysAgo(3),
        statusChangedAt: hoursAgo(48),
      })
    );
    store.create(
      makeTodo({
        id: 'not-stale',
        status: 'inbox',
        createdAt: daysAgo(1),
        statusChangedAt: daysAgo(1),
      })
    );

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(2);
    const ids = stale.map((t) => t.id).sort();
    expect(ids).toEqual(['stale-active', 'stale-inbox']);
  });
});

// ---------------------------------------------------------------------------
// findLoopingItems
// ---------------------------------------------------------------------------

describe('findLoopingItems', () => {
  it('returns items with staleCount >= 2', () => {
    store.create(
      makeTodo({ id: 'loop-1', status: 'inbox', staleCount: 2 })
    );
    store.create(
      makeTodo({ id: 'loop-2', status: 'active', staleCount: 5 })
    );
    store.create(
      makeTodo({ id: 'not-loop', status: 'inbox', staleCount: 1 })
    );

    const looping = findLoopingItems(store);
    expect(looping).toHaveLength(2);
    const ids = looping.map((t) => t.id).sort();
    expect(ids).toEqual(['loop-1', 'loop-2']);
  });

  it('excludes done items even with high staleCount', () => {
    store.create(
      makeTodo({
        id: 'done-loop',
        status: 'done',
        done: true,
        staleCount: 10,
      })
    );

    const looping = findLoopingItems(store);
    expect(looping).toHaveLength(0);
  });

  it('returns empty array when no items are looping', () => {
    store.create(makeTodo({ id: 'ok-1', staleCount: 0 }));
    store.create(makeTodo({ id: 'ok-2', staleCount: 1 }));

    expect(findLoopingItems(store)).toHaveLength(0);
  });
});

// ---------------------------------------------------------------------------
// markStale
// ---------------------------------------------------------------------------

describe('markStale', () => {
  it('sets status to "stale" and enqueues sync action', () => {
    const todo = makeTodo({ id: 'mark-1', status: 'inbox' });
    store.create(todo);

    markStale(store, 'mark-1');

    const updated = store.getById('mark-1');
    expect(updated!.status).toBe('stale');

    // Should have enqueued a sync action
    const actions = store.pendingSyncActions();
    expect(actions.length).toBeGreaterThanOrEqual(1);
    const staleAction = actions.find((a) => a.todoId === 'mark-1');
    expect(staleAction).toBeDefined();
  });

  it('updates statusChangedAt timestamp', () => {
    const oldTime = daysAgo(10);
    const todo = makeTodo({
      id: 'mark-2',
      status: 'inbox',
      statusChangedAt: oldTime,
    });
    store.create(todo);

    markStale(store, 'mark-2');

    const updated = store.getById('mark-2');
    // statusChangedAt should be updated to a recent time
    expect(new Date(updated!.statusChangedAt).getTime()).toBeGreaterThan(
      new Date(oldTime).getTime()
    );
  });
});

// ---------------------------------------------------------------------------
// reviveTodo
// ---------------------------------------------------------------------------

describe('reviveTodo', () => {
  it('sets status to "inbox" and increments staleCount', () => {
    const todo = makeTodo({ id: 'revive-1', status: 'stale', staleCount: 1 });
    store.create(todo);

    reviveTodo(store, 'revive-1');

    const updated = store.getById('revive-1');
    expect(updated!.status).toBe('inbox');
    expect(updated!.staleCount).toBe(2);
  });

  it('enqueues sync action', () => {
    const todo = makeTodo({ id: 'revive-2', status: 'stale', staleCount: 0 });
    store.create(todo);

    reviveTodo(store, 'revive-2');

    const actions = store.pendingSyncActions();
    expect(actions.length).toBeGreaterThanOrEqual(1);
    const reviveAction = actions.find((a) => a.todoId === 'revive-2');
    expect(reviveAction).toBeDefined();
  });

  it('updates statusChangedAt timestamp', () => {
    const oldTime = daysAgo(5);
    const todo = makeTodo({
      id: 'revive-3',
      status: 'stale',
      staleCount: 2,
      statusChangedAt: oldTime,
    });
    store.create(todo);

    reviveTodo(store, 'revive-3');

    const updated = store.getById('revive-3');
    expect(new Date(updated!.statusChangedAt).getTime()).toBeGreaterThan(
      new Date(oldTime).getTime()
    );
  });
});

// ---------------------------------------------------------------------------
// runStaleCheck
// ---------------------------------------------------------------------------

describe('runStaleCheck', () => {
  it('marks all stale items and returns count', () => {
    store.create(
      makeTodo({
        id: 'auto-stale-1',
        status: 'inbox',
        createdAt: daysAgo(10),
        statusChangedAt: daysAgo(10),
      })
    );
    store.create(
      makeTodo({
        id: 'auto-stale-2',
        status: 'active',
        createdAt: daysAgo(3),
        statusChangedAt: hoursAgo(48),
      })
    );
    store.create(
      makeTodo({
        id: 'not-stale',
        status: 'inbox',
        createdAt: daysAgo(1),
        statusChangedAt: daysAgo(1),
      })
    );

    const count = runStaleCheck(store);
    expect(count).toBe(2);

    // Verify both items are now stale
    expect(store.getById('auto-stale-1')!.status).toBe('stale');
    expect(store.getById('auto-stale-2')!.status).toBe('stale');
    // Non-stale item unchanged
    expect(store.getById('not-stale')!.status).toBe('inbox');
  });

  it('returns 0 when nothing is stale', () => {
    store.create(
      makeTodo({
        id: 'fresh',
        status: 'inbox',
        createdAt: daysAgo(1),
        statusChangedAt: daysAgo(1),
      })
    );

    expect(runStaleCheck(store)).toBe(0);
  });

  it('returns 0 for empty store', () => {
    expect(runStaleCheck(store)).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// Boundary precision tests
// ---------------------------------------------------------------------------

describe('stale boundary precision', () => {
  // These tests protect against off-by-one and off-by-minute errors in the
  // stale detection logic. The boundaries are:
  //   - inbox: stale at >= 7 days (using createdAt)
  //   - active: stale at >= 24 hours (using statusChangedAt)
  //
  // Getting the boundary wrong by even 1 minute means either:
  //   - Items go stale too early (annoying, user thinks system is broken)
  //   - Items never go stale (system dies from accumulation)

  /** Returns an ISO string for exactly N milliseconds ago. */
  function msAgo(ms: number): string {
    return new Date(Date.now() - ms).toISOString();
  }

  const MS_7_DAYS = 7 * 24 * 60 * 60 * 1000;
  const MS_24_HOURS = 24 * 60 * 60 * 1000;
  const MS_1_MINUTE = 60 * 1000;

  // -- Inbox 7-day boundary --

  // Protects: inbox item at exactly 7 days (to the millisecond) should be stale.
  it('inbox item at exactly 7 days is stale', () => {
    const exactTime = msAgo(MS_7_DAYS);
    store.create(
      makeTodo({
        id: 'inbox-exact-7d',
        status: 'inbox',
        createdAt: exactTime,
        statusChangedAt: exactTime,
      })
    );

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(1);
    expect(stale[0].id).toBe('inbox-exact-7d');
  });

  // Protects: inbox item 1 minute before the 7-day boundary should NOT be stale.
  it('inbox item 1 minute under 7 days is NOT stale', () => {
    const justUnder = msAgo(MS_7_DAYS - MS_1_MINUTE);
    store.create(
      makeTodo({
        id: 'inbox-under-7d',
        status: 'inbox',
        createdAt: justUnder,
        statusChangedAt: justUnder,
      })
    );

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(0);
  });

  // Protects: inbox item 1 minute past the 7-day boundary is definitely stale.
  it('inbox item 1 minute over 7 days is stale', () => {
    const justOver = msAgo(MS_7_DAYS + MS_1_MINUTE);
    store.create(
      makeTodo({
        id: 'inbox-over-7d',
        status: 'inbox',
        createdAt: justOver,
        statusChangedAt: justOver,
      })
    );

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(1);
    expect(stale[0].id).toBe('inbox-over-7d');
  });

  // -- Active 24-hour boundary --

  // Protects: active item at exactly 24 hours should be stale.
  it('active item at exactly 24 hours is stale', () => {
    const exactTime = msAgo(MS_24_HOURS);
    store.create(
      makeTodo({
        id: 'active-exact-24h',
        status: 'active',
        createdAt: daysAgo(5), // created long ago
        statusChangedAt: exactTime,
      })
    );

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(1);
    expect(stale[0].id).toBe('active-exact-24h');
  });

  // Protects: active item 1 minute under the 24-hour boundary should NOT be stale.
  it('active item 1 minute under 24 hours is NOT stale', () => {
    const justUnder = msAgo(MS_24_HOURS - MS_1_MINUTE);
    store.create(
      makeTodo({
        id: 'active-under-24h',
        status: 'active',
        createdAt: daysAgo(5),
        statusChangedAt: justUnder,
      })
    );

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(0);
  });

  // Protects: active item 1 minute past the 24-hour boundary is definitely stale.
  it('active item 1 minute over 24 hours is stale', () => {
    const justOver = msAgo(MS_24_HOURS + MS_1_MINUTE);
    store.create(
      makeTodo({
        id: 'active-over-24h',
        status: 'active',
        createdAt: daysAgo(5),
        statusChangedAt: justOver,
      })
    );

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(1);
    expect(stale[0].id).toBe('active-over-24h');
  });

  // Protects: unprocessed items should NEVER become stale regardless of age.
  // They need to go through inbox first.
  it('unprocessed item older than 7 days is NOT stale', () => {
    store.create(
      makeTodo({
        id: 'unproc-old',
        status: 'unprocessed',
        createdAt: daysAgo(30),
        statusChangedAt: daysAgo(30),
      })
    );

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(0);
  });

  // Protects: mixed boundary -- one inbox just under, one just over.
  // Only the "over" one should be stale.
  it('correctly separates items near the 7-day boundary', () => {
    store.create(
      makeTodo({
        id: 'near-under',
        status: 'inbox',
        createdAt: msAgo(MS_7_DAYS - MS_1_MINUTE),
        statusChangedAt: msAgo(MS_7_DAYS - MS_1_MINUTE),
      })
    );
    store.create(
      makeTodo({
        id: 'near-over',
        status: 'inbox',
        createdAt: msAgo(MS_7_DAYS + MS_1_MINUTE),
        statusChangedAt: msAgo(MS_7_DAYS + MS_1_MINUTE),
      })
    );

    const stale = findStaleItems(store);
    expect(stale).toHaveLength(1);
    expect(stale[0].id).toBe('near-over');
  });
});
