import { existsSync, mkdirSync, writeFileSync, unlinkSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { homedir, platform } from 'node:os';
import { execSync } from 'node:child_process';
import { resolveBraindumpHome } from '../config.js';

const SERVICE_LABEL = 'com.braindump.server';

function getBinaryPath(): string {
  // Try to find the installed 'braindump' command first
  try {
    const which = execSync('which braindump', { encoding: 'utf-8' }).trim();
    if (which) return which;
  } catch {}
  // Fall back to Bun.execPath (compiled binary) or process.execPath
  return typeof Bun !== 'undefined' ? Bun.execPath : process.execPath;
}

function isHomebrew(): boolean {
  return getBinaryPath().includes('/homebrew/') || getBinaryPath().includes('/Cellar/');
}

// --- macOS (launchd) ---

function plistPath(): string {
  return join(homedir(), 'Library', 'LaunchAgents', `${SERVICE_LABEL}.plist`);
}

function generatePlist(): string {
  const bin = getBinaryPath();
  const home = resolveBraindumpHome();
  const logPath = join(home, 'braindump.log');

  return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>${SERVICE_LABEL}</string>
  <key>ProgramArguments</key>
  <array>
    <string>${bin}</string>
    <string>server</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>${logPath}</string>
  <key>StandardErrorPath</key>
  <string>${logPath}</string>
  <key>EnvironmentVariables</key>
  <dict>
    <key>BRAINDUMP_HOME</key>
    <string>${home}</string>
    <key>PATH</key>
    <string>${process.env.PATH}</string>
  </dict>
</dict>
</plist>`;
}

// --- Linux (systemd) ---

function unitPath(): string {
  return join(homedir(), '.config', 'systemd', 'user', 'braindump.service');
}

function generateUnit(): string {
  const bin = getBinaryPath();
  const home = resolveBraindumpHome();

  return `[Unit]
Description=Braindump todo sync service
After=network.target

[Service]
Type=simple
ExecStart=${bin} server
Restart=on-failure
Environment=BRAINDUMP_HOME=${home}

[Install]
WantedBy=default.target`;
}

// --- Public API ---

export async function installService(): Promise<void> {
  if (isHomebrew()) {
    console.log('  Installed via Homebrew. Use: brew services start braindump');
    return;
  }

  const os = platform();

  if (os === 'darwin') {
    const path = plistPath();
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, generatePlist());
    execSync(`launchctl load ${path}`);
    console.log('  ✓ Background service installed and started (launchd)');
  } else if (os === 'linux') {
    const path = unitPath();
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, generateUnit());
    execSync('systemctl --user daemon-reload');
    execSync('systemctl --user enable braindump');
    execSync('systemctl --user start braindump');
    console.log('  ✓ Background service installed and started (systemd)');
  } else {
    console.log(`  Service management not supported on ${os}. Run manually: braindump server`);
  }
}

export async function startService(): Promise<void> {
  const os = platform();
  if (os === 'darwin') {
    execSync(`launchctl load ${plistPath()}`);
    console.log('Service started.');
  } else if (os === 'linux') {
    execSync('systemctl --user start braindump');
    console.log('Service started.');
  }
}

export async function stopService(): Promise<void> {
  const os = platform();
  if (os === 'darwin') {
    execSync(`launchctl unload ${plistPath()}`);
    console.log('Service stopped.');
  } else if (os === 'linux') {
    execSync('systemctl --user stop braindump');
    console.log('Service stopped.');
  }
}

export async function serviceStatus(): Promise<void> {
  const os = platform();
  if (os === 'darwin') {
    try {
      const result = execSync(`launchctl list ${SERVICE_LABEL} 2>&1`, { encoding: 'utf-8' });
      console.log('Service is running.');
      console.log(result);
    } catch {
      console.log('Service is not running.');
    }
  } else if (os === 'linux') {
    try {
      const result = execSync('systemctl --user status braindump 2>&1', { encoding: 'utf-8' });
      console.log(result);
    } catch {
      console.log('Service is not running.');
    }
  }
}

export async function uninstallService(): Promise<void> {
  const os = platform();
  if (os === 'darwin') {
    const path = plistPath();
    try { execSync(`launchctl unload ${path}`); } catch {}
    if (existsSync(path)) unlinkSync(path);
    console.log('Service uninstalled.');
  } else if (os === 'linux') {
    try { execSync('systemctl --user stop braindump'); } catch {}
    try { execSync('systemctl --user disable braindump'); } catch {}
    const path = unitPath();
    if (existsSync(path)) unlinkSync(path);
    execSync('systemctl --user daemon-reload');
    console.log('Service uninstalled.');
  }
}
