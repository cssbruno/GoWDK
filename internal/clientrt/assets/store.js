(() => {
  const registry = window.__gowdkStores || (window.__gowdkStores = {
    stores: Object.create(null),
    listeners: Object.create(null)
  });
  registry.persist = registry.persist || Object.create(null);
  registry.fields = registry.fields || Object.create(null);
  registry.seeds = registry.seeds || Object.create(null);
  const warned = Object.create(null);

  const storageFor = (scope) => {
    try {
      return scope === "session" ? window.sessionStorage : window.localStorage;
    } catch (error) {
      return null;
    }
  };

  const projectFields = (name, source) => {
    const projected = Object.create(null);
    (registry.fields[name] || []).forEach((field) => {
      if (Object.prototype.hasOwnProperty.call(source, field)) projected[field] = source[field];
    });
    return projected;
  };

  const sameFieldSet = (left, right) => {
    if (left.length !== right.length) return false;
    const sortedLeft = left.slice().sort();
    const sortedRight = right.slice().sort();
    for (let index = 0; index < sortedLeft.length; index++) {
      if (sortedLeft[index] !== sortedRight[index]) return false;
    }
    return true;
  };

  const decodePersisted = (config, fields, raw) => {
    if (!raw) return null;
    let blob = null;
    try {
      blob = JSON.parse(raw);
    } catch (error) {
      return null;
    }
    if (!blob || blob.v !== config.version || typeof blob.s !== "object" || blob.s === null) return null;
    const restored = Object.create(null);
    fields.forEach((field) => {
      if (Object.prototype.hasOwnProperty.call(blob.s, field)) restored[field] = blob.s[field];
    });
    return restored;
  };

  const readPersisted = (config, fields) => {
    const storage = storageFor(config.scope);
    if (!storage) return null;
    let raw = null;
    try {
      raw = storage.getItem(config.key);
    } catch (error) {
      return null;
    }
    return decodePersisted(config, fields, raw);
  };

  const writePersisted = (name) => {
    const config = registry.persist[name];
    if (!config) return;
    const storage = storageFor(config.scope);
    if (!storage) return;
    try {
      storage.setItem(config.key, JSON.stringify({ v: config.version, s: projectFields(name, registry.stores[name] || {}) }));
    } catch (error) {
      // Quota, private-mode, or disabled storage must never break the island.
      if (!warned[name] && typeof console !== "undefined" && console.warn) {
        warned[name] = true;
        console.warn("GOWDK: could not persist store \"" + name + "\" (storage unavailable or full); continuing without persistence.");
      }
    }
  };

  const notify = (name) => {
    (registry.listeners[name] || []).slice().forEach((listener) => {
      try {
        listener(registry.get(name));
      } catch (error) {
        if (typeof console !== "undefined" && console.error) {
          console.error("GOWDK: store subscriber for \"" + name + "\" failed; continuing notification.", error);
        }
      }
    });
  };

  registry.init = (name, state, persist) => {
    if (!name) return;
    const seed = Object.assign(Object.create(null), state || {});
    const fields = Object.keys(seed);
    const hasPersist = !!(persist && persist.scope && persist.key && persist.version);

    if (registry.stores[name]) {
      // The store already exists (for example SPA navigation reached a later
      // route that declares the same store).
      const prior = registry.persist[name];
      // Re-seed when the field set changes, when an already-persisted store's
      // version (shape hash) changes, OR when this declaration FIRST adds
      // persistence (!prior). In every case the current route's islands must read
      // the fields, seed and version they declared, with any saved value restored
      // on top. Adopting persistence without re-seeding is unsafe: two routes can
      // share top-level field names yet declare a different nested seed, and on a
      // fresh storage slot (nothing to restore) the later route's islands would
      // otherwise mount on the earlier route's seed. Divergent shapes that share a
      // storage key are reported at build time by page_store_persist_key_conflict;
      // a conflicting scope is kept first-wins (page_store_persist_scope_conflict),
      // so navigation cannot thrash storage.
      const shapeChanged =
        !sameFieldSet(registry.fields[name] || [], fields) ||
        (hasPersist && (!prior || prior.version !== persist.version));
      if (shapeChanged) {
        registry.fields[name] = fields;
        registry.seeds[name] = Object.assign(Object.create(null), seed);
        delete registry.persist[name];
        if (hasPersist) {
          registry.persist[name] = persist;
          const restored = readPersisted(persist, fields);
          if (restored) Object.assign(seed, restored);
        }
        registry.stores[name] = seed;
        notify(name);
        return;
      }
      // Same field set and (if persisted) same version. If this route declares
      // the store WITHOUT persistence but an earlier route persisted it, honor
      // the current declaration and stop persisting, so set() does not keep
      // writing to storage this route never opted into. The in-memory value is
      // left intact so the store stays shared across routes.
      if (!hasPersist && prior) {
        delete registry.persist[name];
      }
      return;
    }

    registry.fields[name] = fields;
    registry.seeds[name] = Object.assign(Object.create(null), seed);
    if (hasPersist) {
      registry.persist[name] = persist;
      const restored = readPersisted(persist, fields);
      if (restored) Object.assign(seed, restored);
    }
    registry.stores[name] = seed;
  };

  registry.get = (name) => {
    return Object.assign({}, registry.stores[name] || {});
  };

  registry.set = (name, next) => {
    if (!name) return;
    registry.stores[name] = Object.assign({}, registry.stores[name] || {}, next || {});
    writePersisted(name);
    notify(name);
  };

  // clear drops the persisted copy and resets the in-memory store to its build
  // -time seed, then notifies islands. Use after checkout, logout, or reset.
  registry.clear = (name) => {
    if (!name) return;
    const config = registry.persist[name];
    if (config) {
      const storage = storageFor(config.scope);
      if (storage) {
        try {
          storage.removeItem(config.key);
        } catch (error) {}
      }
    }
    if (registry.seeds[name]) registry.stores[name] = Object.assign({}, registry.seeds[name]);
    notify(name);
  };

  registry.subscribe = (name, listener) => {
    if (!name || typeof listener !== "function") return () => {};
    if (!registry.listeners[name]) registry.listeners[name] = [];
    registry.listeners[name].push(listener);
    return () => {
      registry.listeners[name] = (registry.listeners[name] || []).filter((item) => item !== listener);
    };
  };

  // Cross-tab sync: when another tab writes a persisted LOCAL store, mirror the
  // value here and notify islands. Only localStorage is shared across tabs on the
  // origin, so its "storage" event is what carries cross-tab writes. sessionStorage
  // is partitioned per top-level tab, so session-scoped stores are deliberately
  // tab-local and skipped here (a "storage" event for them only fires within the
  // same page session, e.g. iframes). We never write back, so tabs cannot loop.
  //
  // Register exactly once per registry. SPA navigation can re-execute this script
  // (the head swap drops the stores.js tag while window.__gowdkStores stays alive,
  // so a later store page makes activateNewScripts treat it as new); a second
  // listener would notify islands — and rerun render/effects — twice per write.
  if (!registry.storageListenerAttached && typeof window.addEventListener === "function") {
    registry.storageListenerAttached = true;
    window.addEventListener("storage", (event) => {
      if (!event) return;
      // A bulk Storage.clear() in another tab fires a "storage" event with a null
      // key (and null newValue) rather than one event per removed key. Reset every
      // local-scoped persisted store backed by the cleared area to its seed, so
      // this tab does not keep values whose persisted backing is gone. Keyed
      // setItem/removeItem (including __gowdkStores.clear) falls through below.
      if (!event.key) {
        if (event.storageArea && event.storageArea !== storageFor("local")) return;
        Object.keys(registry.persist).forEach((name) => {
          const config = registry.persist[name];
          if (!config || config.scope !== "local") return;
          if (registry.seeds[name]) registry.stores[name] = Object.assign({}, registry.seeds[name]);
          notify(name);
        });
        return;
      }
      Object.keys(registry.persist).forEach((name) => {
        const config = registry.persist[name];
        if (!config || config.scope !== "local" || config.key !== event.key) return;
        // Only local stores reach here, but keep the storageArea guard: a
        // session store can share the gowdk:store:<name> key, and older browsers
        // omit storageArea, where the key + scope match alone is used.
        if (event.storageArea && event.storageArea !== storageFor(config.scope)) return;
        if (event.newValue == null) {
          if (registry.seeds[name]) registry.stores[name] = Object.assign({}, registry.seeds[name]);
        } else {
          const restored = decodePersisted(config, registry.fields[name] || [], event.newValue);
          if (restored) registry.stores[name] = Object.assign({}, registry.stores[name] || {}, restored);
        }
        notify(name);
      });
    });
  }

  // hydrate scans the current document for store seeds and initializes any not
  // already in the registry. It is idempotent (init bails on existing stores),
  // so the SPA navigation runtime can call it after swapping page content to
  // pick up stores first declared on a later route.
  registry.hydrate = () => {
    document.querySelectorAll("script[type=\"application/json\"][data-gowdk-store]").forEach((node) => {
      const name = node.getAttribute("data-gowdk-store");
      let persist = null;
      const scope = node.getAttribute("data-gowdk-persist");
      if (scope) {
        persist = {
          scope: scope,
          key: node.getAttribute("data-gowdk-persist-key") || ("gowdk:store:" + name),
          version: node.getAttribute("data-gowdk-persist-version") || ""
        };
      }
      try {
        registry.init(name, JSON.parse(node.textContent || "{}"), persist);
      } catch (error) {
        registry.init(name, {}, persist);
      }
    });
  };
  registry.hydrate();
})();
