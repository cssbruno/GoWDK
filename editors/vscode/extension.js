const vscode = require('vscode');
const childProcess = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');

const LANGUAGE_ID = 'gwdk';

function activate(context) {
  const diagnostics = vscode.languages.createDiagnosticCollection('gowdk');
  const pending = new Map();
  const siteMapTree = new SiteMapTreeProvider();

  context.subscriptions.push(diagnostics);
  context.subscriptions.push(vscode.window.registerTreeDataProvider('gowdk.siteMapTree', siteMapTree));

  context.subscriptions.push(vscode.workspace.onDidOpenTextDocument((doc) => validateSoon(doc, diagnostics, pending)));
  context.subscriptions.push(vscode.workspace.onDidChangeTextDocument((event) => validateSoon(event.document, diagnostics, pending)));
  context.subscriptions.push(vscode.workspace.onDidSaveTextDocument((doc) => validateNow(doc, diagnostics)));
  context.subscriptions.push(vscode.workspace.onDidCloseTextDocument((doc) => diagnostics.delete(doc.uri)));
  context.subscriptions.push(vscode.workspace.onDidSaveTextDocument((doc) => {
    if (doc.languageId === LANGUAGE_ID) {
      siteMapTree.refresh();
    }
  }));

  const watcher = vscode.workspace.createFileSystemWatcher('**/*.gwdk');
  context.subscriptions.push(watcher);
  context.subscriptions.push(watcher.onDidCreate(() => siteMapTree.refresh()));
  context.subscriptions.push(watcher.onDidDelete(() => siteMapTree.refresh()));
  context.subscriptions.push(watcher.onDidChange(() => siteMapTree.refresh()));

  context.subscriptions.push(vscode.languages.registerDocumentFormattingEditProvider(LANGUAGE_ID, {
    provideDocumentFormattingEdits(document) {
      if (!config().get('enableFormatting')) {
        return [];
      }
      return withDocumentFile(document, (file) => runGowdk(['fmt', file], document).then(({ stdout }) => {
        const lastLine = document.lineAt(document.lineCount - 1);
        const fullRange = new vscode.Range(0, 0, document.lineCount - 1, lastLine.text.length);
        return [vscode.TextEdit.replace(fullRange, stdout)];
      }).catch((error) => {
        vscode.window.showErrorMessage(`GOWDK format failed: ${error.message}`);
        return [];
      }));
    }
  }));

  context.subscriptions.push(vscode.languages.registerCompletionItemProvider(LANGUAGE_ID, {
    provideCompletionItems() {
      return completions();
    }
  }, '@'));

  context.subscriptions.push(vscode.commands.registerCommand('gowdk.checkCurrentFile', async () => {
    const document = activeGWDKDocument();
    if (document) {
      await validateNow(document, diagnostics);
    }
  }));

  context.subscriptions.push(vscode.commands.registerCommand('gowdk.showManifest', async () => {
    const document = activeGWDKDocument();
    if (!document) {
      return;
    }
    try {
      const output = await withDocumentFile(document, (file) => {
        const args = ['manifest'];
        if (config().get('enableSsrAddon')) {
          args.push('--ssr');
        }
        args.push(file);
        return runGowdk(args, document).then((result) => result.stdout);
      });
      const manifest = await vscode.workspace.openTextDocument({ content: output, language: 'json' });
      await vscode.window.showTextDocument(manifest, { preview: true });
    } catch (error) {
      vscode.window.showErrorMessage(`GOWDK manifest failed: ${error.message}`);
    }
  }));

  context.subscriptions.push(vscode.commands.registerCommand('gowdk.showTokens', async () => {
    const document = activeGWDKDocument();
    if (!document) {
      return;
    }
    try {
      const output = await withDocumentFile(document, (file) => runGowdk(['tokens', file], document).then((result) => result.stdout));
      const tokenDoc = await vscode.workspace.openTextDocument({ content: output, language: 'plaintext' });
      await vscode.window.showTextDocument(tokenDoc, { preview: true });
    } catch (error) {
      vscode.window.showErrorMessage(`GOWDK tokenization failed: ${error.message}`);
    }
  }));

  context.subscriptions.push(vscode.commands.registerCommand('gowdk.showSiteMap', async () => {
    await showSiteMap(context);
  }));

  context.subscriptions.push(vscode.commands.registerCommand('gowdk.refreshSiteMapTree', () => {
    siteMapTree.refresh();
  }));

  context.subscriptions.push(vscode.commands.registerCommand('gowdk.openPageFile', async (item) => {
    if (item && item.page && item.page.source) {
      await openFile(item.page.source);
    }
  }));

  context.subscriptions.push(vscode.commands.registerCommand('gowdk.movePageFile', async (item) => {
    if (item && item.page && item.page.source) {
      await moveFile(item.page.source);
      siteMapTree.refresh();
    }
  }));

  for (const document of vscode.workspace.textDocuments) {
    validateSoon(document, diagnostics, pending);
  }
}

