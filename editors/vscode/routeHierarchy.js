const path = require('path');

function buildRouteHierarchy(pages) {
  const root = createGroup('', '');
  for (const page of sortedPages(pages)) {
    const route = normalizedRoute(page.route);
    const segments = routeSegments(route);
    if (segments.length === 0) {
      root.children.push(createPage(page));
      continue;
    }
    let current = root;
    let currentPath = '';
    for (const segment of segments) {
      currentPath = `${currentPath}/${segment}`;
      current = getOrCreateGroup(current, segment, currentPath);
    }
    current.children.push(createPage(page));
  }
  sortNodes(root.children);
  return root.children;
}

function buildDirectoryHierarchy(pages, rootPath = '') {
  const root = createDirectory('', '');
  for (const page of sortedPagesBySource(pages)) {
    const sourcePath = relativeSourcePath(page.source, rootPath);
    const dir = path.posix.dirname(sourcePath);
    const segments = dir === '.' ? [] : dir.split('/').filter(Boolean);
    let current = root;
    let currentPath = '';
    for (const segment of segments) {
      currentPath = currentPath ? `${currentPath}/${segment}` : segment;
      current = getOrCreateDirectory(current, segment, currentPath);
    }
    current.children.push(createPage(page));
  }
  sortDirectoryNodes(root.children);
  return root.children;
}

function sortedPages(pages) {
  return (pages || []).slice().sort((left, right) => normalizedRoute(left.route).localeCompare(normalizedRoute(right.route)));
}

function sortedPagesBySource(pages) {
  return (pages || []).slice().sort((left, right) => {
    const leftSource = String(left.source || '');
    const rightSource = String(right.source || '');
    if (leftSource !== rightSource) {
      return leftSource.localeCompare(rightSource);
    }
    return normalizedRoute(left.route).localeCompare(normalizedRoute(right.route));
  });
}

function normalizedRoute(route) {
  const value = String(route || '').trim();
  if (!value || value === '/') {
    return '/';
  }
  return value.startsWith('/') ? value : `/${value}`;
}

function routeSegments(route) {
  if (!route || route === '/') {
    return [];
  }
  return route.replace(/^\/+|\/+$/g, '').split('/').filter(Boolean);
}

function getOrCreateGroup(parent, label, path) {
  let group = parent.children.find((child) => child.type === 'group' && child.label === label);
  if (!group) {
    group = createGroup(label, path);
    parent.children.push(group);
  }
  return group;
}

function getOrCreateDirectory(parent, label, directoryPath) {
  let group = parent.children.find((child) => child.type === 'directory' && child.label === label);
  if (!group) {
    group = createDirectory(label, directoryPath);
    parent.children.push(group);
  }
  return group;
}

function createGroup(label, path) {
  return {
    type: 'group',
    label,
    path,
    children: []
  };
}

function createDirectory(label, directoryPath) {
  return {
    type: 'directory',
    label,
    path: directoryPath,
    children: []
  };
}

function createPage(page) {
  return {
    type: 'page',
    label: page.id || normalizedRoute(page.route) || '(missing page)',
    page
  };
}

function sortNodes(nodes) {
  nodes.sort((left, right) => {
    if (left.type !== right.type) {
      return left.type === 'page' ? -1 : 1;
    }
    return left.label.localeCompare(right.label);
  });
  for (const node of nodes) {
    if (node.children) {
      sortNodes(node.children);
    }
  }
}

function sortDirectoryNodes(nodes) {
  nodes.sort((left, right) => {
    if (left.type !== right.type) {
      return left.type === 'directory' ? -1 : 1;
    }
    return left.label.localeCompare(right.label);
  });
  for (const node of nodes) {
    if (node.children) {
      sortDirectoryNodes(node.children);
    }
  }
}

function relativeSourcePath(source, rootPath) {
  const value = String(source || '');
  if (!value) {
    return '';
  }
  const relative = rootPath ? path.relative(rootPath, value) : value;
  return relative.replace(/\\/g, '/');
}

module.exports = {
  buildDirectoryHierarchy,
  buildRouteHierarchy,
  normalizedRoute,
  routeSegments
};
