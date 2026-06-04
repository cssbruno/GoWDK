const path = require('path');

const SEMANTIC_TOKEN_TYPES = [
  'namespace',
  'keyword',
  'class',
  'property',
  'enumMember',
  'function'
];

function parseDiagnostics(stdout) {
  if (!stdout.trim()) {
    return [];
  }
  const parsed = JSON.parse(stdout);
  return parsed.diagnostics || [];
}

function diagnosticPosition(item) {
  return {
    line: Math.max((item.pos && item.pos.line ? item.pos.line : 1) - 1, 0),
    column: Math.max((item.pos && item.pos.column ? item.pos.column : 1) - 1, 0)
  };
}

function diagnosticRange(item) {
  if (item.range && item.range.start && item.range.end) {
    return {
      start: diagnosticEditorPosition(item.range.start),
      end: diagnosticEditorPosition(item.range.end)
    };
  }
  const position = diagnosticPosition(item);
  return {
    start: position,
    end: { line: position.line, column: position.column + 1 }
  };
}

function diagnosticEditorPosition(position) {
  return {
    line: Math.max((position.line || 1) - 1, 0),
    column: Math.max((position.column || 1) - 1, 0)
  };
}

function diagnosticSeverity(item) {
  return item.severity === 'warning' ? 'warning' : 'error';
}

function projectCommandArgs(command, options = {}) {
  const args = [command];
  if (options.json) {
    args.push('--json');
  }
  if (options.configPath) {
    args.push('--config', options.configPath);
  }
  if (options.ssr) {
    args.push('--ssr');
  }
  for (const file of options.files || []) {
    args.push(file);
  }
  return args;
}

function groupDiagnosticsByFile(diagnostics) {
  const files = {};
  const global = [];
  for (const item of diagnostics) {
    if (!item.file) {
      global.push(item);
      continue;
    }
    const key = normalizePath(item.file);
    if (!files[key]) {
      files[key] = [];
    }
    files[key].push(item);
  }
  return { files, global };
}

function normalizePath(value) {
  return path.resolve(String(value || '')).replace(/\\/g, '/');
}

