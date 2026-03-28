import Fastify from 'fastify';
import { LinearClient } from '@linear/sdk';
import { bootstrap } from '../bootstrap.js';
import { SyncEngine } from '../core/sync.js';
import { LinearBridge } from '../core/linear.js';
import { Handlers } from './handlers.js';
import { registerRoutes } from './routes.js';

const { config, db, store } = bootstrap();

// Set up Linear sync if API key is configured
let syncEngine: SyncEngine | null = null;
if (config.linearApiKey && config.linearTeamId) {
  const client = new LinearClient({ apiKey: config.linearApiKey });
  const bridge = new LinearBridge(client, config.linearTeamId);
  syncEngine = new SyncEngine(store, bridge);
}

// No-op sync engine for handlers when Linear is not configured
const syncForHandlers = syncEngine ?? { drainQueue: async () => {} };
const handlers = new Handlers(store, syncForHandlers as any);

const app = Fastify({ logger: true });
registerRoutes(app, handlers);

// Periodic sync drain
let syncInterval: ReturnType<typeof setInterval> | null = null;
if (syncEngine) {
  syncInterval = setInterval(async () => {
    try {
      await syncEngine!.drainQueue();
    } catch (err) {
      app.log.error(err, 'Sync drain failed');
    }
  }, config.syncIntervalMs);
}

// Graceful shutdown
async function shutdown() {
  if (syncInterval) clearInterval(syncInterval);
  await app.close();
  db.close();
  process.exit(0);
}

process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);

// Start server
app.listen({ port: config.port, host: '0.0.0.0' }, (err, address) => {
  if (err) {
    app.log.error(err);
    process.exit(1);
  }
  app.log.info(`Server listening on ${address}`);
  if (syncEngine) {
    app.log.info(`Linear sync enabled (every ${config.syncIntervalMs / 1000}s)`);
  }
});
