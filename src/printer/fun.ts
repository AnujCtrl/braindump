// src/printer/fun.ts
//
// Decorative elements for receipt-style todo printouts.
// Ported from Go internal/printer/fun.go.

const headers: string[] = [
  "QUEST ACTIVE",
  "WORK ORDER",
  "MISSION BRIEF",
  "TODAY'S BOSS FIGHT",
  "TICKET TO RIDE",
  "BRAIN ACTIVATED",
  "FOCUS LOCKED",
  "OBJECTIVE SET",
  "NEW OBJECTIVE",
  "TARGET ACQUIRED",
];

const messages: string[] = [
  "you got this.",
  "ship it.",
  "one thing at a time.",
  "future you says thanks.",
  "momentum is everything.",
  "done > perfect.",
  "begin.",
  "lock in.",
  "trust the process.",
  "just start.",
  "keep it moving.",
  "do the next thing.",
  "one bite at a time.",
  "you've done harder.",
  "action beats anxiety.",
  "good enough is good.",
  "focus is a superpower.",
  "now or never.",
  "tick tock.",
  "make it happen.",
  "no zero days.",
  "prove them wrong.",
  "be relentless.",
  "grind time.",
  "execute.",
];

const signoffs: string[] = [
  "GO FORTH",
  "LOCK IN",
  "LET'S GO",
  "BEGIN",
  "EXECUTE",
  "DO THE THING",
  "COMMENCE",
  "ENGAGE",
  "INITIATE",
  "LAUNCH",
];

const borderChars: string[] = ["=", "~", "#", "*", "+", "-", ".", ">"];

const receiptArt: string[] = [
  // Cat
  "  /\\_/\\\n (o o )\n  > ^ <",
  // Cow
  "   __\n  (oo)\n  /--\\\n / |  |",
  // Shield
  "  /----\\\n |      |\n |  ()  |\n  \\----/",
  // Mountain
  "    /\\\n   /  \\\n  / /\\ \\\n /______\\",
  // Checkbox
  " +------+\n |      |\n |  OK  |\n +------+",
  // Diamond
  "    *\n   * *\n  *   *\n   * *\n    *",
  // Rocket
  "    /\\\n   /  \\\n  | () |\n  |    |\n  /----\\",
  // Bunny
  " (\\  /)\n ( >.< )\n (\")(\")",
  // Flag
  " +----\n |####\n |####\n |\n |",
  // Star
  "    *\n  * * *\n *******\n  * * *\n    *",
  // Trophy
  "  _____\n |     |\n |     |\n  \\___/\n   | |\n  _|_|_",
  // Sword
  "   /\\\n  /  \\\n |    |\n  \\  /\n   \\/\n    |",
  // Potion
  "   _\n  | |\n / ~ \\\n|~~~~~|\n \\_O_/",
  // Smiley
  "  -----\n (o   o)\n  \\ ^ /\n   ---",
  // Brick wall
  " +--+--+--+\n |  |  |  |\n +--+--+--+\n |  |  |  |\n +--+--+--+",
  // Zen
  "  _   _\n (.) (.)\n   \\_/\n   |||",
  // Chalice
  "  /---\\\n /     \\\n|       |\n \\     /\n  |   |\n  |___|",
];

function pickRandom<T>(arr: T[]): T {
  return arr[Math.floor(Math.random() * arr.length)];
}

/**
 * Wraps text at word boundaries so no line exceeds maxWidth characters.
 * Words longer than maxWidth are split at the character boundary.
 */
export function wordWrap(text: string, maxWidth: number): string[] {
  const words = text.split(/\s+/).filter((w) => w.length > 0);
  if (words.length === 0) {
    return [text];
  }

  // Split a single word into chunks of at most maxWidth chars.
  function splitLong(word: string): string[] {
    if (word.length <= maxWidth) {
      return [word];
    }
    const chunks: string[] = [];
    let remaining = word;
    while (remaining.length > maxWidth) {
      chunks.push(remaining.slice(0, maxWidth));
      remaining = remaining.slice(maxWidth);
    }
    if (remaining.length > 0) {
      chunks.push(remaining);
    }
    return chunks;
  }

  const lines: string[] = [];
  let current = "";

  for (const word of words) {
    const parts = splitLong(word);
    for (const part of parts) {
      if (current === "") {
        current = part;
      } else if (current.length + 1 + part.length <= maxWidth) {
        current += " " + part;
      } else {
        lines.push(current);
        current = part;
      }
    }
  }
  if (current !== "") {
    lines.push(current);
  }
  return lines;
}

/**
 * Pads a string with spaces on both sides to center it within width.
 * Returns a string of exactly width characters (or the original if already >= width).
 */
export function centerText(s: string, width: number): string {
  if (s.length >= width) {
    return s;
  }
  const totalPad = width - s.length;
  const leftPad = Math.floor(totalPad / 2);
  const rightPad = totalPad - leftPad;
  return " ".repeat(leftPad) + s + " ".repeat(rightPad);
}

/** Returns a random header string for receipts. */
export function randomHeader(): string {
  return pickRandom(headers);
}

/** Returns random ASCII art that fits on a receipt. */
export function randomArt(): string {
  return pickRandom(receiptArt);
}

/**
 * Returns a decorative border string of the given width.
 * Picks a random border character and repeats it to fill the width.
 */
export function randomBorder(width: number): string {
  const ch = pickRandom(borderChars);
  return ch.repeat(width);
}

/** Returns a random motivational message for receipts. */
export function randomMessage(): string {
  return pickRandom(messages);
}

/** Returns a random sign-off message. */
export function randomSignoff(): string {
  return pickRandom(signoffs);
}

/** Returns a greeting based on the current time of day. */
export function randomTimeGreeting(): string {
  const hour = new Date().getHours();
  if (hour < 6) return "NIGHT OWL MODE";
  if (hour < 12) return "EARLY BIRD MODE";
  if (hour < 17) return "AFTERNOON GRIND";
  return "EVENING SESSION";
}

/** Returns a flavor string based on the current day of week. */
export function randomDayFlavor(): string {
  const day = new Date().getDay(); // 0=Sun, 1=Mon, ..., 6=Sat
  switch (day) {
    case 1:
      return "FRESH START";
    case 5:
      return "FINISH STRONG";
    case 0:
    case 6:
      return "WEEKEND WARRIOR";
    default:
      return "MIDWEEK HUSTLE";
  }
}

/** Returns true with approximately 5% probability. */
export function isLegendary(): boolean {
  return Math.random() < 0.05;
}