function siteMapHTML(siteMap, root) {
  const pages = siteMapPages(siteMap).slice().sort((a, b) => String(a.route || '').localeCompare(String(b.route || '')));
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
      cursor: pointer;
      display: inline-flex;
      align-items: center;
      justify-content: center;
    }
    button.secondary {
      background: var(--vscode-button-secondaryBackground);
      color: var(--vscode-button-secondaryForeground);
    }
    .icon-button {
      width: 28px;
      height: 28px;
      padding: 0;
    }
    .icon-button svg {
      width: 16px;
      height: 16px;
      stroke: currentColor;
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
    .flow {
      color: var(--vscode-descriptionForeground);
      font-family: var(--vscode-editor-font-family);
      font-size: 12px;
      margin-top: 4px;
      word-break: break-word;
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
    .details {
      display: grid;
      gap: 3px;
      margin-top: 6px;
      font-size: 12px;
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
    <button id="refresh" class="icon-button" title="Refresh" aria-label="Refresh">${iconSVG('refresh')}</button>
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
  const css = page.css || [];
  const components = page.components || [];
  const staticAssets = page.staticAssets || [];
  const tags = [
    page.render,
    ...blocks,
    ...actions.map((name) => `act:${name}`),
    ...apis.map((name) => `api:${name}`),
    ...(page.layouts || []).map((layout) => `layout:${layout}`),
    ...css.map((name) => `css:${name}`)
  ].filter(Boolean);
  const details = [
    css.length ? `CSS: ${css.join(', ')}` : '',
    components.length ? `Components: ${components.join(', ')}` : '',
    staticAssets.length ? `Assets: ${staticAssets.join(', ')}` : ''
  ].filter(Boolean);
  return `<section class="page">
    <div class="route">${escapeHTML(page.route || '(missing route)')}</div>
    <div class="flow">${escapeHTML(pageFlow(page))}</div>
    <div class="meta">${tags.map((tag) => `<span class="pill">${escapeHTML(tag)}</span>`).join('')}</div>
    <div class="file">${escapeHTML(page.id)} · ${escapeHTML(rel || page.source || '')}</div>
    ${details.length ? `<div class="details">${details.map((item) => `<div>${escapeHTML(item)}</div>`).join('')}</div>` : ''}
    <div class="actions">
      <button class="icon-button" data-open="${escapeAttr(page.source)}" title="Open Page File" aria-label="Open Page File">${iconSVG('open')}</button>
      <button class="icon-button secondary" data-move="${escapeAttr(page.source)}" title="Move File" aria-label="Move File">${iconSVG('move')}</button>
    </div>
  </section>`;
}

function iconSVG(name) {
  if (name === 'refresh') {
    return '<svg viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M13.25 5.25A5.5 5.5 0 0 0 3.1 4.7L2 6.25M2 6.25H5.75M2 6.25V2.5M2.75 10.75A5.5 5.5 0 0 0 12.9 11.3L14 9.75M14 9.75H10.25M14 9.75V13.5" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>';
  }
  if (name === 'move') {
    return '<svg viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M8 2.25V13.75M8 2.25L5.75 4.5M8 2.25L10.25 4.5M8 13.75L5.75 11.5M8 13.75L10.25 11.5M2.25 8H13.75M2.25 8L4.5 5.75M2.25 8L4.5 10.25M13.75 8L11.5 5.75M13.75 8L11.5 10.25" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>';
  }
  return '<svg viewBox="0 0 16 16" fill="none" aria-hidden="true"><path d="M3.75 2.25H8.5L12.25 6V13.75H3.75V2.25Z" stroke-width="1.5" stroke-linejoin="round"/><path d="M8.5 2.25V6H12.25" stroke-width="1.5" stroke-linejoin="round"/><path d="M6.25 10.25H10.25M10.25 10.25L8.75 8.75M10.25 10.25L8.75 11.75" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>';
}

function pageFlow(page) {
  const route = page.route || '(missing route)';
  const render = page.render || 'static';
  const artifacts = page.artifacts || [];
  const htmlArtifact = artifacts.find((artifact) => artifact.kind === 'html' && artifact.path);
  const output = render === 'ssr'
    ? 'request-time HTML'
    : (htmlArtifact && htmlArtifact.path) || 'generated HTML';
  const steps = [`GET ${route}`, render, output];
  const actions = (page.blocks && page.blocks.actions) || [];
  const apis = (page.blocks && page.blocks.apis) || [];
  const sideEffects = [
    ...actions.map((name) => `POST act:${name}`),
    ...apis.map((name) => `API ${name || '(unnamed)'}`)
  ];
  return sideEffects.length ? `${steps.join(' -> ')} | ${sideEffects.join(' | ')}` : steps.join(' -> ');
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

function siteMapPages(input = {}) {
  if (input.siteMap || input.manifest) {
    return projectPages(input);
  }
  return input.pages || [];
}

function projectPages(metadata = {}) {
  const pages = new Map();
  for (const page of (metadata.siteMap && metadata.siteMap.pages) || []) {
    const key = pageKey(page);
    pages.set(key, { ...page });
  }
  for (const [id, page] of Object.entries((metadata.manifest && metadata.manifest.pages) || {})) {
    const manifestPage = { id, ...page };
    const existingKey = pageKey(manifestPage);
    const sourceKey = findPageKeyBySource(pages, manifestPage.source);
    const key = pages.has(existingKey) ? existingKey : sourceKey || existingKey;
    pages.set(key, mergePageMetadata(pages.get(key), manifestPage));
  }
  return Array.from(pages.values()).sort((left, right) => {
    const leftRoute = left.route || '';
    const rightRoute = right.route || '';
    if (leftRoute !== rightRoute) {
      return leftRoute.localeCompare(rightRoute);
    }
    return String(left.id || left.source || '').localeCompare(String(right.id || right.source || ''));
  });
}

function mergePageMetadata(left = {}, right = {}) {
  return {
    ...left,
    ...right,
    blocks: {
      ...(left.blocks || {}),
      ...(right.blocks || {})
    }
  };
}

function pageKey(page = {}) {
  return String(page.id || page.source || page.route || '');
}

function findPageKeyBySource(pages, source) {
  if (!source) {
    return '';
  }
  for (const [key, page] of pages.entries()) {
    if (page.source === source) {
      return key;
    }
  }
  return '';
}

function completionEntries() {
  return [
    ['@page', 'Declare the page id.'],
    ['@route', 'Declare the route path.'],
    ['@layout', 'Declare one or more layout ids.'],
    ['@render', 'Declare render mode: static, action, hybrid, or ssr.'],
    ['@guard', 'Declare route guards.'],
    ['@css', 'Select page CSS inputs: default, page, none, or discovered CSS names.'],
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
}

function completionContext(linePrefix) {
  const prefix = String(linePrefix || '');
  if (/@layout\s+(?:[A-Za-z0-9_.-]+,\s*)*[A-Za-z0-9_.-]*$/.test(prefix)) {
    return 'layout';
  }
  if (/@css\s+(?:[A-Za-z0-9_.-]+,\s*)*[A-Za-z0-9_.-]*$/.test(prefix)) {
    return 'css';
  }
  if (/<[A-Z][A-Za-z0-9_]*$/.test(prefix)) {
    return 'component';
  }
  if (/(@route|->|GET|POST|PUT|PATCH|DELETE)\s+"[^"]*$/.test(prefix)) {
    return 'route';
  }
  return 'keyword';
}

function projectCompletionEntries(context, metadata = {}) {
  if (context === 'layout') {
    return unique(projectPages(metadata).flatMap((page) => page.layouts || []))
      .map((layout) => [layout, 'Layout id from project metadata.']);
  }
  if (context === 'route') {
    return unique(projectPages(metadata).map((page) => page.route).filter(Boolean))
      .map((route) => [route, 'Route from project metadata.']);
  }
  if (context === 'component') {
    return unique(Object.keys((metadata.manifest && metadata.manifest.components) || {}))
      .map((component) => [component, 'Component from project manifest.']);
  }
  if (context === 'css') {
    return cssCompletionEntries(metadata);
  }
  return completionEntries();
}

function cssCompletionEntries(metadata = {}) {
  const entries = new Map([
    ['default', 'Built-in CSS input: configured default CSS, or global.css when present.'],
    ['page', 'Built-in CSS input: CSS file matching the page id when present.'],
    ['none', 'Built-in CSS input: disable GOWDK-managed page CSS for this page.']
  ]);
  for (const cssFile of cssFileEntries(metadata)) {
    entries.set(cssFile.name, `Discovered CSS file: ${cssFile.file}`);
  }
  for (const name of projectPages(metadata).flatMap((page) => page.css || [])) {
    if (!entries.has(name)) {
      entries.set(name, 'CSS input referenced by project pages.');
    }
  }
  return Array.from(entries.entries()).sort((left, right) => left[0].localeCompare(right[0]));
}

function cssFileEntries(metadata = {}) {
  const cssFiles = metadata.cssFiles || {};
  if (Array.isArray(cssFiles)) {
    return cssFiles.map((entry) => normalizeCSSFileEntry(entry.name, entry)).filter(Boolean);
  }
  return Object.entries(cssFiles).flatMap(([name, value]) => {
    const values = Array.isArray(value) ? value : [value];
    return values.map((entry) => normalizeCSSFileEntry(name, entry)).filter(Boolean);
  });
}

function normalizeCSSFileEntry(name, entry) {
  if (!entry) {
    return undefined;
  }
  if (typeof entry === 'string') {
    return { name: String(name || cssInputNameFromFile(entry)), file: entry };
  }
  const file = entry.file || entry.path || '';
  const cssName = entry.name || name || cssInputNameFromFile(file);
  if (!file || !cssName) {
    return undefined;
  }
  return { name: String(cssName), file: String(file) };
}

function cssInputNameFromFile(file) {
  const base = path.basename(String(file || ''));
  return base.endsWith('.css') ? base.slice(0, -4) : base;
}

function cssFileDefinitions(name, metadata = {}) {
  return cssFileEntries(metadata).filter((entry) => entry.name === name);
}

function cssReferencingPages(name, metadata = {}) {
  return projectPages(metadata).filter((page) => (page.css || []).includes(name));
}

function cssInputMarkdown(name, metadata = {}) {
  const files = cssFileDefinitions(name, metadata);
  const pages = cssReferencingPages(name, metadata);
  if (files.length === 0 && pages.length === 0) {
    return '';
  }
  const lines = [`**GOWDK CSS input** \`${escapeMarkdown(name)}\``];
  if (files.length > 0) {
    lines.push('', `File: \`${escapeMarkdown(files[0].file)}\``);
  }
  if (pages.length > 0) {
    lines.push('', `Referenced by ${pages.length} page${pages.length === 1 ? '' : 's'}.`);
  }
  if (name === 'default') {
    lines.push('', 'Built-in default input resolves to configured default CSS, or `global.css` when present.');
  }
  if (name === 'page') {
    lines.push('', 'Built-in page input resolves to the CSS file matching the page id when present.');
  }
  if (name === 'none') {
    lines.push('', 'Built-in none disables GOWDK-managed page CSS and must be used alone.');
  }
  return lines.join('\n');
}

function hoverMarkdown(token, metadata = {}) {
  const value = String(token || '');
  if (!value) {
    return '';
  }
  const pages = projectPages(metadata);
  const manifest = metadata.manifest || {};
  const page = pages.find((item) => item.id === value);
  if (page) {
    return [
      `**GOWDK page** \`${escapeMarkdown(value)}\``,
      '',
      `Route: \`${escapeMarkdown(page.route || '')}\``,
      `Render: \`${escapeMarkdown(page.render || 'static')}\``
    ].join('\n');
  }
  if (manifest.components && manifest.components[value]) {
    const component = manifest.components[value];
    const props = (component.props || []).map((prop) => `${prop.name} ${prop.type}`).join(', ');
    return [
      `**GOWDK component** \`${escapeMarkdown(value)}\``,
      '',
      props ? `Props: \`${escapeMarkdown(props)}\`` : 'Props: none'
    ].join('\n');
  }
  const layoutPages = pages.filter((item) => (item.layouts || []).includes(value));
  if (layoutPages.length > 0) {
    return [
      `**GOWDK layout** \`${escapeMarkdown(value)}\``,
      '',
      `Referenced by ${layoutPages.length} page${layoutPages.length === 1 ? '' : 's'}.`
    ].join('\n');
  }
  const cssMarkdown = cssInputMarkdown(value, metadata);
  if (cssMarkdown) {
    return cssMarkdown;
  }
  for (const item of pages) {
    const actions = (item.blocks && item.blocks.actions) || [];
    if (actions.includes(value)) {
      return `**GOWDK action** \`${escapeMarkdown(value)}\`\n\nPage: \`${escapeMarkdown(item.id || '')}\``;
    }
    const apis = (item.blocks && item.blocks.apis) || [];
    if (apis.includes(value)) {
      return `**GOWDK API** \`${escapeMarkdown(value)}\`\n\nPage: \`${escapeMarkdown(item.id || '')}\``;
    }
  }
  return '';
}

function definitionTarget(token, metadata = {}) {
  const value = String(token || '');
  if (!value) {
    return undefined;
  }
  const pages = projectPages(metadata);
  const manifest = metadata.manifest || {};
  const page = pages.find((item) => item.id === value);
  if (page && page.source) {
    return { file: page.source, line: 0, column: 0 };
  }
  const component = manifest.components && manifest.components[value];
  if (component && component.source) {
    return { file: component.source, line: 0, column: 0 };
  }
  for (const item of pages) {
    if (!item.source) {
      continue;
    }
    if ((item.layouts || []).includes(value) || (item.guard || []).includes(value)) {
      return { file: item.source, line: 0, column: 0 };
    }
    const actions = (item.blocks && item.blocks.actions) || [];
    const apis = (item.blocks && item.blocks.apis) || [];
    if (actions.includes(value) || apis.includes(value)) {
      return { file: item.source, line: 0, column: 0 };
    }
  }
  const cssDefinition = cssFileDefinitions(value, metadata)[0];
  if (cssDefinition) {
    return { file: cssDefinition.file, line: 0, column: 0 };
  }
  return undefined;
}

function symbolReferences(token, metadata = {}, options = {}) {
	const value = String(token || '');
	if (!value) {
    return [];
  }
  const pages = projectPages(metadata);
  const manifest = metadata.manifest || {};
  const refs = [];
  const includeDeclaration = options.includeDeclaration !== false;
  const isProjectSymbol = projectSymbolExists(value, metadata);

  const page = pages.find((item) => item.id === value);
  if (page && page.source && includeDeclaration) {
    refs.push(fileLocation(page.source));
  }

  const component = manifest.components && manifest.components[value];
  if (component && component.source && includeDeclaration) {
    refs.push(fileLocation(component.source));
  }

  if (includeDeclaration && !isProjectSymbol) {
    for (const cssDefinition of cssFileDefinitions(value, metadata)) {
      refs.push(fileLocation(cssDefinition.file));
    }
  }

  const manifestPages = Object.values(manifest.pages || {});
  for (const item of pages) {
    if (!item.source) {
      continue;
    }
    if ((item.layouts || []).includes(value) || (item.guard || []).includes(value)) {
      refs.push(fileLocation(item.source));
    }
    const actions = (item.blocks && item.blocks.actions) || [];
    const apis = (item.blocks && item.blocks.apis) || [];
    if (actions.includes(value) || apis.includes(value)) {
      refs.push(fileLocation(item.source));
    }
    if (!isProjectSymbol && (item.css || []).includes(value)) {
      refs.push(fileLocation(item.source));
    }
  }
  for (const item of manifestPages) {
    if (!item.source) {
      continue;
    }
    if ((item.components || []).includes(value)) {
      refs.push(fileLocation(item.source));
    }
  }
	return uniqueLocations(refs);
}

function canRenameSymbol(token, metadata = {}) {
  if (!projectSymbolExists(token, metadata) && cssInputMarkdown(token, metadata)) {
    return false;
  }
  return Boolean(definitionTarget(token, metadata));
}

function projectSymbolExists(token, metadata = {}) {
  const value = String(token || '');
  const pages = projectPages(metadata);
  const manifest = metadata.manifest || {};
  if (pages.some((page) => page.id === value)) {
    return true;
  }
  if (manifest.components && manifest.components[value]) {
    return true;
  }
  return pages.some((page) => {
    const actions = (page.blocks && page.blocks.actions) || [];
    const apis = (page.blocks && page.blocks.apis) || [];
    return (page.layouts || []).includes(value)
      || (page.guard || []).includes(value)
      || actions.includes(value)
      || apis.includes(value);
  });
}

function validRenameValue(value) {
  return /^[A-Za-z_][A-Za-z0-9_.-]*$/.test(String(value || ''));
}

function renameEditsForSource(source, token, newName) {
  const value = String(token || '');
  if (!value || !validRenameValue(newName)) {
    return [];
  }
  const pattern = new RegExp(`(^|[^A-Za-z0-9_.-])(${escapeRegExp(value)})(?=$|[^A-Za-z0-9_.-])`, 'g');
  const text = String(source || '');
  const edits = [];
  for (const match of text.matchAll(pattern)) {
    const index = match.index + match[1].length;
    const start = positionAt(text, index);
    edits.push({
      start,
      end: { line: start.line, column: start.column + value.length },
      text: String(newName)
    });
  }
  return edits;
}

function semanticTokens(source) {
	const tokens = [];
	const lines = String(source || '').split(/\r?\n/);
  for (let line = 0; line < lines.length; line++) {
    const text = lines[line];
    collectPatternTokens(tokens, line, text, /@[A-Za-z_][A-Za-z0-9_]*/g, 'namespace');
    collectPatternTokens(tokens, line, text, /\b(paths|build|load|act|api|view|props)\b/g, 'keyword');
    collectPatternTokens(tokens, line, text, /\b(static|action|hybrid|ssr)\b/g, 'enumMember');
    collectPatternTokens(tokens, line, text, /\bg:(post|target|swap)\b/g, 'property');
    collectPatternTokens(tokens, line, text, /<\/?([A-Z][A-Za-z0-9_]*)\b/g, 'class', 1);
    collectPatternTokens(tokens, line, text, /\b(?:act|api)\s+([A-Za-z_][A-Za-z0-9_]*)/g, 'function', 1);
    collectCSSReferenceTokens(tokens, line, text);
  }
  return withoutOverlaps(tokens).sort((a, b) => a.line - b.line || a.column - b.column || a.length - b.length);
}

function collectCSSReferenceTokens(tokens, line, text) {
  const match = text.match(/@css\s+(.+)$/);
  if (!match) {
    return;
  }
  const start = match.index + match[0].indexOf(match[1]);
  for (const item of match[1].matchAll(/[A-Za-z_][A-Za-z0-9_.-]*/g)) {
    tokens.push({
      line,
      column: start + item.index,
      length: item[0].length,
      tokenType: 'property'
    });
  }
}

function collectPatternTokens(tokens, line, text, pattern, tokenType, capture = 0) {
  for (const match of text.matchAll(pattern)) {
    const lexeme = capture === 0 ? match[0] : match[capture];
    if (!lexeme) {
      continue;
    }
    tokens.push({
      line,
      column: match.index + (capture === 0 ? 0 : match[0].indexOf(lexeme)),
      length: lexeme.length,
      tokenType
    });
  }
}

function withoutOverlaps(tokens) {
  const out = [];
  for (const token of tokens.sort((a, b) => a.line - b.line || a.column - b.column || b.length - a.length)) {
    const end = token.column + token.length;
    const overlaps = out.some((existing) => {
      if (existing.line !== token.line) {
        return false;
      }
      const existingEnd = existing.column + existing.length;
      return token.column < existingEnd && end > existing.column;
    });
    if (!overlaps) {
      out.push(token);
    }
  }
  return out;
}

function fileLocation(file) {
  return { file, line: 0, column: 0 };
}

function uniqueLocations(locations) {
  const seen = new Set();
  const out = [];
  for (const location of locations) {
    const key = `${location.file}:${location.line}:${location.column}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    out.push(location);
  }
  return out;
}

function unique(values) {
	return Array.from(new Set(values)).sort((a, b) => a.localeCompare(b));
}

function positionAt(source, offset) {
  const lines = String(source || '').slice(0, offset).split(/\r?\n/);
  return {
    line: lines.length - 1,
    column: lines[lines.length - 1].length
  };
}

function escapeRegExp(value) {
  return String(value || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

function escapeMarkdown(value) {
	return String(value || '').replace(/[`\\]/g, '\\$&');
}

module.exports = {
  SEMANTIC_TOKEN_TYPES,
  completionEntries,
  completionContext,
  canRenameSymbol,
  cssCompletionEntries,
  cssFileEntries,
  definitionTarget,
  diagnosticPosition,
  diagnosticRange,
  diagnosticSeverity,
  escapeHTML,
  groupDiagnosticsByFile,
  hoverMarkdown,
  parseDiagnostics,
  pageFlow,
  projectPages,
  projectCompletionEntries,
  projectCommandArgs,
  renameEditsForSource,
  semanticTokens,
  siteMapHTML,
  symbolReferences,
  validRenameValue
};
