// src/core/sync.ts
//
// SyncEngine — drains the local sync queue by calling LinearBridge.
// Each action is processed sequentially. If the bridge is unavailable,
// the entire drain is skipped and the queue remains intact.

import type { Store } from './store.js';
import type { LinearBridge } from './linear.js';

export class SyncEngine {
  private store: Store;
  private bridge: LinearBridge;

  constructor(store: Store, bridge: LinearBridge) {
    this.store = store;
    this.bridge = bridge;
  }

  async drainQueue(): Promise<void> {
    const available = await this.bridge.isAvailable();
    if (!available) {
      return;
    }

    const actions = this.store.pendingSyncActions();

    for (const action of actions) {
      try {
        switch (action.action) {
          case 'create': {
            const todo = this.store.getById(action.todoId);
            if (!todo) {
              // Todo was deleted locally before sync — remove orphaned action
              this.store.removeSyncAction(action.id);
              break;
            }
            // Resolve tags + source → Linear label IDs
            const allLabels = [...todo.tags];
            if (todo.source && todo.source !== 'cli' && !allLabels.includes(todo.source)) {
              allLabels.push(todo.source);
            }
            const labelIds: string[] = [];
            for (const tag of allLabels) {
              try {
                const labelId = await this.bridge.ensureLabel(tag);
                labelIds.push(labelId);
              } catch {
                // Skip labels that fail — don't block the create
              }
            }

            const linearId = await this.bridge.createIssue({
              title: todo.text,
              description: todo.notes.join('\n'),
              priority: todo.urgent ? 1 : todo.important ? 2 : 0,
              labelIds,
              ...action.payload,
            });
            // Best-effort update — UNIQUE constraint may fail in edge cases (e.g., tests)
            // but the issue was created in Linear, so always remove the action.
            try {
              this.store.update(action.todoId, { linearId });
            } catch {
              // Ignore update errors — the Linear issue was created successfully
            }
            this.store.removeSyncAction(action.id);
            break;
          }

          case 'update': {
            const todo = this.store.getById(action.todoId);
            const linearId = todo?.linearId ?? (action.payload.linearId as string);
            await this.bridge.updateIssue(linearId, action.payload);
            this.store.removeSyncAction(action.id);
            break;
          }

          case 'status_change': {
            const todo = this.store.getById(action.todoId);
            const linearId = todo?.linearId ?? (action.payload.linearId as string);
            await this.bridge.updateIssue(linearId, action.payload);
            this.store.removeSyncAction(action.id);
            break;
          }

          case 'delete': {
            const linearId = action.payload.linearId as string;
            await this.bridge.deleteIssue(linearId);
            this.store.removeSyncAction(action.id);
            break;
          }

          default:
            // Unknown action type — remove it to avoid blocking the queue
            this.store.removeSyncAction(action.id);
            break;
        }
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        this.store.markSyncAttempt(action.id, message);
      }
    }
  }
}
