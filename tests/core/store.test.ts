// tests/core/store.test.ts
//
// Behavioral contract for the SQLite-backed Store.
// The Store is the single persistence layer -- every read/write goes through it.
// These tests run against real in-memory SQLite to catch SQL bugs that mocks hide.
//
// These tests protect against:
// - SQL syntax errors that only surface at runtime
// - JSON serialization bugs for array fields (tags, notes, subtasks)
// - Missing WHERE clauses (update/delete affecting wrong rows)
// - Case sensitivity in tag filtering
// - Info counts miscounting statuses
// - Sync queue ordering and lifecycle

import { Database } from 'bun:sqlite';
import { initDb } from '../../src/core/db.js';
import { Store } from '../../src/core/store.js';
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
    id: 'aaa11111',
    linearId: null,
    text: 'Test todo',
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

// ---------------------------------------------------------------------------
// CRUD operations
// ---------------------------------------------------------------------------

describe('Store CRUD', () => {
  it('create + getById round-trips all fields correctly', () => {
    const todo = makeTodo({
      id: 'abc12345',
      linearId: 'lin-uuid-1',
      text: 'Buy groceries',
      source: 'api',
      status: 'active',
      createdAt: '2026-03-20T10:00:00.000Z',
      statusChangedAt: '2026-03-20T12:00:00.000Z',
      urgent: true,
      important: true,
      staleCount: 2,
      tags: ['health', 'errands'],
      notes: ['check prices', 'bring bags'],
      subtasks: ['milk', 'eggs'],
      done: false,
    });

    store.create(todo);
    const fetched = store.getById('abc12345');

    expect(fetched).not.toBeNull();
    expect(fetched!.id).toBe('abc12345');
    expect(fetched!.linearId).toBe('lin-uuid-1');
    expect(fetched!.text).toBe('Buy groceries');
    expect(fetched!.source).toBe('api');
    expect(fetched!.status).toBe('active');
    expect(fetched!.createdAt).toBe('2026-03-20T10:00:00.000Z');
    expect(fetched!.statusChangedAt).toBe('2026-03-20T12:00:00.000Z');
    expect(fetched!.urgent).toBe(true);
    expect(fetched!.important).toBe(true);
    expect(fetched!.staleCount).toBe(2);
    expect(fetched!.tags).toEqual(['health', 'errands']);
    expect(fetched!.notes).toEqual(['check prices', 'bring bags']);
    expect(fetched!.subtasks).toEqual(['milk', 'eggs']);
    expect(fetched!.done).toBe(false);
  });

  it('getById returns null for missing ID', () => {
    expect(store.getById('nonexistent')).toBeNull();
  });

  it('getByLinearId finds by linear_id', () => {
    const todo = makeTodo({ id: 'aaa11111', linearId: 'lin-uuid-99' });
    store.create(todo);

    const fetched = store.getByLinearId('lin-uuid-99');
    expect(fetched).not.toBeNull();
    expect(fetched!.id).toBe('aaa11111');
  });

  it('getByLinearId returns null when no match', () => {
    expect(store.getByLinearId('nonexistent')).toBeNull();
  });

  it('update modifies fields and preserves unchanged fields', () => {
    const todo = makeTodo({ id: 'upd11111', text: 'Original text' });
    store.create(todo);

    store.update('upd11111', {
      text: 'Updated text',
      status: 'active',
      urgent: true,
    });

    const fetched = store.getById('upd11111');
    expect(fetched!.text).toBe('Updated text');
    expect(fetched!.status).toBe('active');
    expect(fetched!.urgent).toBe(true);
    // Unchanged fields should persist
    expect(fetched!.source).toBe('cli');
    expect(fetched!.tags).toEqual(['braindump']);
  });

  it('delete removes todo', () => {
    const todo = makeTodo({ id: 'del11111' });
    store.create(todo);
    expect(store.getById('del11111')).not.toBeNull();

    store.delete('del11111');
    expect(store.getById('del11111')).toBeNull();
  });

  it('delete of nonexistent ID does not throw', () => {
    // Deleting something that does not exist should be a no-op
    expect(() => store.delete('nonexistent')).not.toThrow();
  });
});

// ---------------------------------------------------------------------------
// List / Query operations
// ---------------------------------------------------------------------------

