const { execFileSync } = require('node:child_process');
const fs = require('node:fs');
const path = require('node:path');

const extensionRoot = path.resolve(__dirname, '..');
const manifest = require(path.join(extensionRoot, 'package.json'));
const output = path.join(extensionRoot, `${manifest.name}-${manifest.version}.vsix`);

for (const file of fs.readdirSync(extensionRoot)) {
  if (file.endsWith('.vsix')) {
    fs.rmSync(path.join(extensionRoot, file), { force: true });
  }
}

const args = ['--no-install', 'vsce', 'package', '--no-dependencies'];
const repository = process.env.GITHUB_REPOSITORY;
const refName = process.env.GITHUB_REF_NAME;

if (repository && refName) {
  args.push(
    '--baseContentUrl',
    `https://github.com/${repository}/blob/${refName}`,
    '--baseImagesUrl',
    `https://github.com/${repository}/raw/${refName}`,
  );
}

execFileSync('npx', args, { cwd: extensionRoot, stdio: 'inherit' });

if (!fs.existsSync(output)) {
  throw new Error(`Expected VS Code package ${path.basename(output)} was not created.`);
}

console.log(`Created ${path.basename(output)}.`);
