import { writeFileSync, renameSync, unlinkSync, chmodSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const REPO = 'anujctrl/braindump';
const VERSION = '2.0.0'; // Overridden at compile time via --define

function getBinaryPath(): string {
  return typeof Bun !== 'undefined' ? Bun.execPath : process.execPath;
}

function isHomebrew(): boolean {
  const path = getBinaryPath();
  return path.includes('/homebrew/') || path.includes('/Cellar/');
}

function getAssetName(): string {
  const os = process.platform === 'darwin' ? 'darwin' : 'linux';
  const arch = process.arch === 'arm64' ? 'arm64' : 'x64';
  return `braindump-${os}-${arch}`;
}

export async function runUpdate(): Promise<void> {
  if (isHomebrew()) {
    console.log('Installed via Homebrew. Run: brew upgrade braindump');
    return;
  }

  console.log(`Current version: ${VERSION}`);
  console.log('Checking for updates...');

  const res = await fetch(`https://api.github.com/repos/${REPO}/releases/latest`);
  if (!res.ok) {
    console.error(`Failed to check for updates: ${res.status} ${res.statusText}`);
    return;
  }

  const release = await res.json() as {
    tag_name: string;
    assets: Array<{ name: string; browser_download_url: string }>;
  };

  const latest = release.tag_name.replace(/^v/, '');
  if (latest === VERSION) {
    console.log('Already up to date.');
    return;
  }

  console.log(`New version available: ${latest}`);

  const assetName = getAssetName();
  const asset = release.assets.find((a) => a.name === assetName);
  if (!asset) {
    console.error(`No binary found for your platform (${assetName})`);
    console.error('Available assets:', release.assets.map((a) => a.name).join(', '));
    return;
  }

  console.log(`Downloading ${assetName}...`);
  const downloadRes = await fetch(asset.browser_download_url);
  if (!downloadRes.ok) {
    console.error(`Download failed: ${downloadRes.status}`);
    return;
  }

  const tmpPath = join(tmpdir(), `braindump-update-${Date.now()}`);
  const buffer = await downloadRes.arrayBuffer();
  writeFileSync(tmpPath, Buffer.from(buffer));
  chmodSync(tmpPath, 0o755);

  const binPath = getBinaryPath();
  const backupPath = `${binPath}.bak`;

  try {
    renameSync(binPath, backupPath);
    renameSync(tmpPath, binPath);
    unlinkSync(backupPath);
    console.log(`Updated to v${latest}`);
  } catch (err) {
    // Restore backup on failure
    try { renameSync(backupPath, binPath); } catch {}
    try { unlinkSync(tmpPath); } catch {}
    console.error(`Update failed: ${err instanceof Error ? err.message : err}`);
  }
}
