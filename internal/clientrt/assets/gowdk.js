(function () {
  async function submitPartial(event) {
    var form = event.target && event.target.closest && event.target.closest('form[data-gowdk-target]');
    if (!form) {
      return;
    }

    var target = document.querySelector(form.dataset.gowdkTarget);
    if (!target) {
      return;
    }

    var before = new CustomEvent('gowdk:before-request', {
      cancelable: true,
      detail: { form: form, target: target }
    });
    if (!form.dispatchEvent(before)) {
      event.preventDefault();
      return;
    }

    if (!validateFormBeforePartialSubmit(form, target)) {
      event.preventDefault();
      return;
    }

    event.preventDefault();
    form.setAttribute('aria-busy', 'true');
    var focused = focusTarget(document.activeElement);
    try {
      var response = await fetch(form.getAttribute('action') || window.location.href, {
        method: (form.getAttribute('method') || 'POST').toUpperCase(),
        body: new FormData(form),
        headers: {
          'X-GOWDK-Partial': '1',
          'X-GOWDK-Target': form.dataset.gowdkTarget,
          'X-GOWDK-Swap': form.dataset.gowdkSwap || ''
        }
      });
      if (!response.ok) {
        throw await partialRequestError(response);
      }
      if (response.headers.get('X-GOWDK-Reload') === '1') {
        reloadPage();
        return;
      }
      var html = await response.text();
      var swap = response.headers.get('X-GOWDK-Fragment-Swap') || form.dataset.gowdkSwap || 'innerHTML';
      if (typeof window !== 'undefined' && window.__gowdkDestroyIslands) {
        window.__gowdkDestroyIslands(target, swap === 'outerHTML');
      }
      if (swap === 'outerHTML') {
        target.outerHTML = html;
      } else {
        target.innerHTML = html;
      }
      if (typeof window !== 'undefined' && window.__gowdkStores && window.__gowdkStores.hydrate) {
        window.__gowdkStores.hydrate();
      }
      if (typeof window !== 'undefined' && window.__gowdkMountIslands) {
        window.__gowdkMountIslands();
      }
      if (typeof window !== 'undefined' && window.__gowdkMountClientGoBlocks) {
        window.__gowdkMountClientGoBlocks();
      }
      restoreFocus(focused);
      form.dispatchEvent(new CustomEvent('gowdk:after-swap', {
        detail: { form: form, target: target, swap: swap }
      }));
    } catch (error) {
      form.dispatchEvent(new CustomEvent('gowdk:request-error', {
        detail: {
          form: form,
          target: target,
          error: error,
          status: error && error.status || 0,
          body: error && error.body || '',
          response: error && error.response || null
        }
      }));
    } finally {
      form.removeAttribute('aria-busy');
    }
  }

  function validateFormBeforePartialSubmit(form, target) {
    if (typeof form.checkValidity !== 'function' || form.checkValidity()) {
      return true;
    }
    form.dispatchEvent(new CustomEvent('gowdk:validation-blocked', {
      detail: { form: form, target: target }
    }));
    if (typeof form.reportValidity === 'function') {
      form.reportValidity();
    }
    return false;
  }

  document.addEventListener('submit', submitPartial);
  document.addEventListener('click', navigateLink);
  document.addEventListener('mouseover', prefetchLink);
  document.addEventListener('mouseout', cancelHoverPrefetch);
  document.addEventListener('focusin', prefetchLink);
  document.addEventListener('touchstart', prefetchLink, { passive: true });
  if (typeof window !== 'undefined' && window.addEventListener) {
    window.addEventListener('popstate', function () {
      loadDocument(window.location.href, false, null);
    });
  }

  var prefetchedDocuments = {};
  var prefetchOrder = [];
  var prefetchLimit = 8;
  var hoverPrefetchDelay = 65;
  var hoverPrefetchTimer = 0;
  var hoverPrefetchURL = '';

  async function navigateLink(event) {
    if (event.defaultPrevented || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
      return;
    }
    var link = event.target && event.target.closest && event.target.closest('a[href]');
    if (!link || link.target || link.hasAttribute('download')) {
      return;
    }
    var url;
    try {
      url = new URL(link.href, window.location.href);
    } catch (error) {
      return;
    }
    if (!isNavigableURL(url)) {
      return;
    }
    if (url.hash && url.pathname === window.location.pathname && url.search === window.location.search) {
      return;
    }
    var before = new CustomEvent('gowdk:before-navigate', {
      cancelable: true,
      detail: { link: link, url: url.href }
    });
    if (!document.dispatchEvent(before)) {
      event.preventDefault();
      return;
    }
    event.preventDefault();
    try {
      await loadDocument(url.href, true, link);
    } catch (error) {
      document.dispatchEvent(new CustomEvent('gowdk:navigate-error', {
        detail: { link: link, url: url.href, error: error }
      }));
      window.location.href = url.href;
    }
  }

  function prefetchTarget(event) {
    var link = event.target && event.target.closest && event.target.closest('a[href]');
    if (!link || link.target || link.hasAttribute('download')) {
      return '';
    }
    var url;
    try {
      url = new URL(link.href, window.location.href);
    } catch (error) {
      return '';
    }
    if (!isNavigableURL(url)) {
      return '';
    }
    if (url.hash && url.pathname === window.location.pathname && url.search === window.location.search) {
      return '';
    }
    return url.href;
  }

  function prefetchLink(event) {
    var href = prefetchTarget(event);
    if (!href) {
      return;
    }
    // Pointer hovers are noisy and often incidental, so wait a beat before
    // spending a request; focus and touch are deliberate, so prefetch at once.
    if (event.type === 'mouseover') {
      if (href === hoverPrefetchURL) {
        return;
      }
      cancelHoverPrefetch();
      hoverPrefetchURL = href;
      hoverPrefetchTimer = setTimeout(function () {
        hoverPrefetchTimer = 0;
        hoverPrefetchURL = '';
        prefetchDocument(href).catch(function () {});
      }, hoverPrefetchDelay);
      return;
    }
    prefetchDocument(href).catch(function () {});
  }

  function cancelHoverPrefetch() {
    if (hoverPrefetchTimer) {
      clearTimeout(hoverPrefetchTimer);
      hoverPrefetchTimer = 0;
    }
    hoverPrefetchURL = '';
  }

  function prefetchDocument(url) {
    if (!prefetchedDocuments[url]) {
      prefetchedDocuments[url] = fetchDocument(url, true).catch(function (error) {
        forgetPrefetched(url);
        throw error;
      });
      rememberPrefetched(url);
    }
    return prefetchedDocuments[url];
  }

  // rememberPrefetched bounds the prefetch cache so a long session that hovers
  // many links cannot retain an unbounded set of full documents in memory.
  function rememberPrefetched(url) {
    prefetchOrder.push(url);
    while (prefetchOrder.length > prefetchLimit) {
      var oldest = prefetchOrder.shift();
      if (oldest !== url) {
        delete prefetchedDocuments[oldest];
      }
    }
  }

  function forgetPrefetched(url) {
    delete prefetchedDocuments[url];
    var index = prefetchOrder.indexOf(url);
    if (index !== -1) {
      prefetchOrder.splice(index, 1);
    }
  }

  async function partialRequestError(response) {
    var body = '';
    try {
      body = await response.text();
    } catch (error) {
      body = '';
    }
    var error = new Error('partial request failed with status ' + response.status);
    error.status = response.status;
    error.body = body;
    error.response = response;
    return error;
  }

  function reloadPage() {
    if (window.location && window.location.reload) {
      window.location.reload();
      return;
    }
    if (window.location) {
      window.location.href = window.location.href;
    }
  }

  async function loadDocument(url, push, source) {
    setNavigationBusy(true, source);
    try {
      var fetched = prefetchedDocuments[url]
        ? await prefetchedDocuments[url]
        : await fetchDocument(url, false);
      forgetPrefetched(url);
      if (!fetched || !fetched.html) {
        window.location.href = url;
        return;
      }
      var next = new DOMParser().parseFromString(fetched.html, 'text/html');
      if (!next || !next.body) {
        window.location.href = url;
        return;
      }
      var focused = focusTarget(document.activeElement);
      if (typeof window !== 'undefined' && window.__gowdkDestroyIslands) {
        window.__gowdkDestroyIslands(document.body, false);
      }
      var previousScripts = scriptSources(document);
      document.title = next.title || document.title;
      document.head.innerHTML = next.head ? next.head.innerHTML : '';
      document.body.innerHTML = next.body.innerHTML;
      await activateNewScripts(previousScripts);
      if (push && window.history && window.history.pushState) {
        window.history.pushState({}, document.title, url);
      }
      if (typeof window !== 'undefined' && window.__gowdkStores && window.__gowdkStores.hydrate) {
        window.__gowdkStores.hydrate();
      }
      if (typeof window !== 'undefined' && window.__gowdkMountIslands) {
        window.__gowdkMountIslands();
      }
      if (typeof window !== 'undefined' && window.__gowdkMountClientGoBlocks) {
        window.__gowdkMountClientGoBlocks();
      }
      restoreFocus(focused);
      if (window.location.hash) {
        var target = document.getElementById(window.location.hash.slice(1));
        if (target && target.scrollIntoView) {
          target.scrollIntoView();
        }
      } else if (window.scrollTo) {
        window.scrollTo(0, 0);
      }
      document.dispatchEvent(new CustomEvent('gowdk:after-navigate', {
        detail: { url: url, source: source || null }
      }));
    } finally {
      setNavigationBusy(false, source);
    }
  }

  async function fetchDocument(url, prefetch) {
    var headers = {
      'Accept': 'text/html'
    };
    headers[prefetch ? 'X-GOWDK-Prefetch' : 'X-GOWDK-Navigate'] = '1';
    var response = await fetch(url, {
      headers: headers,
      credentials: 'same-origin'
    });
    if (!response.ok) {
      throw new Error('navigation request failed with status ' + response.status);
    }
    var type = response.headers && response.headers.get && response.headers.get('Content-Type') || '';
    if (type && type.indexOf('text/html') === -1) {
      return null;
    }
    var html = await response.text();
    return { html: html };
  }

  function setNavigationBusy(active, source) {
    if (!document.documentElement) {
      return;
    }
    if (active) {
      document.documentElement.setAttribute('data-gowdk-navigating', 'true');
      document.dispatchEvent(new CustomEvent('gowdk:navigate-start', {
        detail: { source: source || null }
      }));
      return;
    }
    document.documentElement.removeAttribute('data-gowdk-navigating');
    document.dispatchEvent(new CustomEvent('gowdk:navigate-end', {
      detail: { source: source || null }
    }));
  }

  function isNavigableURL(url) {
    if (!window || !window.location) {
      return false;
    }
    if (url.origin !== window.location.origin) {
      return false;
    }
    return url.protocol === 'http:' || url.protocol === 'https:';
  }

  function scriptSources(doc) {
    var sources = {};
    Array.prototype.forEach.call(doc.querySelectorAll('script[src]'), function (script) {
      sources[script.src] = true;
    });
    return sources;
  }

  function activateNewScripts(previousScripts) {
    var storeScripts = [];
    var otherScripts = [];
    Array.prototype.forEach.call(document.querySelectorAll('script[src]'), function (script) {
      if (previousScripts[script.src]) {
        return;
      }
      if (script.hasAttribute('data-gowdk-store-runtime')) {
        storeScripts.push(script);
      } else {
        otherScripts.push(script);
      }
    });
    // Run the store runtime first, then hydrate the registry, before island
    // bundles execute. Island bundles auto-mount on execution and read
    // window.__gowdkStores during mount, so a store loaded after them would leave
    // islands mounted with no subscription or persisted value -- and the
    // post-navigation mount pass skips already-mounted roots. Hydrating here also
    // covers the case where stores.js already ran on the previous page (skipped
    // above) yet this route introduces new store seeds.
    return runScripts(storeScripts).then(function () {
      if (typeof window !== 'undefined' && window.__gowdkStores && window.__gowdkStores.hydrate) {
        window.__gowdkStores.hydrate();
      }
      return runScripts(otherScripts);
    });
  }

  function runScripts(scripts) {
    var pending = [];
    scripts.forEach(function (script) {
      var active = document.createElement('script');
      Array.prototype.forEach.call(script.attributes, function (attr) {
        active.setAttribute(attr.name, attr.value);
      });
      // Dynamically inserted scripts default to async; force ordered execution so
      // a dependency (the store runtime) cannot lose a race with its dependents.
      active.async = false;
      pending.push(new Promise(function (resolve, reject) {
        active.onload = resolve;
        active.onerror = reject;
      }));
      script.parentNode.replaceChild(active, script);
    });
    return Promise.all(pending);
  }

  function focusTarget(element) {
    if (!element || element === document.body) {
      return {};
    }
    if (element.id) {
      return { id: element.id };
    }
    if (element.name) {
      return { name: element.name };
    }
    return {};
  }

  function restoreFocus(target) {
    if (!target) {
      return;
    }
    var element = target.id
      ? document.getElementById(target.id)
      : target.name
        ? document.querySelector('[name="' + escapeAttr(target.name) + '"]')
        : null;
    if (element && element.focus) {
      element.focus();
    }
  }

  function escapeAttr(value) {
    return String(value).replace(/["\\]/g, '\\$&');
  }
}());
