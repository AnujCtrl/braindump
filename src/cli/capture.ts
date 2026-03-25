import type { Store } from '../core/store.js';
import type { Todo } from '../core/models.js';
import { generateId } from '../core/id.js';

export interface CaptureOpts {
  text: string;
  tags?: string[];
  source?: string;
  urgent?: boolean;
  important?: boolean;
  notes?: string[];
  defaultSource?: string;
  knownTags?: string[];
}

/**
 * Creates a todo from already-parsed capture options.
 * Throws if text is empty or whitespace-only.
 * Returns the created Todo.
 */
export function handleCapture(store: Store, opts: CaptureOpts): Todo {
  const text = opts.text.trim();
  if (!text) {
    throw new Error('Capture text must not be empty');
  }

  // Resolve source: explicit > defaultSource > 'cli'
  const source = opts.source ?? opts.defaultSource ?? 'cli';

  // Resolve tags: use provided tags if non-empty, otherwise default to ['braindump']
  let tags = opts.tags && opts.tags.length > 0 ? [...opts.tags] : ['braindump'];

  // Source auto-adds matching tag when source is in knownTags
  const knownTags = opts.knownTags ?? [];
  if (knownTags.includes(source) && !tags.includes(source)) {
    tags.push(source);
  }

  const urgent = opts.urgent ?? false;
  const important = opts.important ?? false;
  const notes = opts.notes ?? [];
  const now = new Date().toISOString();

  const todo: Todo = {
    id: generateId(),
    linearId: null,
    text,
    source,
    status: 'inbox',
    createdAt: now,
    statusChangedAt: now,
    urgent,
    important,
    staleCount: 0,
    tags,
    notes,
    subtasks: [],
    done: false,
    updatedAt: now,
  };

  store.create(todo);
  store.enqueueSyncAction(todo.id, 'create');

  return todo;
}
