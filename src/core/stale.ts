// src/core/stale.ts
//
// Stale detection, marking, and revival functions.
// All functions are synchronous — no awaiting needed.

import type { Store } from './store.js';
import type { Todo } from './models.js';

/**
 * Returns todos that have become stale based on their status and age:
 * - inbox: stale after 7 days (using createdAt)
 * - active: stale after 24 hours (using statusChangedAt)
 * Never returns done, waiting, or already-stale items.
 */
export function findStaleItems(store: Store): Todo[] {
  const all = store.listAll();
  const now = Date.now();
  const ms7days = 7 * 24 * 60 * 60 * 1000;
  const ms24hours = 24 * 60 * 60 * 1000;

  return all.filter((todo) => {
    if (todo.status === 'inbox') {
      const age = now - new Date(todo.createdAt).getTime();
      return age >= ms7days;
    }
    if (todo.status === 'active') {
      const age = now - new Date(todo.statusChangedAt).getTime();
      return age >= ms24hours;
    }
    return false;
  });
}

/**
 * Returns todos that are in a "looping" pattern:
 * staleCount >= 2 and not done.
 */
export function findLoopingItems(store: Store): Todo[] {
  const all = store.listAll();
  return all.filter((todo) => todo.staleCount >= 2 && todo.status !== 'done');
}

/**
 * Marks a todo as stale and enqueues a sync action.
 */
export function markStale(store: Store, todoId: string): void {
  store.update(todoId, {
    status: 'stale',
    statusChangedAt: new Date().toISOString(),
  });
  store.enqueueSyncAction(todoId, 'status_change', { status: 'stale' });
}

/**
 * Revives a stale todo back to inbox and increments its staleCount.
 */
export function reviveTodo(store: Store, todoId: string): void {
  const todo = store.getById(todoId);
  if (!todo) return;

  store.update(todoId, {
    status: 'inbox',
    staleCount: todo.staleCount + 1,
    statusChangedAt: new Date().toISOString(),
  });
  store.enqueueSyncAction(todoId, 'status_change', { status: 'inbox' });
}

/**
 * Runs a stale check across the entire store.
 * Marks all stale items and returns the count of items marked.
 */
export function runStaleCheck(store: Store): number {
  const staleItems = findStaleItems(store);
  for (const todo of staleItems) {
    markStale(store, todo.id);
  }
  return staleItems.length;
}
