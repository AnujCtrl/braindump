import Database from 'better-sqlite3';
import type { Todo, SyncQueueEntry } from './models.js';

interface DbTodoRow {
  id: string;
  linear_id: string | null;
  text: string;
  source: string;
  status: string;
  created_at: string;
  status_changed_at: string;
  urgent: number;
  important: number;
  stale_count: number;
  tags: string;
  notes: string;
  subtasks: string;
  done: number;
  updated_at: string;
}

interface DbSyncQueueRow {
  id: number;
  todo_id: string;
  action: string;
  payload: string;
  created_at: string;
  attempts: number;
  last_error: string | null;
}

interface DbLabelRow {
  linear_id: string;
  name: string;
  color: string | null;
}

interface DbWorkflowStateRow {
  linear_id: string;
  name: string;
  type: string;
}

function rowToTodo(row: DbTodoRow): Todo {
  return {
    id: row.id,
    linearId: row.linear_id,
    text: row.text,
    source: row.source,
    status: row.status as Todo['status'],
    createdAt: row.created_at,
    statusChangedAt: row.status_changed_at,
    urgent: row.urgent !== 0,
    important: row.important !== 0,
    staleCount: row.stale_count,
    tags: JSON.parse(row.tags) as string[],
    notes: JSON.parse(row.notes) as string[],
    subtasks: JSON.parse(row.subtasks) as string[],
    done: row.done !== 0,
    updatedAt: row.updated_at,
  };
}

function rowToSyncQueueEntry(row: DbSyncQueueRow): SyncQueueEntry {
  return {
    id: row.id,
    todoId: row.todo_id,
    action: row.action,
    payload: JSON.parse(row.payload) as Record<string, unknown>,
    createdAt: row.created_at,
    attempts: row.attempts,
    lastError: row.last_error,
  };
}

export class Store {
  private db: Database.Database;

  constructor(db: Database.Database) {
    this.db = db;
  }

  // ---------------------------------------------------------------------------
  // CRUD
  // ---------------------------------------------------------------------------

  create(todo: Todo): void {
    const stmt = this.db.prepare(`
      INSERT INTO todos (
        id, linear_id, text, source, status,
        created_at, status_changed_at,
        urgent, important, stale_count,
        tags, notes, subtasks, done, updated_at
      ) VALUES (
        ?, ?, ?, ?, ?,
        ?, ?,
        ?, ?, ?,
        ?, ?, ?, ?, ?
      )
    `);

    stmt.run(
      todo.id,
      todo.linearId,
      todo.text,
      todo.source,
      todo.status,
      todo.createdAt,
      todo.statusChangedAt,
      todo.urgent ? 1 : 0,
      todo.important ? 1 : 0,
      todo.staleCount,
      JSON.stringify(todo.tags),
      JSON.stringify(todo.notes),
      JSON.stringify(todo.subtasks),
      todo.done ? 1 : 0,
      todo.updatedAt,
    );
  }

  getById(id: string): Todo | null {
    const row = this.db
      .prepare('SELECT * FROM todos WHERE id = ?')
      .get(id) as DbTodoRow | undefined;
    return row ? rowToTodo(row) : null;
  }

  getByLinearId(linearId: string): Todo | null {
    const row = this.db
      .prepare('SELECT * FROM todos WHERE linear_id = ?')
      .get(linearId) as DbTodoRow | undefined;
    return row ? rowToTodo(row) : null;
  }

  update(id: string, fields: Partial<Todo>): void {
    const existing = this.getById(id);
    if (!existing) return;

    const merged: Todo = { ...existing, ...fields };

    const stmt = this.db.prepare(`
      UPDATE todos SET
        linear_id = ?,
        text = ?,
        source = ?,
        status = ?,
        created_at = ?,
        status_changed_at = ?,
        urgent = ?,
        important = ?,
        stale_count = ?,
        tags = ?,
        notes = ?,
        subtasks = ?,
        done = ?,
        updated_at = ?
      WHERE id = ?
    `);

    stmt.run(
      merged.linearId,
      merged.text,
      merged.source,
      merged.status,
      merged.createdAt,
      merged.statusChangedAt,
      merged.urgent ? 1 : 0,
      merged.important ? 1 : 0,
      merged.staleCount,
      JSON.stringify(merged.tags),
      JSON.stringify(merged.notes),
      JSON.stringify(merged.subtasks),
      merged.done ? 1 : 0,
      merged.updatedAt,
      id,
    );
  }

  delete(id: string): void {
    this.db.prepare('DELETE FROM todos WHERE id = ?').run(id);
  }

  // ---------------------------------------------------------------------------
  // Listing
  // ---------------------------------------------------------------------------

