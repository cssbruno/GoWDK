const fs = require('fs');
const path = require('path');

const GOWDK_MODULE_PATH = 'github.com/cssbruno/gowdk';
const DOCS_BASE_URL = 'https://github.com/cssbruno/GoWDK/blob/main/';

const SURFACE_STATUS = {
  page: {
    status: 'partial',
    docs: 'docs/reference/routing.md',
    limit: 'Full pages default to build-time SPA output; request-time page rendering is explicit with `load {}` or `go ssr {}`.'
  },
  route: {
    status: 'implemented',
    docs: 'docs/reference/routing.md',
    limit: 'Routes are declared in source files; folder layout is not route truth.'
  },
  action: {
    status: 'partial',
    docs: 'docs/language/actions.md',
    limit: 'Generated typed action adapters cover the supported form subset; uploads stay in user-owned API/server handlers.'
  },
  api: {
    status: 'partial',
    docs: 'docs/language/api.md',
    limit: 'Generated API handlers currently target the documented response-helper signatures.'
  },
  component: {
    status: 'partial',
    docs: 'docs/language/components.md',
    limit: 'Component props, state, slots, CSS/assets, client behavior, and WASM islands have first-slice support; recursive and dynamic component selection are rejected.'
  },
  componentEvent: {
    status: 'partial',
    docs: 'docs/language/components.md',
    limit: 'Component events are local component metadata; backend-owned events use the contract runtime.'
  },
  layout: {
    status: 'partial',
    docs: 'docs/language/layouts.md',
    limit: 'Layouts compose declared pages; request-aware layout behavior belongs to the SSR lane.'
  },
  css: {
    status: 'partial',
    docs: 'docs/reference/css.md',
    limit: 'CSS processor support is addon-driven; Tailwind and external processors remain optional.'
  },
  store: {
    status: 'partial',
    docs: 'docs/language/components.md',
    limit: 'Stores are page/island scoped; app-global stores remain deferred.'
  },
  goContract: {
    status: 'partial',
    docs: 'docs/reference/go-interop.md',
    limit: 'Application behavior stays in normal Go; generated Go remains adapter glue.'
  },
  dataField: {
    status: 'partial',
    docs: 'docs/reference/go-interop.md',
    limit: '`build {}` is build-time data; `load {}` is request-time data and requires SSR.'
  },
  spa: {
    status: 'implemented',
    docs: 'docs/reference/routing.md',
    limit: 'SPA/build-time output is the default full-page lane.'
  },
  ssr: {
    status: 'partial',
    docs: 'docs/language/ssr.md',
    limit: 'SSR is an integrated non-default request-time page lane gated by the SSR addon.'
  },
  hybrid: {
    status: 'planned',
    docs: 'docs/product/requirements.md',
    limit: 'Hybrid route metadata exists internally; a stable source contract is still deferred.'
  },
  unsupported: {
    status: 'unsupported',
    docs: 'docs/product/requirements.md',
    limit: 'This surface is intentionally outside the current GOWDK source contract.'
  }
};

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

function goModRequiresGOWDK(source) {
  return String(source || '').split(/\r?\n/).some((line) => {
    const text = line.replace(/\/\/.*$/, '').trim();
    return text === GOWDK_MODULE_PATH ||
      text.startsWith(`${GOWDK_MODULE_PATH} `) ||
      text.startsWith(`require ${GOWDK_MODULE_PATH} `);
  });
}

function goModModulePath(source) {
  for (const line of String(source || '').split(/\r?\n/)) {
    const text = line.replace(/\/\/.*$/, '').trim();
    const match = text.match(/^module\s+(\S+)$/);
    if (match) {
      return match[1];
    }
  }
  return '';
}

function gowdkModuleRunArgs(args) {
  return ['run', `${GOWDK_MODULE_PATH}/cmd/gowdk`, ...args];
}

function gowdkSourceRunInvocation(args, cwd) {
  return { command: 'go', args: ['run', './cmd/gowdk', ...args], cwd };
}

function toolInvocation(args, options = {}) {
  const cwd = options.cwd;
  if (options.cliPath) {
    return { command: options.cliPath, args, cwd, source: 'cliPath' };
  }
  if (options.isSourceWorkspace) {
    return { ...gowdkSourceRunInvocation(args, cwd), source: 'sourceWorkspace' };
  }
  if (options.localBinary) {
    return { command: options.localBinary, args, cwd, source: 'localBinary' };
  }
  return { command: 'gowdk', args, cwd, source: 'path' };
}

function isMissingExecutableError(error = {}) {
  return error.code === 'ENOENT' || /\bENOENT\b/.test(String(error.message || ''));
}

function missingExecutableMessage(invocation = {}, error = {}) {
  if (!isMissingExecutableError(error)) {
    return '';
  }
  const command = String(invocation.command || 'gowdk');
  if (command === 'gowdk') {
    return 'Missing GOWDK binary. Install gowdk, add it to PATH, or set gowdk.cliPath.';
  }
  if (invocation.source === 'cliPath') {
    return `Missing configured GOWDK binary: ${command}. Update gowdk.cliPath.`;
  }
  if (command === 'go') {
    return 'Missing Go binary. Install Go, fix PATH, or set gowdk.cliPath to a built GOWDK binary.';
  }
  return `Missing GOWDK binary: ${command}. Update gowdk.cliPath or fix PATH.`;
}

function diagnosticCodeForMessage(message) {
  const text = String(message || '');
  if (/gowdk\.config\.go is required/i.test(text)) {
    return 'missing_gowdk_config';
  }
  if (/missing configured GOWDK binary|missing GOWDK binary/i.test(text)) {
    return 'missing_gowdk_binary';
  }
  if (/missing Go binary/i.test(text)) {
    return 'missing_go_binary';
  }
  if (/missing_ssr_addon|SSR addon/i.test(text)) {
    return 'missing_ssr_addon';
  }
  return '';
}

function quickFixesForDiagnostic(diagnostic = {}) {
  const code = diagnosticCodeForMessage(diagnostic.message) || String(diagnostic.code || '');
  switch (code) {
    case 'missing_gowdk_binary':
    case 'missing_go_binary':
      return [
        quickFix('Set gowdk.cliPath', 'gowdk.openCliPathSetting', true),
        quickFix('Open GOWDK install docs', 'gowdk.openInstallDocs', false)
      ];
    case 'missing_gowdk_config':
      return [
        quickFix('Create gowdk.config.go', 'gowdk.createConfig', true),
        quickFix('Open config docs', 'gowdk.openConfigDocs', false)
      ];
    case 'missing_ssr_addon':
      return [
        quickFix('Enable SSR validation setting', 'gowdk.enableSsrAddon', true),
        quickFix('Open SSR docs', 'gowdk.openSsrDocs', false)
      ];
    default:
      return [];
  }
}

function quickFix(title, command, preferred) {
  return { title, command, preferred };
}

function isGOWDKSourceDir(dir) {
  if (!dir || !fs.existsSync(path.join(dir, 'cmd', 'gowdk'))) {
    return false;
  }
  try {
    return goModModulePath(fs.readFileSync(path.join(dir, 'go.mod'), 'utf8')) === GOWDK_MODULE_PATH;
  } catch (_error) {
    return false;
  }
}

function nearbyGOWDKSourceRoot(startPath) {
  if (!startPath) {
    return '';
  }
  const checked = new Set();
  let current = path.resolve(startPath);
  while (true) {
    for (const candidate of gowdkSourceCandidates(current)) {
      const normalized = path.resolve(candidate);
      if (checked.has(normalized)) {
        continue;
      }
      checked.add(normalized);
      if (isGOWDKSourceDir(normalized)) {
        return normalized;
      }
    }
    const parent = path.dirname(current);
    if (parent === current) {
      break;
    }
    current = parent;
  }
  return '';
}

function gowdkSourceCandidates(dir) {
  return [
    dir,
    path.join(dir, 'GOWDK'),
    path.join(dir, 'gowdk')
  ];
}

