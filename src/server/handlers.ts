import type { FastifyRequest, FastifyReply } from 'fastify';
import type { Store } from '../core/store.js';
import type { SyncEngine } from '../core/sync.js';
import type { Todo } from '../core/models.js';
import { generateId } from '../core/id.js';

interface CreateTodoBody {
  text?: string;
  source?: string;
  tags?: string[];
  urgent?: boolean;
  important?: boolean;
  notes?: string[];
}

interface UpdateTodoBody {
  text?: string;
  source?: string;
  tags?: string[];
  urgent?: boolean;
  important?: boolean;
  notes?: string[];
  subtasks?: string[];
  linearId?: string | null;
  staleCount?: number;
  done?: boolean;
}

interface StatusChangeBody {
  status: string;
}

interface DumpBody {
  items?: string[];
}

interface TodoIdParams {
  id: string;
}

interface ListTodoQuery {
  status?: string;
  tag?: string;
}

export class Handlers {
  private store: Store;
  private syncEngine: SyncEngine;

  constructor(store: Store, syncEngine: SyncEngine) {
    this.store = store;
    this.syncEngine = syncEngine;
  }

  async createTodo(
    request: FastifyRequest<{ Body: CreateTodoBody }>,
    reply: FastifyReply,
  ): Promise<void> {
    const { text, source, tags, urgent, important, notes } = request.body ?? {};

    if (!text || text.trim() === '') {
      reply.status(400).send({ error: 'text is required' });
      return;
    }

    const now = new Date().toISOString();
    const id = generateId();

    const todo: Todo = {
      id,
      linearId: null,
      text: text.trim(),
      source: source ?? 'api',
      status: 'inbox',
      createdAt: now,
      statusChangedAt: now,
      urgent: urgent ?? false,
      important: important ?? false,
      staleCount: 0,
      tags: tags && tags.length > 0 ? tags : ['braindump'],
      notes: notes ?? [],
      subtasks: [],
      done: false,
      updatedAt: now,
    };

    this.store.create(todo);
    this.store.enqueueSyncAction(todo.id, 'create');

    reply.status(201).send({ id });
  }

  async listTodos(
    request: FastifyRequest<{ Querystring: ListTodoQuery }>,
    reply: FastifyReply,
  ): Promise<void> {
    const { status, tag } = request.query;

    let todos: Todo[];
    if (status) {
      todos = this.store.listByStatus(status);
    } else if (tag) {
      todos = this.store.listByTag(tag);
    } else {
      todos = this.store.listOpen();
    }

    reply.status(200).send(todos);
  }

  async updateTodo(
    request: FastifyRequest<{ Params: TodoIdParams; Body: UpdateTodoBody }>,
    reply: FastifyReply,
  ): Promise<void> {
    const { id } = request.params;
    const existing = this.store.getById(id);

    if (!existing) {
      reply.status(404).send({ error: 'not found' });
      return;
    }

    this.store.update(id, request.body);
    reply.status(200).send({ ok: true });
  }

  async deleteTodo(
    request: FastifyRequest<{ Params: TodoIdParams }>,
    reply: FastifyReply,
  ): Promise<void> {
    const { id } = request.params;

    // Check for linearId BEFORE deleting
    const existing = this.store.getById(id);
    if (existing?.linearId) {
      this.store.enqueueSyncAction(id, 'delete', { linearId: existing.linearId });
    }

    this.store.delete(id);
    reply.status(204).send();
  }

  async changeStatus(
    request: FastifyRequest<{ Params: TodoIdParams; Body: StatusChangeBody }>,
    reply: FastifyReply,
  ): Promise<void> {
    const { id } = request.params;
    const { status } = request.body;

    const existing = this.store.getById(id);
    if (!existing) {
      reply.status(404).send({ error: 'not found' });
      return;
    }

    const now = new Date().toISOString();
    this.store.update(id, { status: status as Todo['status'], statusChangedAt: now });
    this.store.enqueueSyncAction(id, 'status_change', { status });

    reply.status(200).send({ ok: true });
  }

  async bulkDump(
    request: FastifyRequest<{ Body: DumpBody }>,
    reply: FastifyReply,
  ): Promise<void> {
    const { items } = request.body ?? {};

    if (!items || !Array.isArray(items) || items.length === 0) {
      reply.status(400).send({ error: 'items array is required and must not be empty' });
      return;
    }

    const ids: string[] = [];
    const now = new Date().toISOString();

    for (const item of items) {
      const id = generateId();

      const todo: Todo = {
        id,
        linearId: null,
        text: item,
        source: 'api',
        status: 'unprocessed',
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
      };

      this.store.create(todo);
      this.store.enqueueSyncAction(todo.id, 'create');
      ids.push(id);
    }

    reply.status(201).send({ ids });
  }

  async getInfo(
    _request: FastifyRequest,
    reply: FastifyReply,
  ): Promise<void> {
    const counts = this.store.getInfoCounts();
    reply.status(200).send(counts);
  }

  async healthCheck(
    _request: FastifyRequest,
    reply: FastifyReply,
  ): Promise<void> {
    reply.status(200).send({ status: 'ok' });
  }
}
