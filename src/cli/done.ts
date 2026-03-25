import type { Store } from '../core/store.js';

/**
 * Marks a todo as done.
 * Throws if the ID is not found or the todo is already done.
 * Enqueues a 'status_change' sync action.
 */
export function handleDone(store: Store, id: string): void {
  const todo = store.getById(id);
  if (!todo) {
    throw new Error(`Todo not found: ${id}`);
  }
  if (todo.done) {
    throw new Error(`Todo is already done: ${id}`);
  }

  const now = new Date().toISOString();
  store.update(id, {
    status: 'done',
    done: true,
    statusChangedAt: now,
  });

  store.enqueueSyncAction(id, 'status_change');
}
