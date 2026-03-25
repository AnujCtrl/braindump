import { randomBytes } from 'crypto';

/**
 * Generates an 8-character lowercase hex ID using 4 cryptographically random bytes.
 */
export function generateId(): string {
  return randomBytes(4).toString('hex');
}

/**
 * Checks whether an ID is unique (not present in the existing set).
 * The check is exact and case-sensitive.
 */
export function isUniqueId(id: string, existing: Set<string>): boolean {
  return !existing.has(id);
}
