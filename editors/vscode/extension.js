const vscode = require('vscode');
const childProcess = require('child_process');
const fs = require('fs');
const os = require('os');
const path = require('path');
const { buildDirectoryHierarchy, buildRouteHierarchy } = require('./routeHierarchy');
const core = require('./extension-core');

const LANGUAGE_ID = 'gwdk';
const semanticLegend = new vscode.SemanticTokensLegend(core.SEMANTIC_TOKEN_TYPES, []);
const missingExecutableNotifications = new Set();
const treeIconColors = {
  page: new vscode.ThemeColor('charts.blue'),
  layout: new vscode.ThemeColor('charts.purple'),
  component: new vscode.ThemeColor('charts.green')
};
const fileDecorations = [
  { suffix: '.page.gwdk', badge: 'P', tooltip: 'GOWDK page', color: treeIconColors.page },
  { suffix: '.layout.gwdk', badge: 'L', tooltip: 'GOWDK layout', color: treeIconColors.layout },
  { suffix: '.cmp.gwdk', badge: 'C', tooltip: 'GOWDK component', color: treeIconColors.component }
];

function activate(context) {
  const diagnostics = vscode.languages.createDiagnosticCollection('gowdk');
  const pending = new Map();
  const siteMapTree = new SiteMapTreeProvider();
  const directoryOutline = new DirectoryOutlineTreeProvider();
  const refreshProjectViews = () => {
    siteMapTree.refresh();
    directoryOutline.refresh();
  };

  context.subscriptions.push(diagnostics);
  context.subscriptions.push(vscode.window.registerTreeDataProvider('gowdk.siteMapTree', siteMapTree));
  context.subscriptions.push(vscode.window.registerTreeDataProvider('gowdk.directoryOutline', directoryOutline));
  context.subscriptions.push(vscode.window.registerFileDecorationProvider(new GOWDKFileDecorationProvider()));

  context.subscriptions.push(vscode.workspace.onDidOpenTextDocument((doc) => validateSoon(doc, diagnostics, pending)));
  context.subscriptions.push(vscode.workspace.onDidChangeTextDocument((event) => validateSoon(event.document, diagnostics, pending)));
  context.subscriptions.push(vscode.workspace.onDidSaveTextDocument((doc) => validateNow(doc, diagnostics)));
  context.subscriptions.push(vscode.workspace.onDidCloseTextDocument((doc) => diagnostics.delete(doc.uri)));
  context.subscriptions.push(vscode.workspace.onDidSaveTextDocument((doc) => {
    if (doc.languageId === LANGUAGE_ID) {
      refreshProjectViews();
    }
  }));

  const watcher = vscode.workspace.createFileSystemWatcher('**/*.gwdk');
  context.subscriptions.push(watcher);
  context.subscriptions.push(watcher.onDidCreate(refreshProjectViews));
  context.subscriptions.push(watcher.onDidDelete(refreshProjectViews));
  context.subscriptions.push(watcher.onDidChange(refreshProjectViews));

  const cssWatcher = vscode.workspace.createFileSystemWatcher('**/*.css');
  context.subscriptions.push(cssWatcher);
  context.subscriptions.push(cssWatcher.onDidCreate(refreshProjectViews));
  context.subscriptions.push(cssWatcher.onDidDelete(refreshProjectViews));
  context.subscriptions.push(cssWatcher.onDidChange(refreshProjectViews));

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
    async provideCompletionItems(document, position) {
      const linePrefix = document.lineAt(position.line).text.slice(0, position.character);
      const completionContext = core.completionContext(linePrefix);
      if (completionContext === 'keyword') {
        return completionItems(core.completionEntries());
      }
      try {
        const metadata = await loadCompletionMetadata(document);
        const entries = core.projectCompletionEntries(completionContext, metadata);
        return completionItems(entries.length ? entries : core.completionEntries());
      } catch (_error) {
        return completionItems(core.completionEntries());
      }
    }
  }, '@', '<', '"', ',', ':', '.', ' ', '{'));

  context.subscriptions.push(vscode.languages.registerHoverProvider(LANGUAGE_ID, {
    async provideHover(document, position) {
      const range = document.getWordRangeAtPosition(position, /[A-Za-z_][A-Za-z0-9_.-]*/);
      if (!range) {
        return undefined;
      }
      const token = document.getText(range);
      try {
        const metadata = await loadCompletionMetadata(document);
        const markdown = core.hoverMarkdown(token, metadata, symbolContextForDocument(document, position));
        if (!markdown) {
          return undefined;
        }
        return new vscode.Hover(new vscode.MarkdownString(markdown), range);
      } catch (_error) {
        return undefined;
      }
    }
  }));

  context.subscriptions.push(vscode.languages.registerDefinitionProvider(LANGUAGE_ID, {
    async provideDefinition(document, position) {
      const range = document.getWordRangeAtPosition(position, /[A-Za-z_][A-Za-z0-9_.-]*/);
      if (!range) {
        return undefined;
      }
      const token = document.getText(range);
      try {
        const metadata = await loadCompletionMetadata(document);
        const target = core.definitionTarget(token, metadata, symbolContextForDocument(document, position));
        if (!target) {
          return undefined;
        }
        return new vscode.Location(vscode.Uri.file(target.file), new vscode.Position(target.line, target.column));
      } catch (_error) {
        return undefined;
      }
    }
  }));

  context.subscriptions.push(vscode.languages.registerReferenceProvider(LANGUAGE_ID, {
    async provideReferences(document, position, referenceContext) {
      const range = document.getWordRangeAtPosition(position, /[A-Za-z_][A-Za-z0-9_.-]*/);
      if (!range) {
        return [];
      }
      const token = document.getText(range);
      try {
        const metadata = await loadCompletionMetadata(document);
        return core.symbolReferences(token, metadata, {
          includeDeclaration: referenceContext.includeDeclaration,
          symbolContext: symbolContextForDocument(document, position)
        }).map((target) => new vscode.Location(vscode.Uri.file(target.file), new vscode.Position(target.line, target.column)));
      } catch (_error) {
        return [];
      }
    }
  }));

  context.subscriptions.push(vscode.languages.registerRenameProvider(LANGUAGE_ID, {
    async provideRenameEdits(document, position, newName) {
      const range = document.getWordRangeAtPosition(position, /[A-Za-z_][A-Za-z0-9_.-]*/);
      if (!range || !core.validRenameValue(newName)) {
        return undefined;
      }
      const token = document.getText(range);
      try {
        const metadata = await loadCompletionMetadata(document);
        if (!core.canRenameSymbol(token, metadata)) {
          return undefined;
        }
        const references = core.symbolReferences(token, metadata, { includeDeclaration: true });
        const edit = new vscode.WorkspaceEdit();
        for (const target of references) {
          const uri = vscode.Uri.file(target.file);
          const targetDocument = await vscode.workspace.openTextDocument(uri);
          for (const item of core.renameEditsForSource(targetDocument.getText(), token, newName)) {
            edit.replace(uri, new vscode.Range(item.start.line, item.start.column, item.end.line, item.end.column), item.text);
          }
        }
        return edit;
      } catch (_error) {
        return undefined;
      }
    },
    prepareRename(document, position) {
      return document.getWordRangeAtPosition(position, /[A-Za-z_][A-Za-z0-9_.-]*/);
    }
  }));

  context.subscriptions.push(vscode.languages.registerDocumentSemanticTokensProvider(LANGUAGE_ID, {
    provideDocumentSemanticTokens(document) {
      const builder = new vscode.SemanticTokensBuilder(semanticLegend);
      for (const token of core.semanticTokens(document.getText())) {
        builder.push(token.line, token.column, token.length, token.tokenType, []);
      }
      return builder.build();
    }
  }, semanticLegend));

  context.subscriptions.push(vscode.languages.registerDocumentSymbolProvider(LANGUAGE_ID, {
    provideDocumentSymbols(document) {
      return core.documentOutlineItems(document.getText()).map(documentSymbol);
    }
  }));

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
        if (ssrEnabledForDocument(document)) {
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
    refreshProjectViews();
  }));

  context.subscriptions.push(vscode.commands.registerCommand('gowdk.openPageFile', async (item) => {
    if (item && item.page && item.page.source) {
      await openFile(item.page.source);
    }
  }));

  context.subscriptions.push(vscode.commands.registerCommand('gowdk.movePageFile', async (item) => {
    if (item && item.page && item.page.source) {
      await moveFile(item.page.source);
      refreshProjectViews();
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
  const root = projectRoot(document);
  const configPath = workspaceConfigPath(root);
  if (!configPath) {
    const diagnostic = new vscode.Diagnostic(
      new vscode.Range(0, 0, 0, 1),
      'gowdk.config.go is required; run gowdk init before validating .gwdk files.',
      vscode.DiagnosticSeverity.Error
    );
    diagnostic.source = 'gowdk';
    diagnostics.set(document.uri, [diagnostic]);
    return;
  }
  try {
    if (document.uri.scheme === 'file' && !document.isDirty) {
      const projectReport = await loadProjectDiagnostics(document, configPath);
      if (projectReport) {
        setProjectDiagnostics(diagnostics, projectReport, document);
        return;
      }
    }

    const report = await withDocumentFile(document, (file) => {
      const args = core.projectCommandArgs('check', {
        json: true,
        configPath,
        ssr: ssrEnabledForDocument(document),
        files: [file]
      });
      return runGowdk(args, document).then(({ stdout }) => core.parseDiagnostics(stdout));
    });
    diagnostics.set(document.uri, report.map(toVSCodeDiagnostic));
  } catch (error) {
    const diagnostic = new vscode.Diagnostic(new vscode.Range(0, 0, 0, 1), error.message, vscode.DiagnosticSeverity.Error);
    diagnostic.source = 'gowdk';
    diagnostics.set(document.uri, [diagnostic]);
  }
}

async function loadProjectDiagnostics(document, configPath) {
  const root = projectRoot(document);
  const ssr = ssrEnabledForRoot(root);
  const args = core.projectCommandArgs('check', {
    json: true,
    configPath,
    ssr
  });
  return runGowdk(args, document).then(({ stdout }) => core.parseDiagnostics(stdout));
}

function setProjectDiagnostics(diagnostics, report, fallbackDocument) {
  diagnostics.clear();
  const grouped = core.groupDiagnosticsByFile(report);
  for (const [file, items] of Object.entries(grouped.files)) {
    diagnostics.set(vscode.Uri.file(file), items.map(toVSCodeDiagnostic));
  }
  if (grouped.global.length > 0) {
    diagnostics.set(fallbackDocument.uri, grouped.global.map(toVSCodeDiagnostic));
  }
}

function toVSCodeDiagnostic(item) {
  const range = core.diagnosticRange(item);
  const severity = core.diagnosticSeverity(item) === 'warning'
    ? vscode.DiagnosticSeverity.Warning
    : vscode.DiagnosticSeverity.Error;
  const diagnostic = new vscode.Diagnostic(new vscode.Range(range.start.line, range.start.column, range.end.line, range.end.column), item.message, severity);
  diagnostic.source = 'gowdk';
  if (item.code) {
    diagnostic.code = item.code;
  }
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
        notifyMissingExecutable(invocation, error);
        error.message = stderr.trim() || stdout.trim() || core.missingExecutableMessage(invocation, error) || error.message;
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

function notifyMissingExecutable(invocation, error) {
  const message = core.missingExecutableMessage(invocation, error);
  if (!message) {
    return;
  }
  const key = `${invocation.cwd || ''}:${invocation.command}`;
  if (missingExecutableNotifications.has(key)) {
    return;
  }
  missingExecutableNotifications.add(key);
  vscode.window.showErrorMessage(message, 'Open Settings').then((selection) => {
    if (selection === 'Open Settings') {
      vscode.commands.executeCommand('workbench.action.openSettings', 'gowdk.cliPath');
    }
  });
}

async function showSiteMap(context) {
  const root = projectRoot();
  if (!root) {
    vscode.window.showWarningMessage('Open a workspace to show the GOWDK site map.');
    return;
  }
  if (!workspaceConfigPath(root)) {
    const uris = await findProjectFiles(root, '**/*.gwdk', '**/{.git,node_modules,vendor}/**');
    if (uris.length === 0) {
      vscode.window.showInformationMessage('No .gwdk files found in this workspace.');
      return;
    }
  }

  try {
    const metadata = await loadProjectMetadata();
    const panel = vscode.window.createWebviewPanel('gowdkSiteMap', 'GOWDK Site Map', vscode.ViewColumn.Beside, {
      enableScripts: true,
      retainContextWhenHidden: true
    });
    panel.webview.html = core.siteMapHTML(metadata, root);
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
  panel.webview.html = core.siteMapHTML(await loadProjectMetadata(), root);
}

async function loadSiteMap(document) {
  const root = projectRoot(document);
  const ssr = ssrEnabledForRoot(root);
  const configPath = workspaceConfigPath(root);
  if (!configPath) {
    return { pages: [] };
  }
  const args = core.projectCommandArgs('sitemap', {
    configPath,
    ssr
  });
  const { stdout } = await runGowdk(args, document);
  return JSON.parse(stdout);
}

async function loadManifest(document) {
  const root = projectRoot(document);
  const ssr = ssrEnabledForRoot(root);
  const configPath = workspaceConfigPath(root);
  if (!configPath) {
    return { pages: {}, components: {} };
  }
  const args = core.projectCommandArgs('manifest', {
    configPath,
    ssr
  });
  const { stdout } = await runGowdk(args, document);
  return JSON.parse(stdout);
}

async function loadCompletionMetadata(document) {
  const [siteMap, manifest, cssFiles] = await Promise.all([
    loadSiteMap(document),
    loadManifest(document),
    loadCSSFiles(document)
  ]);
  return {
    siteMap,
    manifest,
    cssFiles,
    dataFields: core.documentDataFields(document.getText(), {
      fileName: document.fileName,
      projectRoot: projectRoot(document)
    })
  };
}

async function loadProjectMetadata(document) {
  return loadCompletionMetadata(document);
}

function symbolContextForDocument(document, position) {
  return core.symbolContext(document.getText(), document.offsetAt(position));
}

async function loadCSSFiles(document) {
  const root = projectRoot(document);
  if (!root) {
    return [];
  }
  const uris = await findProjectFiles(root, '**/*.css', '**/{.git,node_modules,vendor}/**');
  return uris.map((uri) => ({
    name: path.basename(uri.fsPath, '.css'),
    file: uri.fsPath
  })).sort((left, right) => left.name.localeCompare(right.name) || left.file.localeCompare(right.file));
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
      const root = projectRoot();
      const metadata = await loadProjectMetadata();
      const pages = core.projectPages(metadata);
      if (pages.length === 0) {
        const empty = new vscode.TreeItem('No .gwdk pages found', vscode.TreeItemCollapsibleState.None);
        empty.iconPath = new vscode.ThemeIcon('info');
        return [empty];
      }
      return buildRouteHierarchy(pages).map((node) => siteMapTreeItem(node, root));
    } catch (error) {
      const fallback = new vscode.TreeItem(`Site map unavailable: ${error.message}`, vscode.TreeItemCollapsibleState.None);
      fallback.iconPath = new vscode.ThemeIcon('warning');
      return [fallback];
    }
  }
}

function siteMapTreeItem(node, root) {
  if (node.type === 'group' || node.type === 'directory') {
    return new SiteMapRouteGroupItem(node, root);
  }
  return new SiteMapPageItem(node.page, root);
}

class SiteMapRouteGroupItem extends vscode.TreeItem {
  constructor(node, root) {
    super(node.label, vscode.TreeItemCollapsibleState.Expanded);
    this.contextValue = 'gwdkRouteGroup';
    this.description = node.path;
    this.tooltip = node.type === 'directory'
      ? `Source directory ${node.path || '.'}`
      : `Declared route group ${node.path}`;
    this.iconPath = new vscode.ThemeIcon('folder');
    this.children = node.children.map((child) => siteMapTreeItem(child, root));
  }
}

class DirectoryOutlineTreeProvider {
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
      const root = projectRoot();
      const metadata = await loadProjectMetadata();
      const pages = core.projectPages(metadata);
      if (pages.length === 0) {
        const empty = new vscode.TreeItem('No .gwdk pages found', vscode.TreeItemCollapsibleState.None);
        empty.iconPath = new vscode.ThemeIcon('info');
        return [empty];
      }
      return buildDirectoryHierarchy(pages, root).map((node) => siteMapTreeItem(node, root));
    } catch (error) {
      const fallback = new vscode.TreeItem(`Source outline unavailable: ${error.message}`, vscode.TreeItemCollapsibleState.None);
      fallback.iconPath = new vscode.ThemeIcon('warning');
      return [fallback];
    }
  }
}

class GOWDKFileDecorationProvider {
  provideFileDecoration(uri) {
    if (!uri || uri.scheme !== 'file') {
      return undefined;
    }
    const file = uri.fsPath.toLowerCase();
    const match = fileDecorations.find((decoration) => file.endsWith(decoration.suffix));
    if (!match) {
      return undefined;
    }
    return new vscode.FileDecoration(match.badge, match.tooltip, match.color);
  }
}

class SiteMapPageItem extends vscode.TreeItem {
  constructor(page, root) {
    super(page.id || page.route || '(missing page)', vscode.TreeItemCollapsibleState.Collapsed);
    this.page = page;
    this.contextValue = 'gwdkPage';
    this.description = [page.render || 'spa', page.route || ''].filter(Boolean).join(' ');
    this.tooltip = `${page.id}\n${page.source}`;
    this.iconPath = new vscode.ThemeIcon(page.render === 'ssr' ? 'server' : 'globe', treeIconColors.page);
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
  children.push(infoItem(`Flow: ${core.pageFlow(page)}`, 'git-pull-request'));
  children.push(infoItem(`File: ${rel || page.source || '(unknown)'}`, 'file'));
  children.push(infoItem(`Page: ${page.id || '(missing id)'}`, 'symbol-method', treeIconColors.page));
  if (page.layouts && page.layouts.length) {
    children.push(infoItem(`Layouts: ${page.layouts.join(', ')}`, 'layers', treeIconColors.layout));
  }
  if (page.guard && page.guard.length) {
    children.push(infoItem(`Guards: ${page.guard.join(', ')}`, 'shield'));
  }
  if (page.css && page.css.length) {
    children.push(infoItem(`CSS: ${page.css.join(', ')}`, 'symbol-color'));
  }
  if (page.components && page.components.length) {
    children.push(infoItem(`Components: ${page.components.join(', ')}`, 'symbol-class', treeIconColors.component));
  }
  if (page.assets && page.assets.length) {
    children.push(infoItem(`Assets: ${page.assets.join(', ')}`, 'file-media'));
  }
  if (page.cssClasses && page.cssClasses.length) {
    children.push(infoItem(`Classes: ${page.cssClasses.join(', ')}`, 'symbol-key'));
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

function infoItem(label, icon, color) {
  const item = new vscode.TreeItem(label, vscode.TreeItemCollapsibleState.None);
  item.iconPath = color ? new vscode.ThemeIcon(icon, color) : new vscode.ThemeIcon(icon);
  return item;
}

function toolInvocation(args, document) {
  const cliPath = config().get('cliPath');
  const cwd = projectRoot(document);
  return core.toolInvocation(args, {
    cliPath,
    cwd,
    isSourceWorkspace: isGOWDKSourceWorkspace(cwd),
    localBinary: localGOWDKBinary(cwd),
    sourceWorkspaceRoot: configuredGOWDKSourceRoot() || core.nearbyGOWDKSourceRoot(cwd),
    requiresGOWDK: cwd && workspaceRequiresGOWDK(cwd)
  });
}

function localGOWDKBinary(cwd) {
  if (!cwd) {
    return '';
  }
  const name = process.platform === 'win32' ? 'gowdk.exe' : 'gowdk';
  const file = path.join(cwd, name);
  return fs.existsSync(file) ? file : '';
}

function workspaceRequiresGOWDK(cwd) {
  try {
    return core.goModRequiresGOWDK(fs.readFileSync(path.join(cwd, 'go.mod'), 'utf8'));
  } catch (_error) {
    return false;
  }
}

function configuredGOWDKSourceRoot() {
  const sourcePath = config().get('sourcePath');
  if (!sourcePath) {
    return '';
  }
  const resolved = path.resolve(expandHome(sourcePath));
  return core.isGOWDKSourceDir(resolved) ? resolved : '';
}

function expandHome(value) {
  if (value === '~') {
    return os.homedir();
  }
  if (typeof value === 'string' && value.startsWith(`~${path.sep}`)) {
    return path.join(os.homedir(), value.slice(2));
  }
  return value;
}

function workspaceRoot(document) {
  if (!document) {
    const folder = vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders[0];
    return folder ? folder.uri.fsPath : process.cwd();
  }
  const folder = vscode.workspace.getWorkspaceFolder(document.uri);
  return folder ? folder.uri.fsPath : process.cwd();
}

function projectRoot(document) {
  const root = workspaceRoot(document);
  const source = documentForProjectRoot(document);
  if (!source || source.uri.scheme !== 'file') {
    return root;
  }
  return core.nearestProjectRoot(path.dirname(source.uri.fsPath), root);
}

function documentForProjectRoot(document) {
  if (document) {
    return document;
  }
  const active = vscode.window.activeTextEditor && vscode.window.activeTextEditor.document;
  return active && active.uri.scheme === 'file' ? active : undefined;
}

function findProjectFiles(root, include, exclude) {
  const pattern = root ? new vscode.RelativePattern(root, include) : include;
  return vscode.workspace.findFiles(pattern, projectFileExclude(root, exclude));
}

function projectFileExclude(root, exclude) {
  if (!isGOWDKSourceWorkspace(root)) {
    return exclude;
  }
  if (exclude === '**/{.git,node_modules}/**') {
    return '**/{.git,node_modules,vendor,testdata}/**';
  }
  if (exclude === '**/{.git,node_modules,vendor}/**') {
    return '**/{.git,node_modules,vendor,testdata}/**';
  }
  return exclude || '**/testdata/**';
}

function ssrEnabledForDocument(document) {
  return ssrEnabledForRoot(projectRoot(document));
}

function ssrEnabledForRoot(root) {
  return config().get('enableSsrAddon') || isGOWDKSourceWorkspace(root);
}

function isGOWDKSourceWorkspace(root) {
  if (!root || !fs.existsSync(path.join(root, 'cmd', 'gowdk'))) {
    return false;
  }
  try {
    return core.goModModulePath(fs.readFileSync(path.join(root, 'go.mod'), 'utf8')) === core.GOWDK_MODULE_PATH;
  } catch (_error) {
    return false;
  }
}

function workspaceConfigPath(root) {
  if (!root) {
    return undefined;
  }
  const configPath = path.join(root, 'gowdk.config.go');
  return fs.existsSync(configPath) ? configPath : undefined;
}

function activeGWDKDocument() {
  const editor = vscode.window.activeTextEditor;
  if (!editor || editor.document.languageId !== LANGUAGE_ID) {
    vscode.window.showWarningMessage('Open a .gwdk file first.');
    return undefined;
  }
  return editor.document;
}

function completionItems(entries) {
  return entries.map(([label, detail]) => {
    const item = new vscode.CompletionItem(label, vscode.CompletionItemKind.Keyword);
    item.detail = detail;
    return item;
  });
}

function documentSymbol(item) {
  const symbol = new vscode.DocumentSymbol(
    item.name,
    item.detail || '',
    documentSymbolKind(item.kind),
    documentRange(item.range),
    documentRange(item.selectionRange)
  );
  symbol.children = (item.children || []).map(documentSymbol);
  return symbol;
}

function documentRange(range) {
  return new vscode.Range(
    range.start.line,
    range.start.column,
    range.end.line,
    range.end.column
  );
}

function documentSymbolKind(kind) {
  const kinds = {
    class: vscode.SymbolKind.Class,
    function: vscode.SymbolKind.Function,
    module: vscode.SymbolKind.Module,
    namespace: vscode.SymbolKind.Namespace,
    object: vscode.SymbolKind.Object,
    property: vscode.SymbolKind.Property,
    struct: vscode.SymbolKind.Struct
  };
  return kinds[kind] || vscode.SymbolKind.Property;
}

function config() {
  return vscode.workspace.getConfiguration('gowdk');
}

module.exports = {
  activate,
  deactivate
};
