// tests/server/handlers.test.ts
//
// Behavioral contract for the HTTP API handlers.
// These tests use Fastify's inject() to make real HTTP requests without
// starting a network server. The Store is a real in-memory SQLite instance
// to catch SQL bugs that mocks would hide. Only the sync engine is mocked.
//
// These tests protect against:
// - POST /api/todo returning wrong status code (must be 201, not 200)
// - Missing request body validation (text is required)
// - GET /api/todo ignoring query parameters (status, tag)
// - PUT returning 200 for nonexistent todo (should be 404)
// - DELETE not returning 204 or not enqueuing sync for Linear-linked items
// - POST /api/dump not setting status to 'unprocessed'
// - GET /api/info returning wrong count structure
// - GET /api/health missing or wrong response body

import Fastify, { type FastifyInstance } from 'fastify';
import { Database } from 'bun:sqlite';
import { mock, describe, it, expect, beforeEach, afterEach } from 'bun:test';
import { initDb } from '../../src/core/db.js';
import { Store } from '../../src/core/store.js';
import { Handlers } from '../../src/server/handlers.js';
import { registerRoutes } from '../../src/server/routes.js';
import type { Todo } from '../../src/core/models.js';
import type { SyncEngine } from '../../src/core/sync.js';

let db: Database;
let store: Store;
let app: FastifyInstance;

/** Creates a mock SyncEngine. */
function createMockSyncEngine(): SyncEngine {
  return {
    drainQueue: mock().mockResolvedValue(undefined),
  } as unknown as SyncEngine;
}

/** Helper to build a valid Todo with sensible defaults. */
function makeTodo(overrides: Partial<Todo> = {}): Todo {
  const now = new Date().toISOString();
  return {
    id: 'srv11111',
    linearId: null,
    text: 'Server test',
    source: 'api',
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

beforeEach(async () => {
  db = new Database(':memory:');
  initDb(db);
  store = new Store(db);

  app = Fastify();
  const syncEngine = createMockSyncEngine();
  const handlers = new Handlers(store, syncEngine);
  registerRoutes(app, handlers);
  await app.ready();
});

afterEach(async () => {
  await app.close();
  db.close();
});

// ---------------------------------------------------------------------------
// POST /api/todo
// ---------------------------------------------------------------------------

describe('POST /api/todo', () => {
  it('creates a todo and returns 201 with the id', async () => {
    // Protects: wrong status code (200 vs 201) breaking client expectations.
    const response = await app.inject({
      method: 'POST',
      url: '/api/todo',
      payload: { text: 'buy groceries' },
    });

    expect(response.statusCode).toBe(201);
    const body = JSON.parse(response.body);
    expect(body.id).toBeDefined();
    expect(typeof body.id).toBe('string');
    expect(body.id.length).toBeGreaterThan(0);

    // Verify the todo actually exists in the store
    const todo = store.getById(body.id);
    expect(todo).not.toBeNull();
    expect(todo!.text).toBe('buy groceries');
  });

  it('creates todo with provided source, tags, urgent, important', async () => {
    // Protects: optional fields being silently dropped.
    const response = await app.inject({
      method: 'POST',
      url: '/api/todo',
      payload: {
        text: 'fix server',
        source: 'api',
        tags: ['homelab', 'server'],
        urgent: true,
        important: true,
      },
    });

    expect(response.statusCode).toBe(201);
    const body = JSON.parse(response.body);
    const todo = store.getById(body.id);
    expect(todo!.source).toBe('api');
    expect(todo!.tags).toEqual(['homelab', 'server']);
    expect(todo!.urgent).toBe(true);
    expect(todo!.important).toBe(true);
  });

  it('creates todo with notes', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/todo',
      payload: {
        text: 'check logs',
        notes: ['look at stderr output'],
      },
    });

    expect(response.statusCode).toBe(201);
    const body = JSON.parse(response.body);
    const todo = store.getById(body.id);
    expect(todo!.notes).toEqual(['look at stderr output']);
  });

  it('returns 400 when text is missing', async () => {
    // Protects: empty todos being created via API.
    const response = await app.inject({
      method: 'POST',
      url: '/api/todo',
      payload: {},
    });

    expect(response.statusCode).toBe(400);
  });

  it('returns 400 when text is empty string', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/todo',
      payload: { text: '' },
    });

    expect(response.statusCode).toBe(400);
  });

  it('sets default source to "api" when not provided', async () => {
    // Protects: API-created todos showing "cli" as source.
    const response = await app.inject({
      method: 'POST',
      url: '/api/todo',
      payload: { text: 'api todo' },
    });

    const body = JSON.parse(response.body);
    const todo = store.getById(body.id);
    expect(todo!.source).toBe('api');
  });

  it('sets status to "inbox" for newly created todos', async () => {
    // Protects: new todos not starting in inbox status.
    const response = await app.inject({
      method: 'POST',
      url: '/api/todo',
      payload: { text: 'inbox test' },
    });

    const body = JSON.parse(response.body);
    const todo = store.getById(body.id);
    expect(todo!.status).toBe('inbox');
  });
});

