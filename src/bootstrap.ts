import { Database } from 'bun:sqlite';
import { initDb } from './core/db.js';
import { Store } from './core/store.js';
import { loadConfig, ensureHome, type Config } from './config.js';

export interface AppContext {
  config: Config;
  db: Database;
  store: Store;
}

export function bootstrap(): AppContext {
  const config = loadConfig();
  ensureHome(config.home);
  const db = new Database(config.braindumpDb);
  initDb(db);
  const store = new Store(db);
  return { config, db, store };
}