  listByStatus(status: string): Todo[] {
    const rows = this.db
      .prepare('SELECT * FROM todos WHERE status = ?')
      .all(status) as DbTodoRow[];
    return rows.map(rowToTodo);
  }

  listOpen(): Todo[] {
    const rows = this.db
      .prepare("SELECT * FROM todos WHERE status != 'done'")
      .all() as DbTodoRow[];
    return rows.map(rowToTodo);
  }

  listAll(): Todo[] {
    const rows = this.db.prepare('SELECT * FROM todos').all() as DbTodoRow[];
    return rows.map(rowToTodo);
  }

  listByTag(tag: string): Todo[] {
    // Search for the tag as a JSON string value within the tags array.
    // The LIKE pattern '%"tagname"%' matches any JSON array containing that tag as a string element.
    const rows = this.db
      .prepare('SELECT * FROM todos WHERE tags LIKE ?')
      .all(`%"${tag}"%`) as DbTodoRow[];
    return rows.map(rowToTodo);
  }

  allIds(): Set<string> {
    const rows = this.db
      .prepare('SELECT id FROM todos')
      .all() as { id: string }[];
    return new Set(rows.map((r) => r.id));
  }

  // ---------------------------------------------------------------------------
  // Info counts
  // ---------------------------------------------------------------------------

  getInfoCounts(): { unprocessed: number; active: number; looping: number } {
    const unprocessed = (
      this.db
        .prepare("SELECT COUNT(*) as count FROM todos WHERE status = 'unprocessed'")
        .get() as { count: number }
    ).count;

    const active = (
      this.db
        .prepare(
          "SELECT COUNT(*) as count FROM todos WHERE status = 'active' AND stale_count < 2",
        )
        .get() as { count: number }
    ).count;

    const looping = (
      this.db
        .prepare(
          "SELECT COUNT(*) as count FROM todos WHERE stale_count >= 2 AND status != 'done'",
        )
        .get() as { count: number }
    ).count;

    return { unprocessed, active, looping };
  }

  // ---------------------------------------------------------------------------
  // Sync queue
  // ---------------------------------------------------------------------------

  enqueueSyncAction(todoId: string, action: string, payload: object = {}): void {
    this.db
      .prepare(
        'INSERT INTO sync_queue (todo_id, action, payload, created_at) VALUES (?, ?, ?, ?)',
      )
      .run(todoId, action, JSON.stringify(payload), new Date().toISOString());
  }

  pendingSyncActions(): SyncQueueEntry[] {
    const rows = this.db
      .prepare('SELECT * FROM sync_queue ORDER BY id ASC')
      .all() as DbSyncQueueRow[];
    return rows.map(rowToSyncQueueEntry);
  }

  removeSyncAction(id: number): void {
    this.db.prepare('DELETE FROM sync_queue WHERE id = ?').run(id);
  }

  markSyncAttempt(id: number, error: string): void {
    this.db
      .prepare(
        'UPDATE sync_queue SET attempts = attempts + 1, last_error = ? WHERE id = ?',
      )
      .run(error, id);
  }

  // ---------------------------------------------------------------------------
  // Labels
  // ---------------------------------------------------------------------------

  upsertLabel(label: { linearId: string; name: string; color: string | null }): void {
    this.db
      .prepare(
        'INSERT OR REPLACE INTO labels (linear_id, name, color) VALUES (?, ?, ?)',
      )
      .run(label.linearId, label.name, label.color);
  }

  getLabelByName(name: string): { linearId: string; name: string; color: string | null } | null {
    const row = this.db
      .prepare('SELECT * FROM labels WHERE LOWER(name) = LOWER(?)')
      .get(name) as DbLabelRow | undefined;
    if (!row) return null;
    return { linearId: row.linear_id, name: row.name, color: row.color };
  }

  allLabels(): Array<{ linearId: string; name: string; color: string | null }> {
    const rows = this.db.prepare('SELECT * FROM labels').all() as DbLabelRow[];
    return rows.map((r) => ({ linearId: r.linear_id, name: r.name, color: r.color }));
  }

  // ---------------------------------------------------------------------------
  // Workflow states
  // ---------------------------------------------------------------------------

  upsertWorkflowState(state: { linearId: string; name: string; type: string }): void {
    this.db
      .prepare(
        'INSERT OR REPLACE INTO workflow_states (linear_id, name, type) VALUES (?, ?, ?)',
      )
      .run(state.linearId, state.name, state.type);
  }

  getWorkflowStateByName(
    name: string,
  ): { linearId: string; name: string; type: string } | null {
    const row = this.db
      .prepare('SELECT * FROM workflow_states WHERE name = ?')
      .get(name) as DbWorkflowStateRow | undefined;
    if (!row) return null;
    return { linearId: row.linear_id, name: row.name, type: row.type };
  }
}
