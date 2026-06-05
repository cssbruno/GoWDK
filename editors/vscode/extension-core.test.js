const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const test = require('node:test');
const core = require('./extension-core');

test('parseDiagnostics returns an empty list for blank output', () => {
  assert.deepEqual(core.parseDiagnostics(' \n\t'), []);
});

test('parseDiagnostics reads diagnostics from gowdk JSON output', () => {
  const output = JSON.stringify({
    diagnostics: [
      {
        code: 'missing_view_block',
        severity: 'error',
        message: 'page must declare view block',
        pos: { line: 4, column: 2 }
      }
    ]
  });

  assert.deepEqual(core.parseDiagnostics(output), [
    {
      code: 'missing_view_block',
      severity: 'error',
      message: 'page must declare view block',
      pos: { line: 4, column: 2 }
    }
  ]);
});

test('diagnosticPosition converts one-based compiler positions to zero-based editor positions', () => {
  assert.deepEqual(core.diagnosticPosition({ pos: { line: 8, column: 3 } }), {
    line: 7,
    column: 2
  });
  assert.deepEqual(core.diagnosticPosition({}), {
    line: 0,
    column: 0
  });
});

test('diagnosticRange prefers compiler ranges and falls back to one-character positions', () => {
  assert.deepEqual(core.diagnosticRange({
    range: {
      start: { line: 3, column: 1 },
      end: { line: 3, column: 13 }
    },
    pos: { line: 8, column: 3 }
  }), {
    start: { line: 2, column: 0 },
    end: { line: 2, column: 12 }
  });
  assert.deepEqual(core.diagnosticRange({ pos: { line: 8, column: 3 } }), {
    start: { line: 7, column: 2 },
    end: { line: 7, column: 3 }
  });
});

test('diagnosticSeverity keeps warnings distinct and treats unknown severities as errors', () => {
  assert.equal(core.diagnosticSeverity({ severity: 'warning' }), 'warning');
  assert.equal(core.diagnosticSeverity({ severity: 'info' }), 'error');
  assert.equal(core.diagnosticSeverity({}), 'error');
});

test('groupDiagnosticsByFile separates file diagnostics from global diagnostics', () => {
  const diagnostics = [
    { file: '/workspace/pages/home.page.gwdk', message: 'home' },
    { file: '/workspace/pages/about.page.gwdk', message: 'about' },
    { message: 'global' }
  ];

  const grouped = core.groupDiagnosticsByFile(diagnostics);

  assert.deepEqual(Object.keys(grouped.files).sort(), [
    '/workspace/pages/about.page.gwdk',
    '/workspace/pages/home.page.gwdk'
  ]);
  assert.deepEqual(grouped.files['/workspace/pages/home.page.gwdk'], [
    { file: '/workspace/pages/home.page.gwdk', message: 'home' }
  ]);
  assert.deepEqual(grouped.global, [{ message: 'global' }]);
});

test('projectCommandArgs builds config-aware CLI arguments', () => {
  assert.deepEqual(core.projectCommandArgs('check', {
    json: true,
    configPath: '/workspace/gowdk.config.go',
    ssr: true
  }), ['check', '--json', '--config', '/workspace/gowdk.config.go', '--ssr']);

  assert.deepEqual(core.projectCommandArgs('sitemap', {
    files: ['/workspace/home.page.gwdk']
  }), ['sitemap', '/workspace/home.page.gwdk']);
});

test('goModRequiresGOWDK detects GOWDK app workspaces', () => {
  assert.equal(core.goModRequiresGOWDK(`module example.com/app

go 1.26

require github.com/cssbruno/gowdk v0.0.0
`), true);
  assert.equal(core.goModRequiresGOWDK(`module example.com/app

require (
  github.com/cssbruno/gowdk v0.0.0
)
`), true);
  assert.equal(core.goModRequiresGOWDK(`module example.com/app

require github.com/cssbruno/other v0.0.0
`), false);
});

test('gowdkModuleRunArgs builds a go run invocation for app workspaces', () => {
  assert.deepEqual(core.gowdkModuleRunArgs(['check', '--json']), [
    'run',
    'github.com/cssbruno/gowdk/cmd/gowdk',
    'check',
    '--json'
  ]);
});

