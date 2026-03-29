// src/core/tags.ts
//
// Tag validation and fuzzy matching using Levenshtein distance.

/**
 * Computes the Levenshtein edit distance between two strings.
 * Uses a single-row (two-row rolling) optimization for space efficiency.
 */
export function levenshtein(a: string, b: string): number {
  const la = a.length;
  const lb = b.length;

  if (la === 0) return lb;
  if (lb === 0) return la;

  let prev: number[] = [];
  for (let j = 0; j <= lb; j++) {
    prev[j] = j;
  }

  for (let i = 1; i <= la; i++) {
    const curr: number[] = new Array(lb + 1);
    curr[0] = i;
    for (let j = 1; j <= lb; j++) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1;
      curr[j] = Math.min(
        prev[j] + 1,      // deletion
        curr[j - 1] + 1,  // insertion
        prev[j - 1] + cost // substitution
      );
    }
    prev = curr;
  }

  return prev[lb];
}

/**
 * TagValidator validates tags against a known set and provides fuzzy matching.
 * All comparisons are case-insensitive.
 */
export class TagValidator {
  private knownTags: string[];

  constructor(knownTags: string[]) {
    this.knownTags = [...knownTags];
  }

  /**
   * Returns true if the tag is in the known set (case-insensitive).
   */
  isValid(tag: string): boolean {
    if (tag === '') return false;
    const lower = tag.toLowerCase();
    return this.knownTags.some((t) => t.toLowerCase() === lower);
  }

  /**
   * Returns the closest known tag within Levenshtein distance <= 3,
   * or null if no match is within the threshold.
   * Comparison is case-insensitive; the original casing of the known tag is returned.
   */
  fuzzyMatch(input: string): string | null {
    const lower = input.toLowerCase();
    let bestMatch: string | null = null;
    let bestDist = 4; // threshold: must be <= 3

    for (const tag of this.knownTags) {
      const d = levenshtein(lower, tag.toLowerCase());
      if (d < bestDist) {
        bestDist = d;
        bestMatch = tag;
      }
    }

    return bestMatch;
  }

  /**
   * Adds a tag to the known set. No-op if the tag already exists (case-insensitive).
   */
  addTag(tag: string): void {
    if (!this.isValid(tag)) {
      this.knownTags.push(tag);
    }
  }
}
