const assert = require('node:assert/strict');
const test = require('node:test');
const { buildDirectoryHierarchy, buildRouteHierarchy, normalizedRoute, routeSegments } = require('./routeHierarchy');

test('buildRouteHierarchy groups pages by declared route segments', () => {
  const tree = buildRouteHierarchy([
    { id: 'admin.users.detail', route: '/admin/users/{id}' },
    { id: 'home', route: '/' },
    { id: 'admin.index', route: '/admin' },
    { id: 'admin.users', route: '/admin/users' }
  ]);

  assert.deepEqual(summarize(tree), [
    { type: 'page', label: 'home', route: '/' },
    {
      type: 'group',
      label: 'admin',
      path: '/admin',
      children: [
        { type: 'page', label: 'admin.index', route: '/admin' },
        {
          type: 'group',
          label: 'users',
          path: '/admin/users',
          children: [
            { type: 'page', label: 'admin.users', route: '/admin/users' },
            {
              type: 'group',
              label: '{id}',
              path: '/admin/users/{id}',
              children: [
                { type: 'page', label: 'admin.users.detail', route: '/admin/users/{id}' }
              ]
            }
          ]
        }
      ]
    }
  ]);
});

test('normalizedRoute keeps route hierarchy independent of file paths', () => {
  assert.equal(normalizedRoute('admin/settings'), '/admin/settings');
  assert.equal(normalizedRoute(''), '/');
  assert.deepEqual(routeSegments('/admin/users/'), ['admin', 'users']);
});

test('buildDirectoryHierarchy groups pages by source directories', () => {
  const tree = buildDirectoryHierarchy([
    { id: 'home', route: '/', source: '/workspace/home.page.gwdk' },
    { id: 'admin.users', route: '/admin/users', source: '/workspace/pages/admin/users.page.gwdk' },
    { id: 'admin.index', route: '/admin', source: '/workspace/pages/admin/index.page.gwdk' },
    { id: 'blog.post', route: '/blog/{slug}', source: '/workspace/pages/blog/post.page.gwdk' }
  ], '/workspace');

  assert.deepEqual(summarize(tree), [
    {
      type: 'directory',
      label: 'pages',
      path: 'pages',
      children: [
        {
          type: 'directory',
          label: 'admin',
          path: 'pages/admin',
          children: [
            { type: 'page', label: 'admin.index', route: '/admin' },
            { type: 'page', label: 'admin.users', route: '/admin/users' }
          ]
        },
        {
          type: 'directory',
          label: 'blog',
          path: 'pages/blog',
          children: [
            { type: 'page', label: 'blog.post', route: '/blog/{slug}' }
          ]
        }
      ]
    },
    { type: 'page', label: 'home', route: '/' }
  ]);
});

function summarize(nodes) {
  return nodes.map((node) => {
    if (node.type === 'page') {
      return {
        type: 'page',
        label: node.label,
        route: node.page.route
      };
    }
    return {
      type: node.type,
      label: node.label,
      path: node.path,
      children: summarize(node.children)
    };
  });
}
