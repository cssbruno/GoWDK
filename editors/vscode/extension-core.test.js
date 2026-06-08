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

test('goModModulePath reads module declarations', () => {
  assert.equal(core.goModModulePath(`module github.com/cssbruno/gowdk

go 1.26
`), 'github.com/cssbruno/gowdk');
  assert.equal(core.goModModulePath(`// module ignored
module example.com/app // inline comment
`), 'example.com/app');
  assert.equal(core.goModModulePath('go 1.26'), '');
});

test('gowdkModuleRunArgs builds a go run invocation for app workspaces', () => {
  assert.deepEqual(core.gowdkModuleRunArgs(['check', '--json']), [
    'run',
    'github.com/cssbruno/gowdk/cmd/gowdk',
    'check',
    '--json'
  ]);
});

test('toolInvocation prefers source workspace go run over PATH lookup', () => {
  assert.deepEqual(core.toolInvocation(['version'], {
    cwd: '/workspace/GOWDK',
    isSourceWorkspace: true,
    localBinary: '/workspace/GOWDK/gowdk',
    requiresGOWDK: true
  }), {
    command: 'go',
    args: ['run', './cmd/gowdk', 'version'],
    cwd: '/workspace/GOWDK',
    source: 'sourceWorkspace'
  });
});

test('toolInvocation uses a workspace-local binary before bare gowdk', () => {
  assert.deepEqual(core.toolInvocation(['check', 'home.page.gwdk'], {
    cwd: '/workspace/app',
    localBinary: '/workspace/app/gowdk'
  }), {
    command: '/workspace/app/gowdk',
    args: ['check', 'home.page.gwdk'],
    cwd: '/workspace/app',
    source: 'localBinary'
  });
});

test('toolInvocation uses PATH binary for app modules without local binary', () => {
  assert.deepEqual(core.toolInvocation(['check', '--json'], {
    cwd: '/workspace/app',
    sourceWorkspaceRoot: '/workspace/GOWDK',
    requiresGOWDK: true
  }), {
    command: 'gowdk',
    args: ['check', '--json'],
    cwd: '/workspace/app',
    source: 'path'
  });
});

test('toolInvocation ignores source checkout for non-source workspaces', () => {
  assert.deepEqual(core.toolInvocation(['check', '--json'], {
    cwd: '/workspace/docs',
    sourceWorkspaceRoot: '/workspace/GOWDK'
  }), {
    command: 'gowdk',
    args: ['check', '--json'],
    cwd: '/workspace/docs',
    source: 'path'
  });
});

test('toolInvocation uses PATH binary for GOWDK app modules', () => {
  assert.deepEqual(core.toolInvocation(['sitemap'], {
    cwd: '/workspace/app',
    requiresGOWDK: true
  }), {
    command: 'gowdk',
    args: ['sitemap'],
    cwd: '/workspace/app',
    source: 'path'
  });
});

test('missingExecutableMessage explains missing GOWDK CLI resolution', () => {
  assert.equal(core.isMissingExecutableError({ code: 'ENOENT' }), true);
  assert.equal(core.isMissingExecutableError(new Error('spawn gowdk ENOENT')), true);
  assert.equal(core.isMissingExecutableError(new Error('exit status 1')), false);

  assert.equal(
    core.missingExecutableMessage({ command: 'gowdk', source: 'path' }, { code: 'ENOENT' }),
    'Missing GOWDK binary. Install gowdk, add it to PATH, or set gowdk.cliPath.'
  );
  assert.equal(
    core.missingExecutableMessage({ command: '/missing/gowdk', source: 'cliPath' }, { code: 'ENOENT' }),
    'Missing configured GOWDK binary: /missing/gowdk. Update gowdk.cliPath.'
  );
  assert.equal(
    core.missingExecutableMessage({ command: 'go', source: 'module' }, { code: 'ENOENT' }),
    'Missing Go binary. Install Go, fix PATH, or set gowdk.cliPath to a built GOWDK binary.'
  );
});