function nearestProjectRoot(startPath, workspaceRoot) {
  if (!startPath) {
    return workspaceRoot;
  }
  let current = path.resolve(startPath);
  while (true) {
    if (isProjectRoot(current)) {
      return current;
    }
    if (current === path.dirname(current)) {
      break;
    }
    current = path.dirname(current);
  }
  return workspaceRoot ? path.resolve(workspaceRoot) : undefined;
}

function isProjectRoot(dir) {
  return fs.existsSync(path.join(dir, 'go.mod')) ||
    fs.existsSync(path.join(dir, 'gowdk.config.go')) ||
    fs.existsSync(path.join(dir, 'cmd', 'gowdk'));
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
  const routes = pages.map((page) => pageCard(page, root, siteMap)).join('');
  const spaCount = pages.filter((page) => page.render === 'spa').length;
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
    .node-link {
      appearance: none;
      border: 0;
      background: transparent;
      color: inherit;
      cursor: pointer;
      display: inline;
      font: inherit;
      padding: 0;
      text-align: left;
    }
    .node-link:hover {
      text-decoration: underline;
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
    button.pill {
      display: inline-flex;
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
      <div class="summary">${pages.length} pages · ${spaCount} spa · ${ssrCount} ssr</div>
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
    document.querySelectorAll('[data-definition-file]').forEach((button) => {
      button.addEventListener('click', () => vscode.postMessage({
        type: 'definition',
        file: button.dataset.definitionFile,
        line: Number(button.dataset.definitionLine || 0),
        column: Number(button.dataset.definitionColumn || 0)
      }));
    });
    document.querySelectorAll('[data-move]').forEach((button) => {
      button.addEventListener('click', () => vscode.postMessage({ type: 'move', file: button.dataset.move }));
    });
  </script>
</body>
</html>`;
}

function pageCard(page, root, metadata = {}) {
  const rel = path.relative(root, page.source || '').replace(/\\/g, '/');
  const blocks = Object.entries(page.blocks || {})
    .filter(([key, value]) => key !== 'actions' && key !== 'apis' && value)
    .map(([key]) => key);
  const actions = (page.blocks && page.blocks.actions) || [];
  const apis = (page.blocks && page.blocks.apis) || [];
  const fragments = (page.blocks && page.blocks.fragments) || [];
  const css = page.css || [];
  const components = page.components || [];
  const assets = page.assets || [];
  const routeNodes = routeNodesForPage(page, metadata);
  const endpointNodes = endpointNodesForPage(page, metadata);
  const contractRefs = pageContractRefs(page);
  const tags = [
    { label: page.render },
    ...blocks.map((block) => ({
      label: block,
      target: definitionTargetForNode({ kind: 'block', value: block, pageId: page.id }, metadata) || sourceLocation(page)
    })),
    ...actions.map((name) => ({
      label: `act:${name}`,
      target: definitionTargetForNode({ kind: 'endpoint', endpointKind: 'action', value: name, pageId: page.id }, metadata) || sourceLocation(page)
    })),
    ...apis.map((name) => ({
      label: `api:${name}`,
      target: definitionTargetForNode({ kind: 'endpoint', endpointKind: 'api', value: name, pageId: page.id }, metadata) || sourceLocation(page)
    })),
    ...fragments.map((name) => ({
      label: `fragment:${name}`,
      target: definitionTargetForNode({ kind: 'endpoint', endpointKind: 'fragment', value: name, pageId: page.id }, metadata) || sourceLocation(page)
    })),
    ...(page.layouts || []).map((layout) => ({
      label: `layout:${layout}`,
      target: definitionTargetForNode({ kind: 'layout', value: layout, pageId: page.id }, metadata)
    })),
    ...css.map((name) => ({
      label: `css:${name}`,
      target: definitionTargetForNode({ kind: 'css', value: name, pageId: page.id }, metadata)
    }))
  ].filter(Boolean);
  const details = [
    routeNodes.length ? detailNodeList('Routes', routeNodes) : '',
    endpointNodes.length ? detailNodeList('Endpoints', endpointNodes) : '',
    css.length ? detailNodeList('CSS', css.map((name) => ({
      label: name,
      target: definitionTargetForNode({ kind: 'css', value: name, pageId: page.id }, metadata)
    }))) : '',
    components.length ? detailNodeList('Components', components.map((name) => ({
      label: name,
      target: definitionTargetForNode({ kind: 'component', value: name, pageId: page.id }, metadata)
    }))) : '',
    contractRefs.length ? detailNodeList('Contracts', contractRefs.map((contract) => ({
      label: contract.label,
      target: definitionTargetForNode({ kind: 'contract', value: contract.value, pageId: page.id }, metadata) || sourceLocation(page)
    }))) : '',
    assets.length ? `Assets: ${escapeHTML(assets.join(', '))}` : ''
  ].filter(Boolean);
  return `<section class="page">
    <div class="route">${nodeLink(page.route || '(missing route)', definitionTargetForNode({ kind: 'route', value: page.route, pageId: page.id }, metadata) || sourceLocation(page), 'node-link route-link')}</div>
    <div class="flow">${escapeHTML(pageFlow(page))}</div>
    <div class="meta">${tags.map((tag) => chipNode(tag.label, tag.target)).join('')}</div>
    <div class="file">${escapeHTML(page.id)} · ${escapeHTML(rel || page.source || '')}</div>
    ${details.length ? `<div class="details">${details.map((item) => `<div>${item}</div>`).join('')}</div>` : ''}
    <div class="actions">
      <button class="icon-button" data-open="${escapeAttr(page.source)}" title="Open Page File" aria-label="Open Page File">${iconSVG('open')}</button>
      <button class="icon-button secondary" data-move="${escapeAttr(page.source)}" title="Move File" aria-label="Move File">${iconSVG('move')}</button>
    </div>
  </section>`;
}

function detailNodeList(label, nodes) {
  return `${escapeHTML(label)}: ${nodes.map((node) => nodeLink(node.label, node.target)).join(', ')}`;
}

function routeNodesForPage(page = {}, metadata = {}) {
  const routes = siteMapRouteEntries(metadata).filter((route) => routeBelongsToPage(route, page));
  if (routes.length > 0) {
    return routes.map((route) => ({
      label: routeNodeLabel(route),
      target: sourceLocation(route) || definitionTargetForNode({ kind: 'route', value: route.route, pageId: route.pageId || route.pageID || page.id }, metadata) || sourceLocation(page)
    }));
  }
  if (!page.route) {
    return [];
  }
  return [{
    label: routeNodeLabel({
      kind: page.render || 'spa',
      method: 'GET',
      route: page.route,
      pageId: page.id
    }),
    target: definitionTargetForNode({ kind: 'route', value: page.route, pageId: page.id }, metadata) || sourceLocation(page)
  }];
}

function endpointNodesForPage(page = {}, metadata = {}) {
  const endpoints = siteMapEndpointEntries(metadata).filter((endpoint) => endpointBelongsToPage(endpoint, page));
  if (endpoints.length > 0) {
    return endpoints.map((endpoint) => ({
      label: endpointNodeLabel(endpoint),
      target: sourceLocation(endpoint) || endpointFallbackTarget(endpoint, page, metadata)
    }));
  }
  const blocks = page.blocks || {};
  return [
    ...(blocks.actions || []).map((name) => fallbackEndpointNode(page, metadata, 'action', name, 'POST')),
    ...(blocks.apis || []).map((name) => fallbackEndpointNode(page, metadata, 'api', name, 'API')),
    ...(blocks.fragments || []).map((name) => fallbackEndpointNode(page, metadata, 'fragment', name, 'GET'))
  ];
}

function fallbackEndpointNode(page, metadata, kind, name, method) {
  return {
    label: endpointNodeLabel({
      kind,
      method,
      symbol: name,
      route: '',
      bindingStatus: ''
    }),
    target: definitionTargetForNode({ kind: 'endpoint', endpointKind: kind, value: name, pageId: page.id }, metadata) || sourceLocation(page)
  };
}

function siteMapRouteEntries(metadata = {}) {
  return [
    ...(((metadata.siteMap || {}).routes) || []),
    ...(metadata.routes || [])
  ];
}

function siteMapEndpointEntries(metadata = {}) {
  return [
    ...(((metadata.siteMap || {}).endpoints) || []),
    ...(metadata.endpoints || [])
  ];
}

function routeBelongsToPage(route = {}, page = {}) {
  const pageID = route.pageId || route.pageID;
  if (pageID && page.id) {
    return pageID === page.id;
  }
  return Boolean(route.route && page.route && route.route === page.route);
}

function endpointBelongsToPage(endpoint = {}, page = {}) {
  const pageID = endpoint.pageId || endpoint.pageID;
  if (pageID && page.id) {
    return pageID === page.id;
  }
  return Boolean(endpoint.route && page.route && endpoint.route === page.route);
}

function routeNodeLabel(route = {}) {
  return [
    route.method || 'GET',
    route.route || '(missing route)',
    route.kind || 'route',
    `[${routeStatus(route)}]`
  ].join(' ');
}

function endpointNodeLabel(endpoint = {}) {
  return [
    endpoint.method || endpointMethodFallback(endpoint.kind),
    endpoint.route || '',
    `${endpoint.kind || 'endpoint'}:${endpointName(endpoint)}`,
    `[${endpointStatus(endpoint)}]`
  ].filter(Boolean).join(' ');
}

function endpointName(endpoint = {}) {
  return endpoint.symbol ||
    endpoint.name ||
    (endpoint.contract && endpoint.contract.name) ||
    endpoint.handler ||
    '(unnamed)';
}

function endpointFallbackTarget(endpoint, page, metadata) {
  return definitionTargetForNode({
    kind: 'endpoint',
    endpointKind: endpoint.kind,
    value: endpointName(endpoint),
    pageId: endpoint.pageId || endpoint.pageID || page.id,
    route: endpoint.route,
    method: endpoint.method
  }, metadata) || sourceLocation(page);
}

function endpointMethodFallback(kind) {
  if (kind === 'action' || kind === 'command') {
    return 'POST';
  }
  return 'GET';
}

function routeStatus(route = {}) {
  switch (route.kind) {
    case 'static':
    case 'spa':
      return 'implemented';
    case 'ssr':
      return 'partial';
    case 'hybrid':
      return 'planned';
    default:
      return 'partial';
  }
}

function endpointStatus(endpoint = {}) {
  const status = endpoint.bindingStatus || endpoint.status || (endpoint.contract && endpoint.contract.status) || '';
  switch (status) {
    case 'bound':
      return 'implemented';
    case 'missing':
      return 'missing';
    case 'unsupported_signature':
    case 'invalid':
      return 'unsupported';
    default:
      return 'partial';
  }
}

function chipNode(label, target) {
  if (!label) {
    return '';
  }
  if (!target) {
    return `<span class="pill">${escapeHTML(label)}</span>`;
  }
  return nodeLink(label, target, 'pill node-link');
}

function nodeLink(label, target, className = 'node-link') {
  if (!target || !target.file) {
    return escapeHTML(label);
  }
  return `<button class="${escapeAttr(className)}" ${definitionTargetAttrs(target)} title="Open source">${escapeHTML(label)}</button>`;
}

function definitionTargetAttrs(target) {
  const line = target.line === undefined || target.line === null ? 0 : target.line;
  const column = target.column === undefined || target.column === null ? 0 : target.column;
  return [
    `data-definition-file="${escapeAttr(target.file)}"`,
    `data-definition-line="${escapeAttr(String(line))}"`,
    `data-definition-column="${escapeAttr(String(column))}"`
  ].join(' ');
}

function pageContractRefs(page = {}) {
  const refs = [];
  for (const item of page.imports || []) {
    if (item.alias) {
      refs.push({ label: item.alias, value: item.alias });
    }
  }
  for (const store of page.stores || []) {
    collectContractRef(refs, store.type || store.Type);
    collectContractRef(refs, store.init || store.Init);
  }
  return uniqueBy(refs, (item) => item.label);
}

function collectContractRef(refs, ref) {
  const label = formatGoRef(ref);
  if (!label) {
    return;
  }
  refs.push({ label, value: ref.name || ref.Name || ref.alias || ref.Alias || label });
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
  const render = page.render || 'spa';
  const artifacts = page.artifacts || [];
  const htmlArtifact = artifacts.find((artifact) => artifact.kind === 'html' && artifact.path);
  const output = render === 'ssr'
    ? 'request-time HTML'
    : (htmlArtifact && htmlArtifact.path) || 'generated HTML';
  const steps = [`GET ${route}`, render, output];
  const actions = (page.blocks && page.blocks.actions) || [];
  const apis = (page.blocks && page.blocks.apis) || [];
  const fragments = (page.blocks && page.blocks.fragments) || [];
  const sideEffects = [
    ...actions.map((name) => `POST act:${name}`),
    ...apis.map((name) => `API ${name || '(unnamed)'}`),
    ...fragments.map((name) => `FRAGMENT ${name || '(unnamed)'}`)
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

function projectLayouts(metadata = {}) {
  const layouts = new Map();
  for (const [id, layout] of Object.entries((metadata.manifest && metadata.manifest.layouts) || {})) {
    layouts.set(id, { id, ...layout, pages: [] });
  }
  for (const page of projectPages(metadata)) {
    for (const layoutID of page.layouts || []) {
      const existing = layouts.get(layoutID) || { id: layoutID, pages: [] };
      layouts.set(layoutID, {
        ...existing,
        pages: [...(existing.pages || []), page]
      });
    }
  }
  return Array.from(layouts.values()).sort((left, right) => left.id.localeCompare(right.id));
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
    ['package', 'Declare the GOWDK package name.'],
    ['import', 'Import a normal Go package for build functions or typed contracts.'],
    ['use', 'Import a discovered GOWDK source package with an alias.'],
    ['page', 'Declare the page id.'],
    ['route', 'Declare the route path.'],
    ['title', 'Declare the generated document title.'],
    ['description', 'Declare the generated document description.'],
    ['canonical', 'Declare the generated canonical URL.'],
    ['image', 'Declare the generated social preview image URL.'],
    ['layout', 'Declare one or more layout ids.'],
    ['guard', 'Declare route guards.'],
    ['component', 'Declare the component name.'],
    ['css', 'Select page CSS inputs: default, page, none, or discovered CSS names.'],
    ['paths', 'Build-time dynamic route path block.'],
    ['build', 'Build-time data block.'],
    ['load', 'Request-time data block.'],
    ['act', 'Action endpoint declaration: act Submit POST "/path".'],
    ['api', 'API endpoint declaration: api Status GET "/path".'],
    ['fragment', 'Server fragment block inside an action.'],
    ['props', 'Component prop declarations block.'],
    ['state', 'Component state contract declaration.'],
    ['client', 'Component browser island behavior block.'],
    ['emits', 'Component event declarations block.'],
    ['script', 'Inline script block.'],
    ['view', 'Markup render block.'],
    ['fn', 'Declare a component client function.'],
    ['async fn', 'Declare an async component client function.'],
    ['computed', 'Declare a computed component-local value.'],
    ['on mount', 'Declare island setup statements.'],
    ['on destroy', 'Declare island cleanup statements.'],
    ['effect when', 'Declare state-dependent island effect statements.'],
    ['ref', 'Declare a safe DOM ref.'],
    ['emit', 'Dispatch a declared component event.'],
    ['let', 'Declare a scalar client local.'],
    ['return', 'Return a client helper, computed value, or effect cleanup block.'],
    ['await fetchJSON', 'Fetch JSON inside an async client function.'],
    ['append', 'Append an item to a state array.'],
    ['remove', 'Remove an item from a state array.'],
    ['move', 'Move an item inside a state array.'],
    ['len', 'Return the length of a string or array.'],
    ['lower', 'Lowercase a string expression.'],
    ['upper', 'Uppercase a string expression.'],
    ['contains', 'Check whether one string contains another.'],
    ['string', 'Convert a scalar expression to string.'],
    ['int', 'Convert a string or number expression to int.'],
    ['float', 'Convert a string or number expression to float.'],
    ...directiveCompletionEntries()
  ];
}

function completionContext(linePrefix) {
  const prefix = String(linePrefix || '');
  if (/\{[A-Za-z0-9_.-]*$/.test(prefix)) {
    return 'dataField';
  }
  if (/layout\s+(?:[A-Za-z0-9_.-]+,\s*)*[A-Za-z0-9_.-]*$/.test(prefix)) {
    return 'layout';
  }
  if (/css\s+(?:[A-Za-z0-9_.-]+(?:\s*,\s*|\s+))*[A-Za-z0-9_.-]*$/.test(prefix)) {
    return 'css';
  }
  if (/\bg:island\s*=\s*"?[A-Za-z]*$/.test(prefix)) {
    return 'island';
  }
  if (/\b(?:g:|class:|style:)[A-Za-z0-9_.:%-]*$/.test(prefix)) {
    return 'directive';
  }
  if (/<[A-Z][A-Za-z0-9_]*$/.test(prefix)) {
    return 'component';
  }
  if (/(route|->|GET|POST|PUT|PATCH|DELETE)\s+"[^"]*$/.test(prefix)) {
    return 'route';
  }
  return 'keyword';
}

function projectCompletionEntries(context, metadata = {}) {
  if (context === 'dataField') {
    return projectDataFields(metadata)
      .map((field) => [field.name, dataFieldDetail(field)]);
  }
  if (context === 'island') {
    return [['wasm', 'Use explicit WASM island assets for this component call.']];
  }
  if (context === 'directive') {
    return directiveCompletionEntries();
  }
  if (context === 'layout') {
    return projectLayouts(metadata)
      .map((layout) => [layout.id, layout.source ? 'Layout from project manifest.' : 'Layout id from project metadata.']);
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

function directiveCompletionEntries() {
  return [
    ['g:post', 'Bind a form to an action.'],
    ['g:target', 'Select partial update target.'],
    ['g:swap', 'Select partial update swap behavior.'],
    ['g:on:', 'Bind a stateful component event listener.'],
    ['g:ref', 'Bind a declared DOM ref.'],
    ['g:if', 'Render a branch when a bool expression is true.'],
    ['g:else-if', 'Continue a conditional branch chain.'],
    ['g:else', 'Declare the fallback branch in a conditional chain.'],
    ['g:for', 'Render rows from an array expression.'],
    ['g:key', 'Declare a stable key for g:for rows.'],
    ['g:bind:value', 'Two-way bind a form value to state.'],
    ['g:bind:checked', 'Two-way bind checkbox checked state.'],
    ['g:island', 'Select component island runtime mode.'],
    ['class:', 'Toggle a CSS class from a bool expression.'],
    ['style:', 'Bind a safe style property from a scalar expression.']
  ];
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
  return withSurfaceStatus(lines, 'css');
}

function languageSurfaceMarkdown(value) {
  switch (value) {
    case 'spa':
      return withSurfaceStatus([`**GOWDK render mode** \`${value}\``], 'spa');
    case 'ssr':
      return withSurfaceStatus([`**GOWDK render mode** \`${value}\``], 'ssr');
    case 'hybrid':
      return withSurfaceStatus([`**GOWDK render mode** \`${value}\``], 'hybrid');
    case 'load':
      return withSurfaceStatus(['**GOWDK block** `load {}`'], 'ssr');
    case 'paths':
      return withSurfaceStatus(['**GOWDK block** `paths {}`'], 'route');
    case 'script':
      return withSurfaceStatus(['**GOWDK block** `script {}`'], 'unsupported');
    default:
      return '';
  }
}

