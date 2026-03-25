// tests/cli/done.test.ts
//
// Behavioral contract for the CLI done command handler.
// handleDone marks a todo as complete. It's the final step in the
// inbox -> active -> done lifecycle. Bugs here mean items stuck as "active"
// or silently double-completed.
//
// These tests protect against:
// - Done not actually setting status to 'done' and done flag to true
// - Sync action not being enqueued (Linear stays out of sync)
// - Missing ID not throwing (silent failure)
// - Already-done items being re-done (double-counting, duplicate sync events)

import { handleDone } from '../../src/cli/done.js';
import type { Store } from '../../src/core/store.js';
import type { Todo } from '../../src/core/models.js';

/** Helper to build a valid Todo with sensible defaults. */
function makeTodo(overrides: Partial<Todo> = {}): Todo {
  const now = new Date().toISOString();
  return {
    id: 'done1111',
    linearId: null,
    text: 'Done test',
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

/** Creates a mock Store with controllable behavior. */
function createMockStore(overrides: Partial<Record<string, unknown>> = {}): Store {
  return {
    create: vi.fn(),
    getById: vi.fn().mockReturnValue(null),
    listByStatus: vi.fn().mockReturnValue([]),
    listOpen: vi.fn().mockReturnValue([]),
    listAll: vi.fn().mockReturnValue([]),
    listByTag: vi.fn().mockReturnValue([]),
    update: vi.fn(),
    delete: vi.fn(),
    allIds: vi.fn().mockReturnValue(new Set()),
    getInfoCounts: vi.fn().mockReturnValue({ unprocessed: 0, active: 0, looping: 0 }),
    enqueueSyncAction: vi.fn(),
    pendingSyncActions: vi.fn().mockReturnValue([]),
    ...overrides,
  } as unknown as Store;
}

// ---------------------------------------------------------------------------
// handleDone
// ---------------------------------------------------------------------------

describe('handleDone', () => {
  it('sets status to "done" and done flag to true', () => {
    // Protects: the core completion behavior. If status or done flag is
    // not set, the todo appears in open lists forever.
    const todo = makeTodo({ id: 'abc12345', status: 'active' });
    const store = createMockStore({ getById: vi.fn().mockReturnValue(todo) });

    handleDone(store, 'abc12345');

    expect(store.update).toHaveBeenCalledTimes(1);
    const updateArgs = (store.update as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(updateArgs[0]).toBe('abc12345');
    expect(updateArgs[1]).toMatchObject({
      status: 'done',
      done: true,
    });
  });

  it('enqueues a "status_change" sync action', () => {
    // Protects: Linear not being notified when a todo is completed.
    // Without the sync action, the Linear issue stays open.
    const todo = makeTodo({ id: 'sync-done', status: 'active' });
    const store = createMockStore({ getById: vi.fn().mockReturnValue(todo) });

    handleDone(store, 'sync-done');

    expect(store.enqueueSyncAction).toHaveBeenCalledTimes(1);
    const syncArgs = (store.enqueueSyncAction as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(syncArgs[0]).toBe('sync-done');
    expect(syncArgs[1]).toBe('status_change');
  });

  it('throws when ID is not found in store', () => {
    // Protects: silent failure when user types a wrong/partial ID.
    // Must throw so the CLI can show an error message.
    const store = createMockStore({ getById: vi.fn().mockReturnValue(null) });

    expect(() => handleDone(store, 'nonexistent')).toThrow();
    expect(store.update).not.toHaveBeenCalled();
    expect(store.enqueueSyncAction).not.toHaveBeenCalled();
  });

  it('throws when todo is already done', () => {
    // Protects: double-completion creating duplicate sync events.
    // Completing an already-done item is always a user error.
    const todo = makeTodo({ id: 'already-done', status: 'done', done: true });
    const store = createMockStore({ getById: vi.fn().mockReturnValue(todo) });

    expect(() => handleDone(store, 'already-done')).toThrow();
    expect(store.update).not.toHaveBeenCalled();
    expect(store.enqueueSyncAction).not.toHaveBeenCalled();
  });

  it('works for inbox items (not just active)', () => {
    // Protects: done only working from "active" status.
    // Users should be able to mark inbox items as done directly.
    const todo = makeTodo({ id: 'inbox-done', status: 'inbox' });
    const store = createMockStore({ getById: vi.fn().mockReturnValue(todo) });

    handleDone(store, 'inbox-done');

    expect(store.update).toHaveBeenCalledTimes(1);
    const updateArgs = (store.update as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(updateArgs[1]).toMatchObject({ status: 'done', done: true });
  });

  it('works for stale items', () => {
    // Protects: stale items being stuck and unable to be completed.
    const todo = makeTodo({ id: 'stale-done', status: 'stale', staleCount: 3 });
    const store = createMockStore({ getById: vi.fn().mockReturnValue(todo) });

    handleDone(store, 'stale-done');

    expect(store.update).toHaveBeenCalledTimes(1);
    const updateArgs = (store.update as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(updateArgs[1]).toMatchObject({ status: 'done', done: true });
  });

  it('works for waiting items', () => {
    // Protects: waiting items unable to be directly completed.
    const todo = makeTodo({ id: 'waiting-done', status: 'waiting' });
    const store = createMockStore({ getById: vi.fn().mockReturnValue(todo) });

    handleDone(store, 'waiting-done');

    expect(store.update).toHaveBeenCalledTimes(1);
  });

  it('updates statusChangedAt when marking done', () => {
    // Protects: statusChangedAt not being updated, causing stale detection
    // to use the old timestamp.
    const todo = makeTodo({ id: 'ts-done', status: 'active' });
    const store = createMockStore({ getById: vi.fn().mockReturnValue(todo) });

    handleDone(store, 'ts-done');

    const updateArgs = (store.update as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(updateArgs[1].statusChangedAt).toBeDefined();
    expect(typeof updateArgs[1].statusChangedAt).toBe('string');
  });
});
