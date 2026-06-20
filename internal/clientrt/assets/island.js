(() => {
  const selectorFor = (component) => "gowdk-island[data-gowdk-component-id=\"" + component + "\"][data-gowdk-runtime=\"js\"],gowdk-island:not([data-gowdk-component-id])[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"js\"]";
  const booleanAttrs = new Set(["allowfullscreen", "async", "autofocus", "autoplay", "checked", "controls", "default", "defer", "disabled", "formnovalidate", "hidden", "inert", "ismap", "loop", "multiple", "muted", "nomodule", "novalidate", "open", "readonly", "required", "reversed", "selected"]);
  const staleAsyncResult = Symbol("gowdk stale async result");
  const bindingTable = Object.freeze([
    { kind: "text", selector: "[data-gowdk-binding-text]", id: "data-gowdk-binding-text", field: "data-gowdk-bind" },
    { kind: "value", selector: "[data-gowdk-binding-value]", id: "data-gowdk-binding-value", field: "data-gowdk-bind-value" },
    { kind: "checked", selector: "[data-gowdk-binding-checked]", id: "data-gowdk-binding-checked", field: "data-gowdk-bind-checked" },
    { kind: "conditional", selector: "[data-gowdk-binding-if]", id: "data-gowdk-binding-if" },
    { kind: "list", selector: "[data-gowdk-binding-list]", id: "data-gowdk-binding-list" },
    { kind: "await", selector: "gowdk-await[data-gowdk-await]", id: "data-gowdk-binding-await" },
    { kind: "class", attrPrefix: "data-gowdk-binding-class-", valuePrefix: "data-gowdk-class-" },
    { kind: "style", attrPrefix: "data-gowdk-binding-style-", valuePrefix: "data-gowdk-style-", unitPrefix: "data-gowdk-style-unit-" },
    { kind: "attr", attrPrefix: "data-gowdk-binding-attr-", valuePrefix: "data-gowdk-attr-" }
  ]);
  const expressionSpec = Object.freeze(__GOWDK_EXPRESSION_SPEC__);
  const expressionOperators = Object.freeze({
    unary: new Set(expressionSpec.unaryOperators || []),
    equality: new Set(expressionSpec.equalityOperators || []),
    compare: new Set(expressionSpec.compareOperators || []),
    term: new Set(expressionSpec.termOperators || []),
    factor: new Set(expressionSpec.factorOperators || []),
    token: new Set(expressionSpec.tokenOperators || [])
  });
  const builtinSpecByName = Object.freeze(Object.fromEntries((expressionSpec.builtins || []).map((builtin) => [builtin.name, builtin])));
  const registry = window.__gowdkIslandRegistry || (window.__gowdkIslandRegistry = { components: Object.create(null), roots: new WeakMap() });
  window.__gowdkMountIslands = () => {
    Object.keys(registry.components).forEach((name) => mountComponentIsland(name, document));
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

  function traceStart(name, lane) {
    if (window.__gowdkTrace && window.__gowdkTrace.enabled && window.__gowdkTrace.enabled()) {
      return window.__gowdkTrace.start(name, lane || "island");
    }
    return null;
  }

  function traceEnd(span, status, message) {
    if (window.__gowdkTrace && window.__gowdkTrace.end) {
      window.__gowdkTrace.end(span, status || "ok", message || "");
    }
  }

  function callHelper(name, args, state, helpers, stack) {
    const helper = helpers && helpers[name];
    if (!helper) throw new Error("unknown client helper function " + JSON.stringify(name));
    stack = stack || [];
    if (stack.indexOf(name) >= 0) throw new Error("recursive GOWDK helper " + name);
    const nextScope = Object.create(null);
    (helper.params || []).forEach((param, index) => {
      nextScope[param] = args[index];
    });
    return valueOf(helper.return || "", state, nextScope, helpers, stack.concat([name]));
  }

  const builtinImpls = Object.freeze({
    len(value) {
      if (typeof value === "string" || Array.isArray(value)) return value.length;
      throw new Error("built-in len expects string or array");
    },
    string(value) {
      if (value == null) return "";
      if (typeof value === "string" || typeof value === "boolean") return String(value);
      if (isNumber(value)) return String(value);
      throw new Error("built-in string expects scalar");
    },
    lower(value) {
      if (typeof value !== "string") throw new Error("built-in lower expects string");
      return value.toLowerCase();
    },
    upper(value) {
      if (typeof value !== "string") throw new Error("built-in upper expects string");
      return value.toUpperCase();
    },
    contains(value, query) {
      if (typeof value !== "string") throw new Error("built-in contains argument 1 expects string");
      if (typeof query !== "string") throw new Error("built-in contains argument 2 expects string");
      return value.includes(query);
    },
    int(value) {
      return Math.trunc(conversionNumber("int", value));
    },
    float(value) {
      return conversionNumber("float", value);
    }
  });
  const builtins = Object.freeze(Object.fromEntries(Object.keys(builtinSpecByName).map((name) => {
    if (!builtinImpls[name]) throw new Error("GOWDK expression builtin " + name + " has no browser implementation");
    return [name, function() {
      expectArgCount(name, arguments.length, builtinSpecByName[name].args);
      return builtinImpls[name].apply(null, arguments);
    }];
  })));

  function expectArgCount(name, got, want) {
    if (got === want) return;
    throw new Error("built-in " + name + " expects " + want + " argument" + (want === 1 ? "" : "s") + ", got " + got);
  }

  function conversionNumber(name, value) {
    if (typeof value === "string") {
      const trimmed = value.trim();
      if (trimmed === "") throw new Error("built-in " + name + " cannot parse " + JSON.stringify(value));
      const parsed = Number(trimmed);
      if (Number.isNaN(parsed)) throw new Error("built-in " + name + " cannot parse " + JSON.stringify(value));
      return parsed;
    }
    if (isNumber(value)) return value;
    throw new Error("built-in " + name + " expects string or number");
  }

  function isNumber(value) {
    return typeof value === "number" && !Number.isNaN(value);
  }

  function requireNumber(op, value) {
    if (isNumber(value)) return value;
    throw new Error("operator " + op + " requires number");
  }

  function requireBool(op, value) {
    if (typeof value === "boolean") return value;
    throw new Error("operator " + op + " requires bool");
  }

  function clientObject(value) {
    return value != null && typeof value === "object" && !Array.isArray(value);
  }

  function own(value, name) {
    return Object.prototype.hasOwnProperty.call(value, name);
  }

  function deepEqual(left, right) {
    if (isNumber(left) && isNumber(right)) return left === right;
    if (left === right) return true;
    if (left == null || right == null) return left === right;
    if (Array.isArray(left) || Array.isArray(right)) {
      if (!Array.isArray(left) || !Array.isArray(right) || left.length !== right.length) return false;
      for (let index = 0; index < left.length; index++) {
        if (!deepEqual(left[index], right[index])) return false;
      }
      return true;
    }
    if (clientObject(left) || clientObject(right)) {
      if (!clientObject(left) || !clientObject(right)) return false;
      const leftKeys = Object.keys(left);
      const rightKeys = Object.keys(right);
      if (leftKeys.length !== rightKeys.length) return false;
      for (const key of leftKeys) {
        if (!own(right, key) || !deepEqual(left[key], right[key])) return false;
      }
      return true;
    }
    return false;
  }

  function addValues(left, right) {
    if (typeof left === "string") {
      if (typeof right !== "string") throw new Error("operator + requires matching types");
      return left + right;
    }
    if (!isNumber(left) || !isNumber(right)) throw new Error("operator + requires numbers");
    return left + right;
  }

  function compareValues(op, left, right) {
    if (typeof left === "string") {
      if (typeof right !== "string") throw new Error("operator " + op + " requires matching types");
      if (op === "<") return left < right;
      if (op === "<=") return left <= right;
      if (op === ">") return left > right;
      return left >= right;
    }
    if (!isNumber(left) || !isNumber(right)) throw new Error("operator " + op + " requires numbers or strings");
    if (op === "<") return left < right;
    if (op === "<=") return left <= right;
    if (op === ">") return left > right;
    return left >= right;
  }

  function moduloValues(left, right) {
    const leftNumber = requireNumber("%", left);
    const rightNumber = requireNumber("%", right);
    if (Math.trunc(rightNumber) === 0) throw new Error("operator % requires a non-zero divisor");
    return Math.trunc(leftNumber) % Math.trunc(rightNumber);
  }

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
      if (expressionOperators.token.has(pair)) {
        tokens.push({ kind: "op", value: pair });
        index += 2;
        continue;
      }
      if ("+-*/%!<>".indexOf(char) >= 0) {
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
        while (this.peek().kind === "op" && expressionOperators.equality.has(this.peek().value)) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseCompare() };
        }
        return expr;
      },
      parseCompare() {
        let expr = this.parseTerm();
        while (this.peek().kind === "op" && expressionOperators.compare.has(this.peek().value)) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseTerm() };
        }
        return expr;
      },
      parseTerm() {
        let expr = this.parseFactor();
        while (this.peek().kind === "op" && expressionOperators.term.has(this.peek().value)) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseFactor() };
        }
        return expr;
      },
      parseFactor() {
        let expr = this.parseUnary();
        while (this.peek().kind === "op" && expressionOperators.factor.has(this.peek().value)) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseUnary() };
        }
        return expr;
      },
      parseUnary() {
        if (this.peek().kind === "op" && expressionOperators.unary.has(this.peek().value)) {
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
    const fetcher = window.__gowdkTrace && window.__gowdkTrace.fetch ? window.__gowdkTrace.fetch : fetch;
    const response = await fetcher(String(url), { headers: { "Accept": "application/json" }, signal }, { name: "island fetch", lane: "island" });
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
        if (scope && own(scope, expr.name)) return scope[expr.name];
        if (own(state, expr.name)) return state[expr.name];
        throw new Error("unknown client value " + JSON.stringify(expr.name));
      case "member": {
        const target = evalExpression(expr.target, state, scope, helpers, stack);
        if (!clientObject(target)) throw new Error("cannot read field " + JSON.stringify(expr.name));
        if (!own(target, expr.name)) throw new Error("unknown client field " + JSON.stringify(expr.name));
        return target[expr.name];
      }
      case "index": {
        const target = evalExpression(expr.target, state, scope, helpers, stack);
        const index = evalExpression(expr.index, state, scope, helpers, stack);
        if (!Number.isInteger(index)) throw new Error("index expression requires int");
        if (!Array.isArray(target)) throw new Error("cannot index expression");
        if (index < 0 || index >= target.length) throw new Error("index " + index + " out of range");
        return target[index];
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
        if (expr.op === "!") return !requireBool("!", value);
        if (expr.op === "-") return -requireNumber("-", value);
        throw new Error("unsupported unary operator " + JSON.stringify(expr.op));
      }
      case "binary":
        return evalBinaryExpression(expr, state, scope, helpers, stack);
      case "if":
        return requireBool("if", evalExpression(expr.cond, state, scope, helpers, stack))
          ? evalExpression(expr.thenExpr, state, scope, helpers, stack)
          : evalExpression(expr.elseExpr, state, scope, helpers, stack);
      default:
        throw new Error("unknown expression node");
    }
  }

  function evalBinaryExpression(expr, state, scope, helpers, stack) {
    const left = evalExpression(expr.left, state, scope, helpers, stack);
    const right = evalExpression(expr.right, state, scope, helpers, stack);
    switch (expr.op) {
      case "==":
        return deepEqual(left, right);
      case "!=":
        return !deepEqual(left, right);
      case "<":
      case "<=":
      case ">":
      case ">=":
        return compareValues(expr.op, left, right);
      case "+":
        return addValues(left, right);
      case "-":
        return requireNumber("-", left) - requireNumber("-", right);
      case "*":
        return requireNumber("*", left) * requireNumber("*", right);
      case "/":
        return requireNumber("/", left) / requireNumber("/", right);
      case "%":
        return moduloValues(left, right);
      case "&&":
        return requireBool("&&", left) && requireBool("&&", right);
      case "||":
        return requireBool("||", left) || requireBool("||", right);
      default:
        throw new Error("unsupported binary operator " + JSON.stringify(expr.op));
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

  function splitStatements(source) {
    source = (source || "").trim();
    if (!source) return [];
    const statements = [];
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
      else if (char === ";" && depth === 0) {
        const piece = source.slice(start, i).trim();
        if (piece) statements.push(piece);
        start = i + 1;
      }
    }
    const tail = source.slice(start).trim();
    if (tail) statements.push(tail);
    return statements;
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
    let clearStore = expr.match(/^clear\s+([A-Za-z_][A-Za-z0-9_.]*)$/);
    if (clearStore) {
      const registry = window.__gowdkStores;
      // The store registry is keyed by the unqualified store name (the page
      // store's own name); a `use alias.store` reference carries a package
      // qualifier that is not part of the registry/storage key, so drop it.
      const storeName = clearStore[1].slice(clearStore[1].lastIndexOf(".") + 1);
      if (registry && typeof registry.clear === "function") registry.clear(storeName);
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

  function interpolateTemplateValue(template, state, scope, helpers) {
    return template.replace(/\{\{([^{}]+)\}\}/g, (_match, expr) => {
      const value = valueOf(expr, state, scope, helpers);
      return value == null ? "" : String(value);
    });
  }

  function interpolateTemplateNode(node, state, scope, helpers) {
    if (node.nodeType === 3) {
      if (node.nodeValue && node.nodeValue.indexOf("{{") >= 0) {
        node.nodeValue = interpolateTemplateValue(node.nodeValue, state, scope, helpers);
      }
      return;
    }
    if (node.nodeType !== 1) return;
    Array.from(node.attributes).forEach((attr) => {
      if (attr.value.indexOf("{{") >= 0) node.setAttribute(attr.name, interpolateTemplateValue(attr.value, state, scope, helpers));
    });
    if (node.content) Array.from(node.content.childNodes).forEach((child) => interpolateTemplateNode(child, state, scope, helpers));
    Array.from(node.childNodes).forEach((child) => interpolateTemplateNode(child, state, scope, helpers));
  }

  function cloneListTemplate(marker, state, scope, helpers) {
    const source = marker.content && marker.content.firstElementChild;
    if (!source) return null;
    const fresh = source.cloneNode(true);
    interpolateTemplateNode(fresh, state, scope, helpers);
    return fresh;
  }

  function childNodesEqual(target, source) {
    if (target.childNodes.length !== source.childNodes.length) return false;
    for (let index = 0; index < target.childNodes.length; index++) {
      if (!target.childNodes[index].isEqualNode(source.childNodes[index])) return false;
    }
    return true;
  }

  function replaceChildNodes(target, source) {
    while (target.firstChild) target.removeChild(target.firstChild);
    Array.from(source.childNodes).forEach((child) => target.appendChild(child.cloneNode(true)));
  }

  function syncElement(target, source) {
    Array.from(target.attributes).forEach((attr) => {
      if (!source.hasAttribute(attr.name)) target.removeAttribute(attr.name);
    });
    Array.from(source.attributes).forEach((attr) => {
      if (target.getAttribute(attr.name) !== attr.value) target.setAttribute(attr.name, attr.value);
    });
    if (!childNodesEqual(target, source)) replaceChildNodes(target, source);
  }

  function matchingNodes(container, selector) {
    const nodes = [];
    if (container.matches && container.matches(selector)) nodes.push(container);
    container.querySelectorAll(selector).forEach((node) => nodes.push(node));
    return nodes;
  }

  function emptyBindings() {
    return { text: [], value: [], checked: [], classes: [], styles: [], attrs: [], conditionals: [], lists: [], awaits: [] };
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
      else if (spec.kind === "await") bindings.awaits.push({ id, node });
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
      if (!id) return;
      const existing = records.get(id);
      if (existing && existing.marker && existing.marker.isConnected) return;
      if (existing) records.delete(id);
      const marker = document.createComment("gowdk-if:" + id);
      const template = node.cloneNode(true);
      node.parentNode.insertBefore(marker, node);
      records.set(id, {
        id,
        marker,
        template,
        current: node,
        awaitBlock: options.awaitRoot && node.closest ? node.closest("gowdk-await[data-gowdk-await]") : null,
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
      const recordScope = scope || awaitScopeForBlock(options.awaitRoot, record.awaitBlock);
      const visible = condition == null || Boolean(valueOf(condition, state, recordScope, helpers));
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
        const recordScope = scope || awaitScopeForBlock(options.awaitRoot, record.awaitBlock);
        const visible = !matched && (condition == null || Boolean(valueOf(condition, state, recordScope, helpers)));
        if (visible) mountConditional(record);
        else unmountConditional(record);
        if (visible) matched = true;
      });
    });
  }

  function parseAwaitFetchExpression(expr) {
    const match = String(expr || "").trim().match(/^fetchJSON\[(.*)\]\((.*)\)$/);
    if (!match) return null;
    return { urlExpr: match[2] };
  }

  function awaitRecords(root) {
    return root.__gowdkAwaitRecords || (root.__gowdkAwaitRecords = new Map());
  }

  function awaitBranchTemplate(block, branch) {
    return Array.from(block.children).find((child) => child.tagName === "TEMPLATE" && child.getAttribute("data-gowdk-await-branch") === branch) || null;
  }

  function awaitContent(block) {
    const id = block.getAttribute("data-gowdk-await") || "";
    let content = Array.from(block.children).find((child) => child.getAttribute && child.getAttribute("data-gowdk-await-content") === id);
    if (content) return content;
    content = document.createElement("span");
    content.setAttribute("data-gowdk-await-content", id);
    block.appendChild(content);
    return content;
  }

  function clearAwaitContent(block) {
    const content = awaitContent(block);
    while (content.firstChild) content.removeChild(content.firstChild);
    return content;
  }

  function awaitErrorObject(error) {
    return { message: error && error.message ? error.message : String(error || "async error") };
  }

  function mergeScopes(base, additions) {
    const scope = Object.create(null);
    Object.keys(base || {}).forEach((name) => {
      scope[name] = base[name];
    });
    Object.keys(additions || {}).forEach((name) => {
      scope[name] = additions[name];
    });
    return scope;
  }

  function awaitScopeForBlock(root, block) {
    if (!block || !ownsNode(root, block)) return null;
    const record = awaitRecords(root).get(block.getAttribute("data-gowdk-await") || "");
    if (!record) return null;
    const scope = Object.create(null);
    if (record.status === "then") {
      scope[block.getAttribute("data-gowdk-await-result") || "value"] = record.value;
      return scope;
    }
    if (record.status === "catch") {
      scope[block.getAttribute("data-gowdk-await-error") || "error"] = awaitErrorObject(record.error);
      return scope;
    }
    return null;
  }

  function awaitScopeForNode(root, node) {
    const block = node && node.closest ? node.closest("gowdk-await[data-gowdk-await]") : null;
    return awaitScopeForBlock(root, block);
  }

  function interpolateAwaitTemplateNode(node, state, scope, helpers) {
    if (node.nodeType === 3) {
      if (node.nodeValue && node.nodeValue.indexOf("{{") >= 0) {
        node.nodeValue = interpolateTemplateValue(node.nodeValue, state, scope, helpers);
      }
      return;
    }
    if (node.nodeType !== 1) return;
    if (node.tagName === "TEMPLATE" && node.hasAttribute("data-gowdk-for")) return;
    Array.from(node.attributes).forEach((attr) => {
      if (attr.value.indexOf("{{") >= 0) node.setAttribute(attr.name, interpolateTemplateValue(attr.value, state, scope, helpers));
    });
    if (node.content) Array.from(node.content.childNodes).forEach((child) => interpolateAwaitTemplateNode(child, state, scope, helpers));
    Array.from(node.childNodes).forEach((child) => interpolateAwaitTemplateNode(child, state, scope, helpers));
  }

  function renderAwaitBranch(block, record, state, helpers) {
    const branch = record.status === "then" ? "then" : record.status === "catch" ? "catch" : "pending";
    const template = awaitBranchTemplate(block, branch);
    const content = clearAwaitContent(block);
    if (!template || !template.content) return;
    const scope = Object.create(null);
    if (branch === "then") {
      scope[block.getAttribute("data-gowdk-await-result") || "value"] = record.value;
    } else if (branch === "catch") {
      scope[block.getAttribute("data-gowdk-await-error") || "error"] = awaitErrorObject(record.error);
    }
    const fragment = document.createDocumentFragment();
    Array.from(template.content.childNodes).forEach((child) => {
      const fresh = child.cloneNode(true);
      interpolateAwaitTemplateNode(fresh, state, scope, helpers);
      fragment.appendChild(fresh);
    });
    content.appendChild(fragment);
  }

  function startAwaitFetch(root, block, record, expr, url, helpers, onAsyncSettle) {
    if (record.controller && typeof record.controller.abort === "function") record.controller.abort();
    const controller = typeof AbortController === "undefined" ? null : new AbortController();
    const token = (record.token || 0) + 1;
    record.expr = expr;
    record.url = url;
    record.token = token;
    record.controller = controller;
    record.status = "pending";
    record.value = null;
    record.error = null;
    fetchJSON(url, controller ? controller.signal : undefined).then((value) => {
      if (!block.isConnected || !root.isConnected || record.token !== token) return;
      record.status = "then";
      record.value = value;
      record.error = null;
      if (typeof onAsyncSettle === "function") onAsyncSettle();
    }).catch((error) => {
      if (error && error.name === "AbortError") return;
      if (!block.isConnected || !root.isConnected || record.token !== token) return;
      record.status = "catch";
      record.value = null;
      record.error = error;
      if (typeof onAsyncSettle === "function") onAsyncSettle();
    });
  }

  function renderAwaitBlocks(root, state, helpers, bindings, onAsyncSettle) {
    const markers = bindings ? bindings.awaits.map((binding) => binding.node).filter((node) => node.isConnected) : Array.from(root.querySelectorAll("gowdk-await[data-gowdk-await]"));
    const records = awaitRecords(root);
    markers.forEach((block) => {
      if (!ownsNode(root, block)) return;
      const id = block.getAttribute("data-gowdk-await");
      if (!id) return;
      const expr = block.getAttribute("data-gowdk-await-expr") || "";
      let record = records.get(id);
      if (!record) {
        record = { status: "pending", token: 0, value: null, error: null, controller: null };
        records.set(id, record);
      }
      const parsed = parseAwaitFetchExpression(expr);
      if (!parsed) {
        record.status = "catch";
        record.error = new Error("await block supports only fetchJSON[T](urlExpr)");
        renderAwaitBranch(block, record, state, helpers);
        return;
      }
      let url = "";
      try {
        url = String(valueOf(parsed.urlExpr, state, null, helpers));
      } catch (error) {
        if (record.controller && typeof record.controller.abort === "function") record.controller.abort();
        record.status = "catch";
        record.error = error;
        renderAwaitBranch(block, record, state, helpers);
        return;
      }
      if (record.expr !== expr || record.url !== url) {
        startAwaitFetch(root, block, record, expr, url, helpers, onAsyncSettle);
      }
      renderAwaitBranch(block, record, state, helpers);
    });
  }

  function abortAwaitRecords(root) {
    const records = root.__gowdkAwaitRecords;
    if (!records) return;
    records.forEach((record) => {
      record.token = (record.token || 0) + 1;
      if (record.controller && typeof record.controller.abort === "function") record.controller.abort();
    });
    records.clear();
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
      const baseScope = awaitScopeForNode(root, marker);
      const items = valueOf(source, state, baseScope, helpers);
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
        const scope = mergeScopes(baseScope, null);
        scope[itemName] = item;
        scope.index = index;
        if (indexName) scope[indexName] = index;
        const key = String(valueOf(keyExpr, state, scope, helpers) ?? "");
        const fresh = cloneListTemplate(marker, state, scope, helpers);
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

  function render(root, state, helpers, bindings, onAsyncSettle) {
    renderAwaitBlocks(root, state, helpers, bindings, onAsyncSettle);
    bindings = collectBindings(root);
    renderListLoops(root, state, helpers, bindings);
    renderConditionals(root, state, null, helpers, { owner: root, skipLoopItems: true, awaitRoot: root });
    bindings = collectBindings(root);
    updateBindings(root, state, helpers, bindings);
    root.setAttribute("data-gowdk-state", JSON.stringify(state));
    return bindings;
  }

  async function mountComponentIsland(component, scope) {
    scope = scope || document;
    scope.querySelectorAll(selectorFor(component)).forEach(async (root) => {
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
      bindings = render(root, state, helpers, bindings, scheduleRender);
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
            const span = traceStart("island parent " + event, "island");
            const eventScope = Object.create(null);
            eventScope.event = customEvent.detail || {};
            try {
              await applyStatements(splitStatements(attr.value), state, handlers, helpers, eventScope, refs, computeds, asyncTokens, root, emitEvents);
              traceEnd(span, "ok");
            } catch (error) {
              if (error !== staleAsyncResult) recordAsyncError(state, error);
              traceEnd(span, "error", error && error.message || String(error || "island event failed"));
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
            const span = traceStart("island " + event, "island");
            try {
              await applyExpression(attr.value, state, handlers, helpers, domEventScope(domEvent), refs, computeds, asyncTokens, root, emitEvents);
              traceEnd(span, "ok");
            } catch (error) {
              if (error !== staleAsyncResult) recordAsyncError(state, error);
              traceEnd(span, "error", error && error.message || String(error || "island event failed"));
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
    const destroyIsland = async function destroyComponentIsland() {
      if (root.getAttribute("data-gowdk-mounted") !== "js") return;
      root.removeAttribute("data-gowdk-mounted");
      registry.roots.delete(root);
      dispatchComponentExports(root, exportNames, state, false);
      abortAwaitRecords(root);
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

  function registerComponentIsland(component) {
    if (!component) return;
    registry.components[component] = true;
    mountComponentIsland(component, document);
  }

  window.__gowdkRegisterJSIsland = registerComponentIsland;
  window.__gowdkMountIslands();
})();