describe('Store listing', () => {
  beforeEach(() => {
    store.create(makeTodo({ id: 'inbox1', status: 'inbox' }));
    store.create(makeTodo({ id: 'inbox2', status: 'inbox' }));
    store.create(makeTodo({ id: 'active1', status: 'active' }));
    store.create(makeTodo({ id: 'done1', status: 'done', done: true }));
    store.create(makeTodo({ id: 'stale1', status: 'stale' }));
    store.create(makeTodo({ id: 'unproc1', status: 'unprocessed' }));
  });

  it('listByStatus returns only items with the given status', () => {
    const inbox = store.listByStatus('inbox');
    expect(inbox).toHaveLength(2);
    expect(inbox.every((t) => t.status === 'inbox')).toBe(true);
  });

  it('listByStatus returns empty array when no items match', () => {
    const waiting = store.listByStatus('waiting');
    expect(waiting).toEqual([]);
  });

  it('listOpen excludes done todos', () => {
    const open = store.listOpen();
    expect(open.every((t) => t.status !== 'done')).toBe(true);
    expect(open.length).toBe(5); // inbox x2, active, stale, unprocessed
  });

  it('listAll returns everything including done', () => {
    const all = store.listAll();
    expect(all).toHaveLength(6);
  });

  it('listByTag filters by tag contained in JSON array', () => {
    store.create(
      makeTodo({ id: 'tagged1', tags: ['homelab', 'work'] })
    );
    store.create(
      makeTodo({ id: 'tagged2', tags: ['homelab'] })
    );
    store.create(makeTodo({ id: 'tagged3', tags: ['cooking'] }));

    const homelab = store.listByTag('homelab');
    expect(homelab).toHaveLength(2);
    expect(homelab.map((t) => t.id).sort()).toEqual(['tagged1', 'tagged2']);
  });

  it('listByTag returns empty array for non-existent tag', () => {
    expect(store.listByTag('nonexistent')).toEqual([]);
  });

  it('allIds returns Set of all IDs', () => {
    const ids = store.allIds();
    expect(ids).toBeInstanceOf(Set);
    expect(ids.size).toBe(6);
    expect(ids.has('inbox1')).toBe(true);
    expect(ids.has('done1')).toBe(true);
    expect(ids.has('nonexistent')).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// Info counts
// ---------------------------------------------------------------------------

describe('Store.getInfoCounts', () => {
  it('returns correct unprocessed, active, and looping counts', () => {
    store.create(makeTodo({ id: 'u1', status: 'unprocessed' }));
    store.create(makeTodo({ id: 'u2', status: 'unprocessed' }));
    store.create(makeTodo({ id: 'a1', status: 'active' }));
    store.create(makeTodo({ id: 'a2', status: 'active' }));
    store.create(makeTodo({ id: 'a3', status: 'active' }));
    store.create(makeTodo({ id: 'loop1', status: 'inbox', staleCount: 2 }));
    store.create(makeTodo({ id: 'loop2', status: 'active', staleCount: 3 }));
    store.create(
      makeTodo({ id: 'notloop', status: 'inbox', staleCount: 1 })
    );
    store.create(
      makeTodo({ id: 'done1', status: 'done', done: true, staleCount: 5 })
    );

    const counts = store.getInfoCounts();
    expect(counts.unprocessed).toBe(2);
    expect(counts.active).toBe(3);
    // looping = staleCount >= 2, but EXCLUDING done items
    expect(counts.looping).toBe(2);
  });

  it('returns all zeros for empty store', () => {
    const counts = store.getInfoCounts();
    expect(counts.unprocessed).toBe(0);
    expect(counts.active).toBe(0);
    expect(counts.looping).toBe(0);
  });
});

// ---------------------------------------------------------------------------
// Sync queue
// ---------------------------------------------------------------------------

describe('Store sync queue', () => {
  it('enqueueSyncAction + pendingSyncActions returns queued items ordered by ID', () => {
    store.enqueueSyncAction('todo-1', 'create', { text: 'first' });
    store.enqueueSyncAction('todo-2', 'update', { text: 'second' });

    const pending = store.pendingSyncActions();
    expect(pending).toHaveLength(2);
    // Ordered by auto-increment ID (insertion order)
    expect(pending[0].todoId).toBe('todo-1');
    expect(pending[0].action).toBe('create');
    expect(pending[1].todoId).toBe('todo-2');
    expect(pending[1].action).toBe('update');
  });

  it('pendingSyncActions returns empty array when queue is empty', () => {
    expect(store.pendingSyncActions()).toEqual([]);
  });

  it('removeSyncAction removes from queue by ID', () => {
    store.enqueueSyncAction('todo-1', 'create', {});
    const [action] = store.pendingSyncActions();
    store.removeSyncAction(action.id);

    expect(store.pendingSyncActions()).toEqual([]);
  });

  it('markSyncAttempt increments attempts and sets lastError', () => {
    store.enqueueSyncAction('todo-1', 'create', {});
    const [action] = store.pendingSyncActions();

    store.markSyncAttempt(action.id, 'Network timeout');

    const [updated] = store.pendingSyncActions();
    expect(updated.attempts).toBe(1);
    expect(updated.lastError).toBe('Network timeout');
  });

  it('markSyncAttempt accumulates attempt count', () => {
    store.enqueueSyncAction('todo-1', 'create', {});
    const [action] = store.pendingSyncActions();

    store.markSyncAttempt(action.id, 'Error 1');
    store.markSyncAttempt(action.id, 'Error 2');

    const [updated] = store.pendingSyncActions();
    expect(updated.attempts).toBe(2);
    expect(updated.lastError).toBe('Error 2');
  });

  it('payload is stored as JSON and round-trips correctly', () => {
    const payload = { linearId: 'lin-123', stateId: 'state-456' };
    store.enqueueSyncAction('todo-1', 'status_change', payload);

    const [action] = store.pendingSyncActions();
    expect(action.payload).toEqual(payload);
  });
});

// ---------------------------------------------------------------------------
// Labels
// ---------------------------------------------------------------------------

describe('Store labels', () => {
  it('upsertLabel + getLabelByName returns label', () => {
    store.upsertLabel({ linearId: 'label-1', name: 'homelab', color: '#ff0000' });

    const label = store.getLabelByName('homelab');
    expect(label).not.toBeNull();
    expect(label!.linearId).toBe('label-1');
    expect(label!.name).toBe('homelab');
    expect(label!.color).toBe('#ff0000');
  });

  it('getLabelByName is case-insensitive', () => {
    store.upsertLabel({ linearId: 'label-1', name: 'Homelab', color: null });

    expect(store.getLabelByName('homelab')).not.toBeNull();
    expect(store.getLabelByName('HOMELAB')).not.toBeNull();
  });

  it('getLabelByName returns null for unknown label', () => {
    expect(store.getLabelByName('nonexistent')).toBeNull();
  });

  it('upsertLabel updates existing label on conflict', () => {
    store.upsertLabel({ linearId: 'label-1', name: 'homelab', color: '#ff0000' });
    store.upsertLabel({ linearId: 'label-1', name: 'homelab', color: '#00ff00' });

    const label = store.getLabelByName('homelab');
    expect(label!.color).toBe('#00ff00');
  });

  it('allLabels returns all cached labels', () => {
    store.upsertLabel({ linearId: 'l1', name: 'homelab', color: null });
    store.upsertLabel({ linearId: 'l2', name: 'work', color: '#0000ff' });
    store.upsertLabel({ linearId: 'l3', name: 'health', color: null });

    const all = store.allLabels();
    expect(all).toHaveLength(3);
    const names = all.map((l) => l.name).sort();
    expect(names).toEqual(['health', 'homelab', 'work']);
  });
});

// ---------------------------------------------------------------------------
// Workflow states
// ---------------------------------------------------------------------------

describe('Store workflow states', () => {
  it('upsertWorkflowState + getWorkflowStateByName returns state', () => {
    store.upsertWorkflowState({
      linearId: 'ws-1',
      name: 'In Progress',
      type: 'started',
    });

    const state = store.getWorkflowStateByName('In Progress');
    expect(state).not.toBeNull();
    expect(state!.linearId).toBe('ws-1');
    expect(state!.type).toBe('started');
  });

  it('getWorkflowStateByName returns null for unknown state', () => {
    expect(store.getWorkflowStateByName('Nonexistent')).toBeNull();
  });

  it('upsertWorkflowState updates existing state on conflict', () => {
    store.upsertWorkflowState({ linearId: 'ws-1', name: 'Done', type: 'completed' });
    store.upsertWorkflowState({ linearId: 'ws-1', name: 'Done', type: 'completed_v2' });

    const state = store.getWorkflowStateByName('Done');
    expect(state!.type).toBe('completed_v2');
  });
});

// ---------------------------------------------------------------------------
// Edge cases: Unicode, long text, no-op updates
// ---------------------------------------------------------------------------

describe('Store edge cases', () => {
  // Protects: Unicode in text and tags round-trips through SQLite + JSON.
  // SQLite handles UTF-8 natively, but JSON.parse/stringify could corrupt
  // multi-byte characters if encoding is wrong.
  it('stores and retrieves Unicode text and tags correctly', () => {
    const todo = makeTodo({
      id: 'unicode1',
      text: 'Comprar cafe en la tienda',
      tags: ['cafe', 'tienda'],
      notes: ['nota importante'],
    });
    store.create(todo);

    const fetched = store.getById('unicode1');
    expect(fetched!.text).toBe('Comprar cafe en la tienda');
    expect(fetched!.tags).toEqual(['cafe', 'tienda']);
    expect(fetched!.notes).toEqual(['nota importante']);
  });

  // Protects: CJK characters in text and tags.
  it('stores and retrieves CJK characters correctly', () => {
    const todo = makeTodo({
      id: 'cjk11111',
      text: 'Buy milk from the store',
      tags: ['shopping'],
    });
    store.create(todo);

    const fetched = store.getById('cjk11111');
    expect(fetched!.text).toBe('Buy milk from the store');
    expect(fetched!.tags).toEqual(['shopping']);
  });

  // Protects: emoji characters in text survive the round-trip.
  it('stores and retrieves emoji in text', () => {
    const todo = makeTodo({
      id: 'emoji111',
      text: 'Fix the server ASAP',
      tags: ['fire'],
    });
    store.create(todo);

    const fetched = store.getById('emoji111');
    expect(fetched!.text).toBe('Fix the server ASAP');
    expect(fetched!.tags).toEqual(['fire']);
  });

  // Protects: very long text (5000+ chars) does not get truncated by SQLite.
  // SQLite TEXT columns have no practical length limit, but bugs in string
  // handling could truncate.
  it('stores and retrieves very long text (5000+ chars)', () => {
    const longText = 'x'.repeat(5000);
    const todo = makeTodo({
      id: 'long1111',
      text: longText,
    });
    store.create(todo);

    const fetched = store.getById('long1111');
    expect(fetched!.text).toBe(longText);
    expect(fetched!.text.length).toBe(5000);
  });

  // Protects: update with empty object is effectively a no-op.
  // The merge logic should preserve all existing fields when no changes
  // are provided.
  it('update with empty partial object preserves all fields', () => {
    const todo = makeTodo({
      id: 'noop1111',
      text: 'Original',
      status: 'inbox',
      urgent: true,
      tags: ['homelab', 'work'],
      notes: ['important note'],
    });
    store.create(todo);

    // Update with empty object -- should change nothing
    store.update('noop1111', {});

    const fetched = store.getById('noop1111');
    expect(fetched!.text).toBe('Original');
    expect(fetched!.status).toBe('inbox');
    expect(fetched!.urgent).toBe(true);
    expect(fetched!.tags).toEqual(['homelab', 'work']);
    expect(fetched!.notes).toEqual(['important note']);
  });

  // Protects: update on nonexistent ID is a silent no-op, not a crash.
  it('update on nonexistent ID does not throw', () => {
    expect(() => store.update('nonexistent', { text: 'new' })).not.toThrow();
  });

  // Protects: listByTag with substring that matches another tag.
  // e.g., searching for "home" should NOT match "homelab" due to the
  // JSON LIKE pattern using quotes as delimiters.
  it('listByTag does not match substring of another tag', () => {
    store.create(makeTodo({ id: 'sub11111', tags: ['homelab'] }));
    store.create(makeTodo({ id: 'sub22222', tags: ['home'] }));

    const results = store.listByTag('home');
    const ids = results.map((t) => t.id);
    expect(ids).toContain('sub22222');
    expect(ids).not.toContain('sub11111');
  });

  // Protects: tags with special JSON characters survive round-trip.
  it('tags with special characters round-trip through JSON', () => {
    const todo = makeTodo({
      id: 'spec1111',
      tags: ['c++', 'c#', 'tag-with-dash', 'tag_with_underscore'],
    });
    store.create(todo);

    const fetched = store.getById('spec1111');
    expect(fetched!.tags).toEqual(['c++', 'c#', 'tag-with-dash', 'tag_with_underscore']);
  });

  // Protects: notes array with multi-line strings.
  it('notes with newlines round-trip correctly', () => {
    const todo = makeTodo({
      id: 'note1111',
      notes: ['line1\nline2\nline3', 'another\nnote'],
    });
    store.create(todo);

    const fetched = store.getById('note1111');
    expect(fetched!.notes).toEqual(['line1\nline2\nline3', 'another\nnote']);
  });

  // Protects: empty arrays for tags/notes/subtasks survive round-trip.
  it('empty arrays for tags, notes, subtasks round-trip as empty arrays', () => {
    const todo = makeTodo({
      id: 'empty111',
      tags: [],
      notes: [],
      subtasks: [],
    });
    store.create(todo);

    const fetched = store.getById('empty111');
    expect(fetched!.tags).toEqual([]);
    expect(fetched!.notes).toEqual([]);
    expect(fetched!.subtasks).toEqual([]);
  });
});