// ---------------------------------------------------------------------------
// GET /api/todo
// ---------------------------------------------------------------------------

describe('GET /api/todo', () => {
  beforeEach(() => {
    store.create(makeTodo({ id: 'get-inbox1', status: 'inbox', text: 'inbox item' }));
    store.create(makeTodo({ id: 'get-active1', status: 'active', text: 'active item' }));
    store.create(makeTodo({ id: 'get-done1', status: 'done', done: true, text: 'done item' }));
    store.create(makeTodo({ id: 'get-tagged1', status: 'inbox', tags: ['homelab'], text: 'homelab item' }));
  });

  it('returns open todos by default', async () => {
    // Protects: done items appearing in default list.
    const response = await app.inject({
      method: 'GET',
      url: '/api/todo',
    });

    expect(response.statusCode).toBe(200);
    const todos = JSON.parse(response.body);
    expect(Array.isArray(todos)).toBe(true);
    // Should include inbox, active, but NOT done
    const ids = todos.map((t: Todo) => t.id);
    expect(ids).toContain('get-inbox1');
    expect(ids).toContain('get-active1');
    expect(ids).not.toContain('get-done1');
  });

  it('filters by status query parameter', async () => {
    // Protects: status filter being ignored.
    const response = await app.inject({
      method: 'GET',
      url: '/api/todo?status=active',
    });

    expect(response.statusCode).toBe(200);
    const todos = JSON.parse(response.body);
    expect(todos.every((t: Todo) => t.status === 'active')).toBe(true);
    expect(todos).toHaveLength(1);
  });

  it('filters by tag query parameter', async () => {
    // Protects: tag filter being ignored, returning all items.
    const response = await app.inject({
      method: 'GET',
      url: '/api/todo?tag=homelab',
    });

    expect(response.statusCode).toBe(200);
    const todos = JSON.parse(response.body);
    expect(todos).toHaveLength(1);
    expect(todos[0].tags).toContain('homelab');
  });

  it('returns empty array when no todos exist', async () => {
    // Create a fresh store with no items
    const freshDb = new Database(':memory:');
    initDb(freshDb);
    const freshStore = new Store(freshDb);
    const freshApp = Fastify();
    const handlers = new Handlers(freshStore, createMockSyncEngine());
    registerRoutes(freshApp, handlers);
    await freshApp.ready();

    const response = await freshApp.inject({
      method: 'GET',
      url: '/api/todo',
    });

    expect(response.statusCode).toBe(200);
    expect(JSON.parse(response.body)).toEqual([]);

    await freshApp.close();
    freshDb.close();
  });
});

// ---------------------------------------------------------------------------
// PUT /api/todo/:id
// ---------------------------------------------------------------------------

describe('PUT /api/todo/:id', () => {
  it('updates todo fields and returns 200', async () => {
    // Protects: update not persisting changes.
    store.create(makeTodo({ id: 'upd-api1', text: 'original text' }));

    const response = await app.inject({
      method: 'PUT',
      url: '/api/todo/upd-api1',
      payload: { text: 'updated text', urgent: true },
    });

    expect(response.statusCode).toBe(200);
    const todo = store.getById('upd-api1');
    expect(todo!.text).toBe('updated text');
    expect(todo!.urgent).toBe(true);
  });

  it('returns 404 for nonexistent todo', async () => {
    // Protects: update silently succeeding on missing ID (no error feedback).
    const response = await app.inject({
      method: 'PUT',
      url: '/api/todo/nonexistent',
      payload: { text: 'nope' },
    });

    expect(response.statusCode).toBe(404);
  });

  it('preserves fields not included in the update payload', async () => {
    // Protects: partial update wiping unrelated fields.
    store.create(makeTodo({
      id: 'partial-upd',
      text: 'original',
      tags: ['homelab'],
      source: 'cli',
    }));

    await app.inject({
      method: 'PUT',
      url: '/api/todo/partial-upd',
      payload: { text: 'changed' },
    });

    const todo = store.getById('partial-upd');
    expect(todo!.text).toBe('changed');
    expect(todo!.tags).toEqual(['homelab']);
    expect(todo!.source).toBe('cli');
  });
});

