#!/usr/bin/env node

const childProcess = require('node:child_process');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const extensionRoot = path.resolve(__dirname, '..');
const packageJSONPath = path.join(extensionRoot, 'package.json');

function fail(message) {
  console.error(message);
  process.exit(1);
}

function argValue(name) {
  const index = process.argv.indexOf(name);
  if (index === -1) {
    return '';
  }
  if (index + 1 >= process.argv.length) {
    fail(`Missing value for ${name}.`);
  }
  return process.argv[index + 1];
}

function xmlEscape(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&apos;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;');
}

function copyFile(source, destination) {
  fs.mkdirSync(path.dirname(destination), { recursive: true });
  fs.copyFileSync(source, destination);
}

function copyDirectory(source, destination) {
  for (const entry of fs.readdirSync(source, { withFileTypes: true })) {
    const sourcePath = path.join(source, entry.name);
    const destinationPath = path.join(destination, entry.name);
    if (entry.isDirectory()) {
      copyDirectory(sourcePath, destinationPath);
      continue;
    }
    if (entry.isFile()) {
      copyFile(sourcePath, destinationPath);
    }
  }
}

function requireFile(relativePath) {
  const absolutePath = path.join(extensionRoot, relativePath);
  if (!fs.existsSync(absolutePath)) {
    fail(`Missing required extension file: ${relativePath}`);
  }
  return absolutePath;
}

function manifest(pkg) {
  const keywords = Array.isArray(pkg.keywords) ? pkg.keywords : [];
  const languageAliases = (pkg.contributes?.languages || [])
    .flatMap((language) => language.aliases || []);
  const extensions = (pkg.contributes?.languages || [])
    .flatMap((language) => language.extensions || [])
    .map((extension) => `__ext_${extension.replace(/^\./, '')}`);
  const tags = [...new Set([...keywords, 'snippet', ...languageAliases, ...extensions])];
  const repositoryURL = pkg.repository?.url || '';
  const supportURL = pkg.bugs?.url || repositoryURL;
  const homepageURL = pkg.homepage || repositoryURL;

  return `<?xml version="1.0" encoding="utf-8"?>
<PackageManifest Version="2.0.0" xmlns="http://schemas.microsoft.com/developer/vsx-schema/2011" xmlns:d="http://schemas.microsoft.com/developer/vsx-schema-design/2011">
  <Metadata>
    <Identity Language="en-US" Id="${xmlEscape(pkg.name)}" Version="${xmlEscape(pkg.version)}" Publisher="${xmlEscape(pkg.publisher)}" />
    <DisplayName>${xmlEscape(pkg.displayName || pkg.name)}</DisplayName>
    <Description xml:space="preserve">${xmlEscape(pkg.description || '')}</Description>
    <Tags>${xmlEscape(tags.join(','))}</Tags>
    <Categories>${xmlEscape((pkg.categories || []).join(','))}</Categories>
    <GalleryFlags>Public</GalleryFlags>
    <Properties>
      <Property Id="Microsoft.VisualStudio.Code.Engine" Value="${xmlEscape(pkg.engines?.vscode || '*')}" />
      <Property Id="Microsoft.VisualStudio.Code.ExtensionDependencies" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.ExtensionPack" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.ExtensionKind" Value="${xmlEscape((pkg.extensionKind || ['workspace']).join(','))}" />
      <Property Id="Microsoft.VisualStudio.Code.LocalizedLanguages" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.EnabledApiProposals" Value="" />
      <Property Id="Microsoft.VisualStudio.Code.ExecutesCode" Value="true" />
      <Property Id="Microsoft.VisualStudio.Services.Links.Source" Value="${xmlEscape(repositoryURL)}" />
      <Property Id="Microsoft.VisualStudio.Services.Links.Getstarted" Value="${xmlEscape(repositoryURL)}" />
      <Property Id="Microsoft.VisualStudio.Services.Links.GitHub" Value="${xmlEscape(repositoryURL)}" />
      <Property Id="Microsoft.VisualStudio.Services.Links.Support" Value="${xmlEscape(supportURL)}" />
      <Property Id="Microsoft.VisualStudio.Services.Links.Learn" Value="${xmlEscape(homepageURL)}" />
      <Property Id="Microsoft.VisualStudio.Services.GitHubFlavoredMarkdown" Value="true" />
      <Property Id="Microsoft.VisualStudio.Services.Content.Pricing" Value="Free"/>
    </Properties>
    <License>extension/LICENSE.md</License>
    <Icon>extension/${xmlEscape(pkg.icon || '')}</Icon>
  </Metadata>
  <Installation>
    <InstallationTarget Id="Microsoft.VisualStudio.Code"/>
  </Installation>
  <Dependencies/>
  <Assets>
    <Asset Type="Microsoft.VisualStudio.Code.Manifest" Path="extension/package.json" Addressable="true" />
    <Asset Type="Microsoft.VisualStudio.Services.Content.Details" Path="extension/readme.md" Addressable="true" />
    <Asset Type="Microsoft.VisualStudio.Services.Content.License" Path="extension/LICENSE.md" Addressable="true" />
    <Asset Type="Microsoft.VisualStudio.Services.Icons.Default" Path="extension/${xmlEscape(pkg.icon || '')}" Addressable="true" />
  </Assets>
</PackageManifest>
`;
}

