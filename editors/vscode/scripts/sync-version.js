#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

const checkOnly = process.argv.includes('--check');
const repoRoot = path.resolve(__dirname, '..', '..', '..');
const cliVersionFile = path.join(repoRoot, 'cmd', 'gowdk', 'main.go');
const packageFile = path.join(repoRoot, 'editors', 'vscode', 'package.json');

const cliVersion = vscodeVersion(readCLIVersion(cliVersionFile));
const manifest = JSON.parse(fs.readFileSync(packageFile, 'utf8'));

if (manifest.version === cliVersion) {
  console.log(`VS Code extension version matches GOWDK ${cliVersion}.`);
  process.exit(0);
}

if (checkOnly) {
  console.error(`VS Code extension version ${manifest.version} does not match GOWDK ${cliVersion}.`);
  console.error('Run: node editors/vscode/scripts/sync-version.js');
  process.exit(1);
}

manifest.version = cliVersion;
fs.writeFileSync(packageFile, `${JSON.stringify(manifest, null, 2)}\n`);
console.log(`Updated VS Code extension version to ${cliVersion}.`);

function readCLIVersion(file) {
  const source = fs.readFileSync(file, 'utf8');
  const match = source.match(/\bversion\s*=\s*"([^"]+)"/);
  if (!match) {
    throw new Error(`Could not find CLI version in ${file}`);
  }
  return match[1];
}

function vscodeVersion(version) {
  const normalized = String(version || '').replace(/^v/, '');
  if (!/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/.test(normalized)) {
    throw new Error(`GOWDK version ${version} is not a valid VS Code extension version.`);
  }
  return normalized;
}