// ---------------------------------------------------------------------------
// DELETE /api/todo/:id
// ---------------------------------------------------------------------------

describe('DELETE /api/todo/:id', () => {
  it('deletes todo and returns 204', async () => {
    // Protects: wrong status code or todo not actually removed.
    store.create(makeTodo({ id: 'del-api1' }));

    const response = await app.inject({
      method: 'DELETE',
      url: '/api/todo/del-api1',
    });

    expect(response.statusCode).toBe(204);
    expect(store.getById('del-api1')).toBeNull();
  });

  it('enqueues delete sync action when todo has linearId', async () => {
    // Protects: Linear issue orphaned when local todo deleted.
    // The sync queue must get a delete action so Linear's copy gets trashed.
    store.create(makeTodo({ id: 'del-sync1', linearId: 'lin-del-1' }));

    await app.inject({
      method: 'DELETE',
      url: '/api/todo/del-sync1',
    });

    const actions = store.pendingSyncActions();
    const deleteAction = actions.find(
      (a) => a.todoId === 'del-sync1' && a.action === 'delete'
    );
    expect(deleteAction).toBeDefined();
  });

  it('does NOT enqueue delete sync when todo has no linearId', async () => {
    // Protects: unnecessary sync attempts for items never pushed to Linear.
    store.create(makeTodo({ id: 'del-nosync', linearId: null }));

    await app.inject({
      method: 'DELETE',
      url: '/api/todo/del-nosync',
    });

    const actions = store.pendingSyncActions();
    const deleteAction = actions.find(
      (a) => a.todoId === 'del-nosync' && a.action === 'delete'
    );
    expect(deleteAction).toBeUndefined();
  });

  it('returns 204 even for nonexistent todo (idempotent)', async () => {
    // Protects: DELETE of missing item returning 404 and breaking retry logic.
    // HTTP DELETE should be idempotent.
    const response = await app.inject({
      method: 'DELETE',
      url: '/api/todo/nonexistent',
    });

    expect(response.statusCode).toBe(204);
  });
});

// ---------------------------------------------------------------------------
// PATCH /api/todo/:id/status
// ---------------------------------------------------------------------------

describe('PATCH /api/todo/:id/status', () => {
  it('changes the status of a todo', async () => {
    // Protects: status change endpoint not actually updating the status.
    store.create(makeTodo({ id: 'patch-1', status: 'inbox' }));

    const response = await app.inject({
      method: 'PATCH',
      url: '/api/todo/patch-1/status',
      payload: { status: 'active' },
    });

    expect(response.statusCode).toBe(200);
    const todo = store.getById('patch-1');
    expect(todo!.status).toBe('active');
  });

  it('returns 404 for nonexistent todo', async () => {
    const response = await app.inject({
      method: 'PATCH',
      url: '/api/todo/nonexistent/status',
      payload: { status: 'active' },
    });

    expect(response.statusCode).toBe(404);
  });

  it('updates statusChangedAt timestamp', async () => {
    // Protects: stale detection using old timestamp after status change.
    const oldTime = '2026-01-01T00:00:00.000Z';
    store.create(makeTodo({ id: 'patch-ts', status: 'inbox', statusChangedAt: oldTime }));

    await app.inject({
      method: 'PATCH',
      url: '/api/todo/patch-ts/status',
      payload: { status: 'active' },
    });

    const todo = store.getById('patch-ts');
    expect(new Date(todo!.statusChangedAt).getTime()).toBeGreaterThan(
      new Date(oldTime).getTime()
    );
  });
});

