import type { Store } from '../core/store.js';
import type { Todo } from '../core/models.js';

export interface ListOpts {
  tag?: string;
  status?: string;
  all?: boolean;
  looping?: boolean;
}

/**
 * Returns filtered todos based on the provided options.
 * No options: returns open (non-done) todos.
 * tag: filters by tag. status: filters by status.
 * all: returns all including done. looping: staleCount >= 2 among open.
 */
export function handleList(store: Store, opts: ListOpts): Todo[] {
  if (opts.tag) {
    return store.listByTag(opts.tag);
  }
  if (opts.status) {
    return store.listByStatus(opts.status);
  }
  if (opts.all) {
    return store.listAll();
  }
  if (opts.looping) {
    return store.listOpen().filter((t) => t.staleCount >= 2);
  }
  return store.listOpen();
}

/**
 * Formats a single todo as a one-line display string.
 * Format: [ ] id text #tag1 #tag2 [sync-indicator]
 *
 * Sync indicators:
 *   * = synced to Linear (has linearId)
 *   ~ = pending sync (hasPendingSync)
 */
export function formatTodoLine(todo: Todo, hasPendingSync: boolean): string {
  const checkbox = todo.done ? '[x]' : '[ ]';
  const tagStr = todo.tags.map((t) => `#${t}`).join(' ');

  let syncIndicator = '';
  if (todo.linearId) {
    syncIndicator = ' *';
  } else if (hasPendingSync) {
    syncIndicator = ' ~';
  }

  const parts = [checkbox, todo.id, todo.text];
  if (tagStr) {
    parts.push(tagStr);
  }

  return parts.join(' ') + syncIndicator;
}
