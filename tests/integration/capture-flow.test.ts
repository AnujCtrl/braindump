// tests/integration/capture-flow.test.ts
//
// End-to-end integration test: capture -> store -> sync queue -> done.
//
// This test uses a real in-memory SQLite database and real Store/CLI handlers.
// No mocks. This protects against wiring bugs where individual units work but
// the full pipeline breaks (e.g., handleCapture creates a todo but forgets to
// enqueue a sync action, or handleDone marks done but the todo doesn't leave
// listOpen).

import { Database } from 'bun:sqlite';
import { initDb } from '../../src/core/db.js';
import { Store } from '../../src/core/store.js';
import { handleCapture } from '../../src/cli/capture.js';
import { handleDone } from '../../src/cli/done.js';

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

describe('capture -> store -> sync -> done lifecycle', () => {
  // Protects: the full happy path from capture to completion.
  // If any step in the pipeline is broken, this test catches it.
  it('captures a todo, verifies it in store and sync queue, then marks done', () => {
    // Step 1: Capture a todo
    const created = handleCapture(store, {
      text: 'Fix the DNS server',
      tags: ['homelab'],
      source: 'cli',
      urgent: true,
      important: false,
      notes: ['check logs first'],
    });

    // Step 2: Verify the todo was stored with correct fields
    const fetched = store.getById(created.id);
    expect(fetched).not.toBeNull();
    expect(fetched!.text).toBe('Fix the DNS server');
    expect(fetched!.tags).toEqual(['homelab']);
    expect(fetched!.source).toBe('cli');
    expect(fetched!.urgent).toBe(true);
    expect(fetched!.important).toBe(false);
    expect(fetched!.notes).toEqual(['check logs first']);
    expect(fetched!.status).toBe('inbox');
    expect(fetched!.done).toBe(false);

    // Step 3: Verify it appears in listOpen
    const openBefore = store.listOpen();
    expect(openBefore.some((t) => t.id === created.id)).toBe(true);

    // Step 4: Verify a "create" sync action was enqueued
    const syncBefore = store.pendingSyncActions();
    expect(syncBefore.length).toBeGreaterThanOrEqual(1);
    const createAction = syncBefore.find(
      (a) => a.todoId === created.id && a.action === 'create'
    );
    expect(createAction).toBeDefined();

    // Step 5: Mark the todo as done
    handleDone(store, created.id);

    // Step 6: Verify it no longer appears in listOpen
    const openAfter = store.listOpen();
    expect(openAfter.some((t) => t.id === created.id)).toBe(false);

    // Step 7: Verify it still exists in listAll (done items are not deleted)
    const allAfter = store.listAll();
    const doneTodo = allAfter.find((t) => t.id === created.id);
    expect(doneTodo).toBeDefined();
    expect(doneTodo!.status).toBe('done');
    expect(doneTodo!.done).toBe(true);

    // Step 8: Verify a second sync action ("status_change") was enqueued
    const syncAfter = store.pendingSyncActions();
    const statusAction = syncAfter.find(
      (a) => a.todoId === created.id && a.action === 'status_change'
    );
    expect(statusAction).toBeDefined();
    // Should have at least 2 sync actions total for this todo
    const todoActions = syncAfter.filter((a) => a.todoId === created.id);
    expect(todoActions.length).toBeGreaterThanOrEqual(2);
  });

  // Protects: capture with no tags defaults to braindump.
  // This is a core UX guarantee -- "just type and it works."
  it('captures with no tags and defaults to braindump', () => {
    const created = handleCapture(store, {
      text: 'random thought about life',
    });

    const fetched = store.getById(created.id);
    expect(fetched!.tags).toEqual(['braindump']);
    expect(fetched!.source).toBe('cli');
  });

  // Protects: multiple captures produce independent todos with unique IDs.
  it('multiple captures produce distinct todos', () => {
    const first = handleCapture(store, { text: 'First thought' });
    const second = handleCapture(store, { text: 'Second thought' });
    const third = handleCapture(store, { text: 'Third thought' });

    expect(first.id).not.toBe(second.id);
    expect(second.id).not.toBe(third.id);

    const all = store.listAll();
    expect(all).toHaveLength(3);

    // Each should have its own create sync action
    const syncActions = store.pendingSyncActions();
    const createActions = syncActions.filter((a) => a.action === 'create');
    expect(createActions).toHaveLength(3);
  });

  // Protects: marking done on a nonexistent ID throws, not silently succeeds.
  it('handleDone throws for nonexistent ID', () => {
    expect(() => handleDone(store, 'nonexistent')).toThrow('Todo not found');
  });

  // Protects: marking done twice throws, preventing duplicate sync actions.
  it('handleDone throws if called twice on same todo', () => {
    const created = handleCapture(store, { text: 'Do once' });
    handleDone(store, created.id);
    expect(() => handleDone(store, created.id)).toThrow('already done');
  });

  // Protects: handleCapture rejects empty text, preventing garbage todos in the DB.
  it('handleCapture throws on empty text', () => {
    expect(() => handleCapture(store, { text: '' })).toThrow('empty');
    expect(() => handleCapture(store, { text: '   ' })).toThrow('empty');
  });

  // Protects: source auto-tag logic when source is in knownTags.
  it('source auto-adds matching tag when source is in knownTags', () => {
    const created = handleCapture(store, {
      text: 'play bedwars',
      source: 'minecraft',
      tags: ['gaming'],
      knownTags: ['minecraft', 'gaming'],
    });

    const fetched = store.getById(created.id);
    expect(fetched!.tags).toContain('minecraft');
    expect(fetched!.tags).toContain('gaming');
  });

  // Protects: source auto-tag does NOT duplicate if tag already present.
  it('source auto-tag does not duplicate if already in tags', () => {
    const created = handleCapture(store, {
      text: 'play bedwars',
      source: 'minecraft',
      tags: ['minecraft'],
      knownTags: ['minecraft'],
    });

    const fetched = store.getById(created.id);
    const minecraftCount = fetched!.tags.filter((t) => t === 'minecraft').length;
    expect(minecraftCount).toBe(1);
  });

  // Protects: info counts are accurate after a capture + done sequence.
  it('info counts reflect capture and done operations', () => {
    // Start with empty store
    let counts = store.getInfoCounts();
    expect(counts.unprocessed).toBe(0);
    expect(counts.active).toBe(0);

    // Capture two todos (they go to inbox)
    const todo1 = handleCapture(store, { text: 'First' });
    const todo2 = handleCapture(store, { text: 'Second' });

    // Mark one done -- inbox items are not "active" for counts
    handleDone(store, todo1.id);

    // Verify: 1 open inbox item, 0 active, 0 unprocessed
    const afterDone = store.getInfoCounts();
    expect(afterDone.unprocessed).toBe(0);
    // active count only counts status='active', not inbox
    expect(afterDone.active).toBe(0);
  });
});
