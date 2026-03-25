export type TodoStatus =
  | "unprocessed"
  | "inbox"
  | "active"
  | "waiting"
  | "done"
  | "stale";

export const VALID_STATUSES: TodoStatus[] = [
  "unprocessed",
  "inbox",
  "active",
  "waiting",
  "done",
  "stale",
];

export interface Todo {
  id: string;
  linearId: string | null;
  text: string;
  source: string;
  status: TodoStatus;
  createdAt: string;
  statusChangedAt: string;
  urgent: boolean;
  important: boolean;
  staleCount: number;
  tags: string[];
  notes: string[];
  subtasks: string[];
  done: boolean;
  updatedAt: string;
}

export interface SyncQueueEntry {
  id: number;
  todoId: string;
  action: string;
  payload: string;
  createdAt: string;
  attempts: number;
  lastError: string | null;
}

export interface InfoLine {
  unprocessed: number;
  active: number;
  looping: number;
}

interface MetaComment {
  source: string;
  staleCount: number;
  localId: string;
}

/**
 * Maps urgent/important flags to Linear priority integers.
 * 0 = No priority, 1 = Urgent, 2 = High
 */
export function toLinearPriority(urgent: boolean, important: boolean): number {
  if (urgent) return 1;
  if (important) return 2;
  return 0;
}

/**
 * Builds an HTML comment embedding braindump metadata for Linear issue descriptions.
 * Format: <!-- braindump:{"source":"cli","staleCount":0,"localId":"a1b2c3d4"} -->
 */
export function buildMetaComment(meta: MetaComment): string {
  const payload: MetaComment = {
    source: meta.source,
    staleCount: meta.staleCount,
    localId: meta.localId,
  };
  return `<!-- braindump:${JSON.stringify(payload)} -->`;
}

/**
 * Extracts braindump metadata from a Linear issue description.
 * Returns null if no braindump comment is found or input is null/undefined.
 */
export function parseMetaComment(description: string): MetaComment | null {
  if (description == null) return null;
  const match = description.match(/braindump:({.*?})/);
  if (!match) return null;
  try {
    const parsed = JSON.parse(match[1]) as MetaComment;
    return {
      source: parsed.source,
      staleCount: parsed.staleCount,
      localId: parsed.localId,
    };
  } catch {
    return null;
  }
}