test('completionContext detects view interpolation data fields', () => {
  assert.equal(core.completionContext('  <h1>{tit'), 'dataField');
  assert.equal(core.completionContext('  <Page title="{user.na'), 'dataField');
});

test('documentDataFields extracts literal build and load fields', () => {
  const fields = core.documentDataFields(`package pages

build {
  => {
    title: "Docs",
    count: 2
  }
}

load {
  => { user.name, account.plan }
}

view {
  <h1>{title}</h1>
}
`);

  assert.deepEqual(fields.map((field) => [field.lane, field.name, field.origin]), [
    ['build', 'title', 'build {}'],
    ['build', 'count', 'build {}'],
    ['load', 'user.name', 'load {}'],
    ['load', 'account.plan', 'load {}']
  ]);
});

test('documentDataFields extracts fields from imported Go build function structs', () => {
  const tmp = fs.mkdtempSync(path.join(process.cwd(), '.tmp-gowdk-vscode-'));
  try {
    const interopDir = path.join(tmp, 'examples', 'go-interop');
    fs.mkdirSync(interopDir, { recursive: true });
    fs.writeFileSync(path.join(tmp, 'go.mod'), `module example.com/app

go 1.26
`, 'utf8');
    fs.writeFileSync(path.join(interopDir, 'catalog.go'), `package gointerop

type FeaturedCopy struct {
  Title string \`json:"title"\`
  Tagline string \`json:"tagline"\`
  Hidden string \`json:"-"\`
}

func FeaturedCopyForBuild() FeaturedCopy {
  return FeaturedCopy{}
}
`, 'utf8');

    const source = `package pages

import interop "example.com/app/examples/go-interop"

build {
  => interop.FeaturedCopyForBuild()
}

view {
  <h1>{title}</h1>
}
`;

    const fields = core.documentDataFields(source, {
      fileName: path.join(tmp, 'pages', 'home.page.gwdk'),
      projectRoot: tmp
    });

    assert.deepEqual(fields.map((field) => ({
      name: field.name,
      type: field.type,
      goField: field.goField,
      lane: field.lane,
      origin: field.origin
    })), [
      {
        name: 'title',
        type: 'string',
        goField: 'Title',
        lane: 'build',
        origin: 'interop.FeaturedCopyForBuild()'
      },
      {
        name: 'tagline',
        type: 'string',
        goField: 'Tagline',
        lane: 'build',
        origin: 'interop.FeaturedCopyForBuild()'
      }
    ]);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('projectCompletionEntries returns data fields and hover explains origin', () => {
  const metadata = {
    dataFields: [
      {
        name: 'title',
        lane: 'build',
        type: 'string',
        origin: 'interop.FeaturedCopyForBuild()',
        goField: 'Title'
      }
    ]
  };

  assert.deepEqual(core.projectCompletionEntries('dataField', metadata), [
    ['title', 'build field string from interop.FeaturedCopyForBuild().']
  ]);

  assert.equal(core.hoverMarkdown('title', metadata), [
    '**GOWDK data field** `title`',
    '',
    'Lane: `build`',
    'Type: `string`',
    'From: `interop.FeaturedCopyForBuild()`',
    'Go field: `Title`'
  ].join('\n'));
});

test('nearestProjectRoot finds nested GOWDK app roots inside broad workspaces', () => {
  const tmp = fs.mkdtempSync(path.join(process.cwd(), '.tmp-gowdk-vscode-'));
  try {
    const app = path.join(tmp, 'gowdk-page');
    const pageDir = path.join(app, 'src', 'pages');
    fs.mkdirSync(pageDir, { recursive: true });
    fs.writeFileSync(path.join(tmp, 'gowdk.config.go'), 'package main\n', 'utf8');
    fs.writeFileSync(path.join(app, 'go.mod'), `module example.com/page

require github.com/cssbruno/gowdk v0.0.0
`, 'utf8');

    assert.equal(core.nearestProjectRoot(pageDir, tmp), app);
    assert.equal(core.nearestProjectRoot(path.join(tmp, 'other'), tmp), tmp);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('nearestProjectRoot finds app root above a subfolder workspace', () => {
  const tmp = fs.mkdtempSync(path.join(process.cwd(), '.tmp-gowdk-vscode-'));
  try {
    const app = path.join(tmp, 'gowdk-page');
    const workspace = path.join(app, 'src', 'pages');
    const pageDir = path.join(workspace, 'docs');
    fs.mkdirSync(pageDir, { recursive: true });
    fs.writeFileSync(path.join(app, 'gowdk.config.go'), 'package main\n', 'utf8');

    assert.equal(core.nearestProjectRoot(pageDir, workspace), app);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
});

test('nearbyGOWDKSourceRoot finds a sibling source checkout', () => {
  const tmp = fs.mkdtempSync(path.join(process.cwd(), '.tmp-gowdk-vscode-'));
  try {
    const app = path.join(tmp, 'gowdk-page');
    const pageDir = path.join(app, 'src', 'pages');
    const source = path.join(tmp, 'GOWDK');
    fs.mkdirSync(pageDir, { recursive: true });
    fs.mkdirSync(path.join(source, 'cmd', 'gowdk'), { recursive: true });
    fs.writeFileSync(path.join(source, 'go.mod'), `module github.com/cssbruno/gowdk

go 1.26
`, 'utf8');

    assert.equal(core.isGOWDKSourceDir(source), true);
    assert.equal(core.nearbyGOWDKSourceRoot(pageDir), source);
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
        assets: ['/assets/admin.png'],
        artifacts: []
      },
      {
        id: '<home>',
        route: '/<home>',
        render: 'spa',
        source: '/workspace/home.page.gwdk',
        blocks: { view: true },
        artifacts: [{ kind: 'html', path: 'index.html' }]
      }
    ]
  }, '/workspace');

  assert.match(html, /2 pages · 1 spa · 1 ssr/);
  assert.ok(html.indexOf('/&lt;home&gt;') < html.indexOf('/z-admin'));
  assert.match(html, /&lt;home&gt; · home\.page\.gwdk/);
  assert.match(html, /act:save/);
  assert.match(html, /api:data/);
  assert.match(html, /layout:shell/);
  assert.match(html, /css:forms/);
  assert.match(html, /Components: AdminPanel/);
  assert.match(html, /Assets: \/assets\/admin\.png/);
  assert.match(html, /GET \/&lt;home&gt; -&gt; spa -&gt; index\.html/);
  assert.match(html, /POST act:save/);
});

test('completionEntries include expected language constructs', () => {
  const labels = core.completionEntries().map(([label]) => label);

  assert.ok(labels.includes('package'));
  assert.ok(labels.includes('import'));
  assert.ok(labels.includes('use'));
  assert.ok(labels.includes('@route'));
  assert.ok(labels.includes('@title'));
  assert.ok(labels.includes('@description'));
  assert.ok(labels.includes('@canonical'));
  assert.ok(labels.includes('@image'));
  assert.ok(labels.includes('@component'));
  assert.ok(labels.includes('@css'));
  assert.ok(labels.includes('paths'));
  assert.ok(labels.includes('client'));
  assert.ok(labels.includes('emits'));
  assert.ok(labels.includes('computed'));
  assert.ok(labels.includes('await fetchJSON'));
  assert.ok(labels.includes('contains'));
  assert.ok(labels.includes('g:post'));
  assert.ok(labels.includes('g:on:'));
  assert.ok(labels.includes('g:bind:value'));
  assert.ok(labels.includes('class:'));
});

test('completionContext identifies project-aware completion contexts', () => {
  assert.equal(core.completionContext('@layout root, '), 'layout');
  assert.equal(core.completionContext('@css default, '), 'css');
  assert.equal(core.completionContext('@css default '), 'css');
  assert.equal(core.completionContext('  <button g:'), 'directive');
  assert.equal(core.completionContext('  <button class:'), 'directive');
  assert.equal(core.completionContext('  <Counter g:island="w'), 'island');
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
      },
      layouts: {
        root: { source: '/workspace/layouts/root.layout.gwdk' },
        shell: { source: '/workspace/layouts/shell.layout.gwdk' }
      }
    },
    cssFiles: [
      { name: 'tokens', file: '/workspace/styles/tokens.css' },
      { name: 'forms', file: '/workspace/styles/forms.css' }
    ]
  };

  assert.deepEqual(core.projectCompletionEntries('layout', metadata), [
    ['app', 'Layout id from project metadata.'],
    ['root', 'Layout from project manifest.'],
    ['shell', 'Layout from project manifest.']
  ]);
  assert.deepEqual(core.projectCompletionEntries('route', metadata), [
    ['/', 'Route from project metadata.'],
    ['/settings', 'Route from project metadata.']
  ]);
  assert.deepEqual(core.projectCompletionEntries('component', metadata), [
    ['Hero', 'Component from project manifest.'],
    ['StatusPanel', 'Component from project manifest.']
	]);
  assert.deepEqual(core.projectCompletionEntries('island', metadata), [
    ['wasm', 'Use explicit WASM island assets for this component call.']
  ]);
  assert.ok(core.projectCompletionEntries('directive', metadata).some(([name]) => name === 'g:if'));
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
          render: 'spa',
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
          spaAssets: ['/assets/home.png']
        }
      },
      components: {
        Hero: {
          source: hero,
          props: [{ name: 'title', type: 'string' }],
          state: {
            type: { alias: 'ui', name: 'HeroState' },
            init: { alias: 'ui', name: 'NewHeroState' }
          },
          emits: [
            { name: 'select', params: [{ name: 'id', type: 'string' }] }
          ]
        }
      },
      layouts: {
        root: { source: path.join(root, 'layouts', 'root.layout.gwdk') }
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
      render: 'spa',
      source: home,
      layouts: ['root'],
      blocks: { view: true, actions: ['subscribe'] },
      css: ['default', 'forms'],
      components: ['Hero'],
      spaAssets: ['/assets/home.png']
    }
  ]);
  assert.deepEqual(core.projectCompletionEntries('component', metadata), [
    ['Hero', 'Component from project manifest.']
  ]);
  assert.deepEqual(core.projectCompletionEntries('css', metadata).filter(([name]) => name === 'forms'), [
    ['forms', `Discovered CSS file: ${path.join(root, 'styles', 'forms.css')}`]
  ]);
  assert.match(core.hoverMarkdown('Hero', metadata), /Props: `title string`/);
  assert.match(core.hoverMarkdown('Hero', metadata), /State: `ui\.HeroState`/);
  assert.match(core.hoverMarkdown('Hero', metadata), /Emits: `select\(id string\)`/);
  assert.match(core.hoverMarkdown('select', metadata), /\*\*GOWDK component event\*\*/);
  assert.match(core.hoverMarkdown('forms', metadata), /\*\*GOWDK CSS input\*\* `forms`/);
  assert.deepEqual(core.definitionTarget('home', metadata), { file: home, line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('root', metadata), { file: path.join(root, 'layouts', 'root.layout.gwdk'), line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('select', metadata), { file: hero, line: 0, column: 0 });
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
  assert.match(core.hoverMarkdown('Hero', metadata), /State: `ui\.HeroState`/);
  assert.match(core.hoverMarkdown('Hero', metadata), /Emits: `select\(id string\)`/);
  assert.match(core.hoverMarkdown('select', metadata), /\*\*GOWDK component event\*\*/);
  assert.match(core.hoverMarkdown('root', metadata), /Referenced by 1 page/);
  assert.match(core.hoverMarkdown('forms', metadata), /Referenced by 1 page/);
  assert.match(core.hoverMarkdown('cart', metadata), /\*\*GOWDK store\*\* `cart`/);
  assert.match(core.hoverMarkdown('CartState', metadata), /\*\*GOWDK Go contract\*\* `CartState`/);
  assert.match(core.hoverMarkdown('ui', metadata), /Import alias: `ui`/);
  assert.match(core.hoverMarkdown('submit', metadata), /\*\*GOWDK action\*\*/);
  assert.match(core.hoverMarkdown('health', metadata), /\*\*GOWDK API\*\*/);
  assert.equal(core.hoverMarkdown('missing', metadata), '');
});

