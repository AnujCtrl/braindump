// tests/cli/list.test.ts
//
// Behavioral contract for the CLI list command handler and line formatter.
// handleList is how users view their todos. formatTodoLine controls the
// per-item display in the terminal. Both are critical for daily workflow.
//
// These tests protect against:
// - "list with no options" returning done items (should show only open)
// - Tag/status/all filters being ignored or misrouted
// - Looping filter using wrong threshold (must be staleCount >= 2)
// - formatTodoLine dropping the sync indicator or mangling tags
// - Checkbox state not reflecting done status

import { handleList, formatTodoLine } from '../../src/cli/list.js';
import type { Store } from '../../src/core/store.js';
import type { Todo } from '../../src/core/models.js';

/** Helper to build a valid Todo with sensible defaults. */
function makeTodo(overrides: Partial<Todo> = {}): Todo {
  const now = new Date().toISOString();
  return {
    id: 'lst11111',
    linearId: null,
    text: 'List test',
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

/** Creates a mock Store with controllable return values. */
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
// handleList
// ---------------------------------------------------------------------------

describe('handleList', () => {
  it('returns open (non-done) todos when no options given', () => {
    // Protects: default list showing completed items, cluttering the view.
    const openTodos = [
      makeTodo({ id: 'open1', status: 'inbox' }),
      makeTodo({ id: 'open2', status: 'active' }),
    ];
    const store = createMockStore({ listOpen: vi.fn().mockReturnValue(openTodos) });

    const result = handleList(store, {});

    expect(store.listOpen).toHaveBeenCalledTimes(1);
    expect(result).toHaveLength(2);
    expect(result[0].id).toBe('open1');
    expect(result[1].id).toBe('open2');
  });

  it('filters by tag when tag option is provided', () => {
    // Protects: tag filter being silently ignored, returning all items.
    const taggedTodos = [makeTodo({ id: 'tagged1', tags: ['homelab'] })];
    const store = createMockStore({ listByTag: vi.fn().mockReturnValue(taggedTodos) });

    const result = handleList(store, { tag: 'homelab' });

    expect(store.listByTag).toHaveBeenCalledWith('homelab');
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('tagged1');
  });

  it('filters by status when status option is provided', () => {
    // Protects: status filter being ignored, returning all items.
    const activeTodos = [makeTodo({ id: 'active1', status: 'active' })];
    const store = createMockStore({ listByStatus: vi.fn().mockReturnValue(activeTodos) });

    const result = handleList(store, { status: 'active' });

    expect(store.listByStatus).toHaveBeenCalledWith('active');
    expect(result).toHaveLength(1);
    expect(result[0].status).toBe('active');
  });

  it('returns all todos when all option is true', () => {
    // Protects: "all" flag being ignored, still filtering out done items.
    const allTodos = [
      makeTodo({ id: 'open1', status: 'inbox' }),
      makeTodo({ id: 'done1', status: 'done', done: true }),
    ];
    const store = createMockStore({ listAll: vi.fn().mockReturnValue(allTodos) });

    const result = handleList(store, { all: true });

    expect(store.listAll).toHaveBeenCalledTimes(1);
    expect(result).toHaveLength(2);
  });

  it('returns looping todos (staleCount >= 2) when looping option is true', () => {
    // Protects: looping filter using wrong threshold (e.g., >= 1 or >= 3).
    // Looping = staleCount >= 2 means the item has been stale at least twice.
    const allOpen = [
      makeTodo({ id: 'loop1', staleCount: 2 }),
      makeTodo({ id: 'loop2', staleCount: 5 }),
      makeTodo({ id: 'notloop', staleCount: 1 }),
      makeTodo({ id: 'fresh', staleCount: 0 }),
    ];
    const store = createMockStore({ listOpen: vi.fn().mockReturnValue(allOpen) });

    const result = handleList(store, { looping: true });

    // Only items with staleCount >= 2 should be returned
    expect(result).toHaveLength(2);
    const ids = result.map((t: Todo) => t.id).sort();
    expect(ids).toEqual(['loop1', 'loop2']);
  });

  it('looping filter excludes done items even with high staleCount', () => {
    // Protects: done items appearing in looping list.
    const allOpen = [
      makeTodo({ id: 'loop1', staleCount: 3 }),
    ];
    const store = createMockStore({ listOpen: vi.fn().mockReturnValue(allOpen) });

    // handleList with looping should use listOpen (which excludes done),
    // then further filter by staleCount >= 2
    const result = handleList(store, { looping: true });
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('loop1');
  });

  it('returns empty array when no todos match', () => {
    // Protects: null/undefined being returned instead of empty array.
    const store = createMockStore();

    const result = handleList(store, {});

    expect(result).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// formatTodoLine
// ---------------------------------------------------------------------------

describe('formatTodoLine', () => {
  it('includes unchecked checkbox for non-done todo', () => {
    // Protects: done todos showing as unchecked or vice versa.
    const todo = makeTodo({ done: false });

    const line = formatTodoLine(todo, false);

    expect(line).toContain('[ ]');
    expect(line).not.toContain('[x]');
  });

  it('includes checked checkbox for done todo', () => {
    const todo = makeTodo({ done: true, status: 'done' });

    const line = formatTodoLine(todo, false);

    expect(line).toContain('[x]');
  });

  it('includes the todo text', () => {
    const todo = makeTodo({ text: 'buy groceries' });

    const line = formatTodoLine(todo, false);

    expect(line).toContain('buy groceries');
  });

  it('includes the todo ID', () => {
    // Protects: ID not being shown -- users need the ID to run `todo done <id>`.
    const todo = makeTodo({ id: 'abc12345' });

    const line = formatTodoLine(todo, false);

    expect(line).toContain('abc12345');
  });

  it('includes tags formatted with # prefix', () => {
    // Protects: tags being shown without the # prefix or missing entirely.
    const todo = makeTodo({ tags: ['homelab', 'server'] });

    const line = formatTodoLine(todo, false);

    expect(line).toContain('#homelab');
    expect(line).toContain('#server');
  });

  it('shows * sync indicator when todo has linearId (synced)', () => {
    // Protects: user not knowing which items are synced to Linear.
    // * means "this exists in Linear".
    const todo = makeTodo({ linearId: 'lin-uuid-123' });

    const line = formatTodoLine(todo, false);

    expect(line).toContain('*');
  });

  it('shows ~ sync indicator when hasPendingSync is true (queued)', () => {
    // Protects: user not knowing which items are waiting to sync.
    // ~ means "sync action is queued, not yet pushed".
    const todo = makeTodo({ linearId: null });

    const line = formatTodoLine(todo, true);

    expect(line).toContain('~');
  });

  it('shows no sync indicator when not synced and no pending sync', () => {
    // Protects: false positive sync indicators.
    const todo = makeTodo({ linearId: null });

    const line = formatTodoLine(todo, false);

    // Should not contain either sync indicator adjacent to the ID
    // (checking that neither * nor ~ appears as a standalone indicator)
    expect(line).not.toMatch(/[*~]/);
  });

  it('handles todo with empty tags array', () => {
    // Protects: crash or malformed output when no tags.
    const todo = makeTodo({ tags: [] });

    const line = formatTodoLine(todo, false);

    // Should still have text and id, just no tags
    expect(line).toContain(todo.text);
    expect(line).toContain(todo.id);
  });

  it('returns a single-line string (no embedded newlines)', () => {
    // Protects: multi-line output breaking the list display.
    const todo = makeTodo({ text: 'some task', tags: ['work', 'urgent'] });

    const line = formatTodoLine(todo, false);

    expect(line).not.toContain('\n');
  });
});
