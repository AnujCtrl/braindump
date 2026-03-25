// tests/cli/capture.test.ts
//
// Behavioral contract for the CLI capture command handler.
// handleCapture is the primary entry point for every captured thought.
// Getting this wrong means todos are silently created with wrong text, missing
// tags, wrong source, or missing sync actions.
//
// These tests protect against:
// - Empty text being silently accepted (should throw)
// - Missing braindump default tag when user provides no tags
// - Source not auto-adding a matching tag (e.g., @minecraft -> #minecraft)
// - Sync action not being enqueued after create
// - Urgent/important flags being lost during creation
// - defaultSource not being used when no @source is given

import { handleCapture } from '../../src/cli/capture.js';
import type { Store } from '../../src/core/store.js';
import type { Todo } from '../../src/core/models.js';

/** Creates a mock Store with vi.fn() for all methods. */
function createMockStore(overrides: Partial<Record<keyof Store, unknown>> = {}): Store {
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
// handleCapture -- basic text capture
// ---------------------------------------------------------------------------

describe('handleCapture', () => {
  it('creates a todo in the store with the parsed text', () => {
    // Protects: basic capture flow -- text must arrive in the store.
    const store = createMockStore();

    handleCapture(store, { text: 'buy groceries' });

    expect(store.create).toHaveBeenCalledTimes(1);
    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.text).toBe('buy groceries');
    expect(created.status).toBe('inbox');
  });

  it('throws when text is empty', () => {
    // Protects: empty captures silently creating blank todos.
    // An empty capture is always a user mistake or a CLI parsing bug.
    const store = createMockStore();

    expect(() => handleCapture(store, { text: '' })).toThrow();
    expect(store.create).not.toHaveBeenCalled();
  });

  it('throws when text is only whitespace', () => {
    // Protects: whitespace-only input sneaking through as valid text.
    const store = createMockStore();

    expect(() => handleCapture(store, { text: '   ' })).toThrow();
    expect(store.create).not.toHaveBeenCalled();
  });

  // ---------------------------------------------------------------------------
  // Tags
  // ---------------------------------------------------------------------------

  it('defaults to ["braindump"] when no tags are provided', () => {
    // Protects: the "no tags = braindump" rule from the capture spec.
    // Every captured thought must have at least one tag for filtering.
    const store = createMockStore();

    handleCapture(store, { text: 'random thought' });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.tags).toEqual(['braindump']);
  });

  it('uses provided tags instead of braindump default', () => {
    // Protects: user-specified tags being overwritten with braindump.
    const store = createMockStore();

    handleCapture(store, { text: 'fix DNS', tags: ['homelab', 'networking'] });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.tags).toEqual(['homelab', 'networking']);
    expect(created.tags).not.toContain('braindump');
  });

  it('defaults to braindump when tags array is empty', () => {
    // Protects: empty array being treated differently from undefined/missing.
    const store = createMockStore();

    handleCapture(store, { text: 'stray thought', tags: [] });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.tags).toEqual(['braindump']);
  });

  // ---------------------------------------------------------------------------
  // Source
  // ---------------------------------------------------------------------------

  it('sets source to defaultSource when no @source is given', () => {
    // Protects: source defaulting logic -- CLI should default to 'cli',
    // API should default to 'api', etc.
    const store = createMockStore();

    handleCapture(store, { text: 'buy milk', defaultSource: 'cli' });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.source).toBe('cli');
  });

  it('uses explicit source when provided', () => {
    // Protects: explicit @source being ignored in favor of defaultSource.
    const store = createMockStore();

    handleCapture(store, { text: 'play bedwars', source: 'minecraft', defaultSource: 'cli' });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.source).toBe('minecraft');
  });

  it('defaults source to "cli" when neither source nor defaultSource given', () => {
    // Protects: null/undefined source causing runtime errors or empty source strings.
    const store = createMockStore();

    handleCapture(store, { text: 'random thought' });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.source).toBe('cli');
  });

  it('source auto-adds matching tag when source name is a known tag', () => {
    // Protects: the spec rule "@minecraft -> also adds #minecraft".
    // This is a convenience feature so users don't have to double-specify.
    const store = createMockStore();

    handleCapture(store, {
      text: 'play bedwars',
      source: 'minecraft',
      tags: ['gaming'],
      knownTags: ['minecraft', 'gaming', 'homelab'],
    });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.tags).toContain('minecraft');
    expect(created.tags).toContain('gaming');
  });

  it('source does NOT auto-add tag when source is not a known tag', () => {
    // Protects: unknown sources being silently added as tags.
    const store = createMockStore();

    handleCapture(store, {
      text: 'call doctor',
      source: 'phone',
      tags: ['health'],
      knownTags: ['health', 'homelab'],
    });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.tags).not.toContain('phone');
    expect(created.tags).toContain('health');
  });

  it('source auto-add does not duplicate tag if already present', () => {
    // Protects: @minecraft #minecraft resulting in ['minecraft', 'minecraft'].
    const store = createMockStore();

    handleCapture(store, {
      text: 'play bedwars',
      source: 'minecraft',
      tags: ['minecraft'],
      knownTags: ['minecraft'],
    });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    const minecraftCount = created.tags.filter((t: string) => t === 'minecraft').length;
    expect(minecraftCount).toBe(1);
  });

  // ---------------------------------------------------------------------------
  // Urgent / Important
  // ---------------------------------------------------------------------------

  it('propagates urgent=true to the created todo', () => {
    // Protects: urgent flag being silently dropped during creation.
    const store = createMockStore();

    handleCapture(store, { text: 'fix server now', urgent: true });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.urgent).toBe(true);
  });

  it('propagates important=true to the created todo', () => {
    // Protects: important flag being silently dropped during creation.
    const store = createMockStore();

    handleCapture(store, { text: 'backup data', important: true });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.important).toBe(true);
  });

  it('defaults urgent and important to false when not specified', () => {
    // Protects: undefined flags being coerced to truthy values.
    const store = createMockStore();

    handleCapture(store, { text: 'casual thought' });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.urgent).toBe(false);
    expect(created.important).toBe(false);
  });

  it('supports both urgent and important simultaneously', () => {
    // Protects: mutual exclusion bugs between the two flags.
    const store = createMockStore();

    handleCapture(store, { text: 'critical fix', urgent: true, important: true });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.urgent).toBe(true);
    expect(created.important).toBe(true);
  });

  // ---------------------------------------------------------------------------
  // Notes
  // ---------------------------------------------------------------------------

  it('attaches notes to the created todo', () => {
    // Protects: notes being silently dropped during capture.
    const store = createMockStore();

    handleCapture(store, { text: 'fix server', notes: ['check logs first'] });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.notes).toEqual(['check logs first']);
  });

  it('defaults notes to empty array when not provided', () => {
    const store = createMockStore();

    handleCapture(store, { text: 'quick thought' });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.notes).toEqual([]);
  });

  // ---------------------------------------------------------------------------
  // Sync
  // ---------------------------------------------------------------------------

  it('enqueues a "create" sync action after creating the todo', () => {
    // Protects: todos being created locally but never synced to Linear.
    // The sync queue is how the system knows what to push upstream.
    const store = createMockStore();

    handleCapture(store, { text: 'sync me' });

    expect(store.enqueueSyncAction).toHaveBeenCalledTimes(1);
    const callArgs = (store.enqueueSyncAction as ReturnType<typeof vi.fn>).mock.calls[0];
    // First arg: todoId (should be a string)
    expect(typeof callArgs[0]).toBe('string');
    // Second arg: action type
    expect(callArgs[1]).toBe('create');
  });

  it('returns the created todo', () => {
    // Protects: callers (CLI output, API response) not getting back the todo.
    const store = createMockStore();

    const result = handleCapture(store, { text: 'return me' });

    expect(result).toBeDefined();
    expect(result.text).toBe('return me');
    expect(result.id).toBeDefined();
    expect(typeof result.id).toBe('string');
    expect(result.id.length).toBeGreaterThan(0);
  });

  // ---------------------------------------------------------------------------
  // Created todo field completeness
  // ---------------------------------------------------------------------------

  it('creates todo with all required fields populated', () => {
    // Protects: missing fields causing runtime errors downstream
    // (e.g., undefined createdAt breaking date display).
    const store = createMockStore();

    handleCapture(store, {
      text: 'full todo',
      tags: ['homelab'],
      source: 'api',
      urgent: true,
      important: false,
      notes: ['note 1'],
      defaultSource: 'cli',
    });

    const created = (store.create as ReturnType<typeof vi.fn>).mock.calls[0][0] as Todo;
    expect(created.id).toBeDefined();
    expect(typeof created.id).toBe('string');
    expect(created.linearId).toBeNull();
    expect(created.text).toBe('full todo');
    expect(created.source).toBe('api');
    expect(created.status).toBe('inbox');
    expect(created.createdAt).toBeDefined();
    expect(created.statusChangedAt).toBeDefined();
    expect(created.urgent).toBe(true);
    expect(created.important).toBe(false);
    expect(created.staleCount).toBe(0);
    expect(created.tags).toEqual(['homelab']);
    expect(created.notes).toEqual(['note 1']);
    expect(created.subtasks).toEqual([]);
    expect(created.done).toBe(false);
    expect(created.updatedAt).toBeDefined();
  });
});