// ---------------------------------------------------------------------------
// POST /api/dump
// ---------------------------------------------------------------------------

describe('POST /api/dump', () => {
  it('bulk creates todos with status "unprocessed"', async () => {
    // Protects: brain dump items skipping the unprocessed queue.
    // Dump items must start as "unprocessed" so they go through triage.
    const response = await app.inject({
      method: 'POST',
      url: '/api/dump',
      payload: {
        items: ['first thought', 'second thought', 'third thought'],
      },
    });

    expect(response.statusCode).toBe(201);

    // Verify all items exist with status 'unprocessed'
    const unprocessed = store.listByStatus('unprocessed');
    expect(unprocessed).toHaveLength(3);
    const texts = unprocessed.map((t) => t.text).sort();
    expect(texts).toEqual(['first thought', 'second thought', 'third thought']);
    expect(unprocessed.every((t) => t.status === 'unprocessed')).toBe(true);
  });

  it('returns created ids', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/dump',
      payload: { items: ['dump item 1'] },
    });

    const body = JSON.parse(response.body);
    expect(body.ids).toBeDefined();
    expect(Array.isArray(body.ids)).toBe(true);
    expect(body.ids).toHaveLength(1);
  });

  it('returns 400 when items array is missing', async () => {
    const response = await app.inject({
      method: 'POST',
      url: '/api/dump',
      payload: {},
    });

    expect(response.statusCode).toBe(400);
  });

  it('returns 400 when items array is empty', async () => {
    // Protects: empty dumps creating no items but returning success.
    const response = await app.inject({
      method: 'POST',
      url: '/api/dump',
      payload: { items: [] },
    });

    expect(response.statusCode).toBe(400);
  });

  it('assigns braindump tag to dump items', async () => {
    // Protects: dump items having no tags, making them unfindable by tag.
    await app.inject({
      method: 'POST',
      url: '/api/dump',
      payload: { items: ['dump thought'] },
    });

    const unprocessed = store.listByStatus('unprocessed');
    expect(unprocessed[0].tags).toContain('braindump');
  });
});

// ---------------------------------------------------------------------------
// GET /api/info
// ---------------------------------------------------------------------------

describe('GET /api/info', () => {
  it('returns unprocessed, active, and looping counts', async () => {
    // Protects: info endpoint returning wrong structure or wrong counts.
    store.create(makeTodo({ id: 'inf-u1', status: 'unprocessed' }));
    store.create(makeTodo({ id: 'inf-u2', status: 'unprocessed' }));
    store.create(makeTodo({ id: 'inf-a1', status: 'active' }));
    store.create(makeTodo({ id: 'inf-loop', status: 'inbox', staleCount: 3 }));
    store.create(makeTodo({ id: 'inf-done', status: 'done', done: true }));

    const response = await app.inject({
      method: 'GET',
      url: '/api/info',
    });

    expect(response.statusCode).toBe(200);
    const body = JSON.parse(response.body);
    expect(body.unprocessed).toBe(2);
    expect(body.active).toBe(1);
    expect(body.looping).toBe(1);
  });

  it('returns all zeros for empty store', async () => {
    // Use a separate empty store for this test
    const freshDb = new Database(':memory:');
    initDb(freshDb);
    const freshStore = new Store(freshDb);
    const freshApp = Fastify();
    const handlers = new Handlers(freshStore, createMockSyncEngine());
    registerRoutes(freshApp, handlers);
    await freshApp.ready();

    const response = await freshApp.inject({
      method: 'GET',
      url: '/api/info',
    });

    expect(response.statusCode).toBe(200);
    const body = JSON.parse(response.body);
    expect(body.unprocessed).toBe(0);
    expect(body.active).toBe(0);
    expect(body.looping).toBe(0);

    await freshApp.close();
    freshDb.close();
  });
});

// ---------------------------------------------------------------------------
// GET /api/health
// ---------------------------------------------------------------------------

describe('GET /api/health', () => {
  it('returns {status: "ok"} with 200', async () => {
    // Protects: health check not working, breaking monitoring/load-balancer
    // health probes.
    const response = await app.inject({
      method: 'GET',
      url: '/api/health',
    });

    expect(response.statusCode).toBe(200);
    const body = JSON.parse(response.body);
    expect(body.status).toBe('ok');
  });
});