function deactivate() {}

function validateSoon(document, diagnostics, pending) {
  if (document.languageId !== LANGUAGE_ID || !config().get('enableDiagnostics')) {
    return;
  }
  const existing = pending.get(document.uri.toString());
  if (existing) {
    clearTimeout(existing);
  }
  pending.set(document.uri.toString(), setTimeout(() => {
    pending.delete(document.uri.toString());
    validateNow(document, diagnostics);
  }, 300));
}

async function validateNow(document, diagnostics) {
  if (document.languageId !== LANGUAGE_ID || !config().get('enableDiagnostics')) {
    return;
  }
  try {
    const report = await withDocumentFile(document, (file) => {
      const args = ['check', '--json'];
      if (config().get('enableSsrAddon')) {
        args.push('--ssr');
      }
      args.push(file);
      return runGowdk(args, document).then(({ stdout }) => parseDiagnostics(stdout));
    });
    diagnostics.set(document.uri, report.map(toVSCodeDiagnostic));
  } catch (error) {
    const diagnostic = new vscode.Diagnostic(new vscode.Range(0, 0, 0, 1), error.message, vscode.DiagnosticSeverity.Error);
    diagnostic.source = 'gowdk';
    diagnostics.set(document.uri, [diagnostic]);
  }
}

function parseDiagnostics(stdout) {
  if (!stdout.trim()) {
    return [];
  }
  const parsed = JSON.parse(stdout);
  return parsed.diagnostics || [];
}

function toVSCodeDiagnostic(item) {
  const line = Math.max((item.pos && item.pos.line ? item.pos.line : 1) - 1, 0);
  const column = Math.max((item.pos && item.pos.column ? item.pos.column : 1) - 1, 0);
  const severity = item.severity === 'warning'
    ? vscode.DiagnosticSeverity.Warning
    : vscode.DiagnosticSeverity.Error;
  const diagnostic = new vscode.Diagnostic(new vscode.Range(line, column, line, column + 1), item.message, severity);
  diagnostic.source = 'gowdk';
  return diagnostic;
}

