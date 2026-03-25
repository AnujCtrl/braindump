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

import Database from 'better-sqlite3';
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

let db: Database.Database;
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
