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
      var response = await traceFetch(form.getAttribute('action') || window.location.href, {
        method: (form.getAttribute('method') || 'POST').toUpperCase(),
        body: new FormData(form),
        headers: {
          'X-GOWDK-Partial': '1',
          'X-GOWDK-Target': form.dataset.gowdkTarget,
          'X-GOWDK-Swap': form.dataset.gowdkSwap || ''
        }
      }, { name: 'partial submit', lane: 'fragment' });
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

  // submitCommand is the g:command write path. A bare submit of a command form
  // would natively navigate to the adapter's raw JSON response; instead we post
  // in the background, then refresh invalidated g:query regions from the
  // X-GOWDK-Queries response header when there is no active realtime stream.
  // When realtime is active, the generated query-invalidation event owns that
  // refresh so the submitter does not race two document refetches.
  async function submitCommand(event) {
    if (event.defaultPrevented) {
      return;
    }
    var form = event.target && event.target.closest && event.target.closest('form[data-gowdk-command]');
    if (!form) {
      return;
    }
    // A form that also declares a partial target is handled by submitPartial.
    if (form.hasAttribute('data-gowdk-target')) {
      return;
    }

    if (!validateFormBeforeCommandSubmit(form)) {
      event.preventDefault();
      return;
    }

    event.preventDefault();
    form.setAttribute('aria-busy', 'true');
    var command = form.dataset.gowdkCommand || '';
    try {
      var response = await traceFetch(form.getAttribute('action') || window.location.href, {
        method: (form.getAttribute('method') || 'POST').toUpperCase(),
        body: commandFormBody(form, event.submitter || null),
        redirect: 'manual',
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded;charset=UTF-8',
          'X-GOWDK-Command': '1'
        }
      }, { name: 'command submit', lane: 'command' });
      if (response.redirected) {
        throw await commandResponseError(response, 'command request redirected');
      }
      if (!response.ok) {
        throw await commandResponseError(response, 'command request failed with status ' + response.status);
      }
      if (!responseIsJSON(response)) {
        throw await commandResponseError(response, 'command response was not JSON');
      }
      var result = await parseCommandResult(response);
      var queries = parseInvalidatedQueriesHeader(response.headers.get('X-GOWDK-Queries'));
      var eventIDs = parseHeaderList(response.headers.get('X-GOWDK-Events'));
      form.dispatchEvent(new CustomEvent('gowdk:command-success', {
        detail: { form: form, command: command, result: result, queries: queries, eventIDs: eventIDs }
      }));
      if (queries.length) {
        var refreshEnvelope = { form: form, command: command, eventIDs: eventIDs };
        refreshInvalidatedQueriesAfterCommand(queries, refreshEnvelope);
      }
    } catch (error) {
      form.dispatchEvent(new CustomEvent('gowdk:command-error', {
        detail: {
          form: form,
          command: command,
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

  function validateFormBeforeCommandSubmit(form) {
    if (typeof form.checkValidity !== 'function' || form.checkValidity()) {
      return true;
    }
    form.dispatchEvent(new CustomEvent('gowdk:validation-blocked', {
      detail: { form: form }
    }));
    if (typeof form.reportValidity === 'function') {
      form.reportValidity();
    }
    return false;
  }

  function commandFormBody(form, submitter) {
    var formData = formDataWithSubmitter(form, submitter);
    if (typeof URLSearchParams !== 'undefined') {
      return new URLSearchParams(formData).toString();
    }
    var pairs = [];
    if (formData && typeof formData.forEach === 'function') {
      formData.forEach(function (value, key) {
        pairs.push(encodeURIComponent(key) + '=' + encodeURIComponent(String(value)));
      });
    }
    return pairs.join('&');
  }

  function formDataWithSubmitter(form, submitter) {
    if (typeof FormData === 'undefined') {
      return null;
    }
    if (!submitter) {
      return new FormData(form);
    }
    var data;
    try {
      data = new FormData(form, submitter);
    } catch (error) {
      data = new FormData(form);
    }
    appendSubmitterIfMissing(data, submitter);
    return data;
  }

  function appendSubmitterIfMissing(data, submitter) {
    var name = submitterFieldName(submitter);
    if (!name || submitter.disabled) {
      return;
    }
    var value = submitterFieldValue(submitter);
    if (typeof data.getAll === 'function') {
      var values = data.getAll(name);
      for (var i = 0; i < values.length; i++) {
        if (String(values[i]) === value) {
          return;
        }
      }
    }
    if (typeof data.append === 'function') {
      data.append(name, value);
    }
  }

  function submitterFieldName(submitter) {
    return submitter && (submitter.name || submitter.getAttribute && submitter.getAttribute('name')) || '';
  }

  function submitterFieldValue(submitter) {
    var value = submitter && submitter.value;
    if (value == null && submitter && submitter.getAttribute) {
      value = submitter.getAttribute('value');
    }
    return value == null ? '' : String(value);
  }

  function parseInvalidatedQueriesHeader(header) {
    return parseHeaderList(header);
  }

  function parseHeaderList(header) {
    if (!header) {
      return [];
    }
    return header.split(',').map(function (query) {
      return query.trim();
    }).filter(function (query) {
      return query;
    });
  }

  async function commandResponseError(response, message) {
    var body = '';
    try {
      body = await response.text();
    } catch (error) {
      body = '';
    }
    var error = new Error(message);
    error.status = response && response.status || 0;
    error.body = body;
    error.response = response;
    return error;
  }

  function responseIsJSON(response) {
    var type = response && response.headers && response.headers.get && response.headers.get('Content-Type') || '';
    type = type.toLowerCase();
    return type.indexOf('application/json') !== -1 || type.indexOf('+json') !== -1;
  }

  async function parseCommandResult(response) {
    var body = '';
    try {
      body = await response.text();
    } catch (error) {
      throw await commandResponseError(response, 'command response body could not be read');
    }
    if (!body) {
      return null;
    }
    try {
      return JSON.parse(body);
    } catch (error) {
      var parseError = new Error('command response contained invalid JSON');
      parseError.status = response && response.status || 0;
      parseError.body = body;
      parseError.response = response;
      throw parseError;
    }
  }

  var prefetchedDocuments = {};
  var prefetchOrder = [];
  var prefetchLimit = 8;
  var hoverPrefetchDelay = 65;
  var hoverPrefetchTimer = 0;
  var hoverPrefetchURL = '';
  var activeQueryRefreshes = {};
  var handledQueryRefreshEventIDs = {};
  var handledQueryRefreshEventIDOrder = [];
  var handledQueryRefreshEventIDLimit = 128;
  var realtimeEventsPath = '/_gowdk/realtime/events';
  var realtimeSource = null;
  var traceEndpoint = '/_gowdk/traces/browser';
  var traceStack = [];

  installTraceBridge();
  document.addEventListener('submit', submitPartial);
  document.addEventListener('submit', submitCommand);
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
    var response = await traceFetch(url, {
      headers: headers,
      credentials: 'same-origin'
    }, { name: prefetch ? 'prefetch document' : 'navigate document', lane: 'nav' });
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

  function installTraceBridge() {
    if (typeof window === 'undefined') {
      return;
    }
    window.__gowdkTrace = {
      enabled: traceEnabled,
      start: startTraceSpan,
      end: endTraceSpan,
      traceparent: function () {
        var ctx = activeTraceContext();
        if (!ctx) {
          if (!traceEnabled()) {
            return '';
          }
          return '00-' + traceHex(16) + '-' + traceHex(8) + '-01';
        }
        return '00-' + ctx.traceId + '-' + ctx.spanId + '-01';
      },
      fetch: traceFetch
    };
  }

  function traceEnabled() {
    return !!(typeof window !== 'undefined' && (
      window.__gowdkTraceEnabled ||
      document.documentElement && document.documentElement.hasAttribute('data-gowdk-trace')
    ));
  }

  function traceHex(bytes) {
    var data = new Uint8Array(bytes);
    if (typeof crypto !== 'undefined' && crypto.getRandomValues) {
      crypto.getRandomValues(data);
    } else {
      for (var i = 0; i < data.length; i++) {
        data[i] = Math.floor(Math.random() * 256);
      }
    }
    var value = '';
    for (var j = 0; j < data.length; j++) {
      value += data[j].toString(16).padStart(2, '0');
    }
    return /^0+$/.test(value) ? traceHex(bytes) : value;
  }

  function traceContextFromHeader(header) {
    var parts = String(header || '').split('-');
    if (parts.length < 4 || parts[0] !== '00' || !/^[0-9a-f]{32}$/.test(parts[1]) || !/^[0-9a-f]{16}$/.test(parts[2])) {
      return null;
    }
    return { traceId: parts[1], spanId: parts[2], sampled: parts[3] !== '00' };
  }

  function traceparentFor(span) {
    if (!span) {
      return '';
    }
    return '00-' + span.traceId + '-' + span.spanId + '-01';
  }

  function activeTraceContext() {
    if (traceStack.length > 0) {
      var active = traceStack[traceStack.length - 1];
      return { traceId: active.traceId, spanId: active.spanId, sampled: true };
    }
    if (typeof window !== 'undefined' && window.__gowdkTraceparent) {
      return traceContextFromHeader(window.__gowdkTraceparent);
    }
    return null;
  }

  function startTraceSpan(name, lane, attrs) {
    if (!traceEnabled()) {
      return null;
    }
    var parent = activeTraceContext();
    var span = {
      traceId: parent ? parent.traceId : traceHex(16),
      spanId: traceHex(8),
      parentSpanId: parent ? parent.spanId : '',
      name: name || 'browser',
      surface: 'frontend',
      lane: lane || 'user',
      attributes: attrs || [],
      events: [],
      status: { code: 'unset' },
      startTime: new Date().toISOString()
    };
    traceStack.push(span);
    return span;
  }

  function endTraceSpan(span, status, message) {
    if (!span) {
      return;
    }
    for (var index = traceStack.length - 1; index >= 0; index--) {
      if (traceStack[index] === span) {
        traceStack.splice(index, 1);
        break;
      }
    }
    span.endTime = new Date().toISOString();
    span.durationNs = Math.max(0, Math.round((Date.parse(span.endTime) - Date.parse(span.startTime)) * 1000000));
    span.status = { code: status || 'ok', message: message || '' };
    postTraceSpan(span);
  }

  function postTraceSpan(span) {
    if (!traceEnabled()) {
      return;
    }
    var payload = JSON.stringify(span);
    try {
      if (navigator.sendBeacon && navigator.sendBeacon(traceEndpoint, new Blob([payload], { type: 'application/json' }))) {
        return;
      }
    } catch (error) {}
    try {
      fetch(traceEndpoint, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: payload, keepalive: true }).catch(function () {});
    } catch (error) {}
  }

  function traceInputURL(url) {
    if (typeof Request !== 'undefined' && url instanceof Request) {
      return url.url || '';
    }
    if (url && typeof url === 'object' && typeof url.url === 'string') {
      return url.url;
    }
    return url;
  }

  function traceInputHeaders(url, options) {
    if (options && options.headers) {
      return options.headers;
    }
    if (url && typeof url === 'object' && url.headers) {
      return url.headers;
    }
    return {};
  }

  function sameOriginURL(url) {
    try {
      return new URL(traceInputURL(url), window.location.href).origin === window.location.origin;
    } catch (error) {
      return false;
    }
  }

  async function traceFetch(url, options, meta) {
    if (!traceEnabled()) {
      return fetch(url, options);
    }
    var span = startTraceSpan(meta && meta.name || 'fetch', meta && meta.lane || 'api', [
      { key: 'url.path', value: String(url || '') }
    ]);
    var traced = Object.assign({}, options || {});
    if (sameOriginURL(url)) {
      var headers = new Headers(traceInputHeaders(url, traced));
      headers.set('traceparent', traceparentFor(span));
      traced.headers = headers;
    }
    try {
      var response = await fetch(url, traced);
      endTraceSpan(span, response.ok ? 'ok' : 'error', response.ok ? '' : 'HTTP ' + response.status);
      return response;
    } catch (error) {
      endTraceSpan(span, 'error', error && error.message || String(error || 'fetch failed'));
      throw error;
    }
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

  function refreshInvalidatedQueries(queries, envelope) {
    var eventIDs = queryRefreshEventIDs(envelope);
    if (!eventIDs.length) {
      return runInvalidatedQueryRefresh(queries, envelope);
    }
    for (var i = 0; i < eventIDs.length; i++) {
      if (handledQueryRefreshEventIDs[eventIDs[i]]) {
        return Promise.resolve();
      }
      if (activeQueryRefreshes[eventIDs[i]]) {
        return activeQueryRefreshes[eventIDs[i]];
      }
    }
    var refresh = runInvalidatedQueryRefresh(queries, envelope).then(function (result) {
      rememberHandledQueryRefreshEventIDs(eventIDs);
      return result;
    }).finally(function () {
      eventIDs.forEach(function (eventID) {
        if (activeQueryRefreshes[eventID] === refresh) {
          delete activeQueryRefreshes[eventID];
        }
      });
    });
    eventIDs.forEach(function (eventID) {
      activeQueryRefreshes[eventID] = refresh;
    });
    return refresh;
  }

  async function runInvalidatedQueryRefresh(queries, envelope) {
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

  function queryRefreshEventIDs(envelope) {
    if (!envelope || typeof envelope !== 'object') {
      return [];
    }
    var value = envelope.value || envelope.Value || null;
    var eventIDs = envelope.eventIDs || envelope.EventIDs || null;
    if ((!eventIDs || !eventIDs.length) && value && typeof value === 'object') {
      eventIDs = value.eventIDs || value.EventIDs || null;
    }
    if (!Array.isArray(eventIDs)) {
      return [];
    }
    return eventIDs.filter(function (eventID) {
      return typeof eventID === 'string' && eventID;
    });
  }

  function rememberHandledQueryRefreshEventIDs(eventIDs) {
    eventIDs.forEach(function (eventID) {
      if (!handledQueryRefreshEventIDs[eventID]) {
        handledQueryRefreshEventIDOrder.push(eventID);
      }
      handledQueryRefreshEventIDs[eventID] = true;
    });
    while (handledQueryRefreshEventIDOrder.length > handledQueryRefreshEventIDLimit) {
      delete handledQueryRefreshEventIDs[handledQueryRefreshEventIDOrder.shift()];
    }
  }

  function refreshInvalidatedQueriesAfterCommand(queries, envelope) {
    if (realtimeInvalidationRefreshActive()) {
      return;
    }
    refreshInvalidatedQueries(queries, envelope).catch(function (error) {
      dispatchRealtimeError(error, {
        envelope: envelope,
        queries: queries,
        form: envelope && envelope.form || null,
        command: envelope && envelope.command || ''
      });
    });
  }

  function realtimeInvalidationRefreshActive() {
    return !!(realtimeSource && realtimeSource.readyState !== 2);
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