function contentTypes() {
  return `<?xml version="1.0" encoding="utf-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension=".js" ContentType="application/javascript"/><Default Extension=".json" ContentType="application/json"/><Default Extension=".md" ContentType="text/markdown"/><Default Extension=".png" ContentType="image/png"/><Default Extension=".svg" ContentType="image/svg+xml"/><Default Extension=".vsixmanifest" ContentType="text/xml"/></Types>
`;
}

const pkg = JSON.parse(fs.readFileSync(packageJSONPath, 'utf8'));
if (!pkg.name || !pkg.version || !pkg.publisher) {
  fail('package.json must define name, version, and publisher.');
}

const outputPath = path.resolve(
  argValue('--output') || path.join(argValue('--out-dir') || extensionRoot, `${pkg.name}-${pkg.version}.vsix`)
);
const tempRoot = fs.mkdtempSync(path.join(os.tmpdir(), 'gowdk-vsix-'));
const extensionDirectory = path.join(tempRoot, 'extension');

try {
  copyFile(requireFile('package.json'), path.join(extensionDirectory, 'package.json'));
  copyFile(requireFile('extension.js'), path.join(extensionDirectory, 'extension.js'));
  copyFile(requireFile('extension-core.js'), path.join(extensionDirectory, 'extension-core.js'));
  copyFile(requireFile('routeHierarchy.js'), path.join(extensionDirectory, 'routeHierarchy.js'));
  copyFile(requireFile('language-configuration.json'), path.join(extensionDirectory, 'language-configuration.json'));
  copyFile(requireFile('LICENSE.md'), path.join(extensionDirectory, 'LICENSE.md'));
  copyFile(requireFile('README.md'), path.join(extensionDirectory, 'readme.md'));
  copyDirectory(requireFile('syntaxes'), path.join(extensionDirectory, 'syntaxes'));
  copyDirectory(requireFile('snippets'), path.join(extensionDirectory, 'snippets'));
  copyDirectory(requireFile('icons'), path.join(extensionDirectory, 'icons'));

  fs.writeFileSync(path.join(tempRoot, 'extension.vsixmanifest'), manifest(pkg));
  fs.writeFileSync(path.join(tempRoot, '[Content_Types].xml'), contentTypes());
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.rmSync(outputPath, { force: true });

  const result = childProcess.spawnSync(
    'zip',
    ['-X', '-r', outputPath, '[Content_Types].xml', 'extension.vsixmanifest', 'extension'],
    { cwd: tempRoot, encoding: 'utf8' }
  );
  if (result.status !== 0) {
    process.stderr.write(result.stderr || result.stdout);
    fail('Failed to create VSIX package. Ensure the zip command is installed.');
  }
  console.log(`Packaged ${path.relative(process.cwd(), outputPath)}`);
} finally {
  fs.rmSync(tempRoot, { recursive: true, force: true });
}
