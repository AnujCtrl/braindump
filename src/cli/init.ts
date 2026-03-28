import { existsSync, appendFileSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { homedir } from 'node:os';
import { input, confirm, select } from '@inquirer/prompts';
import { LinearClient } from '@linear/sdk';
import { Database } from 'bun:sqlite';
import { initDb } from '../core/db.js';
import { Store } from '../core/store.js';
import {
  resolveBraindumpHome,
  ensureHome,
  saveConfigFile,
  configExists,
} from '../config.js';

const DEFAULT_TAGS = [
  'braindump', 'homelab', 'work', 'health', 'errands',
  'learning', 'finance', 'minecraft', 'quick-win', 'deep-focus', 'low-energy',
];

export async function runInit(): Promise<void> {
  console.log('\nWelcome to braindump!\n');

  const home = resolveBraindumpHome();

  // Guard against re-init
  if (configExists(home)) {
    const overwrite = await confirm({
      message: `Config already exists at ${home}/config.yaml. Reconfigure?`,
      default: false,
    });
    if (!overwrite) {
      console.log('Keeping existing config.');
      return;
    }
  }

  ensureHome(home);
  console.log(`  Data directory: ${home}\n`);

  // Linear setup (optional)
  let linearApiKey: string | undefined;
  let linearTeamId: string | undefined;

  const wantLinear = await confirm({
    message: 'Connect to Linear for issue sync?',
    default: false,
  });

  if (wantLinear) {
    linearApiKey = await input({
      message: 'Linear API key:',
      validate: (v) => (v.trim() ? true : 'API key is required'),
    });

    try {
      const client = new LinearClient({ apiKey: linearApiKey.trim() });
      const teams = await client.teams();
      const teamNodes = teams.nodes;

      if (teamNodes.length === 0) {
        console.log('  No teams found in your Linear workspace.');
        linearApiKey = undefined;
      } else if (teamNodes.length === 1) {
        linearTeamId = teamNodes[0].id;
        console.log(`  ✓ Connected to Linear (team: ${teamNodes[0].name})\n`);
      } else {
        const choice = await select({
          message: 'Select your Linear team:',
          choices: teamNodes.map((t: any) => ({ name: t.name, value: t.id })),
        });
        linearTeamId = choice;
        const teamName = teamNodes.find((t: any) => t.id === choice)?.name;
        console.log(`  ✓ Connected to Linear (team: ${teamName})\n`);
      }
    } catch (err) {
      console.log(`  ✗ Failed to connect to Linear: ${err instanceof Error ? err.message : err}`);
      console.log('  Skipping Linear setup. You can configure it later.\n');
      linearApiKey = undefined;
      linearTeamId = undefined;
    }
  }

  // Save config
  saveConfigFile(home, {
    ...(linearApiKey ? { linearApiKey: linearApiKey.trim() } : {}),
    ...(linearTeamId ? { linearTeamId } : {}),
  });
  console.log('  ✓ Config saved\n');

  // Initialize database
  const dbPath = join(home, 'braindump.db');
  const db = new Database(dbPath);
  initDb(db);

  // Seed default tags if this is a fresh DB
  const store = new Store(db);
  const existingTags = store.allLabels();
  if (existingTags.length === 0) {
    for (const tag of DEFAULT_TAGS) {
      store.upsertLabel({ linearId: `local-${tag}`, name: tag, color: null });
    }
    console.log(`  ✓ Default tags created (${DEFAULT_TAGS.length} tags)\n`);
  }
  db.close();

  // Shell alias
  const wantAlias = await confirm({
    message: 'Set up a shell alias? (so you can type "todo" instead of "braindump")',
    default: true,
  });

  if (wantAlias) {
    const aliasName = await input({
      message: 'Alias name:',
      default: 'todo',
    });

    const shell = process.env.SHELL ?? '/bin/zsh';
    const rcFile = shell.includes('zsh')
      ? join(homedir(), '.zshrc')
      : join(homedir(), '.bashrc');

    const aliasLine = `alias ${aliasName}="braindump"`;

    // Check if alias already exists
    if (existsSync(rcFile)) {
      const content = readFileSync(rcFile, 'utf-8');
      if (content.includes(aliasLine)) {
        console.log(`  ✓ Alias already exists in ${rcFile}\n`);
      } else {
        appendFileSync(rcFile, `\n# braindump\n${aliasLine}\n`);
        console.log(`  ✓ Added alias to ${rcFile}\n`);
      }
    } else {
      appendFileSync(rcFile, `\n# braindump\n${aliasLine}\n`);
      console.log(`  ✓ Created ${rcFile} with alias\n`);
    }
  }

  // System service
  const wantService = await confirm({
    message: 'Install braindump as a background service? (for HTTP API + Linear sync)',
    default: true,
  });

  if (wantService) {
    try {
      const { installService } = await import('./service.js');
      await installService();
    } catch {
      console.log('  Service setup will be available after build. Run: braindump service install\n');
    }
  }

  // Summary
  console.log('You\'re all set! Try:');
  const alias = wantAlias ? 'todo' : 'braindump';
  console.log(`  ${alias} fix something #braindump`);
  console.log(`  ${alias} ls`);
  console.log('');
}