async function withDocumentFile(document, callback) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gowdk-vscode-'));
  const file = path.join(dir, path.basename(document.fileName || 'untitled.gwdk').replace(/[^A-Za-z0-9_.-]/g, '_') || 'document.gwdk');
  fs.writeFileSync(file, document.getText(), 'utf8');
  try {
    return await callback(file);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

function runGowdk(args, document) {
  const invocation = toolInvocation(args, document);
  return new Promise((resolve, reject) => {
    childProcess.execFile(invocation.command, invocation.args, {
      cwd: invocation.cwd,
      timeout: 10000,
      maxBuffer: 1024 * 1024
    }, (error, stdout, stderr) => {
      if (error) {
        error.message = stderr.trim() || stdout.trim() || error.message;
        if (stdout.trim() && args.includes('--json')) {
          resolve({ stdout, stderr });
          return;
        }
        reject(error);
        return;
      }
      resolve({ stdout, stderr });
    });
  });
}

async function showSiteMap(context) {
  const root = workspaceRoot();
  if (!root) {
    vscode.window.showWarningMessage('Open a workspace to show the GOWDK site map.');
    return;
  }
  const uris = await vscode.workspace.findFiles('**/*.gwdk', '**/{.git,node_modules}/**');
  if (uris.length === 0) {
    vscode.window.showInformationMessage('No .gwdk files found in this workspace.');
    return;
  }

  try {
    const siteMap = await loadSiteMap();
    const panel = vscode.window.createWebviewPanel('gowdkSiteMap', 'GOWDK Site Map', vscode.ViewColumn.Beside, {
      enableScripts: true,
      retainContextWhenHidden: true
    });
    panel.webview.html = siteMapHTML(siteMap, root);
    panel.webview.onDidReceiveMessage(async (message) => {
      if (message.type === 'open') {
        await openFile(message.file);
      }
      if (message.type === 'move') {
        await moveFile(message.file);
        await refreshSiteMap(panel, root);
      }
      if (message.type === 'refresh') {
        await refreshSiteMap(panel, root);
      }
    }, undefined, context.subscriptions);
  } catch (error) {
    vscode.window.showErrorMessage(`GOWDK site map failed: ${error.message}`);
  }
}

async function refreshSiteMap(panel, root) {
  panel.webview.html = siteMapHTML(await loadSiteMap(), root);
}

async function loadSiteMap() {
  const uris = await vscode.workspace.findFiles('**/*.gwdk', '**/{.git,node_modules}/**');
  if (uris.length === 0) {
    return { pages: [] };
  }
  const { stdout } = await runGowdk(['sitemap', ...uris.map((uri) => uri.fsPath)], undefined);
  return JSON.parse(stdout);
}

async function openFile(file) {
  const document = await vscode.workspace.openTextDocument(vscode.Uri.file(file));
  await vscode.window.showTextDocument(document, { preview: false });
}

async function moveFile(file) {
  const current = vscode.Uri.file(file);
  const target = await vscode.window.showSaveDialog({
    defaultUri: current,
    filters: {
      'GOWDK files': ['gwdk']
    },
    saveLabel: 'Move Page File'
  });
  if (!target || target.fsPath === current.fsPath) {
    return;
  }
  await vscode.workspace.fs.createDirectory(vscode.Uri.file(path.dirname(target.fsPath)));
  await vscode.workspace.fs.rename(current, target, { overwrite: false });
  vscode.window.showInformationMessage(`Moved ${path.basename(current.fsPath)} to ${target.fsPath}. Route declarations stayed inside the file.`);
}

function siteMapHTML(siteMap, root) {
  const pages = (siteMap.pages || []).slice().sort((a, b) => a.route.localeCompare(b.route));
  const routes = pages.map((page) => pageCard(page, root)).join('');
  const staticCount = pages.filter((page) => page.render === 'static').length;
  const ssrCount = pages.filter((page) => page.render === 'ssr').length;
  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>GOWDK Site Map</title>
  <style>
    :root {
      color-scheme: light dark;
      font-family: var(--vscode-font-family);
      color: var(--vscode-foreground);
      background: var(--vscode-editor-background);
    }
    body {
      margin: 0;
      padding: 20px;
    }
    header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      margin-bottom: 18px;
    }
    h1 {
      font-size: 20px;
      margin: 0 0 4px;
      font-weight: 650;
    }
    .summary {
      color: var(--vscode-descriptionForeground);
      font-size: 12px;
    }
    button {
      appearance: none;
      border: 1px solid var(--vscode-button-border, transparent);
      background: var(--vscode-button-background);
      color: var(--vscode-button-foreground);
      border-radius: 4px;
      padding: 5px 10px;
      cursor: pointer;
    }
    button.secondary {
      background: var(--vscode-button-secondaryBackground);
      color: var(--vscode-button-secondaryForeground);
    }
    .grid {
      display: grid;
      gap: 10px;
    }
    .page {
      border: 1px solid var(--vscode-panel-border);
      border-radius: 6px;
      padding: 12px;
      background: var(--vscode-editorWidget-background);
    }
    .route {
      font-size: 15px;
      font-weight: 650;
    }
    .meta {
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      margin: 8px 0;
    }
    .pill {
      border: 1px solid var(--vscode-badge-background);
      border-radius: 999px;
      padding: 2px 7px;
      font-size: 11px;
      color: var(--vscode-badge-foreground);
      background: var(--vscode-badge-background);
    }
    .file {
      color: var(--vscode-descriptionForeground);
      font-size: 12px;
      word-break: break-all;
    }
    .actions {
      display: flex;
      gap: 8px;
      margin-top: 10px;
    }
  </style>
</head>
<body>
  <header>
    <div>
      <h1>GOWDK Site Map</h1>
      <div class="summary">${pages.length} pages · ${staticCount} static · ${ssrCount} ssr</div>
    </div>
    <button id="refresh">Refresh</button>
  </header>
  <main class="grid">
    ${routes || '<p>No pages found.</p>'}
  </main>
  <script>
    const vscode = acquireVsCodeApi();
    document.getElementById('refresh').addEventListener('click', () => vscode.postMessage({ type: 'refresh' }));
    document.querySelectorAll('[data-open]').forEach((button) => {
      button.addEventListener('click', () => vscode.postMessage({ type: 'open', file: button.dataset.open }));
    });
    document.querySelectorAll('[data-move]').forEach((button) => {
      button.addEventListener('click', () => vscode.postMessage({ type: 'move', file: button.dataset.move }));
    });
  </script>
</body>
</html>`;
}

function pageCard(page, root) {
  const rel = path.relative(root, page.source || '').replace(/\\/g, '/');
  const blocks = Object.entries(page.blocks || {})
    .filter(([key, value]) => key !== 'actions' && key !== 'apis' && value)
    .map(([key]) => key);
  const actions = (page.blocks && page.blocks.actions) || [];
  const apis = (page.blocks && page.blocks.apis) || [];
  const tags = [page.render, ...blocks, ...actions.map((name) => `act:${name}`), ...apis.map((name) => `api:${name}`), ...(page.layouts || []).map((layout) => `layout:${layout}`)];
  return `<section class="page">
    <div class="route">${escapeHTML(page.route || '(missing route)')}</div>
    <div class="meta">${tags.map((tag) => `<span class="pill">${escapeHTML(tag)}</span>`).join('')}</div>
    <div class="file">${escapeHTML(page.id)} · ${escapeHTML(rel || page.source || '')}</div>
    <div class="actions">
      <button data-open="${escapeAttr(page.source)}">Open</button>
      <button class="secondary" data-move="${escapeAttr(page.source)}">Move File</button>
    </div>
  </section>`;
}

function escapeHTML(value) {
  return String(value || '').replace(/[&<>"']/g, (char) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[char]));
}

function escapeAttr(value) {
  return escapeHTML(value);
}

class SiteMapTreeProvider {
  constructor() {
    this._onDidChangeTreeData = new vscode.EventEmitter();
    this.onDidChangeTreeData = this._onDidChangeTreeData.event;
  }

  refresh() {
    this._onDidChangeTreeData.fire();
  }

  getTreeItem(item) {
    return item;
  }

  async getChildren(item) {
    if (item && item.children) {
      return item.children;
    }
    try {
      const root = workspaceRoot();
      const siteMap = await loadSiteMap();
      const pages = siteMap.pages || [];
      if (pages.length === 0) {
        const empty = new vscode.TreeItem('No .gwdk pages found', vscode.TreeItemCollapsibleState.None);
        empty.iconPath = new vscode.ThemeIcon('info');
        return [empty];
      }
      return pages
        .slice()
        .sort((a, b) => a.route.localeCompare(b.route))
        .map((page) => new SiteMapPageItem(page, root));
    } catch (error) {
      const fallback = new vscode.TreeItem(`Site map unavailable: ${error.message}`, vscode.TreeItemCollapsibleState.None);
      fallback.iconPath = new vscode.ThemeIcon('warning');
      return [fallback];
    }
  }
}

class SiteMapPageItem extends vscode.TreeItem {
  constructor(page, root) {
    super(page.route || '(missing route)', vscode.TreeItemCollapsibleState.Collapsed);
    this.page = page;
    this.contextValue = 'gwdkPage';
    this.description = page.render || 'static';
    this.tooltip = `${page.id}\n${page.source}`;
    this.iconPath = new vscode.ThemeIcon(page.render === 'ssr' ? 'server' : 'globe');
    this.command = {
      command: 'gowdk.openPageFile',
      title: 'Open Page File',
      arguments: [this]
    };
    this.children = pageChildren(page, root);
  }
}

function pageChildren(page, root) {
  const children = [];
  const rel = path.relative(root, page.source || '').replace(/\\/g, '/');
  children.push(infoItem(`File: ${rel || page.source || '(unknown)'}`, 'file'));
  children.push(infoItem(`Page: ${page.id || '(missing id)'}`, 'symbol-method'));
  if (page.layouts && page.layouts.length) {
    children.push(infoItem(`Layouts: ${page.layouts.join(', ')}`, 'layers'));
  }
  if (page.guard && page.guard.length) {
    children.push(infoItem(`Guards: ${page.guard.join(', ')}`, 'shield'));
  }
  const blocks = [];
  if (page.blocks) {
    for (const key of ['paths', 'build', 'load', 'view']) {
      if (page.blocks[key]) {
        blocks.push(key);
      }
    }
    for (const action of page.blocks.actions || []) {
      blocks.push(`act:${action}`);
    }
    for (const api of page.blocks.apis || []) {
      blocks.push(api ? `api:${api}` : 'api');
    }
  }
  if (blocks.length) {
    children.push(infoItem(`Blocks: ${blocks.join(', ')}`, 'list-tree'));
  }
  if (page.dynamicParams && page.dynamicParams.length) {
    children.push(infoItem(`Params: ${page.dynamicParams.join(', ')}`, 'symbol-parameter'));
  }
  return children;
}

function infoItem(label, icon) {
  const item = new vscode.TreeItem(label, vscode.TreeItemCollapsibleState.None);
  item.iconPath = new vscode.ThemeIcon(icon);
  return item;
}

function toolInvocation(args, document) {
  const cliPath = config().get('cliPath');
  const cwd = workspaceRoot(document);
  if (cliPath) {
    return { command: cliPath, args, cwd };
  }
  if (cwd && fs.existsSync(path.join(cwd, 'cmd', 'gowdk')) && fs.existsSync(path.join(cwd, 'go.mod'))) {
    return { command: 'go', args: ['run', './cmd/gowdk', ...args], cwd };
  }
  return { command: 'gowdk', args, cwd };
}

function workspaceRoot(document) {
  if (!document) {
    const folder = vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders[0];
    return folder ? folder.uri.fsPath : process.cwd();
  }
  const folder = vscode.workspace.getWorkspaceFolder(document.uri);
  return folder ? folder.uri.fsPath : process.cwd();
}

function activeGWDKDocument() {
  const editor = vscode.window.activeTextEditor;
  if (!editor || editor.document.languageId !== LANGUAGE_ID) {
    vscode.window.showWarningMessage('Open a .gwdk file first.');
    return undefined;
  }
  return editor.document;
}

function completions() {
  const entries = [
    ['@page', 'Declare the page id.'],
    ['@route', 'Declare the route path.'],
    ['@layout', 'Declare one or more layout ids.'],
    ['@render', 'Declare render mode: static, action, hybrid, or ssr.'],
    ['@guard', 'Declare route guards.'],
    ['static', 'Build-time HTML render mode.'],
    ['action', 'Static page with backend actions.'],
    ['hybrid', 'Static by default with selected request-time behavior.'],
    ['ssr', 'Request-time full-page rendering through the SSR addon.'],
    ['paths', 'Build-time dynamic route path block.'],
    ['build', 'Build-time data block.'],
    ['load', 'Request-time data block.'],
    ['act', 'Action block for POST/form behavior.'],
    ['api', 'API handler block.'],
    ['view', 'Markup render block.'],
    ['g:post', 'Bind a form to an action.'],
    ['g:target', 'Select partial update target.'],
    ['g:swap', 'Select partial update swap behavior.']
  ];
  return entries.map(([label, detail]) => {
    const item = new vscode.CompletionItem(label, vscode.CompletionItemKind.Keyword);
    item.detail = detail;
    return item;
  });
}

function config() {
  return vscode.workspace.getConfiguration('gowdk');
}

module.exports = {
  activate,
  deactivate
};
