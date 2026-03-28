#!/usr/bin/env bun
import { Command } from 'commander';
import { bootstrap } from '../bootstrap.js';
import { configExists, resolveBraindumpHome } from '../config.js';
import { parseCapture } from '../core/parser.js';
import { handleCapture } from './capture.js';
import { handleList, formatTodoLine } from './list.js';
import { handleDone } from './done.js';
import { getInfoLine, formatInfoLine } from '../core/info.js';

const program = new Command();

program
  .name('braindump')
  .description('Fast, frictionless todo capture')
  .version('2.0.0');

// Guard: require init for most commands
function requireInit(): void {
  const home = resolveBraindumpHome();
  if (!configExists(home)) {
    console.error('braindump is not set up yet. Run: braindump init');
    process.exit(1);
  }
}

function printInfo(): void {
  const { store } = bootstrap();
  const info = getInfoLine(store);
  const line = formatInfoLine(info);
  if (line) console.log(line);
}

// Default command: capture
program
  .argument('[text...]', 'todo text with capture syntax')
  .option('--source <source>', 'override source')
  .action((textParts: string[], opts: { source?: string }) => {
    if (textParts.length === 0) {
      program.help();
      return;
    }
    requireInit();
    const { store } = bootstrap();
    const raw = textParts.join(' ');
    const parsed = parseCapture(raw);
    const todo = handleCapture(store, {
      text: parsed.text,
      tags: parsed.tags.length > 0 ? parsed.tags : undefined,
      source: opts.source ?? parsed.source ?? undefined,
      urgent: parsed.urgent,
      important: parsed.important,
      notes: parsed.notes,
    });
    console.log(`Created: ${todo.text} [${todo.id}]`);
    printInfo();
  });

// ls
program
  .command('ls')
  .description('List todos')
  .option('-t, --tag <tag>', 'filter by tag')
  .option('-s, --status <status>', 'filter by status')
  .option('-a, --all', 'include done todos')
  .option('-l, --looping', 'show looping todos (stale 2+ times)')
  .action((opts) => {
    requireInit();
    const { store } = bootstrap();
    const todos = handleList(store, opts);
    for (const todo of todos) {
      console.log(formatTodoLine(todo));
    }
    printInfo();
  });

// done
program
  .command('done <id>')
  .description('Mark a todo as done')
  .action((id: string) => {
    requireInit();
    const { store } = bootstrap();
    handleDone(store, id);
    console.log(`Done: ${id}`);
    printInfo();
  });

// dump
program
  .command('dump')
  .description('Brain dump mode — rapid multi-line capture')
  .action(async () => {
    requireInit();
    const { store } = bootstrap();
    console.log('Brain dump mode. Enter todos one per line. Empty line to finish.');
    const reader = require('readline').createInterface({
      input: process.stdin,
      output: process.stdout,
      prompt: '> ',
    });
    const items: string[] = [];
    reader.prompt();
    for await (const line of reader) {
      const trimmed = (line as string).trim();
      if (!trimmed) break;
      items.push(trimmed);
      reader.prompt();
    }
    if (items.length === 0) {
      console.log('No items captured.');
      return;
    }
    for (const item of items) {
      const parsed = parseCapture(item);
      handleCapture(store, {
        text: parsed.text,
        tags: parsed.tags.length > 0 ? parsed.tags : undefined,
        source: parsed.source ?? undefined,
        urgent: parsed.urgent,
        important: parsed.important,
        notes: parsed.notes,
      });
    }
    console.log(`Created ${items.length} todos.`);
    printInfo();
  });

// init (placeholder — Phase 5 implements this fully)
program
  .command('init')
  .description('Set up braindump')
  .action(async () => {
    const { runInit } = await import('./init.js');
    await runInit();
  });

// server
program
  .command('server')
  .description('Start the HTTP API server')
  .action(async () => {
    requireInit();
    // Dynamic import to avoid loading Fastify for CLI commands
    await import('../server/index.js');
  });

// service
const service = program.command('service').description('Manage background service');
service
  .command('install')
  .description('Install the background service')
  .action(async () => {
    const { installService } = await import('./service.js');
    await installService();
  });
service
  .command('start')
  .description('Start the background service')
  .action(async () => {
    const { startService } = await import('./service.js');
    await startService();
  });
service
  .command('stop')
  .description('Stop the background service')
  .action(async () => {
    const { stopService } = await import('./service.js');
    await stopService();
  });
service
  .command('status')
  .description('Check background service status')
  .action(async () => {
    const { serviceStatus } = await import('./service.js');
    await serviceStatus();
  });
service
  .command('uninstall')
  .description('Remove the background service')
  .action(async () => {
    const { uninstallService } = await import('./service.js');
    await uninstallService();
  });

// update
program
  .command('update')
  .description('Update braindump to the latest version')
  .action(async () => {
    const { runUpdate } = await import('./update.js');
    await runUpdate();
  });

program.parse();