function withSurfaceStatus(lines, surface) {
  const status = SURFACE_STATUS[surface];
  if (!status) {
    return formatHoverLines(lines);
  }
  const next = lines.slice();
  next.push('', `Status: \`${status.status}\``);
  if (status.docs) {
    next.push(`Docs: [${status.docs}](${DOCS_BASE_URL}${status.docs})`);
  }
  if (status.limit) {
    next.push(`Current limit: ${status.limit}`);
  }
  return formatHoverLines(next);
}

function formatHoverLines(lines) {
  const compact = [];
  for (const line of lines) {
    if (!line) {
      if (compact.length > 0 && compact[compact.length - 1] !== '') {
        compact.push('');
      }
      continue;
    }
    compact.push(line);
  }
  while (compact[compact.length - 1] === '') {
    compact.pop();
  }
  return compact.join('\n');
}

function hoverMarkdown(token, metadata = {}, context = {}) {
  const value = String(token || '');
  if (!value) {
    return '';
  }
  const languageSurface = languageSurfaceMarkdown(value);
  if (languageSurface) {
    return languageSurface;
  }
  const dataField = projectDataFields(metadata).find((field) => field.name === value);
  if (dataField) {
    const lines = [
      `**GOWDK data field** \`${escapeMarkdown(value)}\``,
      '',
      `Lane: \`${escapeMarkdown(dataField.lane)}\``
    ];
    if (dataField.type) {
      lines.push(`Type: \`${escapeMarkdown(dataField.type)}\``);
    }
    if (dataField.origin) {
      lines.push(`From: \`${escapeMarkdown(dataField.origin)}\``);
    }
    if (dataField.goField) {
      lines.push(`Go field: \`${escapeMarkdown(dataField.goField)}\``);
    }
    return withSurfaceStatus(lines, 'dataField');
  }
  const pages = projectPages(metadata);
  const manifest = metadata.manifest || {};
  const page = pages.find((item) => item.id === value);
  if (page) {
    return withSurfaceStatus([
      `**GOWDK page** \`${escapeMarkdown(value)}\``,
      '',
      `Route: \`${escapeMarkdown(page.route || '')}\``,
      `Render: \`${escapeMarkdown(page.render || 'spa')}\``
    ], 'page');
  }
  const routePage = pages.find((item) => item.route === value);
  if (routePage) {
    return withSurfaceStatus([
      `**GOWDK route** \`${escapeMarkdown(value)}\``,
      '',
      routePage.id ? `Page: \`${escapeMarkdown(routePage.id)}\`` : '',
      `Render: \`${escapeMarkdown(routePage.render || 'spa')}\``
    ], 'route');
  }
  if (manifest.components && manifest.components[value]) {
    const component = manifest.components[value];
    const props = (component.props || []).map((prop) => `${prop.name} ${prop.type}`).join(', ');
    const emits = (component.emits || []).map(formatEmit).join(', ');
    const state = formatState(component.state);
    return withSurfaceStatus([
      `**GOWDK component** \`${escapeMarkdown(value)}\``,
      '',
      props ? `Props: \`${escapeMarkdown(props)}\`` : 'Props: none',
      state ? `State: \`${escapeMarkdown(state)}\`` : '',
      emits ? `Emits: \`${escapeMarkdown(emits)}\`` : ''
    ], 'component');
  }
  const event = componentEvent(value, metadata, context);
  if (event) {
    return withSurfaceStatus([
      `**GOWDK component event** \`${escapeMarkdown(value)}\``,
      '',
      `Component: \`${escapeMarkdown(event.component)}\``,
      event.params.length ? `Payload: \`${escapeMarkdown(event.params.join(', '))}\`` : 'Payload: none'
    ], 'componentEvent');
  }
  const store = projectStores(metadata).find((item) => item.name === value);
  if (store) {
    return withSurfaceStatus([
      `**GOWDK store** \`${escapeMarkdown(value)}\``,
      '',
      store.page ? `Page: \`${escapeMarkdown(store.page)}\`` : '',
      store.type ? `Type: \`${escapeMarkdown(store.type)}\`` : '',
      store.init ? `Init: \`${escapeMarkdown(store.init)}\`` : ''
    ], 'store');
  }
  const goContract = projectGoContracts(metadata).find((item) => item.name === value || item.alias === value);
  if (goContract) {
    return withSurfaceStatus([
      `**GOWDK Go contract** \`${escapeMarkdown(value)}\``,
      '',
      goContract.alias ? `Import alias: \`${escapeMarkdown(goContract.alias)}\`` : '',
      goContract.path ? `Import path: \`${escapeMarkdown(goContract.path)}\`` : '',
      goContract.owner ? `Declared by: \`${escapeMarkdown(goContract.owner)}\`` : ''
    ], 'goContract');
  }
  const layout = projectLayouts(metadata).find((item) => item.id === value);
  if (layout) {
    const lines = [
      `**GOWDK layout** \`${escapeMarkdown(value)}\``,
      ''
    ];
    if (layout.source) {
      lines.push(`Source: \`${escapeMarkdown(layout.source)}\``);
    }
    const layoutPages = layout.pages || [];
    if (layoutPages.length > 0) {
      lines.push(`Referenced by ${layoutPages.length} page${layoutPages.length === 1 ? '' : 's'}.`);
    }
    return withSurfaceStatus(lines, 'layout');
  }
  const cssMarkdown = cssInputMarkdown(value, metadata);
  if (cssMarkdown) {
    return cssMarkdown;
  }
  for (const item of pages) {
    const actions = (item.blocks && item.blocks.actions) || [];
    if (actions.includes(value)) {
      return withSurfaceStatus([
        `**GOWDK action** \`${escapeMarkdown(value)}\``,
        '',
        `Page: \`${escapeMarkdown(item.id || '')}\``
      ], 'action');
    }
    const apis = (item.blocks && item.blocks.apis) || [];
    if (apis.includes(value)) {
      return withSurfaceStatus([
        `**GOWDK API** \`${escapeMarkdown(value)}\``,
        '',
        `Page: \`${escapeMarkdown(item.id || '')}\``
      ], 'api');
    }
  }
  return '';
}