test('definitionTarget resolves project symbols to owning source files', () => {
  const metadata = symbolMetadata();

  assert.deepEqual(core.definitionTarget('home', metadata), { file: '/workspace/home.page.gwdk', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('Hero', metadata), { file: '/workspace/hero.cmp.gwdk', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('select', metadata), { file: '/workspace/hero.cmp.gwdk', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('forms', metadata), { file: '/workspace/styles/forms.css', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('root', metadata), { file: '/workspace/root.layout.gwdk', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('submit', metadata), { file: '/workspace/home.page.gwdk', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('cart', metadata), { file: '/workspace/home.page.gwdk', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('CartState', metadata), { file: '/workspace/home.page.gwdk', line: 0, column: 0 });
  assert.deepEqual(core.definitionTarget('HeroState', metadata), { file: '/workspace/hero.cmp.gwdk', line: 0, column: 0 });
  assert.equal(core.definitionTarget('missing', metadata), undefined);
});

test('symbolReferences finds project metadata references at file granularity', () => {
  const metadata = symbolMetadata();

  assert.deepEqual(core.symbolReferences('Hero', metadata), [
    { file: '/workspace/hero.cmp.gwdk', line: 0, column: 0 },
    { file: '/workspace/home.page.gwdk', line: 0, column: 0 }
  ]);
  assert.deepEqual(core.symbolReferences('root', metadata), [
    { file: '/workspace/root.layout.gwdk', line: 0, column: 0 },
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
  assert.deepEqual(core.symbolReferences('select', metadata), [
    { file: '/workspace/hero.cmp.gwdk', line: 0, column: 0 }
  ]);
  assert.deepEqual(core.symbolReferences('cart', metadata), [
    { file: '/workspace/home.page.gwdk', line: 0, column: 0 }
  ]);
  assert.deepEqual(core.symbolReferences('HeroState', metadata), [
    { file: '/workspace/hero.cmp.gwdk', line: 0, column: 0 }
  ]);
  assert.deepEqual(core.symbolReferences('missing', metadata), []);
});

test('component event helpers use component context and avoid ambiguous globals', () => {
  const metadata = {
    manifest: {
      components: {
        Hero: {
          source: '/workspace/hero.cmp.gwdk',
          emits: [{ name: 'select', params: [{ name: 'id', type: 'string' }] }]
        },
        Menu: {
          source: '/workspace/menu.cmp.gwdk',
          emits: [{ name: 'select', params: [{ name: 'index', type: 'int' }] }]
        }
      }
    }
  };

  assert.equal(core.hoverMarkdown('select', metadata), '');
  assert.equal(core.definitionTarget('select', metadata), undefined);
  assert.match(core.hoverMarkdown('select', metadata, { component: 'Menu' }), /Component: `Menu`/);
  assert.deepEqual(core.definitionTarget('select', metadata, { component: 'Menu' }), {
    file: '/workspace/menu.cmp.gwdk',
    line: 0,
    column: 0
  });
  assert.deepEqual(core.symbolReferences('select', metadata, { symbolContext: { component: 'Hero' } }), [
    { file: '/workspace/hero.cmp.gwdk', line: 0, column: 0 }
  ]);
});

test('symbolContext finds the surrounding component call', () => {
  const source = [
    '<Hero',
    '  title="Welcome"',
    '  g:on:select={selected = event.id}',
    '/>',
    '<button g:on:select={noop()}>Select</button>'
  ].join('\n');

  assert.deepEqual(core.symbolContext(source, source.indexOf('select')), { component: 'Hero' });
  assert.deepEqual(core.symbolContext(source, source.lastIndexOf('select')), {});
});

test('rename helpers validate symbols and return exact source edits', () => {
  const metadata = symbolMetadata();
  assert.equal(core.canRenameSymbol('Hero', metadata), true);
  assert.equal(core.canRenameSymbol('select', metadata), false);
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
    'package app',
    'import interop "github.com/acme/app/interop"',
    'use ui "components"',
    '@page home',
    '@title "Home"',
    '@description "Home page"',
    '@canonical "https://example.com/"',
    '@image "https://example.com/social.png"',
    '@component Counter',
    '@css default page forms',
    'act Submit POST "/submit"',
    'api Status GET "/api/status"',
    'script {',
    '  console.log("ready")',
    '}',
    'emits {',
    '  select(id string)',
    '}',
    'client {',
    '  computed Visible bool {',
    '    return contains(lower(Query), "go")',
    '  }',
    '  async fn Refresh() {',
    '    Items = await fetchJSON[[]ui.Item]("/api/items")',
    '  }',
    '  effect when Count {',
    '    emit select(Query)',
    '  }',
    '}',
    'view {',
    '  form g:post={Submit} g:target="#panel" g:swap="innerHTML" {',
    '    <Hero g:on:select={Query = event.id} g:island="wasm" />',
    '    <button g:if={Visible} g:bind:value={Query} class:active={Visible} style:height.px={Count}>Save</button>',
    '  }',
    '}'
  ].join('\n');

  const simplified = core.semanticTokens(source).map((token) => ({
    line: token.line,
    text: source.split('\n')[token.line].slice(token.column, token.column + token.length),
    tokenType: token.tokenType
  }));

  assert.deepEqual(simplified.filter((token) => token.text === 'package'), [
    { line: 0, text: 'package', tokenType: 'keyword' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === 'import'), [
    { line: 1, text: 'import', tokenType: 'keyword' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === 'use'), [
    { line: 2, text: 'use', tokenType: 'keyword' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === '@page'), [
    { line: 3, text: '@page', tokenType: 'namespace' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === '@title'), [
    { line: 4, text: '@title', tokenType: 'namespace' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === '@description'), [
    { line: 5, text: '@description', tokenType: 'namespace' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === '@canonical'), [
    { line: 6, text: '@canonical', tokenType: 'namespace' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === '@image'), [
    { line: 7, text: '@image', tokenType: 'namespace' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === '@component'), [
    { line: 8, text: '@component', tokenType: 'namespace' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === 'forms'), [
    { line: 9, text: 'forms', tokenType: 'property' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === 'Submit'), [
    { line: 10, text: 'Submit', tokenType: 'function' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === 'POST'), [
    { line: 10, text: 'POST', tokenType: 'enumMember' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === 'Status'), [
    { line: 11, text: 'Status', tokenType: 'function' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === 'GET'), [
    { line: 11, text: 'GET', tokenType: 'enumMember' }
  ]);
  assert.deepEqual(simplified.filter((token) => token.text === 'script'), [
    { line: 12, text: 'script', tokenType: 'keyword' }
  ]);
  assert.ok(simplified.some((token) => token.text === 'emits' && token.tokenType === 'keyword'));
  assert.ok(simplified.some((token) => token.text === 'client' && token.tokenType === 'keyword'));
  assert.ok(simplified.some((token) => token.text === 'computed' && token.tokenType === 'keyword'));
  assert.ok(simplified.some((token) => token.text === 'Visible' && token.tokenType === 'property'));
  assert.ok(simplified.some((token) => token.text === 'contains' && token.tokenType === 'function'));
  assert.ok(simplified.some((token) => token.text === 'Refresh' && token.tokenType === 'function'));
  assert.ok(simplified.some((token) => token.text === 'fetchJSON' && token.tokenType === 'function'));
  assert.ok(simplified.some((token) => token.text === 'effect' && token.tokenType === 'keyword'));
  assert.ok(simplified.some((token) => token.text === 'select' && token.tokenType === 'function'));
  assert.ok(simplified.some((token) => token.text === 'act' && token.tokenType === 'keyword'));
  assert.ok(simplified.some((token) => token.text === 'api' && token.tokenType === 'keyword'));
  assert.ok(simplified.some((token) => token.text === 'g:post' && token.tokenType === 'property'));
  assert.ok(simplified.some((token) => token.text === 'g:on:select' && token.tokenType === 'property'));
  assert.ok(simplified.some((token) => token.text === 'g:if' && token.tokenType === 'property'));
  assert.ok(simplified.some((token) => token.text === 'g:bind:value' && token.tokenType === 'property'));
  assert.ok(simplified.some((token) => token.text === 'class:active' && token.tokenType === 'property'));
  assert.ok(simplified.some((token) => token.text === 'style:height.px' && token.tokenType === 'property'));
  assert.ok(simplified.some((token) => token.text === 'Hero' && token.tokenType === 'class'));
});

test('documentOutlineItems indexes annotations and major blocks', () => {
  const outline = core.documentOutlineItems([
    'package app',
    '',
    '@page home',
    '@layout root',
    '',
    'build {',
    '  => { title: "Home" }',
    '}',
    '',
    'script {',
    '  console.log("ready")',
    '}',
    '',
    'view {',
    '  <main><Hero /></main>',
    '}'
  ].join('\n'));

  assert.deepEqual(outline.map((item) => ({
    name: item.name,
    detail: item.detail,
    kind: item.kind,
    startLine: item.range.start.line,
    endLine: item.range.end.line
  })), [
    { name: '@page home', detail: 'home', kind: 'namespace', startLine: 2, endLine: 2 },
    { name: '@layout root', detail: 'root', kind: 'namespace', startLine: 3, endLine: 3 },
    { name: 'build', detail: 'block', kind: 'property', startLine: 5, endLine: 7 },
    { name: 'script', detail: 'script block', kind: 'module', startLine: 9, endLine: 11 },
    { name: 'view', detail: 'markup block', kind: 'object', startLine: 13, endLine: 15 }
  ]);
});

function symbolMetadata() {
  return {
    siteMap: {
      pages: [
        {
          id: 'home',
          route: '/',
          render: 'spa',
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
          imports: [{ alias: 'ui', path: 'example.com/app/ui' }],
          css: ['default', 'forms'],
          components: ['Hero'],
          stores: [{
            name: 'cart',
            type: { alias: 'ui', name: 'CartState' },
            init: { alias: 'ui', name: 'NewCartState' }
          }]
        }
      },
      components: {
        Hero: {
          source: '/workspace/hero.cmp.gwdk',
          imports: [{ alias: 'ui', path: 'example.com/app/ui' }],
          propsType: { alias: 'ui', name: 'HeroProps' },
          props: [{ name: 'title', type: 'string' }],
          state: {
            type: { alias: 'ui', name: 'HeroState' },
            init: { alias: 'ui', name: 'NewHeroState' }
          },
          emits: [
            { name: 'select', params: [{ name: 'id', type: 'string' }] }
          ]
        }
      },
      layouts: {
        root: { source: '/workspace/root.layout.gwdk' }
      }
    },
    cssFiles: [
      { name: 'forms', file: '/workspace/styles/forms.css' }
    ]
  };
}
