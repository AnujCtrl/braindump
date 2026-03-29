import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { parse, stringify } from 'yaml';

export interface Config {
  home: string;
  linearApiKey: string | undefined;
  linearTeamId: string | undefined;
  braindumpDb: string;
  printerDevice: string | undefined;
  port: number;
  alias: string;
  syncIntervalMs: number;
}

interface ConfigFile {
  linearApiKey?: string;
  linearTeamId?: string;
  printerDevice?: string;
  port?: number;
  alias?: string;
  syncIntervalMs?: number;
}

export function resolveBraindumpHome(): string {
  if (process.env.BRAINDUMP_HOME) return process.env.BRAINDUMP_HOME;
  if (process.env.BRAINDUMP_DOCKER) return '/data';
  return join(homedir(), '.config', 'braindump');
}

export function ensureHome(home: string): void {
  if (!existsSync(home)) {
    mkdirSync(home, { recursive: true });
  }
}

export function loadConfigFile(home: string): ConfigFile {
  const configPath = join(home, 'config.yaml');
  if (!existsSync(configPath)) return {};
  const raw = readFileSync(configPath, 'utf-8');
  return (parse(raw) as ConfigFile) ?? {};
}

export function saveConfigFile(home: string, config: ConfigFile): void {
  ensureHome(home);
  const configPath = join(home, 'config.yaml');
  writeFileSync(configPath, stringify(config), 'utf-8');
}

export function configExists(home: string): boolean {
  return existsSync(join(home, 'config.yaml'));
}

function parsePort(raw: string | undefined): number | undefined {
  if (raw === undefined) return undefined;
  const port = parseInt(raw, 10);
  if (Number.isNaN(port)) throw new Error(`Invalid PORT: "${raw}"`);
  return port;
}

export function loadConfig(): Config {
  const home = resolveBraindumpHome();
  const file = loadConfigFile(home);

  return {
    home,
    linearApiKey: process.env.LINEAR_API_KEY ?? file.linearApiKey,
    linearTeamId: process.env.LINEAR_TEAM_ID ?? file.linearTeamId,
    braindumpDb: process.env.BRAINDUMP_DB ?? join(home, 'braindump.db'),
    printerDevice: process.env.PRINTER_DEVICE ?? file.printerDevice,
    port: parsePort(process.env.PORT) ?? file.port ?? 8080,
    alias: file.alias ?? 'todo',
    syncIntervalMs: file.syncIntervalMs ?? 30000,
  };
}
