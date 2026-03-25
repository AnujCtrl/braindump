// src/printer/celebration.ts
//
// Done celebration messages.
// Not tested -- minimal stub ported from Go internal/printer/celebration.go.

import type { Todo } from "../core/models.js";

const shortCelebrations: string[] = [
  ">>> DONE. %s -- gone.",
  "*** CRUSHED IT *** %s",
  "[x] Another one bites the dust: %s",
  "VICTORY. %s is HISTORY.",
  "=== COMPLETE === %s",
  ">> SHIPPED << %s",
  "BOOM. %s -- destroyed.",
  "++ DONE ++ %s",
  "NAILED IT. %s -- checked off.",
  "--- FINISHED --- %s",
  "DONE AND DUSTED: %s",
  "CONQUERED: %s",
];

const bigCelebrations: string[] = [
  "     *  .  *\n   .  *  .  *  .\n     * DONE! *\n   .  *  .  *  .\n     *  .  *\n\n  [x] %s\n  -- QUEST COMPLETE --",
  "  ___________\n |           |\n |   DONE!   |\n |    [x]    |\n |___________|\n    |     |\n  %s",
  "    \\o/\n     |\n    / \\\n  VICTORY!\n  %s",
  "  =========\n  | DONE! |\n  =========\n  %s",
  "    ***\n   *   *\n  * [x] *\n   *   *\n    ***\n  %s",
];

const legendaryCelebrations: string[] = [
  "=============================\n=                           =\n=   *** LEGENDARY DONE ***  =\n=                           =\n=  %s\n=                           =\n=============================",
  "  +-+-+-+-+-+-+-+-+-+-+-+-+-+\n  |L|E|G|E|N|D|A|R|Y|!|!|!|\n  +-+-+-+-+-+-+-+-+-+-+-+-+-+\n  %s",
  "  ###########################\n  #   LEGENDARY COMPLETE!   #\n  ###########################\n  #  %s\n  ###########################",
];

function pickRandom<T>(arr: T[]): T {
  return arr[Math.floor(Math.random() * arr.length)];
}

/** Returns a random celebration message for completing a todo. */
export function randomCelebration(todo: Todo): string {
  const roll = Math.floor(Math.random() * 10);
  let tmpl: string;
  if (roll === 0) {
    // 10% legendary
    tmpl = pickRandom(legendaryCelebrations);
  } else if (roll < 4) {
    // 30% big
    tmpl = pickRandom(bigCelebrations);
  } else {
    // 60% short
    tmpl = pickRandom(shortCelebrations);
  }
  return tmpl.replace("%s", todo.text);
}
