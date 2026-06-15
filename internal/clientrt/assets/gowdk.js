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
      ensureRealtime();
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

  var prefetchedDocuments = {};
  var prefetchOrder = [];
  var prefetchLimit = 8;
  var hoverPrefetchDelay = 65;
  var hoverPrefetchTimer = 0;
  var hoverPrefetchURL = '';
  var realtimeEventsPath = '/_gowdk/realtime/events';
  var realtimeSource = null;

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
  ensureRealtime();

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
      ensureRealtime();
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

  function ensureRealtime() {
    if (!hasRealtimeRegions()) {
      closeRealtime();
      return;
    }
    if (realtimeSource || typeof window === 'undefined' || !window.EventSource) {
      return;
    }
    try {
      realtimeSource = new window.EventSource(realtimeEventsPath);
    } catch (error) {
      realtimeSource = null;
      dispatchRealtimeError(error, { source: null });
      return;
    }
    if (realtimeSource.addEventListener) {
      realtimeSource.addEventListener('gowdk-presentation', handleRealtimeEvent);
    } else {
      realtimeSource.onmessage = handleRealtimeEvent;
    }
    realtimeSource.onerror = function (event) {
      dispatchRealtimeError(new Error('realtime stream error'), { source: realtimeSource, event: event });
    };
  }

  function closeRealtime() {
    if (!realtimeSource) {
      return;
    }
    if (typeof realtimeSource.close === 'function') {
      realtimeSource.close();
    }
    realtimeSource = null;
  }

  function hasRealtimeRegions() {
    return !!(document.querySelector && document.querySelector('[data-gowdk-subscribe], [data-gowdk-query-type]'));
  }

  function handleRealtimeEvent(event) {
    var envelope;
    try {
      envelope = JSON.parse(event.data || '{}');
    } catch (error) {
      dispatchRealtimeError(error, { event: event });
      return;
    }
    try {
      applyRealtimeEnvelope(envelope);
    } catch (error) {
      dispatchRealtimeError(error, { event: event, envelope: envelope });
    }
  }

  function applyRealtimeEnvelope(envelope) {
    var category = envelope.category || envelope.Category || '';
    var eventType = envelope.type || envelope.Type || '';
    if (category !== 'presentation' || !eventType) {
      return;
    }
    if (eventType === 'gowdk.query.invalidate') {
      refreshInvalidatedQueries(normalizeInvalidatedQueries(envelope.value || envelope.Value), envelope).catch(function (error) {
        dispatchRealtimeError(error, { envelope: envelope });
      });
      return;
    }
    var regions = realtimeRegionsForEvent(eventType);
    if (!regions.length) {
      return;
    }
    var patches = normalizeRealtimePatches(envelope.value || envelope.Value);
    regions.forEach(function (region) {
      patches.forEach(function (patch) {
        applyRealtimePatch(region, patch, envelope);
      });
    });
  }

  function realtimeRegionsForEvent(eventType) {
    if (!document.querySelectorAll) {
      return [];
    }
    var regions = [];
    Array.prototype.forEach.call(document.querySelectorAll('[data-gowdk-subscribe]'), function (region) {
      var boundType = region.getAttribute('data-gowdk-subscribe-type') || '';
      var sourceRef = region.getAttribute('data-gowdk-subscribe') || '';
      if ((boundType && boundType === eventType) || (!boundType && subscriptionMatchesEventType(sourceRef, eventType))) {
        regions.push(region);
      }
    });
    return regions;
  }

  function normalizeInvalidatedQueries(value) {
    if (!value || typeof value !== 'object') {
      throw new Error('query invalidation event value must contain queries');
    }
    var queries = Array.isArray(value.queries) ? value.queries : Array.isArray(value.Queries) ? value.Queries : null;
    if (!queries || !queries.length) {
      throw new Error('query invalidation event value must contain queries');
    }
    return queries.filter(function (query) {
      return typeof query === 'string' && query;
    });
  }

  async function refreshInvalidatedQueries(queries, envelope) {
    var regions = queryRegionsForInvalidation(document, queries);
    if (!regions.length || typeof DOMParser === 'undefined') {
      return;
    }
    var focused = focusTarget(document.activeElement);
    var fetched = await fetchDocument(window.location.href, false);
    if (!fetched || !fetched.html) {
      return;
    }
    var next = new DOMParser().parseFromString(fetched.html, 'text/html');
    if (!next || !next.querySelectorAll) {
      return;
    }
    var replacements = queryRegionsForInvalidation(next, queries);
    regions.forEach(function (region, index) {
      var replacement = replacementQueryRegion(region, replacements, index);
      if (!replacement) {
        return;
      }
      if (typeof window !== 'undefined' && window.__gowdkDestroyIslands) {
        window.__gowdkDestroyIslands(region, true);
      }
      region.outerHTML = replacement.outerHTML;
    });
    if (typeof window !== 'undefined' && window.__gowdkStores && window.__gowdkStores.hydrate) {
      window.__gowdkStores.hydrate();
    }
    if (typeof window !== 'undefined' && window.__gowdkMountIslands) {
      window.__gowdkMountIslands();
    }
    if (typeof window !== 'undefined' && window.__gowdkMountClientGoBlocks) {
      window.__gowdkMountClientGoBlocks();
    }
    ensureRealtime();
    restoreFocus(focused);
    document.dispatchEvent(new CustomEvent('gowdk:query-refresh', {
      detail: { queries: queries, envelope: envelope }
    }));
  }

  function replacementQueryRegion(region, replacements, index) {
    if (region.id) {
      for (var i = 0; i < replacements.length; i++) {
        if (replacements[i].id === region.id) {
          return replacements[i];
        }
      }
    }
    return replacements[index] || null;
  }

  function queryRegionsForInvalidation(root, queries) {
    if (!root.querySelectorAll) {
      return [];
    }
    var regions = [];
    Array.prototype.forEach.call(root.querySelectorAll('[data-gowdk-query]'), function (region) {
      if (queryRegionMatches(region, queries)) {
        regions.push(region);
      }
    });
    return regions;
  }

  function queryRegionMatches(region, queries) {
    if (region.getAttribute('data-gowdk-subscribe')) {
      return false;
    }
    var queryType = region.getAttribute('data-gowdk-query-type') || '';
    var queryRef = region.getAttribute('data-gowdk-query') || '';
    for (var i = 0; i < queries.length; i++) {
      if (queries[i] === queryType || queries[i] === queryRef) {
        return true;
      }
    }
    return false;
  }

  function subscriptionMatchesEventType(sourceRef, eventType) {
    return sourceRef === eventType || eventType.slice(-sourceRef.length - 1) === '.' + sourceRef || eventType.slice(-sourceRef.length - 1) === '/' + sourceRef;
  }

  function normalizeRealtimePatches(value) {
    if (!value || typeof value !== 'object') {
      throw new Error('realtime event value must contain a patch object');
    }
    var patches = Array.isArray(value.patches) ? value.patches : null;
    if (!patches && value.patch) {
      patches = [value.patch];
    }
    if (!patches || !patches.length) {
      throw new Error('realtime event value must contain patch or patches');
    }
    return patches.map(normalizeRealtimePatch);
  }

  function normalizeRealtimePatch(patch) {
    if (!patch || typeof patch !== 'object') {
      throw new Error('realtime patch must be an object');
    }
    if (patch.op !== 'replaceHTML') {
      throw new Error('unsupported realtime patch operation');
    }
    if (typeof patch.html !== 'string') {
      throw new Error('realtime replaceHTML patch requires html');
    }
    var swap = patch.swap || 'innerHTML';
    if (swap !== 'innerHTML' && swap !== 'outerHTML') {
      throw new Error('unsupported realtime patch swap');
    }
    return { op: patch.op, html: patch.html, swap: swap };
  }

  function applyRealtimePatch(region, patch, envelope) {
    var focused = focusTarget(document.activeElement);
    if (typeof window !== 'undefined' && window.__gowdkDestroyIslands) {
      window.__gowdkDestroyIslands(region, patch.swap === 'outerHTML');
    }
    if (patch.swap === 'outerHTML') {
      region.outerHTML = patch.html;
    } else {
      region.innerHTML = patch.html;
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
    region.dispatchEvent(new CustomEvent('gowdk:realtime-patch', {
      detail: { region: region, patch: patch, envelope: envelope }
    }));
  }

  function dispatchRealtimeError(error, detail) {
    document.dispatchEvent(new CustomEvent('gowdk:realtime-error', {
      detail: Object.assign({ error: error }, detail || {})
    }));
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