test('nearestProjectRoot finds nested GOWDK app roots inside broad workspaces', () => {
  const tmp = fs.mkdtempSync(path.join(process.cwd(), '.tmp-gowdk-vscode-'));
  try {
    const app = path.join(tmp, 'gowdk-page');
    const pageDir = path.join(app, 'src', 'pages');
    fs.mkdirSync(pageDir, { recursive: true });
    fs.writeFileSync(path.join(app, 'go.mod'), `module example.com/page

require github.com/cssbruno/gowdk v0.0.0
`, 'utf8');

    assert.equal(core.nearestProjectRoot(pageDir, tmp), app);
    assert.equal(core.nearestProjectRoot(path.join(tmp, 'other'), tmp), tmp);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('siteMapHTML sorts pages and escapes route, source, and tag data', () => {
  const html = core.siteMapHTML({
    pages: [
      {
        id: 'admin',
        route: '/z-admin',
        render: 'ssr',
        source: '/workspace/admin.page.gwdk',
        blocks: { load: true, view: true, actions: ['save'], apis: ['data'] },
        layouts: ['shell'],
        css: ['default', 'forms'],
        components: ['AdminPanel'],
        staticAssets: ['/assets/admin.png'],
        artifacts: []
      },
      {
        id: '<home>',
        route: '/<home>',
        render: 'static',
        source: '/workspace/home.page.gwdk',
        blocks: { view: true },
        artifacts: [{ kind: 'html', path: 'index.html' }]
      }
    ]
  }, '/workspace');

  assert.match(html, /2 pages · 1 static · 1 ssr/);
  assert.ok(html.indexOf('/&lt;home&gt;') < html.indexOf('/z-admin'));
  assert.match(html, /&lt;home&gt; · home\.page\.gwdk/);
  assert.match(html, /act:save/);
  assert.match(html, /api:data/);
  assert.match(html, /layout:shell/);
  assert.match(html, /css:forms/);
  assert.match(html, /Components: AdminPanel/);
  assert.match(html, /Assets: \/assets\/admin\.png/);
  assert.match(html, /GET \/&lt;home&gt; -&gt; static -&gt; index\.html/);
  assert.match(html, /POST act:save/);
});

test('completionEntries include expected language constructs', () => {
  const labels = core.completionEntries().map(([label]) => label);

  assert.ok(labels.includes('@route'));
  assert.ok(labels.includes('@css'));
  assert.ok(labels.includes('paths'));
  assert.ok(labels.includes('g:post'));
});

test('completionContext identifies project-aware completion contexts', () => {
  assert.equal(core.completionContext('@layout root, '), 'layout');
  assert.equal(core.completionContext('@css default, '), 'css');
  assert.equal(core.completionContext('  <He'), 'component');
  assert.equal(core.completionContext('  -> "'), 'route');
  assert.equal(core.completionContext('@'), 'keyword');
});

test('projectCompletionEntries derive layouts routes components and CSS from metadata', () => {
	const metadata = {
    siteMap: {
      pages: [
        { route: '/settings', layouts: ['root', 'app'] },
        { route: '/', layouts: ['root'] }
      ]
    },
    manifest: {
      pages: {
        settings: { css: ['forms'] }
      },
      components: {
        Hero: {},
        StatusPanel: {}
      }
    },
    cssFiles: [
      { name: 'tokens', file: '/workspace/styles/tokens.css' },
      { name: 'forms', file: '/workspace/styles/forms.css' }
    ]
  };

  assert.deepEqual(core.projectCompletionEntries('layout', metadata), [
    ['app', 'Layout id from project metadata.'],
    ['root', 'Layout id from project metadata.']
  ]);
  assert.deepEqual(core.projectCompletionEntries('route', metadata), [
    ['/', 'Route from project metadata.'],
    ['/settings', 'Route from project metadata.']
  ]);
  assert.deepEqual(core.projectCompletionEntries('component', metadata), [
    ['Hero', 'Component from project manifest.'],
    ['StatusPanel', 'Component from project manifest.']
	]);
  assert.deepEqual(core.projectCompletionEntries('css', metadata), [
    ['default', 'Built-in CSS input: configured default CSS, or global.css when present.'],
    ['forms', 'Discovered CSS file: /workspace/styles/forms.css'],
    ['none', 'Built-in CSS input: disable GOWDK-managed page CSS for this page.'],
    ['page', 'Built-in CSS input: CSS file matching the page id when present.'],
    ['tokens', 'Discovered CSS file: /workspace/styles/tokens.css']
  ]);
});

test('project metadata helpers work with a fixture workspace', () => {
  const root = path.join(__dirname, 'testdata', 'workspace');
  const home = path.join(root, 'pages', 'home.page.gwdk');
  const hero = path.join(root, 'components', 'hero.cmp.gwdk');
  assert.equal(fs.existsSync(home), true);
  assert.equal(fs.existsSync(hero), true);

  const metadata = {
    siteMap: {
      pages: [
        {
          id: 'home',
          route: '/',
          render: 'static',
          source: home,
          layouts: ['root'],
          blocks: { view: true, actions: ['subscribe'] }
        }
      ]
    },
    manifest: {
      pages: {
        home: {
          source: home,
          css: ['default', 'forms'],
          components: ['Hero'],
          staticAssets: ['/assets/home.png']
        }
      },
      components: {
        Hero: {
          source: hero,
          props: [{ name: 'title', type: 'string' }]
        }
      }
    },
    cssFiles: [
      { name: 'forms', file: path.join(root, 'styles', 'forms.css') }
    ]
  };

  assert.deepEqual(core.projectPages(metadata), [
    {
      id: 'home',
      route: '/',
      render: 'static',
      source: home,
      layouts: ['root'],
      blocks: { view: true, actions: ['subscribe'] },
      css: ['default', 'forms'],
      components: ['Hero'],
      staticAssets: ['/assets/home.png']
    }
  ]);
  assert.deepEqual(core.projectCompletionEntries('component', metadata), [
    ['Hero', 'Component from project manifest.']
  ]);
  assert.deepEqual(core.projectCompletionEntries('css', metadata).filter(([name]) => name === 'forms'), [
    ['forms', `Discovered CSS file: ${path.join(root, 'styles', 'forms.css')}`]
  ]);
  assert.match(core.hoverMarkdown('Hero', metadata), /Props: `title string`/);
  assert.match(core.hoverMarkdown('forms', metadata), /\*\*GOWDK CSS input\*\* `forms`/);
  assert.deepEqual(core.definitionTarget('home', metadata), { file: home, line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('forms', metadata), { file: path.join(root, 'styles', 'forms.css'), line: 0, column: 0 });
  assert.deepEqual(core.symbolReferences('Hero', metadata), [
    { file: hero, line: 0, column: 0 },
    { file: home, line: 0, column: 0 }
  ]);
  assert.deepEqual(core.symbolReferences('forms', metadata), [
    { file: path.join(root, 'styles', 'forms.css'), line: 0, column: 0 },
    { file: home, line: 0, column: 0 }
  ]);
  assert.equal(core.canRenameSymbol('forms', metadata), false);
});

test('hoverMarkdown describes project symbols from metadata', () => {
	const metadata = symbolMetadata();

  assert.match(core.hoverMarkdown('home', metadata), /\*\*GOWDK page\*\* `home`/);
  assert.match(core.hoverMarkdown('Hero', metadata), /Props: `title string`/);
  assert.match(core.hoverMarkdown('root', metadata), /Referenced by 1 page/);
  assert.match(core.hoverMarkdown('forms', metadata), /Referenced by 1 page/);
  assert.match(core.hoverMarkdown('submit', metadata), /\*\*GOWDK action\*\*/);
  assert.match(core.hoverMarkdown('health', metadata), /\*\*GOWDK API\*\*/);
  assert.equal(core.hoverMarkdown('missing', metadata), '');
});

test('definitionTarget resolves project symbols to owning source files', () => {
  const metadata = symbolMetadata();

  assert.deepEqual(core.definitionTarget('home', metadata), { file: '/workspace/home.page.gwdk', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('Hero', metadata), { file: '/workspace/hero.cmp.gwdk', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('forms', metadata), { file: '/workspace/styles/forms.css', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('root', metadata), { file: '/workspace/home.page.gwdk', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('submit', metadata), { file: '/workspace/home.page.gwdk', line: 0, column: 0 });
  assert.equal(core.definitionTarget('missing', metadata), undefined);
});

test('symbolReferences finds project metadata references at file granularity', () => {
  const metadata = symbolMetadata();

  assert.deepEqual(core.symbolReferences('Hero', metadata), [
    { file: '/workspace/hero.cmp.gwdk', line: 0, column: 0 },
    { file: '/workspace/home.page.gwdk', line: 0, column: 0 }
  ]);
  assert.deepEqual(core.symbolReferences('root', metadata), [
    { file: '/workspace/home.page.gwdk', line: 0, column: 0 }
  ]);
  assert.deepEqual(core.symbolReferences('forms', metadata), [
    { file: '/workspace/styles/forms.css', line: 0, column: 0 },
    { file: '/workspace/home.page.gwdk', line: 0, column: 0 }
  ]);
  assert.deepEqual(core.symbolReferences('forms', metadata, { includeDeclaration: false }), [
    { file: '/workspace/home.page.gwdk', line: 0, column: 0 }
  ]);
  assert.deepEqual(core.symbolReferences('Hero', metadata, { includeDeclaration: false }), [
    { file: '/workspace/home.page.gwdk', line: 0, column: 0 }
  ]);
  assert.deepEqual(core.symbolReferences('missing', metadata), []);
});

test('rename helpers validate symbols and return exact source edits', () => {
  const metadata = symbolMetadata();
  assert.equal(core.canRenameSymbol('Hero', metadata), true);
  assert.equal(core.canRenameSymbol('forms', metadata), false);
  assert.equal(core.canRenameSymbol('Missing', metadata), false);
  assert.equal(core.validRenameValue('HeroCard'), true);
  assert.equal(core.validRenameValue('bad name'), false);

  assert.deepEqual(core.renameEditsForSource('<Hero title="Hero" />\n<HeroCard />', 'Hero', 'Banner'), [
    {
      start: { line: 0, column: 1 },
      end: { line: 0, column: 5 },
      text: 'Banner'
    },
    {
      start: { line: 0, column: 13 },
      end: { line: 0, column: 17 },
      text: 'Banner'
    }
  ]);
});

test('semanticTokens classifies first-slice GOWDK language tokens', () => {
  const source = [
    '@page home',
    '@css default page forms',
    '@render static',
    'act submit {',
    '  form g:post={submit} g:target="#panel" {',
    '    <Hero title="Welcome" />',
    '  }',
    '}'
  ].join('\n');

  const simplified = core.semanticTokens(source).map((token) => ({
    line: token.line,
    text: source.split('\n')[token.line].slice(token.column, token.column + token.length),
    tokenType: token.tokenType
  }));

  assert.deepEqual(simplified.filter((token) => token.text === '@page'), [
    { line: 0, text: '@page', tokenType: 'namespace' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === 'forms'), [
    { line: 1, text: 'forms', tokenType: 'property' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === 'static'), [
    { line: 2, text: 'static', tokenType: 'enumMember' }
  ]);
  assert.ok(simplified.some((token) => token.text === 'act' && token.tokenType === 'keyword'));
  assert.ok(simplified.some((token) => token.text === 'submit' && token.tokenType === 'function'));
  assert.ok(simplified.some((token) => token.text === 'g:post' && token.tokenType === 'property'));
  assert.ok(simplified.some((token) => token.text === 'Hero' && token.tokenType === 'class'));
});

function symbolMetadata() {
  return {
    siteMap: {
      pages: [
        {
          id: 'home',
          route: '/',
          render: 'static',
          source: '/workspace/home.page.gwdk',
          layouts: ['root'],
          guard: ['auth.required'],
          blocks: { actions: ['submit'], apis: ['health'] }
        }
      ]
    },
    manifest: {
      pages: {
        home: {
          source: '/workspace/home.page.gwdk',
          css: ['default', 'forms'],
          components: ['Hero']
        }
      },
      components: {
        Hero: {
          source: '/workspace/hero.cmp.gwdk',
          props: [{ name: 'title', type: 'string' }]
        }
      }
    },
    cssFiles: [
      { name: 'forms', file: '/workspace/styles/forms.css' }
    ]
  };
}
