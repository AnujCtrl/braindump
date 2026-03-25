// src/printer/receipt.ts
//
// Plain-text receipt formatter for thermal printer output.
// Ported from Go internal/printer/receipt.go FormatPlainReceipt.

import type { Todo } from "../core/models.js";
import {
  wordWrap,
  centerText,
  randomHeader,
  randomArt,
  randomBorder,
  randomMessage,
  randomSignoff,
  randomTimeGreeting,
  randomDayFlavor,
  isLegendary,
} from "./fun.js";

/**
 * Formats a todo into a plain ASCII receipt string.
 * Each line respects the given width for thermal printer output.
 */
export function formatPlainReceipt(
  todo: Todo,
  streak: number,
  width: number,
): string {
  const lines: string[] = [];
  const writeln = (s: string) => lines.push(s);

  const legendary = isLegendary();

  // --- Header ---
  let border = randomBorder(width);
  if (legendary) {
    border = "*".repeat(width);
  }
  writeln(border);

  let header = randomHeader();
  if (legendary) {
    header = "*** LEGENDARY TICKET ***";
  }
  writeln(centerText(header, width));
  writeln(border);

  // Time greeting + day flavor
  writeln(randomTimeGreeting());
  const flavor = randomDayFlavor();
  if (flavor !== "") {
    writeln(flavor);
  }
  writeln(randomMessage());
  writeln("-".repeat(width));

  // --- Body ---
  writeln("");

  // Urgent/Important markers
  if (todo.urgent) {
    writeln("!! URGENT !!");
  }
  if (todo.important) {
    writeln("!!! IMPORTANT !!!");
  }

  // Todo text (word-wrapped)
  for (const line of wordWrap(todo.text, width)) {
    writeln(line);
  }
  writeln("");

  // Metadata
  writeln(`ID: ${todo.id}`);
  if (todo.tags.length > 0) {
    const tagStr = todo.tags.map((t) => `#${t}`).join(" ");
    const fullTagLine = `Tags: ${tagStr}`;
    for (const tl of wordWrap(fullTagLine, width)) {
      writeln(tl);
    }
  }

  const now = new Date();
  const dateStr = [
    now.getFullYear(),
    String(now.getMonth() + 1).padStart(2, "0"),
    String(now.getDate()).padStart(2, "0"),
  ].join("-");
  const timeStr = [
    String(now.getHours()).padStart(2, "0"),
    String(now.getMinutes()).padStart(2, "0"),
  ].join(":");
  writeln(`${dateStr} ${timeStr}`);

  // Streak
  if (streak > 0) {
    writeln(`STREAK: ${streak} days`);
  }

  writeln("");

  // ASCII art
  if (legendary) {
    writeln("=== LEGENDARY ===");
  }
  const art = randomArt();
  for (const artLine of art.split("\n")) {
    writeln(artLine);
  }

  writeln("-".repeat(width));

  // --- Footer ---
  writeln(centerText(`* ${randomSignoff()} *`, width));
  writeln(border);

  // Tear margin (3 blank lines)
  writeln("");
  writeln("");
  writeln("");

  return lines.join("\n");
}
