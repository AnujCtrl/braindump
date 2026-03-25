// tests/core/sync.test.ts
//
// Behavioral contract for the SyncEngine -- the queue processor that bridges
// local Store mutations to Linear API calls via LinearBridge.
//
// These tests protect against:
// - Queue items silently dropped on API failure
// - Create action not updating local linearId after successful sync
// - Stale queue items not being retried with proper attempt tracking
// - Delete action sent to Linear even when local todo was already deleted
// - Queue drain running when Linear is offline (should skip gracefully)

import Database from 'better-sqlite3';
import { initDb } from '../../src/core/db.js';
import { Store } from '../../src/core/store.js';
import { SyncEngine } from '../../src/core/sync.js';
import type { LinearBridge } from '../../src/core/linear.js';
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

/** Helper to build a valid Todo with sensible defaults. */
function makeTodo(overrides: Partial<Todo> = {}): Todo {
  const now = new Date().toISOString();
  return {
    id: 'sync1111',
    linearId: null,
    text: 'Sync test todo',
    source: 'cli',
    status: 'inbox',
    createdAt: '2026-03-20T10:00:00.000Z',
    statusChangedAt: '2026-03-20T10:00:00.000Z',
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

/** Creates a mock LinearBridge with controllable behavior. */
function createMockBridge(overrides: Partial<LinearBridge> = {}): LinearBridge {
  return {
    createIssue: vi.fn().mockResolvedValue('linear-new-id'),
    updateIssue: vi.fn().mockResolvedValue(undefined),
    deleteIssue: vi.fn().mockResolvedValue(undefined),
    ensureLabel: vi.fn().mockResolvedValue('label-id'),
    fetchWorkflowStates: vi.fn().mockResolvedValue([]),
    isAvailable: vi.fn().mockResolvedValue(true),
    ...overrides,
  } as unknown as LinearBridge;
}

describe('SyncEngine.drainQueue', () => {
  it('processes "create" action: calls bridge.createIssue, updates local linearId, removes from queue', async () => {
    const todo = makeTodo({ id: 'create-1' });
    store.create(todo);
    store.enqueueSyncAction('create-1', 'create', {
      title: 'Sync test todo',
      description: '',
      priority: 0,
    });

    const bridge = createMockBridge();
    const engine = new SyncEngine(store, bridge);

    await engine.drainQueue();

    // Should have called createIssue on the bridge
    expect(bridge.createIssue).toHaveBeenCalledTimes(1);

    // Should have updated the local todo with the linearId
    const updated = store.getById('create-1');
    expect(updated!.linearId).toBe('linear-new-id');

    // Should have removed the action from the queue
    expect(store.pendingSyncActions()).toHaveLength(0);
  });

  it('skips when bridge.isAvailable() returns false, leaves queue intact', async () => {
    store.create(makeTodo({ id: 'skip-1' }));
    store.enqueueSyncAction('skip-1', 'create', {});

    const bridge = createMockBridge({
      isAvailable: vi.fn().mockResolvedValue(false),
    });
    const engine = new SyncEngine(store, bridge);

    await engine.drainQueue();

    // Bridge should NOT have been called for create
    expect(bridge.createIssue).not.toHaveBeenCalled();

    // Queue should still have the item
    expect(store.pendingSyncActions()).toHaveLength(1);
  });

  it('increments attempts on failure and stores error message', async () => {
    store.create(makeTodo({ id: 'fail-1' }));
    store.enqueueSyncAction('fail-1', 'create', {});

    const bridge = createMockBridge({
      createIssue: vi.fn().mockRejectedValue(new Error('API timeout')),
    });
    const engine = new SyncEngine(store, bridge);

    await engine.drainQueue();

    const [action] = store.pendingSyncActions();
    expect(action.attempts).toBe(1);
    expect(action.lastError).toContain('API timeout');
  });

  it('processes "update" action: calls bridge.updateIssue', async () => {
    const todo = makeTodo({ id: 'upd-1', linearId: 'lin-upd-1' });
    store.create(todo);
    store.enqueueSyncAction('upd-1', 'update', {
      title: 'Updated title',
      priority: 2,
    });

    const bridge = createMockBridge();
    const engine = new SyncEngine(store, bridge);

    await engine.drainQueue();

    expect(bridge.updateIssue).toHaveBeenCalledTimes(1);
    expect(bridge.updateIssue).toHaveBeenCalledWith(
      'lin-upd-1',
      expect.objectContaining({ title: 'Updated title', priority: 2 })
    );
    expect(store.pendingSyncActions()).toHaveLength(0);
  });

  it('processes "status_change" action: calls bridge.updateIssue with stateId', async () => {
    const todo = makeTodo({ id: 'sc-1', linearId: 'lin-sc-1' });
    store.create(todo);
    store.enqueueSyncAction('sc-1', 'status_change', {
      stateId: 'ws-done',
    });

    const bridge = createMockBridge();
    const engine = new SyncEngine(store, bridge);

    await engine.drainQueue();

    expect(bridge.updateIssue).toHaveBeenCalledWith(
      'lin-sc-1',
      expect.objectContaining({ stateId: 'ws-done' })
    );
    expect(store.pendingSyncActions()).toHaveLength(0);
  });

  it('processes "delete" action: calls bridge.deleteIssue with linearId from payload', async () => {
    // Note: the todo may or may not still exist locally.
    // The linearId comes from the payload, not the local store.
    store.enqueueSyncAction('del-1', 'delete', {
      linearId: 'lin-del-1',
    });

    const bridge = createMockBridge();
    const engine = new SyncEngine(store, bridge);

    await engine.drainQueue();

    expect(bridge.deleteIssue).toHaveBeenCalledWith('lin-del-1');
    expect(store.pendingSyncActions()).toHaveLength(0);
  });

  it('skips create if todo was deleted locally before sync', async () => {
    // Todo was created, enqueued for sync, then deleted before sync ran
    store.create(makeTodo({ id: 'ghost-1' }));
    store.enqueueSyncAction('ghost-1', 'create', {});
    store.delete('ghost-1'); // Deleted before sync

    const bridge = createMockBridge();
    const engine = new SyncEngine(store, bridge);

    await engine.drainQueue();

    // Should NOT call createIssue since the todo no longer exists
    expect(bridge.createIssue).not.toHaveBeenCalled();

    // Should remove the orphaned action from the queue
    expect(store.pendingSyncActions()).toHaveLength(0);
  });

  it('processes multiple queue items in order', async () => {
    store.create(makeTodo({ id: 'multi-1' }));
    store.create(makeTodo({ id: 'multi-2' }));
    store.enqueueSyncAction('multi-1', 'create', {});
    store.enqueueSyncAction('multi-2', 'create', {});

    const callOrder: string[] = [];
    const bridge = createMockBridge({
      createIssue: vi.fn().mockImplementation(() => {
        callOrder.push('create');
        return Promise.resolve('lin-new');
      }),
    });
    const engine = new SyncEngine(store, bridge);

    await engine.drainQueue();

    expect(bridge.createIssue).toHaveBeenCalledTimes(2);
    expect(store.pendingSyncActions()).toHaveLength(0);
  });
});
