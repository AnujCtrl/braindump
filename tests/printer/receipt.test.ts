// tests/printer/receipt.test.ts
//
// Behavioral contract for receipt formatting.
// formatPlainReceipt generates the full printable receipt for a todo item.
// This output goes directly to a thermal printer -- every line must fit
// within the printer's character width, and key information (text, ID, tags,
// urgency markers) must be present.
//
// These tests protect against:
// - Todo text missing from receipt (defeats the purpose of printing)
// - Urgency/importance markers missing (user misses critical flags)
// - ID missing (can't use the ID to mark done via CLI)
// - Tags missing (no organizational context on the printout)
// - Lines exceeding printer width (causes wrapping artifacts on thermal paper)

import { formatPlainReceipt } from '../../src/printer/receipt.js';
import type { Todo } from '../../src/core/models.js';

/** Helper to build a valid Todo with sensible defaults for receipt testing. */
function makeTodo(overrides: Partial<Todo> = {}): Todo {
  const now = new Date().toISOString();
  return {
    id: 'rcpt1111',
    linearId: null,
    text: 'Receipt test todo',
    source: 'cli',
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

const DEFAULT_WIDTH = 32;

// ---------------------------------------------------------------------------
// formatPlainReceipt -- content presence
// ---------------------------------------------------------------------------

describe('formatPlainReceipt content', () => {
  it('contains the todo text', () => {
    // Protects: the most basic failure -- printing a receipt without the actual todo.
    const todo = makeTodo({ text: 'buy groceries' });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(receipt).toContain('buy groceries');
  });

  it('contains the todo ID', () => {
    // Protects: user can't reference the todo from the printout.
    // The ID is needed to run `todo done <id>`.
    const todo = makeTodo({ id: 'abc12345' });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(receipt).toContain('abc12345');
  });

  it('contains "!! URGENT !!" when todo is urgent', () => {
    // Protects: urgent marker missing, user doesn't see the urgency on paper.
    const todo = makeTodo({ urgent: true });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(receipt.toUpperCase()).toContain('URGENT');
  });

  it('does NOT contain urgent marker when not urgent', () => {
    // Protects: false positive urgency indicator.
    const todo = makeTodo({ urgent: false });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    // The word "URGENT" in all-caps should not appear
    expect(receipt.toUpperCase()).not.toMatch(/!+\s*URGENT\s*!+/);
  });

  it('contains "!!! IMPORTANT !!!" when todo is important', () => {
    // Protects: importance marker missing.
    const todo = makeTodo({ important: true });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(receipt.toUpperCase()).toContain('IMPORTANT');
  });

  it('does NOT contain important marker when not important', () => {
    // Protects: false positive importance indicator.
    const todo = makeTodo({ important: false });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(receipt.toUpperCase()).not.toMatch(/!+\s*IMPORTANT\s*!+/);
  });

  it('contains both urgent and important markers when both are set', () => {
    // Protects: mutual exclusion bug between the two markers.
    const todo = makeTodo({ urgent: true, important: true });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(receipt.toUpperCase()).toContain('URGENT');
    expect(receipt.toUpperCase()).toContain('IMPORTANT');
  });

  it('contains tags with # prefix', () => {
    // Protects: tags missing from printout, losing organizational context.
    const todo = makeTodo({ tags: ['homelab', 'server'] });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(receipt).toContain('#homelab');
    expect(receipt).toContain('#server');
  });

  it('contains tags label or section', () => {
    // Protects: tags present but unlabeled (ambiguous on the receipt).
    const todo = makeTodo({ tags: ['work'] });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    // Should contain either "Tags:" or "tags:" or similar label
    expect(receipt.toLowerCase()).toMatch(/tags?:/i);
  });
});

// ---------------------------------------------------------------------------
// formatPlainReceipt -- width constraints
// ---------------------------------------------------------------------------

describe('formatPlainReceipt width constraints', () => {
  it('no line exceeds width + 5 tolerance', () => {
    // Protects: lines overflowing the thermal printer's character width.
    // We allow +5 tolerance for decorative borders that may include
    // edge characters.
    const todo = makeTodo({
      text: 'Fix the server timeout issue that has been happening since last Tuesday',
      tags: ['homelab', 'server', 'networking'],
      urgent: true,
      important: true,
    });

    const receipt = formatPlainReceipt(todo, 5, DEFAULT_WIDTH);
    const lines = receipt.split('\n');

    for (const line of lines) {
      expect(line.length).toBeLessThanOrEqual(DEFAULT_WIDTH + 5);
    }
  });

  it('handles narrow width without crashing', () => {
    // Protects: very narrow width causing infinite loops or crashes.
    const todo = makeTodo({ text: 'narrow' });

    const receipt = formatPlainReceipt(todo, 0, 16);

    expect(typeof receipt).toBe('string');
    expect(receipt.length).toBeGreaterThan(0);
  });

  it('handles long text by wrapping within width', () => {
    // Protects: long text producing a single line that overflows.
    const todo = makeTodo({
      text: 'This is a very long todo text that should definitely be wrapped across multiple lines to fit within the printer width',
    });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);
    const lines = receipt.split('\n');

    for (const line of lines) {
      expect(line.length).toBeLessThanOrEqual(DEFAULT_WIDTH + 5);
    }
  });

  it('handles wider width (48 chars for larger thermal printers)', () => {
    // Protects: width parameter being ignored (always using 32).
    const todo = makeTodo({
      text: 'Testing wider receipt format for 48-column thermal printer',
      tags: ['printer'],
    });

    const receipt = formatPlainReceipt(todo, 0, 48);
    const lines = receipt.split('\n');

    for (const line of lines) {
      expect(line.length).toBeLessThanOrEqual(48 + 5);
    }
  });
});

// ---------------------------------------------------------------------------
// formatPlainReceipt -- streak display
// ---------------------------------------------------------------------------

describe('formatPlainReceipt streak', () => {
  it('includes streak count when streak > 0', () => {
    // Protects: streak not showing on receipt, losing the gamification element.
    const todo = makeTodo({ text: 'streak test' });

    const receipt = formatPlainReceipt(todo, 7, DEFAULT_WIDTH);

    // Should contain the number 7 somewhere in the receipt as part of streak display
    expect(receipt).toContain('7');
  });

  it('does not crash with streak of 0', () => {
    // Protects: zero streak causing display issues.
    const todo = makeTodo({ text: 'no streak' });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(typeof receipt).toBe('string');
    expect(receipt.length).toBeGreaterThan(0);
  });
});

// ---------------------------------------------------------------------------
// formatPlainReceipt -- edge cases
// ---------------------------------------------------------------------------

describe('formatPlainReceipt edge cases', () => {
  it('handles todo with empty tags array', () => {
    // Protects: crash when no tags to display.
    const todo = makeTodo({ tags: [] });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(typeof receipt).toBe('string');
    expect(receipt).toContain(todo.id);
  });

  it('handles todo with many tags', () => {
    // Protects: many tags causing a single extra-long line.
    const todo = makeTodo({
      tags: ['homelab', 'server', 'networking', 'dns', 'critical', 'infrastructure'],
    });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);
    const lines = receipt.split('\n');

    for (const line of lines) {
      expect(line.length).toBeLessThanOrEqual(DEFAULT_WIDTH + 5);
    }
  });

  it('handles special characters in todo text', () => {
    // Protects: special characters causing format corruption.
    const todo = makeTodo({ text: 'fix #42 issue & check "logs"' });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(receipt).toContain('fix #42 issue');
  });

  it('returns a multi-line string (receipts always have structure)', () => {
    // Protects: receipt being a single blob of text.
    const todo = makeTodo({ text: 'structured receipt' });

    const receipt = formatPlainReceipt(todo, 0, DEFAULT_WIDTH);

    expect(receipt).toContain('\n');
    // A receipt should have at least a few sections (header, text, id, footer)
    const lines = receipt.split('\n').filter((l) => l.trim().length > 0);
    expect(lines.length).toBeGreaterThanOrEqual(3);
  });
});
