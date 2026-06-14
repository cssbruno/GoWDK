package buildgen

import (
	"fmt"
	"path"
	"strconv"
)

func islandJSSource(componentName string, includeSourceMap bool) string {
	component := strconv.Quote(componentName)
	name := componentAssetName(componentName)
	mountFunction := "mount" + name + "Island"
	destroyFunction := "destroy" + name + "Island"
	source := fmt.Sprintf(`(() => {
  const component = %s;
  const selector = "gowdk-island[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"js\"]";
  const booleanAttrs = new Set(["allowfullscreen", "async", "autofocus", "autoplay", "checked", "controls", "default", "defer", "disabled", "formnovalidate", "hidden", "inert", "ismap", "loop", "multiple", "muted", "nomodule", "novalidate", "open", "readonly", "required", "reversed", "selected"]);
  const staleAsyncResult = Symbol("gowdk stale async result");
  const bindingTable = Object.freeze([
    { kind: "text", selector: "[data-gowdk-binding-text]", id: "data-gowdk-binding-text", field: "data-gowdk-bind" },
    { kind: "value", selector: "[data-gowdk-binding-value]", id: "data-gowdk-binding-value", field: "data-gowdk-bind-value" },
    { kind: "checked", selector: "[data-gowdk-binding-checked]", id: "data-gowdk-binding-checked", field: "data-gowdk-bind-checked" },
    { kind: "conditional", selector: "[data-gowdk-binding-if]", id: "data-gowdk-binding-if" },
    { kind: "list", selector: "[data-gowdk-binding-list]", id: "data-gowdk-binding-list" },
    { kind: "class", attrPrefix: "data-gowdk-binding-class-", valuePrefix: "data-gowdk-class-" },
    { kind: "style", attrPrefix: "data-gowdk-binding-style-", valuePrefix: "data-gowdk-style-", unitPrefix: "data-gowdk-style-unit-" },
    { kind: "attr", attrPrefix: "data-gowdk-binding-attr-", valuePrefix: "data-gowdk-attr-" }
  ]);
  const registry = window.__gowdkIslandRegistry || (window.__gowdkIslandRegistry = { components: Object.create(null), roots: new WeakMap() });
  window.__gowdkMountIslands = () => {
    Object.keys(registry.components).forEach((name) => registry.components[name](document));
  };
  window.__gowdkDestroyIslands = (scope, includeRoot) => {
    scope = scope || document;
    const roots = [];
    if (includeRoot && scope.matches && scope.matches("gowdk-island")) roots.push(scope);
    if (scope.querySelectorAll) scope.querySelectorAll("gowdk-island").forEach((root) => roots.push(root));
    roots.forEach((root) => {
      const destroy = registry.roots.get(root);
      if (destroy) destroy();
    });
  };

  function callHelper(name, args, state, helpers, stack) {
    const helper = helpers && helpers[name];
    if (!helper) return null;
    stack = stack || [];
    if (stack.indexOf(name) >= 0) throw new Error("recursive GOWDK helper " + name);
    const nextScope = Object.create(null);
    (helper.params || []).forEach((param, index) => {
      nextScope[param] = args[index];
    });
    return valueOf(helper.return || "", state, nextScope, helpers, stack.concat([name]));
  }

  const builtins = Object.freeze({
    len(value) {
      if (value == null) return 0;
      if (typeof value === "string" || Array.isArray(value)) return value.length;
      return 0;
    },
    string(value) {
      if (value == null) return "";
      return String(value);
    },
    lower(value) {
      return String(value == null ? "" : value).toLowerCase();
    },
    upper(value) {
      return String(value == null ? "" : value).toUpperCase();
    },
    contains(value, query) {
      return String(value == null ? "" : value).includes(String(query == null ? "" : query));
    },
    int(value) {
      const next = Number.parseInt(value, 10);
      return Number.isNaN(next) ? 0 : next;
    },
    float(value) {
      const next = Number.parseFloat(value);
      return Number.isNaN(next) ? 0 : next;
    }
  });

  const expressionCache = Object.create(null);

  function parseExpression(source) {
    source = String(source || "").trim();
    if (!expressionCache[source]) expressionCache[source] = expressionParser(tokenizeExpression(source)).parse();
    return expressionCache[source];
  }

  function tokenizeExpression(source) {
    const tokens = [];
    let index = 0;
    while (index < source.length) {
      const char = source[index];
      if (/\s/.test(char)) {
        index++;
        continue;
      }
      if (/[A-Za-z_]/.test(char)) {
        const start = index++;
        while (index < source.length && /[A-Za-z0-9_]/.test(source[index])) index++;
        tokens.push({ kind: "ident", value: source.slice(start, index) });
        continue;
      }
      if (/[0-9]/.test(char)) {
        const start = index++;
        while (index < source.length && /[0-9]/.test(source[index])) index++;
        if (source[index] === ".") {
          index++;
          while (index < source.length && /[0-9]/.test(source[index])) index++;
        }
        tokens.push({ kind: "number", value: source.slice(start, index) });
        continue;
      }
      if (char === "\"") {
        const start = index++;
        let escaped = false;
        while (index < source.length) {
          const next = source[index++];
          if (escaped) {
            escaped = false;
            continue;
          }
          if (next === "\\") escaped = true;
          else if (next === "\"") break;
        }
        tokens.push({ kind: "string", value: source.slice(start, index) });
        continue;
      }
      const pair = source.slice(index, index + 2);
      if (["==", "!=", "<=", ">=", "&&", "||"].indexOf(pair) >= 0) {
        tokens.push({ kind: "op", value: pair });
        index += 2;
        continue;
      }
      if ("+-*/%%!<>".indexOf(char) >= 0) {
        tokens.push({ kind: "op", value: char });
        index++;
        continue;
      }
      if ("()[]{}.,:".indexOf(char) >= 0) {
        tokens.push({ kind: char, value: char });
        index++;
        continue;
      }
      throw new Error("unsupported GOWDK expression token " + char);
    }
    tokens.push({ kind: "eof", value: "" });
    return tokens;
  }

  function expressionParser(tokens) {
    let index = 0;
    const parser = {
      parse() {
        const expr = this.parseConditional();
        this.expect("eof");
        return expr;
      },
      peek() {
        return tokens[index] || { kind: "eof", value: "" };
      },
      match(kind, value) {
        const token = this.peek();
        if (token.kind !== kind || (value != null && token.value !== value)) return false;
        index++;
        return true;
      },
      expect(kind, value) {
        const token = this.peek();
        if (!this.match(kind, value)) throw new Error("expected " + (value || kind) + " in GOWDK expression");
        return token;
      },
      parseConditional() {
        if (this.match("ident", "if")) {
          const cond = this.parseOr();
          this.expect("{");
          const thenExpr = this.parseConditional();
          this.expect("}");
          this.expect("ident", "else");
          this.expect("{");
          const elseExpr = this.parseConditional();
          this.expect("}");
          return { kind: "if", cond, thenExpr, elseExpr };
        }
        return this.parseOr();
      },
      parseOr() {
        let expr = this.parseAnd();
        while (this.match("op", "||")) expr = { kind: "binary", op: "||", left: expr, right: this.parseAnd() };
        return expr;
      },
      parseAnd() {
        let expr = this.parseEquality();
        while (this.match("op", "&&")) expr = { kind: "binary", op: "&&", left: expr, right: this.parseEquality() };
        return expr;
      },
      parseEquality() {
        let expr = this.parseCompare();
        while (this.peek().kind === "op" && (this.peek().value === "==" || this.peek().value === "!=")) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseCompare() };
        }
        return expr;
      },
      parseCompare() {
        let expr = this.parseTerm();
        while (this.peek().kind === "op" && ["<", "<=", ">", ">="].indexOf(this.peek().value) >= 0) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseTerm() };
        }
        return expr;
      },
      parseTerm() {
        let expr = this.parseFactor();
        while (this.peek().kind === "op" && (this.peek().value === "+" || this.peek().value === "-")) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseFactor() };
        }
        return expr;
      },
      parseFactor() {
        let expr = this.parseUnary();
        while (this.peek().kind === "op" && ["*", "/", "%%"].indexOf(this.peek().value) >= 0) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseUnary() };
        }
        return expr;
      },
      parseUnary() {
        if (this.peek().kind === "op" && (this.peek().value === "!" || this.peek().value === "-")) {
          const op = this.expect("op").value;
          return { kind: "unary", op, expr: this.parseUnary() };
        }
        return this.parsePostfix();
      },
      parsePostfix() {
        let expr = this.parsePrimary();
        for (;;) {
          if (this.match(".")) {
            expr = { kind: "member", target: expr, name: this.expect("ident").value };
            continue;
          }
          if (this.match("[")) {
            const item = this.parseConditional();
            this.expect("]");
            expr = { kind: "index", target: expr, index: item };
            continue;
          }
          if (this.match("(")) {
            const args = [];
            if (!this.match(")")) {
              do {
                args.push(this.parseConditional());
              } while (this.match(","));
              this.expect(")");
            }
            if (expr.kind !== "ident") throw new Error("only named GOWDK helpers can be called");
            expr = { kind: "call", name: expr.name, args };
            continue;
          }
          return expr;
        }
      },
      parsePrimary() {
        const token = this.peek();
        if (this.match("number")) return { kind: "literal", value: Number(token.value) };
        if (this.match("string")) return { kind: "literal", value: JSON.parse(token.value) };
        if (this.match("ident", "true")) return { kind: "literal", value: true };
        if (this.match("ident", "false")) return { kind: "literal", value: false };
        if (this.match("ident", "nil") || this.match("ident", "null")) return { kind: "literal", value: null };
        if (this.match("ident")) return { kind: "ident", name: token.value };
        if (this.match("(")) {
          const expr = this.parseConditional();
          this.expect(")");
          return expr;
        }
        if (this.match("{")) {
          const fields = [];
          if (!this.match("}")) {
            do {
              const name = this.expect("ident").value;
              this.expect(":");
              fields.push([name, this.parseConditional()]);
            } while (this.match(","));
            this.expect("}");
          }
          return { kind: "object", fields };
        }
        throw new Error("unsupported GOWDK expression");
      }
    };
    return parser;
  }

  async function fetchJSON(url, signal) {
    const response = await fetch(String(url), { headers: { "Accept": "application/json" }, signal });
    if (!response.ok) throw new Error("GOWDK fetchJSON failed with HTTP " + response.status);
    const contentType = response.headers.get("content-type") || "";
    if (!/\bapplication\/json\b|\+json\b/i.test(contentType)) {
      throw new Error("GOWDK fetchJSON expected JSON response");
    }
    const text = await response.text();
    if (text.trim() === "") return null;
    try {
      return JSON.parse(text);
    } catch (_error) {
      throw new Error("GOWDK fetchJSON received invalid JSON");
    }
  }

  function clearAsyncError(state) {
    if (Object.prototype.hasOwnProperty.call(state, "Error")) state.Error = "";
  }

  function recordAsyncError(state, error) {
    if (Object.prototype.hasOwnProperty.call(state, "Error")) {
      state.Error = error && error.message ? error.message : String(error || "async error");
    }
    if (Object.prototype.hasOwnProperty.call(state, "Loading")) {
      state.Loading = false;
    }
  }

  function valueOf(token, state, scope, helpers, stack) {
    return evalExpression(parseExpression(token), state, scope || null, helpers || {}, stack || []);
  }

  function evalExpression(expr, state, scope, helpers, stack) {
    switch (expr.kind) {
      case "literal":
        return expr.value;
      case "ident":
        if (scope && Object.prototype.hasOwnProperty.call(scope, expr.name)) return scope[expr.name];
        if (Object.prototype.hasOwnProperty.call(state, expr.name)) return state[expr.name];
        return undefined;
      case "member": {
        const target = evalExpression(expr.target, state, scope, helpers, stack);
        return target == null ? undefined : target[expr.name];
      }
      case "index": {
        const target = evalExpression(expr.target, state, scope, helpers, stack);
        const index = evalExpression(expr.index, state, scope, helpers, stack);
        return target == null ? undefined : target[index];
      }
      case "object": {
        const out = {};
        expr.fields.forEach((field) => {
          out[field[0]] = evalExpression(field[1], state, scope, helpers, stack);
        });
        return out;
      }
      case "call": {
        const args = expr.args.map((arg) => evalExpression(arg, state, scope, helpers, stack));
        if (Object.prototype.hasOwnProperty.call(builtins, expr.name)) return builtins[expr.name].apply(null, args);
        return callHelper(expr.name, args, state, helpers, stack);
      }
      case "unary": {
        const value = evalExpression(expr.expr, state, scope, helpers, stack);
        if (expr.op === "!") return !Boolean(value);
        if (expr.op === "-") return -Number(value);
        return undefined;
      }
      case "binary":
        return evalBinaryExpression(expr, state, scope, helpers, stack);
      case "if":
        return Boolean(evalExpression(expr.cond, state, scope, helpers, stack))
          ? evalExpression(expr.thenExpr, state, scope, helpers, stack)
          : evalExpression(expr.elseExpr, state, scope, helpers, stack);
      default:
        return undefined;
    }
  }

  function evalBinaryExpression(expr, state, scope, helpers, stack) {
    if (expr.op === "&&") return Boolean(evalExpression(expr.left, state, scope, helpers, stack)) && Boolean(evalExpression(expr.right, state, scope, helpers, stack));
    if (expr.op === "||") return Boolean(evalExpression(expr.left, state, scope, helpers, stack)) || Boolean(evalExpression(expr.right, state, scope, helpers, stack));
    const left = evalExpression(expr.left, state, scope, helpers, stack);
    const right = evalExpression(expr.right, state, scope, helpers, stack);
    switch (expr.op) {
      case "==":
        return left === right;
      case "!=":
        return left !== right;
      case "<":
        return left < right;
      case "<=":
        return left <= right;
      case ">":
        return left > right;
      case ">=":
        return left >= right;
      case "+":
        return left + right;
      case "-":
        return Number(left) - Number(right);
      case "*":
        return Number(left) * Number(right);
      case "/":
        return Number(left) / Number(right);
      case "%%":
        return Number(left) %% Number(right);
      default:
        return undefined;
    }
  }

  function recomputeComputed(state, computeds, helpers) {
    (computeds || []).forEach((computed) => {
      state[computed.name] = valueOf(computed.expr, state, null, helpers);
    });
  }

  function splitArgs(source) {
    source = source.trim();
    if (!source) return [];
    const args = [];
    let start = 0;
    let depth = 0;
    let inString = false;
    let escaped = false;
    for (let i = 0; i < source.length; i++) {
      const char = source[i];
      if (escaped) {
        escaped = false;
        continue;
      }
      if (inString) {
        if (char === "\\") escaped = true;
        else if (char === "\"") inString = false;
        continue;
      }
      if (char === "\"") inString = true;
      else if (char === "(" || char === "[" || char === "{") depth++;
      else if (char === ")" || char === "]" || char === "}") depth--;
      else if (char === ",") {
        if (depth > 0) continue;
        args.push(source.slice(start, i).trim());
        start = i + 1;
      }
    }
    args.push(source.slice(start).trim());
    return args;
  }

  function emitComponentEvent(root, emitEvents, name, args, state, scope, helpers) {
    const event = emitEvents && emitEvents[name];
    if (!event) return;
    const payload = Object.create(null);
    (event.params || []).forEach((param, index) => {
      payload[param] = valueOf(args[index] || "", state, scope, helpers);
    });
    root.dispatchEvent(new CustomEvent(name, { detail: payload, bubbles: true }));
  }

  async function applyExpression(expr, state, handlers, helpers, scope, refs, computeds, asyncTokens, root, emitEvents) {
    expr = expr.trim();
    let local = expr.match(/^let\s+([A-Za-z_][A-Za-z0-9_]*)\s+[A-Za-z_][A-Za-z0-9_]*\s*=\s*(.+)$/);
    if (local) {
      if (!scope) return;
      scope[local[1]] = valueOf(local[2], state, scope, helpers);
      return;
    }
    let awaitedFetch = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*await\s+fetchJSON\[(.*)\]\((.*)\)$/);
    if (awaitedFetch) {
      const target = awaitedFetch[1];
      const token = (asyncTokens[target] || 0) + 1;
      const controllers = asyncTokens.__controllers || (asyncTokens.__controllers = Object.create(null));
      if (controllers[target] && typeof controllers[target].abort === "function") controllers[target].abort();
      const controller = typeof AbortController === "undefined" ? null : new AbortController();
      controllers[target] = controller;
      asyncTokens[target] = token;
      clearAsyncError(state);
      let next;
      try {
        next = await fetchJSON(valueOf(awaitedFetch[3], state, scope, helpers), controller ? controller.signal : undefined);
      } catch (error) {
        if (asyncTokens[target] !== token || (error && error.name === "AbortError")) throw staleAsyncResult;
        delete asyncTokens[target];
        if (controllers[target] === controller) delete controllers[target];
        throw error;
      }
      if (asyncTokens[target] !== token) throw staleAsyncResult;
      state[target] = next;
      delete asyncTokens[target];
      if (controllers[target] === controller) delete controllers[target];
      return;
    }
    let call = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\((.*)\)$/);
    if (call) {
      if (call[1] === "append" || call[1] === "remove" || call[1] === "move") {
        const args = splitArgs(call[2]);
        const field = (args[0] || "").trim();
        if (!Array.isArray(state[field])) return;
        if (call[1] === "append" && args.length === 2) {
          state[field] = state[field].concat([valueOf(args[1], state, scope, helpers)]);
          return;
        }
        if (call[1] === "remove" && args.length === 2) {
          const index = Number(valueOf(args[1], state, scope, helpers));
          if (!Number.isInteger(index) || index < 0 || index >= state[field].length) return;
          state[field] = state[field].slice(0, index).concat(state[field].slice(index + 1));
          return;
        }
        if (call[1] === "move" && args.length === 3) {
          const from = Number(valueOf(args[1], state, scope, helpers));
          const to = Number(valueOf(args[2], state, scope, helpers));
          if (!Number.isInteger(from) || !Number.isInteger(to) || from < 0 || from >= state[field].length || to < 0 || to >= state[field].length || from === to) return;
          const next = state[field].slice();
          const item = next.splice(from, 1)[0];
          next.splice(to, 0, item);
          state[field] = next;
          return;
        }
        return;
      }
      const handler = handlers[call[1]];
      if (!handler) return;
      const params = handler.params || [];
      const args = splitArgs(call[2]);
      const nextScope = Object.create(null);
      params.forEach((param, index) => {
        nextScope[param] = valueOf(args[index] || "", state, scope, helpers);
      });
      for (const statement of (handler.statements || handler)) {
        await applyExpression(statement, state, handlers, helpers, nextScope, refs, computeds, asyncTokens, root, emitEvents);
        recomputeComputed(state, computeds, helpers);
      }
      return;
    }
    let emit = expr.match(/^emit\s+([A-Za-z_][A-Za-z0-9_]*)\((.*)\)$/);
    if (emit) {
      emitComponentEvent(root, emitEvents, emit[1], splitArgs(emit[2]), state, scope, helpers);
      return;
    }
    let refCall = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\.(Focus|Blur|ScrollIntoView)\(\)$/);
    if (refCall) {
      const node = refs && refs[refCall[1]];
      if (!node) return;
      if (refCall[2] === "Focus" && typeof node.focus === "function") node.focus();
      else if (refCall[2] === "Blur" && typeof node.blur === "function") node.blur();
      else if (refCall[2] === "ScrollIntoView" && typeof node.scrollIntoView === "function") node.scrollIntoView();
      return;
    }
    let match = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)(\+\+|--)$/);
    if (match) {
      const current = Number(state[match[1]] || 0);
      state[match[1]] = match[2] === "++" ? current + 1 : current - 1;
      return;
    }
    match = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$/);
    if (match) {
      const target = match[1];
      const value = match[2].trim();
      const toggle = value.match(/^!\s*([A-Za-z_][A-Za-z0-9_]*)$/);
      state[target] = toggle ? !Boolean(valueOf(toggle[1], state, scope, helpers)) : valueOf(value, state, scope, helpers);
    }
  }

  async function applyStatements(statements, state, handlers, helpers, scope, refs, computeds, asyncTokens, root, emitEvents) {
    for (const statement of (statements || [])) {
      await applyExpression(statement, state, handlers, helpers, scope || null, refs, computeds, asyncTokens, root, emitEvents);
      recomputeComputed(state, computeds, helpers);
    }
  }

  function escapeHTML(value) {
    return String(value).replace(/[&<>"']/g, (char) => {
      if (char === "&") return "&amp;";
      if (char === "<") return "&lt;";
      if (char === ">") return "&gt;";
      if (char === "\"") return "&quot;";
      return "&#39;";
    });
  }

  function interpolateTemplate(template, state, scope, helpers) {
    return template.replace(/\{\{([^{}]+)\}\}/g, (_match, expr) => {
      const value = valueOf(expr, state, scope, helpers);
      return value == null ? "" : escapeHTML(value);
    });
  }

  function firstTemplateElement(html) {
    const holder = document.createElement("template");
    holder.innerHTML = html.trim();
    return holder.content.firstElementChild;
  }

  function syncElement(target, source) {
    Array.from(target.attributes).forEach((attr) => {
      if (!source.hasAttribute(attr.name)) target.removeAttribute(attr.name);
    });
    Array.from(source.attributes).forEach((attr) => {
      if (target.getAttribute(attr.name) !== attr.value) target.setAttribute(attr.name, attr.value);
    });
    if (target.innerHTML !== source.innerHTML) target.innerHTML = source.innerHTML;
  }

  function matchingNodes(container, selector) {
    const nodes = [];
    if (container.matches && container.matches(selector)) nodes.push(container);
    container.querySelectorAll(selector).forEach((node) => nodes.push(node));
    return nodes;
  }

  function emptyBindings() {
    return { text: [], value: [], checked: [], classes: [], styles: [], attrs: [], conditionals: [], lists: [] };
  }

  function collectDirectBinding(bindings, spec, root) {
    matchingNodes(root, spec.selector).forEach((node) => {
      if (!ownsNode(root, node)) return;
      const id = node.getAttribute(spec.id);
      if (spec.kind === "text") bindings.text.push({ id, node, field: node.getAttribute(spec.field) });
      else if (spec.kind === "value") bindings.value.push({ id, node, field: node.getAttribute(spec.field) });
      else if (spec.kind === "checked") bindings.checked.push({ id, node, field: node.getAttribute(spec.field) });
      else if (spec.kind === "conditional") bindings.conditionals.push({ id, node });
      else if (spec.kind === "list") bindings.lists.push({ id, node });
    });
  }

  function collectPrefixBinding(bindings, spec, node, attr) {
    if (!attr.name.startsWith(spec.attrPrefix)) return;
    const name = attr.name.slice(spec.attrPrefix.length);
    if (spec.kind === "class") {
      bindings.classes.push({ id: attr.value, node, name, expression: node.getAttribute(spec.valuePrefix + name) });
    } else if (spec.kind === "style") {
      bindings.styles.push({ id: attr.value, node, name, expression: node.getAttribute(spec.valuePrefix + name), unit: node.getAttribute(spec.unitPrefix + name) || "" });
    } else if (spec.kind === "attr") {
      bindings.attrs.push({ id: attr.value, node, name, expression: node.getAttribute(spec.valuePrefix + name) });
    }
  }

  function collectBindings(root) {
    const bindings = emptyBindings();
    bindingTable.filter((spec) => spec.selector).forEach((spec) => collectDirectBinding(bindings, spec, root));
    const prefixSpecs = bindingTable.filter((spec) => spec.attrPrefix);
    root.querySelectorAll("*").forEach((node) => {
      if (!ownsNode(root, node)) return;
      Array.from(node.attributes).forEach((attr) => {
        prefixSpecs.forEach((spec) => collectPrefixBinding(bindings, spec, node, attr));
      });
    });
    return bindings;
  }

  function conditionalRecords(root, options) {
    options = options || {};
    const records = root.__gowdkConditionalRecords || (root.__gowdkConditionalRecords = new Map());
    const shouldSkip = (node) => {
      if (options.owner && !ownsNode(options.owner, node)) return true;
      if (options.skipLoopItems && node.closest("[data-gowdk-for-item]")) return true;
      return false;
    };
    matchingNodes(root, "[data-gowdk-binding-if]").forEach((node) => {
      if (shouldSkip(node)) return;
      const id = node.getAttribute("data-gowdk-binding-if");
      if (!id || records.has(id)) return;
      const marker = document.createComment("gowdk-if:" + id);
      const template = node.cloneNode(true);
      node.parentNode.insertBefore(marker, node);
      records.set(id, {
        id,
        marker,
        template,
        current: node,
        group: node.getAttribute("data-gowdk-if-group") || "",
        index: Number(node.getAttribute("data-gowdk-if-index") || "0")
      });
    });
    return Array.from(records.values()).filter((record) => record.marker && record.marker.isConnected);
  }

  function mountConditional(record) {
    if (record.current && record.current.isConnected) return record.current;
    const node = record.template.cloneNode(true);
    node.hidden = false;
    record.marker.parentNode.insertBefore(node, record.marker.nextSibling);
    record.current = node;
    return node;
  }

  function unmountConditional(record) {
    if (!record.current || !record.current.isConnected) return;
    record.current.parentNode.removeChild(record.current);
    record.current = null;
  }

  function renderConditionals(container, state, scope, helpers, options) {
    options = options || {};
    const records = conditionalRecords(container, options);
    records.forEach((record) => {
      if (record.group) return;
      const condition = record.template.getAttribute("data-gowdk-if");
      const visible = condition == null || Boolean(valueOf(condition, state, scope, helpers));
      if (visible) mountConditional(record);
      else unmountConditional(record);
    });
    const conditionalGroups = new Map();
    records.forEach((record) => {
      if (!record.group) return;
      if (!conditionalGroups.has(record.group)) conditionalGroups.set(record.group, []);
      conditionalGroups.get(record.group).push(record);
    });
    conditionalGroups.forEach((groupRecords) => {
      groupRecords.sort((left, right) => left.index - right.index);
      let matched = false;
      groupRecords.forEach((record) => {
        const condition = record.template.getAttribute("data-gowdk-if");
        const visible = !matched && (condition == null || Boolean(valueOf(condition, state, scope, helpers)));
        if (visible) mountConditional(record);
        else unmountConditional(record);
        if (visible) matched = true;
      });
    });
  }

  function renderListLoops(root, state, helpers, bindings) {
    const markers = bindings ? bindings.lists.map((binding) => binding.node).filter((node) => node.isConnected) : Array.from(root.querySelectorAll("template[data-gowdk-for]"));
    markers.forEach((marker) => {
      if (!ownsNode(root, marker)) return;
      const group = marker.getAttribute("data-gowdk-for");
      const itemName = marker.getAttribute("data-gowdk-for-var");
      const indexName = marker.getAttribute("data-gowdk-for-index-var");
      const source = marker.getAttribute("data-gowdk-for-source");
      const keyExpr = marker.getAttribute("data-gowdk-for-key");
      const template = marker.getAttribute("data-gowdk-for-template") || "";
      const items = valueOf(source, state, null, helpers);
      const existing = new Map();
      let cursor = marker.nextSibling;
      while (cursor) {
        const next = cursor.nextSibling;
        if (cursor.nodeType !== 1 || cursor.getAttribute("data-gowdk-for-item") !== group) break;
        existing.set(cursor.getAttribute("data-gowdk-key-value") || "", cursor);
        cursor = next;
      }
      if (!Array.isArray(items)) return;
      const fragment = document.createDocumentFragment();
      const used = new Set();
      items.forEach((item, index) => {
        const scope = Object.create(null);
        scope[itemName] = item;
        scope.index = index;
        if (indexName) scope[indexName] = index;
        const key = String(valueOf(keyExpr, state, scope, helpers) ?? "");
        const fresh = firstTemplateElement(interpolateTemplate(template, state, scope, helpers));
        if (!fresh) return;
        renderConditionals(fresh, state, scope, helpers);
        const reused = existing.get(key);
        if (reused && !used.has(key)) {
          syncElement(reused, fresh);
          fragment.appendChild(reused);
          used.add(key);
          return;
        }
        fragment.appendChild(fresh);
        used.add(key);
      });
      marker.parentNode.insertBefore(fragment, marker.nextSibling);
      existing.forEach((node, key) => {
        if (!used.has(key) && node.parentNode) node.parentNode.removeChild(node);
      });
    });
  }

  function eventModifiers(source) {
    const modifiers = { prevent: false, stop: false, once: false, capture: false, debounce: 0, throttle: 0 };
    (source || "").split(/\s+/).filter(Boolean).forEach((item) => {
      if (item === "prevent") modifiers.prevent = true;
      else if (item === "stop") modifiers.stop = true;
      else if (item === "once") modifiers.once = true;
      else if (item === "capture") modifiers.capture = true;
      else if (item.startsWith("debounce:")) modifiers.debounce = Number(item.slice("debounce:".length)) || 0;
      else if (item.startsWith("throttle:")) modifiers.throttle = Number(item.slice("throttle:".length)) || 0;
    });
    return modifiers;
  }

  function domEventScope(domEvent) {
    const target = domEvent && domEvent.target ? domEvent.target : {};
    return {
      event: {
        value: target.value == null ? "" : String(target.value),
        checked: Boolean(target.checked),
        key: domEvent && domEvent.key ? String(domEvent.key) : "",
        code: domEvent && domEvent.code ? String(domEvent.code) : "",
        clientX: domEvent && typeof domEvent.clientX === "number" ? domEvent.clientX : 0,
        clientY: domEvent && typeof domEvent.clientY === "number" ? domEvent.clientY : 0
      }
    };
  }

  function ownsNode(root, node) {
    return node.closest("gowdk-island") === root;
  }

  function syncChildProps(root, state, helpers) {
    root.querySelectorAll("gowdk-island[data-gowdk-props]").forEach((node) => {
      const props = JSON.parse(node.getAttribute("data-gowdk-props") || "{}");
      const payload = Object.create(null);
      Object.keys(props).forEach((name) => {
        payload[name] = valueOf(props[name], state, null, helpers);
      });
      node.dispatchEvent(new CustomEvent("gowdk:props", { detail: payload }));
    });
  }

  function dispatchComponentExports(root, exportNames, state, active) {
    if (!Array.isArray(exportNames) || exportNames.length === 0) return;
    const payload = Object.create(null);
    payload.active = Boolean(active);
    exportNames.forEach((name) => {
      payload[name] = active ? state[name] : null;
    });
    root.__gowdkExports = payload;
    root.dispatchEvent(new CustomEvent("exports", { detail: payload, bubbles: true }));
    root.dispatchEvent(new CustomEvent("gowdk:exports", { detail: payload, bubbles: true }));
  }

  function updateTextBindings(bindings, state) {
    bindings.text.forEach(({ node, field }) => {
      node.textContent = state[field] == null ? "" : String(state[field]);
    });
  }

  function updateValueBindings(bindings, state) {
    bindings.value.forEach(({ node, field }) => {
      if (node.type === "radio") {
        node.checked = String(state[field] == null ? "" : state[field]) === node.value;
        return;
      }
      const value = state[field] == null ? "" : String(state[field]);
      if (document.activeElement !== node && node.value !== value) node.value = value;
    });
  }

  function updateCheckedBindings(bindings, state) {
    bindings.checked.forEach(({ node, field }) => {
      const checked = Boolean(state[field]);
      if (node.checked !== checked) node.checked = checked;
    });
  }

  function updateClassBindings(bindings, state, helpers) {
    bindings.classes.forEach(({ node, name, expression }) => {
      node.classList.toggle(name, Boolean(valueOf(expression, state, null, helpers)));
    });
  }

  function updateStyleBindings(bindings, state, helpers) {
    bindings.styles.forEach(({ node, name, expression, unit }) => {
      const value = valueOf(expression, state, null, helpers);
      if (value == null || value === false || value === "") node.style.removeProperty(name);
      else node.style.setProperty(name, String(value) + unit);
    });
  }

  function updateAttrBindings(bindings, state, helpers) {
    bindings.attrs.forEach(({ node, name, expression }) => {
      const value = valueOf(expression, state, null, helpers);
      if (booleanAttrs.has(name)) {
        if (Boolean(value)) node.setAttribute(name, "");
        else node.removeAttribute(name);
        return;
      }
      if (value == null || value === false) node.removeAttribute(name);
      else node.setAttribute(name, String(value));
    });
  }

  function updateBindings(root, state, helpers, bindings) {
    updateTextBindings(bindings, state);
    updateValueBindings(bindings, state);
    updateCheckedBindings(bindings, state);
    updateClassBindings(bindings, state, helpers);
    updateStyleBindings(bindings, state, helpers);
    updateAttrBindings(bindings, state, helpers);
  }

  function render(root, state, helpers, bindings) {
    renderListLoops(root, state, helpers, bindings);
    bindings = collectBindings(root);
    renderConditionals(root, state, null, helpers, { owner: root, skipLoopItems: true });
    bindings = collectBindings(root);
    updateBindings(root, state, helpers, bindings);
    root.setAttribute("data-gowdk-state", JSON.stringify(state));
    return bindings;
  }

  async function %s(scope) {
    scope = scope || document;
    scope.querySelectorAll(selector).forEach(async (root) => {
    if (root.getAttribute("data-gowdk-mounted") === "js") return;
    root.setAttribute("data-gowdk-mounted", "js");
    const state = JSON.parse(root.getAttribute("data-gowdk-state") || "{}");
    const client = JSON.parse(root.getAttribute("data-gowdk-client") || "{}");
    const hasEnvelope = Boolean(client.handlers || client.helpers || client.emits || client.exports || client.stores || client.mount || client.destroy || client.effects || client.computed);
    const handlers = hasEnvelope ? (client.handlers || {}) : client;
    const helpers = client.helpers || {};
    const emitEvents = client.emits || {};
    const exportNames = client.exports || [];
    const storeNames = Array.isArray(client.stores) ? client.stores : [];
    const storeRegistry = window.__gowdkStores;
    const mountStatements = client.mount || [];
    const destroyStatements = client.destroy || [];
    const effects = client.effects || [];
    const computeds = client.computed || [];
    const refs = Object.create(null);
    const asyncTokens = Object.create(null);
    storeNames.forEach((name) => {
      if (storeRegistry) Object.assign(state, storeRegistry.get(name));
    });
    recomputeComputed(state, computeds, helpers);
    root.querySelectorAll("[data-gowdk-ref]").forEach((node) => {
      if (!ownsNode(root, node)) return;
      refs[node.getAttribute("data-gowdk-ref")] = node;
    });
    const effectValues = Object.create(null);
    const effectCleanups = Object.create(null);
    effects.forEach((effect) => {
      effectValues[effect.field] = state[effect.field];
    });
    let bindings = collectBindings(root);
    const runEffectCleanup = async (effect) => {
      const cleanup = effectCleanups[effect.field];
      if (!cleanup || cleanup.length === 0) return;
      effectCleanups[effect.field] = null;
      await applyStatements(cleanup, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
    };
    const runAllEffectCleanups = async () => {
      for (const effect of effects) {
        await runEffectCleanup(effect);
      }
    };
    const settleEffects = async () => {
      for (let pass = 0; pass < 10; pass++) {
        let ran = false;
        for (const effect of effects) {
          const current = state[effect.field];
          if (Object.is(effectValues[effect.field], current)) continue;
          await runEffectCleanup(effect);
          effectValues[effect.field] = current;
          await applyStatements(effect.statements, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
          effectCleanups[effect.field] = effect.cleanup || null;
          ran = true;
        }
        if (!ran) return;
      }
    };
    let applyingStoreUpdate = false;
    const publishStores = () => {
      if (applyingStoreUpdate || !storeRegistry || storeNames.length === 0) return;
      storeNames.forEach((name) => storeRegistry.set(name, state));
    };
    const rerender = () => {
      bindings = render(root, state, helpers, bindings);
      bindInteractiveNodes();
      syncChildProps(root, state, helpers);
      dispatchComponentExports(root, exportNames, state, true);
      publishStores();
    };
    let renderScheduled = false;
    const scheduleRender = () => {
      if (renderScheduled) return;
      renderScheduled = true;
      const flush = () => {
        renderScheduled = false;
        rerender();
      };
      if (typeof queueMicrotask === "function") queueMicrotask(flush);
      else Promise.resolve().then(flush);
    };
    const storeUnsubscribers = storeNames.map((name) => {
      if (!storeRegistry) return () => {};
      return storeRegistry.subscribe(name, async (next) => {
        applyingStoreUpdate = true;
        try {
          Object.assign(state, next || {});
          recomputeComputed(state, computeds, helpers);
          await settleEffects();
          recomputeComputed(state, computeds, helpers);
          rerender();
        } finally {
          applyingStoreUpdate = false;
        }
      });
    });
    const bindInteractiveNodes = () => {
      root.querySelectorAll("*").forEach((node) => {
        const owned = ownsNode(root, node);
        if (owned && node.hasAttribute("data-gowdk-bind-value") && !node.hasAttribute("data-gowdk-bound-value")) {
          node.setAttribute("data-gowdk-bound-value", "");
          const field = node.getAttribute("data-gowdk-bind-value");
          const type = node.getAttribute("data-gowdk-bind-type") || "string";
          const event = node.tagName === "SELECT" || node.type === "radio" ? "change" : "input";
          node.addEventListener(event, async () => {
            if (node.type === "radio") {
              if (!node.checked) return;
              state[field] = node.value;
            } else if (type === "int") {
              const next = parseInt(node.value, 10);
              state[field] = Number.isNaN(next) ? 0 : next;
            } else if (type === "float") {
              const next = parseFloat(node.value);
              state[field] = Number.isNaN(next) ? 0 : next;
            } else {
              state[field] = node.value;
            }
            recomputeComputed(state, computeds, helpers);
            await settleEffects();
            recomputeComputed(state, computeds, helpers);
            scheduleRender();
          });
        }
        if (owned && node.hasAttribute("data-gowdk-bind-checked") && !node.hasAttribute("data-gowdk-bound-checked")) {
          node.setAttribute("data-gowdk-bound-checked", "");
          const field = node.getAttribute("data-gowdk-bind-checked");
          node.addEventListener("change", async () => {
            state[field] = node.checked;
            recomputeComputed(state, computeds, helpers);
            await settleEffects();
            recomputeComputed(state, computeds, helpers);
            scheduleRender();
          });
        }
        Array.from(node.attributes).forEach((attr) => {
          if (attr.name.startsWith("data-gowdk-parent-on-")) {
            const event = attr.name.slice("data-gowdk-parent-on-".length);
            const boundAttr = "data-gowdk-bound-parent-on-" + event;
            if (node.hasAttribute(boundAttr)) return;
            node.setAttribute(boundAttr, "");
            const modifiers = eventModifiers(node.getAttribute("data-gowdk-parent-event-" + event));
            let debounceTimer = 0;
            let throttleUntil = 0;
            const invoke = async (customEvent) => {
              const eventScope = Object.create(null);
              eventScope.event = customEvent.detail || {};
              try {
                await applyExpression(attr.value, state, handlers, helpers, eventScope, refs, computeds, asyncTokens, root, emitEvents);
              } catch (error) {
                if (error !== staleAsyncResult) recordAsyncError(state, error);
              } finally {
                recomputeComputed(state, computeds, helpers);
                await settleEffects();
                recomputeComputed(state, computeds, helpers);
                scheduleRender();
              }
            };
            const listener = (customEvent) => {
              if (modifiers.prevent) customEvent.preventDefault();
              if (modifiers.stop) customEvent.stopPropagation();
              if (modifiers.debounce > 0) {
                clearTimeout(debounceTimer);
                debounceTimer = setTimeout(() => invoke(customEvent), modifiers.debounce);
                return;
              }
              if (modifiers.throttle > 0) {
                const now = Date.now();
                if (now < throttleUntil) return;
                throttleUntil = now + modifiers.throttle;
              }
              invoke(customEvent);
            };
            node.addEventListener(event, listener, { once: modifiers.once, capture: modifiers.capture });
            if (event === "exports" && node.__gowdkExports) {
              listener({
                detail: node.__gowdkExports,
                preventDefault() {},
                stopPropagation() {}
              });
            }
            return;
          }
          if (!owned) return;
          if (!attr.name.startsWith("data-gowdk-on-")) return;
          const event = attr.name.slice("data-gowdk-on-".length);
          const boundAttr = "data-gowdk-bound-on-" + event;
          if (node.hasAttribute(boundAttr)) return;
          node.setAttribute(boundAttr, "");
          const modifiers = eventModifiers(node.getAttribute("data-gowdk-event-" + event));
          let debounceTimer = 0;
          let throttleUntil = 0;
          const invoke = async (domEvent) => {
            try {
              await applyExpression(attr.value, state, handlers, helpers, domEventScope(domEvent), refs, computeds, asyncTokens, root, emitEvents);
            } catch (error) {
              if (error !== staleAsyncResult) recordAsyncError(state, error);
            } finally {
              recomputeComputed(state, computeds, helpers);
              await settleEffects();
              recomputeComputed(state, computeds, helpers);
              scheduleRender();
            }
          };
          const listener = (domEvent) => {
            if (modifiers.prevent) domEvent.preventDefault();
            if (modifiers.stop) domEvent.stopPropagation();
            if (modifiers.debounce > 0) {
              clearTimeout(debounceTimer);
              debounceTimer = setTimeout(() => invoke(domEvent), modifiers.debounce);
              return;
            }
            if (modifiers.throttle > 0) {
              const now = Date.now();
              if (now < throttleUntil) return;
              throttleUntil = now + modifiers.throttle;
            }
            invoke(domEvent);
          };
          node.addEventListener(event, listener, { once: modifiers.once, capture: modifiers.capture });
        });
      });
    };
    root.addEventListener("gowdk:props", async (event) => {
      const nextProps = event.detail || {};
      let changed = false;
      Object.keys(nextProps).forEach((name) => {
        if (Object.is(state[name], nextProps[name])) return;
        state[name] = nextProps[name];
        changed = true;
      });
      if (!changed) return;
      recomputeComputed(state, computeds, helpers);
      await settleEffects();
      recomputeComputed(state, computeds, helpers);
      scheduleRender();
    });
    await applyStatements(mountStatements, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
    await settleEffects();
    recomputeComputed(state, computeds, helpers);
    const destroyIsland = async function %s() {
      if (root.getAttribute("data-gowdk-mounted") !== "js") return;
      root.removeAttribute("data-gowdk-mounted");
      registry.roots.delete(root);
      dispatchComponentExports(root, exportNames, state, false);
      storeUnsubscribers.forEach((unsubscribe) => unsubscribe());
      if (destroyStatements.length > 0) {
        await runAllEffectCleanups();
        await applyStatements(destroyStatements, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
      } else if (effects.length > 0) {
        await runAllEffectCleanups();
      }
    };
    registry.roots.set(root, destroyIsland);
    window.addEventListener("pagehide", destroyIsland, { once: true });
    rerender();
  });
  }

  registry.components[component] = %s;
  %s(document);
})();
`, component, mountFunction, destroyFunction, mountFunction, mountFunction)
	if includeSourceMap {
		source += "//# sourceMappingURL=" + path.Base(islandJSSourceMapAssetPath(componentName)) + "\n"
	}
	return source
}
