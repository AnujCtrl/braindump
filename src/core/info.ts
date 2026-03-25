// src/core/info.ts
//
// Info line computation and formatting.
// The info line is shown after every CLI command as a quick at-a-glance
// summary of the system state.

import type { Store } from './store.js';

export interface InfoCounts {
  unprocessed: number;
  active: number;
  looping: number;
}

/**
 * Fetches current info counts from the store.
 */
export function getInfoLine(store: Store): InfoCounts {
  return store.getInfoCounts();
}

/**
 * Formats info counts into a human-readable line.
 * Returns empty string if all counts are zero.
 * Otherwise: "-- Unprocessed: N | Active: M | Looping: K --"
 * Only sections with count > 0 are included, in fixed order.
 */
export function formatInfoLine(info: InfoCounts): string {
  if (info.unprocessed === 0 && info.active === 0 && info.looping === 0) {
    return '';
  }

  const parts: string[] = [];
  if (info.unprocessed > 0) parts.push(`Unprocessed: ${info.unprocessed}`);
  if (info.active > 0) parts.push(`Active: ${info.active}`);
  if (info.looping > 0) parts.push(`Looping: ${info.looping}`);

  return `-- ${parts.join(' | ')} --`;
}
