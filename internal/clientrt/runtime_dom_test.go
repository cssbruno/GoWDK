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
    this.innerHTML = '';
    this.id = '';
    this.name = '';
    this.method = '';
    this.action = '';
    this.replacedWith = '';
  }
  closest(selector) {
    if (selector === 'form[data-gowdk-target]' && this.tagName === 'FORM' && this.dataset.gowdkTarget) {
      return this;
    }
    return null;
  }
  setAttribute(name, value) {
    this.attributes[name] = String(value);
  }
  removeAttribute(name) {
    delete this.attributes[name];
  }
  focus() {
    document.activeElement = this;
  }
  set outerHTML(value) {
    this.replacedWith = value;
  }
  get outerHTML() {
    return this.replacedWith;
  }
}

class Document extends EventTarget {
  constructor() {
    super();
    this.body = new Element('body');
    this.activeElement = this.body;
    this.bySelector = {};
    this.byID = {};
  }
  querySelector(selector) {
    return this.bySelector[selector] || null;
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
global.FormData = class {
  constructor(form) {
    this.form = form;
  }
};

const form = new Element('form');
form.method = 'post';
form.action = '/newsletter';
form.dataset.gowdkTarget = '#newsletter';
form.dataset.gowdkSwap = 'innerHTML';
const target = new Element('section');
target.id = 'newsletter';
target.innerHTML = '<p>Old</p>';
const input = new Element('input');
input.id = 'email';

document.bySelector['#newsletter'] = target;
document.byID.newsletter = target;
document.byID.email = input;
document.activeElement = input;

let request;
let swap = 'innerHTML';
global.fetch = async function(url, options) {
  request = { url, options };
  return {
    ok: true,
    status: 200,
    headers: new Headers({ 'X-GOWDK-Fragment-Swap': swap }),
    text: async () => '<p>Updated</p>'
  };
};

` + runtime + `

async function submit() {
  const event = new CustomEvent('submit', { cancelable: true });
  event.target = form;
  document.dispatchEvent(event);
  await new Promise(resolve => setImmediate(resolve));
  return event;
}

(async function() {
  let afterSwap;
  form.addEventListener('gowdk:after-swap', event => {
    afterSwap = event.detail;
  });

  const inner = await submit();
  assert.equal(inner.defaultPrevented, true);
  assert.equal(request.url, '/newsletter');
  assert.equal(request.options.method, 'POST');
  assert.equal(request.options.headers['X-GOWDK-Partial'], '1');
  assert.equal(request.options.headers['X-GOWDK-Target'], '#newsletter');
  assert.equal(request.options.headers['X-GOWDK-Swap'], 'innerHTML');
  assert.equal(target.innerHTML, '<p>Updated</p>');
  assert.equal(form.attributes['aria-busy'], undefined);
  assert.equal(document.activeElement, input);
  assert.equal(afterSwap.form, form);
  assert.equal(afterSwap.target, target);
  assert.equal(afterSwap.swap, 'innerHTML');

  swap = 'outerHTML';
  form.dataset.gowdkSwap = 'outerHTML';
  await submit();
  assert.equal(request.options.headers['X-GOWDK-Swap'], 'outerHTML');
  assert.equal(target.replacedWith, '<p>Updated</p>');
}()).catch(error => {
  console.error(error && error.stack || error);
  process.exitCode = 1;
});
`
}
