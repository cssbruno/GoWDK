package clientrt

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRuntimeSwapsFragmentsInDOMHarness(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}

	dir := t.TempDir()
	script := filepath.Join(dir, "gowdk-clientrt-dom-test.js")
	if err := os.WriteFile(script, []byte(domHarnessScript(string(Source()))), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command(node, script).CombinedOutput()
	if err != nil {
		t.Fatalf("DOM harness failed: %v\n%s", err, output)
	}
}

func domHarnessScript(runtime string) string {
	return `
'use strict';

const assert = require('node:assert/strict');

class CustomEvent {
  constructor(type, options = {}) {
    this.type = type;
    this.cancelable = !!options.cancelable;
    this.detail = options.detail || {};
    this.defaultPrevented = false;
    this.target = null;
  }
  preventDefault() {
    if (this.cancelable) {
      this.defaultPrevented = true;
    }
  }
}

class EventTarget {
  constructor() {
    this.listeners = {};
  }
  addEventListener(type, handler) {
    (this.listeners[type] ||= []).push(handler);
  }
  dispatchEvent(event) {
    event.target ||= this;
    for (const handler of this.listeners[event.type] || []) {
      handler(event);
    }
    return !event.defaultPrevented;
  }
}

class Element extends EventTarget {
  constructor(tagName) {
    super();
    this.tagName = tagName.toUpperCase();
    this.dataset = {};
    this.attributes = {};
    this.fields = [];
    this.innerHTML = '';
    this.id = '';
    this.name = '';
    this.value = '';
    this.method = '';
    this.action = '';
    this.disabled = false;
    this.replacedWith = '';
    this.outerHTMLValue = '';
    this.valid = true;
    this.reported = false;
  }
  closest(selector) {
    if (selector === 'form[data-gowdk-target]' && this.tagName === 'FORM' && this.dataset.gowdkTarget) {
      return this;
    }
    if (selector === 'form[data-gowdk-command]' && this.tagName === 'FORM' && this.dataset.gowdkCommand) {
      return this;
    }
    return null;
  }
  setAttribute(name, value) {
    this.attributes[name] = String(value);
  }
  getAttribute(name) {
    return Object.prototype.hasOwnProperty.call(this.attributes, name) ? this.attributes[name] : null;
  }
  removeAttribute(name) {
    delete this.attributes[name];
  }
  hasAttribute(name) {
    return Object.prototype.hasOwnProperty.call(this.attributes, name);
  }
  checkValidity() {
    return this.valid;
  }
  reportValidity() {
    this.reported = true;
    return this.valid;
  }
  focus() {
    document.activeElement = this;
  }
  set outerHTML(value) {
    this.replacedWith = value;
    this.outerHTMLValue = value;
  }
  get outerHTML() {
    return this.outerHTMLValue || this.replacedWith;
  }
}

class Document extends EventTarget {
  constructor() {
    super();
    this.body = new Element('body');
    this.activeElement = this.body;
    this.bySelector = {};
    this.byID = {};
    this.realtimeRegions = [];
    this.queryRegions = [];
  }
  querySelector(selector) {
    if (selector === '[data-gowdk-subscribe], [data-gowdk-query-type]') {
      return this.realtimeRegions[0] || this.queryRegions[0] || null;
    }
    return this.bySelector[selector] || null;
  }
  querySelectorAll(selector) {
    if (selector === '[data-gowdk-subscribe]') {
      return this.realtimeRegions.slice();
    }
    if (selector === '[data-gowdk-query]') {
      return this.queryRegions.slice();
    }
    return [];
  }
  getElementById(id) {
    return this.byID[id] || null;
  }
}

class Headers {
  constructor(values) {
    this.values = values;
  }
  get(name) {
    return this.values[name] || this.values[name.toLowerCase()] || null;
  }
}

global.CustomEvent = CustomEvent;
global.document = new Document();
global.DOMParser = class {
  parseFromString(html, type) {
    const next = new Document();
    const refreshed = new Element('section');
    refreshed.id = 'invalidated-patients';
    refreshed.innerHTML = '<p>Refetched</p>';
    refreshed.setAttribute('data-gowdk-query', 'patients.GetPatientPage');
    refreshed.setAttribute('data-gowdk-query-type', 'gowdk-generated-app/patients.GetPatientPage');
    refreshed.outerHTMLValue = '<section id="invalidated-patients" data-gowdk-query="patients.GetPatientPage" data-gowdk-query-type="gowdk-generated-app/patients.GetPatientPage"><p>Refetched</p></section>';
    next.queryRegions = [refreshed];
    return next;
  }
};
const islandLifecycle = [];
const eventSources = [];
class EventSourceStub extends EventTarget {
  constructor(url) {
    super();
    this.url = url;
    this.closed = false;
    this.readyState = 1;
    eventSources.push(this);
  }
  close() {
    this.closed = true;
    this.readyState = 2;
  }
  emit(type, data) {
    const event = new CustomEvent(type);
    event.data = JSON.stringify(data);
    this.dispatchEvent(event);
  }
}
global.window = {
  location: {
    reloaded: false,
    href: 'http://example.test/newsletter',
    reload() {
      this.reloaded = true;
    }
  },
  EventSource: EventSourceStub,
  __gowdkDestroyIslands(target, includeRoot) {
    islandLifecycle.push(['destroy', target.id, includeRoot]);
  },
  __gowdkMountIslands() {
    islandLifecycle.push(['mount']);
  }
};
global.FormData = class {
  constructor(form, submitter) {
    this.form = form;
    this.entries = [];
    for (const field of form.fields || []) {
      if (field && field.name && !field.disabled) {
        this.append(field.name, field.value ?? '');
      }
    }
    if (submitter && submitter.includeInNativeFormData && submitter.name && !submitter.disabled) {
      this.append(submitter.name, submitter.value ?? '');
    }
  }
  append(name, value) {
    this.entries.push([String(name), String(value)]);
  }
  getAll(name) {
    return this.entries.filter(entry => entry[0] === String(name)).map(entry => entry[1]);
  }
  [Symbol.iterator]() {
    return this.entries[Symbol.iterator]();
  }
};

const form = new Element('form');
form.method = 'post';
form.action = '/newsletter';
form.setAttribute('method', 'post');
form.setAttribute('action', '/newsletter');
form.dataset.gowdkTarget = '#newsletter';
form.dataset.gowdkSwap = 'innerHTML';
const commandForm = new Element('form');
commandForm.method = 'post';
commandForm.action = '/commands/create';
commandForm.setAttribute('method', 'post');
commandForm.setAttribute('action', '/commands/create');
commandForm.dataset.gowdkCommand = 'patients.CreatePatient';
commandForm.fields = [{ name: 'name', value: 'Ada' }];
const commandSubmitter = new Element('button');
commandSubmitter.name = 'intent';
commandSubmitter.value = 'publish';
const target = new Element('section');
target.id = 'newsletter';
target.innerHTML = '<p>Old</p>';
const liveRegion = new Element('section');
liveRegion.id = 'live-patients';
liveRegion.innerHTML = '<p>Waiting</p>';
liveRegion.setAttribute('data-gowdk-query', 'patients.GetPatientPage');
liveRegion.setAttribute('data-gowdk-query-type', 'gowdk-generated-app/patients.GetPatientPage');
liveRegion.setAttribute('data-gowdk-subscribe', 'patients.PatientNotice');
liveRegion.setAttribute('data-gowdk-subscribe-type', 'gowdk-generated-app/patients.PatientNotice');
const invalidatedRegion = new Element('section');
invalidatedRegion.id = 'invalidated-patients';
invalidatedRegion.innerHTML = '<p>Stale</p>';
invalidatedRegion.setAttribute('data-gowdk-query', 'patients.GetPatientPage');
invalidatedRegion.setAttribute('data-gowdk-query-type', 'gowdk-generated-app/patients.GetPatientPage');
const input = new Element('input');
input.id = 'email';

document.bySelector['#newsletter'] = target;
document.bySelector['[data-gowdk-subscribe]'] = liveRegion;
document.realtimeRegions = [liveRegion];
document.queryRegions = [liveRegion, invalidatedRegion];
document.byID.newsletter = target;
document.byID['live-patients'] = liveRegion;
document.byID['invalidated-patients'] = invalidatedRegion;
document.byID.email = input;
document.activeElement = input;

let request;
let requests = [];
let requestCount = 0;
let swap = 'innerHTML';
let fail = false;
let reload = false;
let commandMode = 'success';
let refreshFail = false;
global.fetch = async function(url, options) {
  requestCount++;
  request = { url, options };
  requests.push(request);
  if (url === '/commands/create') {
    if (commandMode === 'redirect') {
      return {
        ok: true,
        redirected: true,
        status: 200,
        headers: new Headers({ 'Content-Type': 'text/html; charset=utf-8' }),
        text: async () => '<main>Login</main>'
      };
    }
    if (commandMode === 'html') {
      return {
        ok: true,
        redirected: false,
        status: 200,
        headers: new Headers({ 'Content-Type': 'text/html; charset=utf-8' }),
        text: async () => '<main>Login</main>'
      };
    }
    return {
      ok: true,
      redirected: false,
      status: 200,
      headers: new Headers({
        'Content-Type': 'application/json; charset=utf-8',
        'X-GOWDK-Queries': 'gowdk-generated-app/patients.GetPatientPage'
      }),
      text: async () => '{"id":"patient-1"}'
    };
  }
  if (url === window.location.href && refreshFail) {
    return {
      ok: false,
      status: 500,
      headers: new Headers({ 'Content-Type': 'text/html; charset=utf-8' }),
      text: async () => '<main>Refresh failed</main>'
    };
  }
  if (fail) {
    return {
      ok: false,
      status: 422,
      headers: new Headers({}),
      text: async () => '<div data-gowdk-validation>Invalid</div>'
    };
  }
  if (reload) {
    return {
      ok: true,
      status: 204,
      headers: new Headers({ 'X-GOWDK-Reload': '1' }),
      text: async () => ''
    };
  }
  return {
    ok: true,
    status: 200,
    headers: new Headers({ 'X-GOWDK-Fragment-Swap': swap }),
    text: async () => '<p>Updated</p>'
  };
};

` + runtime + `

async function flushRuntime() {
  await new Promise(resolve => setImmediate(resolve));
  await new Promise(resolve => setImmediate(resolve));
}

async function submit(target = form, submitter = null) {
  const event = new CustomEvent('submit', { cancelable: true });
  event.target = target;
  if (submitter) {
    event.submitter = submitter;
  }
  document.dispatchEvent(event);
  await flushRuntime();
  return event;
}

(async function() {
  assert.equal(eventSources.length, 1);
  assert.equal(eventSources[0].url, '/_gowdk/realtime/events');

  let realtimePatch;
  liveRegion.addEventListener('gowdk:realtime-patch', event => {
    realtimePatch = event.detail;
  });
  eventSources[0].emit('gowdk-presentation', {
    Category: 'presentation',
    Type: 'gowdk-generated-app/patients.PatientNotice',
    Value: {
      patch: {
        op: 'replaceHTML',
        html: '<p>Live</p>',
        swap: 'innerHTML'
      }
    }
  });
  assert.equal(liveRegion.innerHTML, '<p>Live</p>');
  assert.deepEqual(islandLifecycle.shift(), ['destroy', 'live-patients', false]);
  assert.deepEqual(islandLifecycle.shift(), ['mount']);
  assert.equal(document.activeElement, input);
  assert.equal(realtimePatch.region, liveRegion);
  assert.equal(realtimePatch.patch.html, '<p>Live</p>');

  let realtimeError;
  document.addEventListener('gowdk:realtime-error', event => {
    realtimeError = event.detail;
  });
  eventSources[0].emit('gowdk-presentation', {
    Category: 'presentation',
    Type: 'gowdk-generated-app/patients.PatientNotice',
    Value: {
      patch: {
        op: 'setText',
        text: 'Unsafe'
      }
    }
  });
  assert.equal(liveRegion.innerHTML, '<p>Live</p>');
  assert.match(realtimeError.error.message, /unsupported realtime patch operation/);

  let queryRefresh;
  document.addEventListener('gowdk:query-refresh', event => {
    queryRefresh = event.detail;
  });
  eventSources[0].emit('gowdk-presentation', {
    Category: 'presentation',
    Type: 'gowdk.query.invalidate',
    Value: {
      queries: ['gowdk-generated-app/patients.GetPatientPage']
    }
  });
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(request.url, 'http://example.test/newsletter');
  assert.equal(liveRegion.replacedWith, '');
  assert.equal(invalidatedRegion.replacedWith, '<section id="invalidated-patients" data-gowdk-query="patients.GetPatientPage" data-gowdk-query-type="gowdk-generated-app/patients.GetPatientPage"><p>Refetched</p></section>');
  assert.deepEqual(islandLifecycle.shift(), ['destroy', 'invalidated-patients', true]);
  assert.deepEqual(islandLifecycle.shift(), ['mount']);
  assert.deepEqual(queryRefresh.queries, ['gowdk-generated-app/patients.GetPatientPage']);
  request = null;
  requests = [];
  requestCount = 0;
  invalidatedRegion.replacedWith = '';
  invalidatedRegion.outerHTMLValue = '';

  let commandSuccess = null;
  let commandError = null;
  commandForm.addEventListener('gowdk:command-success', event => {
    commandSuccess = event.detail;
  });
  commandForm.addEventListener('gowdk:command-error', event => {
    commandError = event.detail;
  });
  function resetCommandHarness() {
    commandSuccess = null;
    commandError = null;
    realtimeError = null;
    queryRefresh = null;
    request = null;
    requests = [];
    requestCount = 0;
    invalidatedRegion.replacedWith = '';
    invalidatedRegion.outerHTMLValue = '';
  }

  let commandSubmit = await submit(commandForm, commandSubmitter);
  assert.equal(commandSubmit.defaultPrevented, true);
  assert.equal(requestCount, 1);
  assert.equal(request.url, '/commands/create');
  assert.equal(request.options.method, 'POST');
  assert.equal(request.options.redirect, 'manual');
  assert.equal(request.options.headers['Content-Type'], 'application/x-www-form-urlencoded;charset=UTF-8');
  assert.equal(request.options.headers['X-GOWDK-Command'], '1');
  assert.equal(typeof request.options.body, 'string');
  const commandBody = new URLSearchParams(request.options.body);
  assert.equal(commandBody.get('name'), 'Ada');
  assert.deepEqual(commandBody.getAll('intent'), ['publish']);
  assert.equal(commandSuccess.command, 'patients.CreatePatient');
  assert.equal(commandSuccess.result.id, 'patient-1');
  assert.deepEqual(commandSuccess.queries, ['gowdk-generated-app/patients.GetPatientPage']);
  assert.equal(commandError, null);
  assert.equal(invalidatedRegion.replacedWith, '');

  resetCommandHarness();
  commandMode = 'html';
  await submit(commandForm, commandSubmitter);
  assert.equal(commandSuccess, null);
  assert.match(commandError.error.message, /command response was not JSON/);
  assert.equal(commandError.status, 200);
  assert.equal(commandError.body, '<main>Login</main>');
  assert.equal(commandForm.attributes['aria-busy'], undefined);

  resetCommandHarness();
  commandMode = 'redirect';
  await submit(commandForm, commandSubmitter);
  assert.equal(commandSuccess, null);
  assert.match(commandError.error.message, /command request redirected/);
  assert.equal(commandError.status, 200);
  assert.equal(commandError.body, '<main>Login</main>');
  assert.equal(commandForm.attributes['aria-busy'], undefined);

  resetCommandHarness();
  commandMode = 'success';
  eventSources[0].close();
  refreshFail = true;
  await submit(commandForm, commandSubmitter);
  await flushRuntime();
  assert.equal(commandSuccess.result.id, 'patient-1');
  assert.equal(commandError, null);
  assert.deepEqual(requests.map(item => item.url), ['/commands/create', 'http://example.test/newsletter']);
  assert.match(realtimeError.error.message, /navigation request failed with status 500/);
  assert.deepEqual(realtimeError.queries, ['gowdk-generated-app/patients.GetPatientPage']);
  assert.equal(realtimeError.form, commandForm);
  assert.equal(commandForm.attributes['aria-busy'], undefined);
  refreshFail = false;
  request = null;
  requests = [];
  requestCount = 0;

  let afterSwap;
  form.addEventListener('gowdk:after-swap', event => {
    afterSwap = event.detail;
  });

  let validationBlocked;
  form.addEventListener('gowdk:validation-blocked', event => {
    validationBlocked = event.detail;
  });
  form.valid = false;
  request = null;
  const invalid = await submit();
  assert.equal(invalid.defaultPrevented, true);
  assert.equal(request, null);
  assert.equal(requestCount, 0);
  assert.equal(form.reported, true);
  assert.equal(validationBlocked.form, form);
  assert.equal(validationBlocked.target, target);
  assert.equal(form.attributes['aria-busy'], undefined);
  form.valid = true;
  form.reported = false;

  const inner = await submit();
  assert.equal(inner.defaultPrevented, true);
  assert.equal(request.url, '/newsletter');
  assert.equal(request.options.method, 'POST');
  assert.equal(request.options.headers['X-GOWDK-Partial'], '1');
  assert.equal(request.options.headers['X-GOWDK-Target'], '#newsletter');
  assert.equal(request.options.headers['X-GOWDK-Swap'], 'innerHTML');
  assert.equal(target.innerHTML, '<p>Updated</p>');
  assert.deepEqual(islandLifecycle.shift(), ['destroy', 'newsletter', false]);
  assert.deepEqual(islandLifecycle.shift(), ['mount']);
  assert.equal(form.attributes['aria-busy'], undefined);
  assert.equal(document.activeElement, input);
  assert.equal(afterSwap.form, form);
  assert.equal(afterSwap.target, target);
  assert.equal(afterSwap.swap, 'innerHTML');

  swap = 'outerHTML';
  form.dataset.gowdkSwap = 'outerHTML';
  await submit();
  assert.equal(request.options.headers['X-GOWDK-Swap'], 'outerHTML');
  assert.deepEqual(islandLifecycle.shift(), ['destroy', 'newsletter', true]);
  assert.deepEqual(islandLifecycle.shift(), ['mount']);
  assert.equal(target.replacedWith, '<p>Updated</p>');

  let requestError;
  form.addEventListener('gowdk:request-error', event => {
    requestError = event.detail;
  });
  fail = true;
  await submit();
  assert.equal(requestError.form, form);
  assert.equal(requestError.target, target);
  assert.equal(requestError.status, 422);
  assert.equal(requestError.body, '<div data-gowdk-validation>Invalid</div>');
  assert.equal(requestError.error.status, 422);
  assert.equal(requestError.error.body, '<div data-gowdk-validation>Invalid</div>');
  assert.equal(requestError.response.status, 422);
  assert.equal(form.attributes['aria-busy'], undefined);

  fail = false;
  reload = true;
  await submit();
  assert.equal(window.location.reloaded, true);
  assert.equal(target.innerHTML, '<p>Updated</p>');
  assert.equal(form.attributes['aria-busy'], undefined);

  document.realtimeRegions = [];
  document.queryRegions = [];
  document.bySelector['[data-gowdk-subscribe]'] = null;
  reload = false;
  await submit();
  assert.equal(eventSources[0].closed, true);
}()).catch(error => {
  console.error(error && error.stack || error);
  process.exitCode = 1;
});
`
}