function definitionTargetForNode(node = {}, metadata = {}, context = {}) {
  const kind = String(node.kind || '');
  const value = String(node.value || node.symbol || node.route || node.id || '');
  const pages = projectPages(metadata);
  if (kind === 'page') {
    const page = pageByIDOrRoute(pages, value, node.pageId);
    return sourceLocation(page);
  }
  if (kind === 'route') {
    const route = String(node.route || value);
    const page = pages.find((item) => item.route === route) || pageByIDOrRoute(pages, '', node.pageId);
    return sourceLocation(page);
  }
  if (kind === 'endpoint' || kind === 'action' || kind === 'api') {
    return endpointDefinitionTarget(node, metadata, pages);
  }
  if (kind === 'block') {
    const page = pageByIDOrRoute(pages, '', node.pageId);
    return sourceLocation(page);
  }
  if (kind === 'contract' || kind === 'goContract') {
    const goContract = projectGoContracts(metadata).find((item) => item.name === value || item.alias === value);
    if (goContract) {
      return sourceLocation(goContract);
    }
  }
  return definitionTarget(value, metadata, context);
}

function definitionTarget(token, metadata = {}, context = {}) {
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
  const routePage = pages.find((item) => item.route === value);
  if (routePage && routePage.source) {
    return sourceLocation(routePage);
  }
  const component = manifest.components && manifest.components[value];
  if (component && component.source) {
    return { file: component.source, line: 0, column: 0 };
  }
  const event = componentEvent(value, metadata, context);
  if (event && event.source) {
    return { file: event.source, line: 0, column: 0 };
  }
  const store = projectStores(metadata).find((item) => item.name === value);
  if (store && store.source) {
    return { file: store.source, line: 0, column: 0 };
  }
  const goContract = projectGoContracts(metadata).find((item) => item.name === value || item.alias === value);
  if (goContract && goContract.source) {
    return { file: goContract.source, line: 0, column: 0 };
  }
  const layout = projectLayouts(metadata).find((item) => item.id === value);
  if (layout && layout.source) {
    return { file: layout.source, line: 0, column: 0 };
  }
  const endpoint = endpointDefinitionTarget({ value }, metadata, pages);
  if (endpoint) {
    return endpoint;
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

function endpointDefinitionTarget(node = {}, metadata = {}, pages = projectPages(metadata)) {
  const value = String(node.value || node.symbol || '');
  const pageID = String(node.pageId || node.pageID || '');
  const endpointKind = String(node.endpointKind || (node.kind === 'action' || node.kind === 'api' ? node.kind : ''));
  const endpoints = [
    ...(((metadata.siteMap || {}).endpoints) || []),
    ...(metadata.endpoints || [])
  ];
  for (const endpoint of endpoints) {
    if (!endpointMatchesNode(endpoint, value, pageID, endpointKind, node)) {
      continue;
    }
    const target = sourceLocation(endpoint);
    if (target) {
      return target;
    }
    const page = pageByIDOrRoute(pages, '', endpoint.pageId || endpoint.pageID || pageID);
    if (page) {
      return sourceLocation(page);
    }
  }
  for (const page of pages) {
    if (pageID && page.id !== pageID) {
      continue;
    }
    const actions = (page.blocks && page.blocks.actions) || [];
    const apis = (page.blocks && page.blocks.apis) || [];
    if ((!endpointKind || endpointKind === 'action') && actions.includes(value)) {
      return sourceLocation(page);
    }
    if ((!endpointKind || endpointKind === 'api') && apis.includes(value)) {
      return sourceLocation(page);
    }
  }
  return undefined;
}

function endpointMatchesNode(endpoint = {}, value, pageID, endpointKind, node = {}) {
  if (pageID && endpoint.pageId !== pageID && endpoint.pageID !== pageID) {
    return false;
  }
  if (endpointKind && endpoint.kind !== endpointKind) {
    return false;
  }
  if (node.method && endpoint.method !== node.method) {
    return false;
  }
  if (node.route && endpoint.route !== node.route) {
    return false;
  }
  if (!value) {
    return true;
  }
  return endpoint.symbol === value ||
    endpoint.blockName === value ||
    endpoint.name === value ||
    endpoint.handler === value ||
    endpoint.functionName === value;
}

function pageByIDOrRoute(pages = [], value = '', pageID = '') {
  if (pageID) {
    const page = pages.find((item) => item.id === pageID);
    if (page) {
      return page;
    }
  }
  return pages.find((item) => item.id === value || item.route === value);
}

function sourceLocation(item = {}) {
  const file = item && (item.source || item.file);
  if (!file) {
    return undefined;
  }
  const directLine = numericValue(item.line);
  const directColumn = numericValue(item.column);
  if (directLine !== undefined || directColumn !== undefined) {
    return {
      file,
      line: Math.max(directLine || 0, 0),
      column: Math.max(directColumn || 0, 0)
    };
  }
  const span = item.sourceSpan || item.SourceSpan || item.span || item.Span;
  const start = span && (span.start || span.Start);
  const spanLine = numericValue(start && (start.line || start.Line));
  const spanColumn = numericValue(start && (start.column || start.Column));
  if (spanLine !== undefined || spanColumn !== undefined) {
    return {
      file,
      line: Math.max((spanLine || 1) - 1, 0),
      column: Math.max((spanColumn || 1) - 1, 0)
    };
  }
  return fileLocation(file);
}

function numericValue(value) {
  if (value === undefined || value === null || value === '') {
    return undefined;
  }
  const number = Number(value);
  return Number.isFinite(number) ? number : undefined;
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
  const event = componentEvent(value, metadata, options.symbolContext);
  if (event && event.source && includeDeclaration) {
    refs.push(fileLocation(event.source));
  }
  const store = projectStores(metadata).find((item) => item.name === value);
  if (store && store.source && includeDeclaration) {
    refs.push(fileLocation(store.source));
  }
  const goContract = projectGoContracts(metadata).find((item) => item.name === value || item.alias === value);
  if (goContract && goContract.source && includeDeclaration) {
    refs.push(fileLocation(goContract.source));
  }
  const layout = projectLayouts(metadata).find((item) => item.id === value);
  if (layout && layout.source && includeDeclaration) {
    refs.push(fileLocation(layout.source));
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
    if ((item.stores || []).some((store) => store.name === value)) {
      refs.push(fileLocation(item.source));
    }
  }
  for (const item of Object.values(manifest.components || {})) {
    if (!item.source) {
      continue;
    }
    if (componentUsesGoContract(item, value)) {
      refs.push(fileLocation(item.source));
    }
  }
	return uniqueLocations(refs);
}

function projectStores(metadata = {}) {
  const stores = [];
  for (const page of projectPages(metadata)) {
    for (const store of page.stores || []) {
      stores.push(normalizeStore(store, page));
    }
  }
  for (const [id, page] of Object.entries((metadata.manifest && metadata.manifest.pages) || {})) {
    for (const store of page.stores || []) {
      stores.push(normalizeStore(store, { id, ...page }));
    }
  }
  return uniqueBy(stores.filter(Boolean), (store) => `${store.source || ''}\0${store.name}`);
}

function normalizeStore(store = {}, page = {}) {
  const name = store.name || store.Name;
  if (!name) {
    return undefined;
  }
  return {
    name: String(name),
    page: page.id || '',
    source: store.source || page.source || '',
    type: formatGoRef(store.type || store.Type),
    init: formatGoRef(store.init || store.Init)
  };
}

function projectGoContracts(metadata = {}) {
  const contracts = [];
  for (const [id, page] of Object.entries((metadata.manifest && metadata.manifest.pages) || {})) {
    collectGoContractsFromOwner(contracts, { id, ...page }, `page ${id}`);
  }
  for (const [name, component] of Object.entries((metadata.manifest && metadata.manifest.components) || {})) {
    collectGoContractsFromOwner(contracts, { name, ...component }, `component ${name}`);
  }
  return uniqueBy(contracts.filter(Boolean), (contract) => `${contract.source || ''}\0${contract.alias || ''}\0${contract.name || ''}`);
}

function collectGoContractsFromOwner(contracts, owner = {}, label = '') {
  for (const item of owner.imports || []) {
    contracts.push({
      alias: item.alias || '',
      name: item.alias || '',
      path: item.path || '',
      source: owner.source || '',
      owner: label
    });
  }
  collectGoRefContract(contracts, owner.propsType, owner, label);
  collectGoRefContract(contracts, owner.state && owner.state.type, owner, label);
  collectGoRefContract(contracts, owner.state && owner.state.init, owner, label);
  for (const store of owner.stores || []) {
    collectGoRefContract(contracts, store.type, owner, label);
    collectGoRefContract(contracts, store.init, owner, label);
  }
}

function collectGoRefContract(contracts, ref, owner = {}, label = '') {
  if (!ref || !ref.name) {
    return;
  }
  contracts.push({
    alias: ref.alias || '',
    name: ref.name || '',
    path: importPathForAlias(owner.imports || [], ref.alias),
    source: owner.source || '',
    owner: label
  });
}

function importPathForAlias(imports = [], alias = '') {
  const item = imports.find((entry) => entry.alias === alias);
  return item ? item.path || '' : '';
}

function componentUsesGoContract(component = {}, value = '') {
  if (!value) {
    return false;
  }
  const refs = [
    component.propsType,
    component.state && component.state.type,
    component.state && component.state.init
  ].filter(Boolean);
  if (refs.some((ref) => ref.name === value || ref.alias === value)) {
    return true;
  }
  return (component.imports || []).some((item) => item.alias === value);
}

function formatGoRef(ref) {
  if (!ref || !ref.name) {
    return '';
  }
  return [ref.alias, ref.name].filter(Boolean).join('.');
}

function canRenameSymbol(token, metadata = {}) {
  if (componentEvent(token, metadata)) {
    return false;
  }
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
  if (componentEvent(value, metadata)) {
    return true;
  }
  if (projectStores(metadata).some((store) => store.name === value)) {
    return true;
  }
  if (projectGoContracts(metadata).some((item) => item.name === value || item.alias === value)) {
    return true;
  }
  if (projectLayouts(metadata).some((layout) => layout.id === value)) {
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
    collectPatternTokens(tokens, line, text, /^(\s*)(page|route|title|description|canonical|image|layout|render|guard|component|css|cache|revalidate|error|wasm|asset)\b/g, 'namespace', 2);
    collectPatternTokens(tokens, line, text, /\b(package|import|use|paths|build|load|act|api|fragment|view|script|go|style|props|state|exports|client|emits)\b/g, 'keyword');
    collectPatternTokens(tokens, line, text, /\b(async|fn|computed|on|mount|destroy|effect|when|ref|let|return|await|if|else|in|emit)\b/g, 'keyword');
    collectPatternTokens(tokens, line, text, /\b(GET|POST|PUT|PATCH|DELETE)\b/g, 'enumMember');
    collectPatternTokens(tokens, line, text, /\b(spa|action|hybrid|ssr)\b/g, 'enumMember');
    collectPatternTokens(tokens, line, text, /\b(string|int|float|bool)\b/g, 'enumMember');
    collectPatternTokens(tokens, line, text, /\bg:(post|target|swap|ref|if|else-if|else|for|key|bind:(?:value|checked)|island)\b/g, 'property');
    collectPatternTokens(tokens, line, text, /\bg:on:[A-Za-z][A-Za-z0-9_-]*(?:\.(?:prevent|stop|once|capture|debounce\([^)]+\)|throttle\([^)]+\)))*/g, 'property');
    collectPatternTokens(tokens, line, text, /\bclass:[A-Za-z_][A-Za-z0-9_-]*/g, 'property');
    collectPatternTokens(tokens, line, text, /\bstyle:[A-Za-z_][A-Za-z0-9_-]*(?:\.(?:%|[A-Za-z][A-Za-z0-9_-]*))?/g, 'property');
    collectPatternTokens(tokens, line, text, /<\/?([A-Z][A-Za-z0-9_]*)\b/g, 'class', 1);
    collectPatternTokens(tokens, line, text, /\b(?:act|api)\s+([A-Za-z_][A-Za-z0-9_]*)/g, 'function', 1);
    collectPatternTokens(tokens, line, text, /\b(?:async\s+)?fn\s+([A-Za-z_][A-Za-z0-9_]*)/g, 'function', 1);
    collectPatternTokens(tokens, line, text, /\bcomputed\s+([A-Za-z_][A-Za-z0-9_]*)/g, 'property', 1);
    collectPatternTokens(tokens, line, text, /\bref\s+([A-Za-z_][A-Za-z0-9_]*)/g, 'property', 1);
    collectPatternTokens(tokens, line, text, /\bemit\s+([A-Za-z_][A-Za-z0-9_]*)/g, 'function', 1);
    collectPatternTokens(tokens, line, text, /\b(len|lower|upper|contains|string|int|float|append|remove|move|fetchJSON)\s*(?:\[|\()/g, 'function', 1);
    collectCSSReferenceTokens(tokens, line, text);
  }
  return withoutOverlaps(tokens).sort((a, b) => a.line - b.line || a.column - b.column || a.length - b.length);
}

function documentOutlineItems(source) {
  const lines = String(source || '').split(/\r?\n/);
  const items = [];
  for (let line = 0; line < lines.length; line++) {
    const text = lines[line];
    const trimmed = text.trim();
    if (!trimmed || trimmed.startsWith('//')) {
      continue;
    }
    const metadata = outlineMetadata(text, line);
    if (metadata) {
      items.push(metadata);
      continue;
    }
    const block = outlineBlock(text, line, lines);
    if (block) {
      items.push(block);
      line = Math.max(line, block.range.end.line);
      continue;
    }
    const endpoint = outlineEndpoint(text, line);
    if (endpoint) {
      items.push(endpoint);
    }
  }
  return items;
}

function outlineMetadata(text, line) {
  const match = text.match(/^(\s*)(page|component|layout|route|title|description|canonical|image|guard|css|render|cache|revalidate|error|wasm|asset)\b\s*(.*)$/);
  if (!match) {
    return undefined;
  }
  const name = match[2];
  const detail = match[3].trim();
  return outlineItem({
    name: detail ? `${name} ${detail}` : name,
    detail,
    kind: metadataOutlineKind(match[2]),
    line,
    column: match[1].length,
    length: text.trimEnd().length - match[1].length
  });
}

function outlineBlock(text, line, lines) {
  const match = text.match(/^(\s*)((?:go(?:\s+[A-Za-z0-9_.-]+)?)|paths|build|load|view|script|style|props|client|emits|exports|fragment|state\s+[^{}]+|act\s+\w+(?:\s+\w+)?(?:\s+"[^"]*")?|api(?:\s+\w+)?(?:\s+\w+)?(?:\s+"[^"]*")?|(?:async\s+)?fn\s+\w+\([^)]*\)(?:\s+\w+)?|computed\s+\w+\s+\w+|on\s+(?:mount|destroy)|effect\s+when\s+\w+)\s*\{/);
  if (!match) {
    return undefined;
  }
  const label = match[2].trim();
  const range = outlineBlockRange(lines, line, match[0].length);
  return {
    name: label,
    detail: blockOutlineDetail(label),
    kind: blockOutlineKind(label),
    range,
    selectionRange: {
      start: { line, column: match[1].length },
      end: { line, column: match[1].length + label.length }
    },
    children: nestedOutlineItems(lines, line + 1, range.end.line)
  };
}

function nestedOutlineItems(lines, startLine, endLine) {
  const items = [];
  for (let line = startLine; line < endLine; line++) {
    const text = lines[line];
    const item = outlineBlock(text, line, lines) || outlineEndpoint(text, line);
    if (!item) {
      continue;
    }
    items.push(item);
    if (item.range) {
      line = Math.max(line, item.range.end.line);
    }
  }
  return items;
}

function outlineEndpoint(text, line) {
  const match = text.match(/^(\s*)(act\s+\w+\s+\w+\s+"[^"]*"|api(?:\s+\w+)?\s+\w+\s+"[^"]*")\b/);
  if (!match) {
    return undefined;
  }
  return outlineItem({
    name: match[2],
    detail: match[2].startsWith('act ') ? 'action endpoint' : 'api endpoint',
    kind: 'function',
    line,
    column: match[1].length,
    length: match[2].length
  });
}

function outlineBlockRange(lines, startLine, headerLength) {
  let depth = 0;
  for (let line = startLine; line < lines.length; line++) {
    const text = lines[line];
    const startColumn = line === startLine ? Math.max(0, headerLength - 1) : 0;
    for (let column = startColumn; column < text.length; column++) {
      const char = text[column];
      if (char === '{') {
        depth++;
      }
      if (char === '}') {
        depth--;
        if (depth <= 0) {
          return {
            start: { line: startLine, column: 0 },
            end: { line, column: column + 1 }
          };
        }
      }
    }
  }
  const lastLine = Math.max(startLine, lines.length - 1);
  return {
    start: { line: startLine, column: 0 },
    end: { line: lastLine, column: (lines[lastLine] || '').length }
  };
}

function outlineItem({ name, detail, kind, line, column, length }) {
  return {
    name,
    detail,
    kind,
    range: {
      start: { line, column },
      end: { line, column: column + Math.max(length, 1) }
    },
    selectionRange: {
      start: { line, column },
      end: { line, column: column + Math.max(String(name || '').split(/\s+/)[0].length, 1) }
    },
    children: []
  };
}

function metadataOutlineKind(name) {
  if (name === 'page' || name === 'layout') {
    return 'namespace';
  }
  if (name === 'component') {
    return 'class';
  }
  return 'property';
}

function blockOutlineKind(label) {
  if (label === 'view') {
    return 'object';
  }
  if (label === 'script' || label === 'client' || label.startsWith('go')) {
    return 'module';
  }
  if (label.startsWith('act ') || label.startsWith('api ') || label.includes('fn ')) {
    return 'function';
  }
  if (label.startsWith('state ') || label.startsWith('props')) {
    return 'struct';
  }
  return 'property';
}

function blockOutlineDetail(label) {
  if (label === 'view') {
    return 'markup block';
  }
  if (label === 'script') {
    return 'script block';
  }
  if (label.startsWith('go')) {
    return 'inline Go block';
  }
  if (label === 'client') {
    return 'client island block';
  }
  return 'block';
}

function componentEvent(name, metadata = {}, context = {}) {
  const events = componentEvents(name, metadata);
  if (context && context.component) {
    return events.find((event) => event.component === context.component);
  }
  return events.length === 1 ? events[0] : undefined;
}

function componentEvents(name, metadata = {}) {
  const manifest = metadata.manifest || {};
  const events = [];
  for (const [componentName, component] of Object.entries(manifest.components || {})) {
    for (const event of component.emits || []) {
      if (event.name === name) {
        events.push({
          component: componentName,
          source: component.source,
          params: (event.params || []).map((param) => `${param.name} ${param.type}`.trim()).filter(Boolean)
        });
      }
    }
  }
  return events;
}

function symbolContext(source, offset) {
  const text = String(source || '');
  const end = Number.isFinite(offset) ? Math.max(0, Math.min(offset, text.length)) : text.length;
  const prefix = text.slice(Math.max(0, end - 4000), end);
  const openIndex = prefix.lastIndexOf('<');
  if (openIndex === -1) {
    return {};
  }
  const fragment = prefix.slice(openIndex);
  if (fragment.startsWith('</') || fragment.includes('>')) {
    return {};
  }
  const match = fragment.match(/^<([A-Z][A-Za-z0-9_]*)\b/);
  return match ? { component: match[1] } : {};
}

function documentDataFields(source, options = {}) {
  const text = String(source || '');
  const fields = [
    ...literalDataFields(text, 'build'),
    ...literalDataFields(text, 'load'),
    ...goCallDataFields(text, options)
  ];
  return uniqueBy(fields.filter(Boolean), (field) => `${field.lane}\0${field.name}\0${field.origin || ''}`);
}

function projectDataFields(metadata = {}) {
  return uniqueBy([...(metadata.dataFields || [])].filter((field) => field && field.name), (field) => `${field.lane || ''}\0${field.name}\0${field.origin || ''}`)
    .sort((left, right) => left.name.localeCompare(right.name) || String(left.lane || '').localeCompare(String(right.lane || '')));
}

function dataFieldDetail(field = {}) {
  const type = field.type ? ` ${field.type}` : '';
  const origin = field.origin ? ` from ${field.origin}` : '';
  return `${field.lane || 'data'} field${type}${origin}.`;
}

function literalDataFields(source, lane) {
  const body = blockBody(source, lane);
  if (!body) {
    return [];
  }
  const fields = [];
  for (const literal of arrowObjectLiterals(body)) {
    for (const part of splitTopLevelCommas(literal)) {
      const item = part.trim().match(/^([A-Za-z_][A-Za-z0-9_.]*)\s*(?=:|$)/);
      if (!item) {
        continue;
      }
      fields.push({
        name: item[1],
        lane,
        origin: `${lane} {}`,
        type: ''
      });
    }
  }
  return fields;
}

function splitTopLevelCommas(source) {
  const parts = [];
  const text = String(source || '');
  let depth = 0;
  let start = 0;
  for (let index = 0; index < text.length; index++) {
    if (text[index] === '{' || text[index] === '[' || text[index] === '(') {
      depth++;
    }
    if (text[index] === '}' || text[index] === ']' || text[index] === ')') {
      depth = Math.max(0, depth - 1);
    }
    if (text[index] === ',' && depth === 0) {
      parts.push(text.slice(start, index));
      start = index + 1;
    }
  }
  parts.push(text.slice(start));
  return parts;
}

function goCallDataFields(source, options = {}) {
  const body = blockBody(source, 'build');
  if (!body) {
    return [];
  }
  const calls = [];
  for (const match of body.matchAll(/=>\s+(?:(([A-Za-z_][A-Za-z0-9_]*)\.)?([A-Za-z_][A-Za-z0-9_]*))\s*\(\s*\)/g)) {
    calls.push({ alias: match[2] || '', functionName: match[3] });
  }
  if (calls.length === 0) {
    return [];
  }
  const imports = gwdkImports(source);
  const goSources = goSourcesForDocument(source, options, imports);
  const fields = [];
  for (const call of calls) {
    const sourceSet = call.alias ? goSources.byAlias.get(call.alias) || [] : goSources.local;
    const resultType = goFunctionReturnType(sourceSet.join('\n'), call.functionName);
    if (!resultType) {
      continue;
    }
    for (const field of goStructFields(sourceSet.join('\n'), resultType)) {
      fields.push({
        ...field,
        lane: 'build',
        origin: call.alias ? `${call.alias}.${call.functionName}()` : `${call.functionName}()`
      });
    }
  }
  return fields;
}

function blockBody(source, name) {
  const text = String(source || '');
  const pattern = new RegExp(`(?:^|\\n)\\s*${escapeRegExp(name)}\\s*\\{`, 'g');
  const match = pattern.exec(text);
  if (!match) {
    return '';
  }
  const open = text.indexOf('{', match.index);
  let depth = 0;
  for (let index = open; index < text.length; index++) {
    const char = text[index];
    if (char === '{') {
      depth++;
    }
    if (char === '}') {
      depth--;
      if (depth === 0) {
        return text.slice(open + 1, index);
      }
    }
  }
  return text.slice(open + 1);
}

function arrowObjectLiterals(body) {
  const text = String(body || '');
  const literals = [];
  for (const match of text.matchAll(/=>\s*\{/g)) {
    const open = match.index + match[0].lastIndexOf('{');
    let depth = 0;
    for (let index = open; index < text.length; index++) {
      if (text[index] === '{') {
        depth++;
      }
      if (text[index] === '}') {
        depth--;
        if (depth === 0) {
          literals.push(text.slice(open + 1, index));
          break;
        }
      }
    }
  }
  return literals;
}

function gwdkImports(source) {
  const imports = new Map();
  for (const match of String(source || '').matchAll(/^\s*import\s+([A-Za-z_][A-Za-z0-9_]*)\s+"([^"]+)"/gm)) {
    imports.set(match[1], match[2]);
  }
  return imports;
}

function goSourcesForDocument(source, options = {}, imports = new Map()) {
  const local = [];
  const byAlias = new Map();
  const inlineGo = blockBody(source, 'go');
  if (inlineGo) {
    local.push(inlineGo);
  }
  const fileName = options.fileName || '';
  if (fileName) {
    local.push(...readGoSources(path.dirname(fileName)));
  }
  const projectRoot = options.projectRoot || '';
  const modulePath = options.modulePath || (projectRoot ? goModModulePath(readText(path.join(projectRoot, 'go.mod'))) : '');
  for (const [alias, importPath] of imports.entries()) {
    const dir = resolveImportDir(importPath, projectRoot, modulePath);
    byAlias.set(alias, dir ? readGoSources(dir) : []);
  }
  return { local, byAlias };
}

function readGoSources(dir) {
  if (!dir) {
    return [];
  }
  try {
    return fs.readdirSync(dir)
      .filter((file) => file.endsWith('.go') && !file.endsWith('_test.go'))
      .map((file) => readText(path.join(dir, file)))
      .filter(Boolean);
  } catch (_error) {
    return [];
  }
}

function resolveImportDir(importPath, projectRoot, modulePath) {
  if (!projectRoot || !modulePath || !importPath.startsWith(modulePath)) {
    return '';
  }
  const suffix = importPath.slice(modulePath.length).replace(/^\/+/, '');
  return path.join(projectRoot, suffix);
}

function readText(file) {
  try {
    return fs.readFileSync(file, 'utf8');
  } catch (_error) {
    return '';
  }
}

function goFunctionReturnType(source, functionName) {
  const name = escapeRegExp(functionName);
  const match = String(source || '').match(new RegExp(`func\\s+${name}\\s*\\([^)]*\\)\\s+(?:\\([^)]*,\\s*error\\)|([A-Za-z_][A-Za-z0-9_]*))`));
  if (!match) {
    return '';
  }
  if (match[1]) {
    return match[1];
  }
  const tuple = match[0].match(/\)\s+\(\s*([A-Za-z_][A-Za-z0-9_]*)\s*,\s*error\s*\)/);
  return tuple ? tuple[1] : '';
}

function goStructFields(source, typeName) {
  const match = String(source || '').match(new RegExp(`type\\s+${escapeRegExp(typeName)}\\s+struct\\s*\\{([\\s\\S]*?)\\n\\}`));
  if (!match) {
    return [];
  }
  const fields = [];
  for (const line of match[1].split(/\r?\n/)) {
    const trimmed = line.trim();
    const field = trimmed.match(/^([A-Z][A-Za-z0-9_]*)\s+([A-Za-z_][A-Za-z0-9_.\[\]]*)(?:\s+`([^`]*)`)?/);
    if (!field) {
      continue;
    }
    const jsonName = jsonTagName(field[3] || '');
    if (jsonName === '-') {
      continue;
    }
    fields.push({
      name: jsonName || lowerFirst(field[1]),
      type: field[2],
      goField: field[1]
    });
  }
  return fields;
}

function jsonTagName(tags) {
  const match = String(tags || '').match(/\bjson:"([^"]*)"/);
  if (!match) {
    return '';
  }
  return match[1].split(',')[0];
}

function lowerFirst(value) {
  const text = String(value || '');
  return text ? text[0].toLowerCase() + text.slice(1) : '';
}

function formatEmit(event = {}) {
  const params = (event.params || []).map((param) => `${param.name} ${param.type}`.trim()).filter(Boolean).join(', ');
  return `${event.name || '(unnamed)'}(${params})`;
}

function formatState(state) {
  if (!state || !state.type || !state.type.name) {
    return '';
  }
  return [state.type.alias, state.type.name].filter(Boolean).join('.');
}

function collectCSSReferenceTokens(tokens, line, text) {
  const match = text.match(/css\s+(.+)$/);
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

function uniqueBy(values, keyFn) {
  const seen = new Set();
  const out = [];
  for (const value of values) {
    const key = keyFn(value);
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    out.push(value);
  }
  return out;
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
  GOWDK_MODULE_PATH,
  SEMANTIC_TOKEN_TYPES,
  completionEntries,
  completionContext,
  canRenameSymbol,
  cssCompletionEntries,
  cssFileEntries,
  definitionTarget,
  definitionTargetForNode,
  diagnosticCodeForMessage,
  diagnosticPosition,
  diagnosticRange,
  diagnosticSeverity,
  documentDataFields,
  documentOutlineItems,
  escapeHTML,
  goModModulePath,
  goModRequiresGOWDK,
  gowdkModuleRunArgs,
  groupDiagnosticsByFile,
  hoverMarkdown,
  isMissingExecutableError,
  isGOWDKSourceDir,
  missingExecutableMessage,
  nearbyGOWDKSourceRoot,
  nearestProjectRoot,
  parseDiagnostics,
  pageFlow,
  projectDataFields,
  projectLayouts,
  projectPages,
  projectCompletionEntries,
  projectCommandArgs,
  quickFixesForDiagnostic,
  renameEditsForSource,
  semanticTokens,
  siteMapHTML,
  symbolContext,
  symbolReferences,
  toolInvocation,
  validRenameValue
};
