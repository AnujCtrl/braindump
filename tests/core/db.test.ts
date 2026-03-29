// tests/core/db.test.ts
//
// Behavioral contract for SQLite database initialization.
// initDb sets up the schema that every other module depends on.
// If a table or column is missing, the store, sync engine, and label cache break.
//
// These tests protect against:
// - Missing tables after schema changes
// - Missing columns in table definitions
// - WAL mode not being enabled (causes write contention in concurrent access)
// - Non-idempotent init (crashing on restart when DB already exists)

import { mkdtempSync, rmSync } from 'fs';
import { join } from 'path';
import { tmpdir } from 'os';
import { Database } from 'bun:sqlite';
import { initDb } from '../../src/core/db.js';

let db: Database;

beforeEach(() => {
  db = new Database(':memory:');
});

afterEach(() => {
  db.close();
});

describe('initDb', () => {
  it('creates the todos table with all required columns', () => {
    initDb(db);

    const columns = db
      .prepare("PRAGMA table_info('todos')")
      .all() as Array<{ name: string; type: string; notnull: number; dflt_value: string | null; pk: number }>;
    const colNames = columns.map((c) => c.name);

    expect(colNames).toContain('id');
    expect(colNames).toContain('linear_id');
    expect(colNames).toContain('text');
    expect(colNames).toContain('source');
    expect(colNames).toContain('status');
    expect(colNames).toContain('created_at');
    expect(colNames).toContain('status_changed_at');
    expect(colNames).toContain('urgent');
    expect(colNames).toContain('important');
    expect(colNames).toContain('stale_count');
    expect(colNames).toContain('tags');
    expect(colNames).toContain('notes');
    expect(colNames).toContain('subtasks');
    expect(colNames).toContain('done');
    expect(colNames).toContain('updated_at');
  });

  it('creates the sync_queue table with all required columns', () => {
    initDb(db);

    const columns = db
      .prepare("PRAGMA table_info('sync_queue')")
      .all() as Array<{ name: string }>;
    const colNames = columns.map((c) => c.name);

    expect(colNames).toContain('id');
    expect(colNames).toContain('todo_id');
    expect(colNames).toContain('action');
    expect(colNames).toContain('payload');
    expect(colNames).toContain('created_at');
    expect(colNames).toContain('attempts');
    expect(colNames).toContain('last_error');
  });

  it('creates the sync_state table', () => {
    initDb(db);

    const columns = db
      .prepare("PRAGMA table_info('sync_state')")
      .all() as Array<{ name: string }>;
    const colNames = columns.map((c) => c.name);

    expect(colNames).toContain('key');
    expect(colNames).toContain('value');
  });

  it('creates the labels table', () => {
    initDb(db);

    const columns = db
      .prepare("PRAGMA table_info('labels')")
      .all() as Array<{ name: string }>;
    const colNames = columns.map((c) => c.name);

    expect(colNames).toContain('linear_id');
    expect(colNames).toContain('name');
    expect(colNames).toContain('color');
  });

  it('creates the workflow_states table', () => {
    initDb(db);

    const columns = db
      .prepare("PRAGMA table_info('workflow_states')")
      .all() as Array<{ name: string }>;
    const colNames = columns.map((c) => c.name);

    expect(colNames).toContain('linear_id');
    expect(colNames).toContain('name');
    expect(colNames).toContain('type');
  });

  it('creates all 5 tables', () => {
    initDb(db);

    const tables = db
      .prepare(
        "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
      )
      .all() as Array<{ name: string }>;
    const tableNames = tables.map((t) => t.name).sort();

    expect(tableNames).toContain('todos');
    expect(tableNames).toContain('sync_queue');
    expect(tableNames).toContain('sync_state');
    expect(tableNames).toContain('labels');
    expect(tableNames).toContain('workflow_states');
  });

  it('is idempotent -- calling twice does not error', () => {
    initDb(db);
    // Second call should not throw
    expect(() => initDb(db)).not.toThrow();
  });

  it('enables WAL journal mode', () => {
    // WAL requires a file-backed database (:memory: always uses 'memory' journal)
    const tmpDir = mkdtempSync(join(tmpdir(), 'braindump-test-'));
    const fileDb = new Database(join(tmpDir, 'test.db'));
    try {
      initDb(fileDb);

      const result = fileDb.prepare('PRAGMA journal_mode').get() as {
        journal_mode: string;
      };
      expect(result.journal_mode).toBe('wal');
    } finally {
      fileDb.close();
      rmSync(tmpDir, { recursive: true });
    }
  });

  it('sets todos.id as PRIMARY KEY', () => {
    initDb(db);

    const columns = db
      .prepare("PRAGMA table_info('todos')")
      .all() as Array<{ name: string; pk: number }>;
    const pkCol = columns.find((c) => c.pk === 1);
    expect(pkCol).toBeDefined();
    expect(pkCol!.name).toBe('id');
  });

  it('has default values for todos columns', () => {
    initDb(db);

    const columns = db
      .prepare("PRAGMA table_info('todos')")
      .all() as Array<{ name: string; dflt_value: string | null }>;

    const defaults = Object.fromEntries(
      columns.map((c) => [c.name, c.dflt_value])
    );

    expect(defaults.source).toBe("'cli'");
    expect(defaults.status).toBe("'inbox'");
    expect(defaults.urgent).toBe('0');
    expect(defaults.important).toBe('0');
    expect(defaults.stale_count).toBe('0');
    expect(defaults.tags).toBe("'[]'");
    expect(defaults.notes).toBe("'[]'");
    expect(defaults.subtasks).toBe("'[]'");
    expect(defaults.done).toBe('0');
  });
});
