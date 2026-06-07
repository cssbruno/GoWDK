package playground

// UIHTML returns a self-contained browser playground shell. It expects
// wasm_exec.js and gowdk.wasm to be served next to the HTML document.
func UIHTML() string {
	return playgroundHTML
}

const playgroundHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>GOWDK Playground</title>
  <style>
    :root { color-scheme: light; font-family: Inter, ui-sans-serif, system-ui, sans-serif; }
    body { margin: 0; background: #f5f6f3; color: #161716; }
    main { display: grid; grid-template-columns: 18rem minmax(24rem, 1fr) minmax(24rem, 1fr); min-height: 100vh; }
    aside, section { padding: 1rem; border-right: 1px solid #d8dcd2; }
    aside { background: #ecefe8; }
    h1, h2 { margin: 0 0 .75rem; line-height: 1.1; }
    h1 { font-size: 1.35rem; }
    h2 { font-size: 1rem; }
    label { display: block; margin: .75rem 0 .35rem; font-size: .8rem; color: #555f52; }
    input, select, textarea, pre, iframe { width: 100%; box-sizing: border-box; border: 1px solid #bcc4b6; border-radius: 6px; background: #fff; color: #151515; }
    input, select { min-height: 2.1rem; padding: .35rem .5rem; }
    textarea { min-height: 31rem; resize: vertical; font: 13px/1.45 ui-monospace, SFMono-Regular, Menlo, monospace; padding: .75rem; }
    pre { min-height: 9rem; max-height: 18rem; overflow: auto; padding: .75rem; white-space: pre-wrap; }
    iframe { height: 22rem; background: #fff; }
    button { border: 1px solid #1f2a24; border-radius: 6px; background: #1f2a24; color: #fff; padding: .5rem .7rem; cursor: pointer; }
    button.secondary { background: #fff; color: #1f2a24; }
    button.file { width: 100%; margin: .2rem 0; text-align: left; background: #fff; color: #1f2a24; }
    button.file.active { background: #d8ead2; border-color: #527148; }
    .toolbar, .tabs { display: flex; flex-wrap: wrap; gap: .45rem; align-items: center; margin: .75rem 0; }
    .status { color: #555f52; font-size: .9rem; }
    .file-list { margin-top: .5rem; }
    .viewer[hidden] { display: none; }
    @media (max-width: 1050px) { main { grid-template-columns: 17rem 1fr; } section.output { grid-column: 1 / -1; border-top: 1px solid #d8dcd2; } }
    @media (max-width: 760px) { main { grid-template-columns: 1fr; } aside, section { border-right: 0; border-bottom: 1px solid #d8dcd2; } textarea { min-height: 22rem; } }
  </style>
</head>
<body>
  <main>
    <aside>
      <h1>GOWDK Playground</h1>
      <label for="starter">Starter</label>
      <select id="starter">
        <option value="basic">Basic page</option>
        <option value="component">Page and component</option>
        <option value="action">Action form</option>
      </select>
      <div class="toolbar">
        <button id="load-starter" class="secondary">Load</button>
        <button id="add-file" class="secondary">Add file</button>
      </div>
      <h2>Project</h2>
      <div class="file-list" id="project-tree"></div>
      <div class="toolbar">
        <button id="export-project" class="secondary">Export</button>
        <button id="share-project" class="secondary">Share link</button>
      </div>
      <input id="import-file" type="file" accept="application/json">
      <p class="status" id="share-status"></p>
    </aside>
    <section>
      <h2>Editor</h2>
      <div class="toolbar">
        <button id="compile">Compile</button>
        <span class="status" id="status">loading compiler</span>
      </div>
      <label for="file-path">File path</label>
      <input id="file-path" spellcheck="false">
      <label for="source">Source</label>
      <textarea id="source" spellcheck="false"></textarea>
      <div class="toolbar">
        <button id="save-file" class="secondary">Save file</button>
        <button id="delete-file" class="secondary">Delete file</button>
      </div>
    </section>
    <section class="output">
      <h2>Preview</h2>
      <iframe id="preview" sandbox="allow-scripts"></iframe>
      <h2>Generated</h2>
      <div class="tabs" id="generated-tabs"></div>
      <pre class="viewer" id="generated-html"></pre>
      <pre class="viewer" id="generated-css" hidden></pre>
      <pre class="viewer" id="generated-js" hidden></pre>
      <pre class="viewer" id="generated-all" hidden></pre>
      <h2>Diagnostics</h2>
      <pre id="diagnostics">[]</pre>
    </section>
  </main>
  <script src="wasm_exec.js"></script>
  <script>
    const status = document.getElementById("status");
    const source = document.getElementById("source");
    const filePath = document.getElementById("file-path");
    const projectTree = document.getElementById("project-tree");
    const diagnostics = document.getElementById("diagnostics");
    const preview = document.getElementById("preview");
    const generatedTabs = document.getElementById("generated-tabs");
    const generatedHTML = document.getElementById("generated-html");
    const generatedCSS = document.getElementById("generated-css");
    const generatedJS = document.getElementById("generated-js");
    const generatedAll = document.getElementById("generated-all");
    const shareStatus = document.getElementById("share-status");
    const starters = {
      basic: {
        "home.page.gwdk": "package app\n\n@page home\n@route \"/\"\n\nview {\n  <main><h1>Hello GOWDK</h1></main>\n}\n"
      },
      component: {
        "home.page.gwdk": "package app\n\n@page home\n@route \"/\"\n\nuse ui \"components\"\n\nview {\n  <main><Hero title=\"Hello component\" /></main>\n}\n",
        "hero.cmp.gwdk": "package components\n\n@component Hero\n\nprops {\n  title string\n}\n\nview {\n  <section><h1>{title}</h1></section>\n}\n"
      },
      action: {
        "newsletter.page.gwdk": "package app\n\n@page newsletter\n@route \"/newsletter\"\n\nact Subscribe POST \"/newsletter\"\n\nview {\n  <main><form g:post={Subscribe}><input name=\"email\" /><button>Join</button></form></main>\n}\n"
      }
    };
    let project = loadProjectFromHash() || clone(starters.basic);
    let activeFile = Object.keys(project).sort()[0];
    let previewObjectURLs = [];
    async function boot() {
      const go = new Go();
      const result = await WebAssembly.instantiateStreaming(fetch("gowdk.wasm"), go.importObject);
      go.run(result.instance);
      status.textContent = "ready";
    }
    function clone(value) {
      return JSON.parse(JSON.stringify(value));
    }
    function renderProjectTree() {
      projectTree.innerHTML = "";
      Object.keys(project).sort().forEach((name) => {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "file" + (name === activeFile ? " active" : "");
        button.textContent = name;
        button.addEventListener("click", () => {
          saveCurrentFile();
          activeFile = name;
          loadActiveFile();
        });
        projectTree.appendChild(button);
      });
    }
    function loadActiveFile() {
      if (!Object.prototype.hasOwnProperty.call(project, activeFile)) activeFile = Object.keys(project).sort()[0] || "home.page.gwdk";
      filePath.value = activeFile;
      source.value = project[activeFile] || "";
      renderProjectTree();
    }
    function saveCurrentFile() {
      const nextPath = filePath.value.trim();
      if (!nextPath) return;
      if (activeFile && activeFile !== nextPath) delete project[activeFile];
      activeFile = nextPath;
      project[activeFile] = source.value;
      renderProjectTree();
    }
    function addFile() {
      saveCurrentFile();
      let index = 1;
      let name = "new.page.gwdk";
      while (project[name]) {
        index++;
        name = "new-" + index + ".page.gwdk";
      }
      project[name] = "package app\n\n@page new\n@route \"/new\"\n\nview {\n  <main>New page</main>\n}\n";
      activeFile = name;
      loadActiveFile();
    }
    function deleteFile() {
      delete project[activeFile];
      activeFile = Object.keys(project).sort()[0] || "";
      loadActiveFile();
    }
    function loadStarter() {
      project = clone(starters[document.getElementById("starter").value] || starters.basic);
      activeFile = Object.keys(project).sort()[0];
      loadActiveFile();
      compile();
    }
    function loadProjectFromHash() {
      if (!location.hash.startsWith("#project=")) return null;
      try {
        const decoded = JSON.parse(atob(decodeURIComponent(location.hash.slice(9))));
        if (!decoded || typeof decoded !== "object" || !decoded.files) return null;
        return decoded.files;
      } catch (_) {
        return null;
      }
    }
    function shareProject() {
      saveCurrentFile();
      const payload = encodeURIComponent(btoa(JSON.stringify({ files: project })));
      history.replaceState(null, "", "#project=" + payload);
      shareStatus.textContent = "link updated";
    }
    function exportProject() {
      saveCurrentFile();
      const blob = new Blob([JSON.stringify({ files: project }, null, 2) + "\n"], { type: "application/json" });
      const link = document.createElement("a");
      link.href = URL.createObjectURL(blob);
      link.download = "gowdk-playground.json";
      link.click();
      URL.revokeObjectURL(link.href);
    }
    function importProject(file) {
      if (!file) return;
      file.text().then((text) => {
        const parsed = JSON.parse(text);
        if (!parsed.files) throw new Error("missing files");
        project = parsed.files;
        activeFile = Object.keys(project).sort()[0];
        loadActiveFile();
        compile();
      }).catch((error) => { status.textContent = "import failed: " + error.message; });
    }
    function compile() {
      saveCurrentFile();
      if (typeof window.gowdkCompile !== "function") {
        status.textContent = "compiler unavailable";
        return;
      }
      const request = { files: project };
      const result = JSON.parse(window.gowdkCompile(JSON.stringify(request)));
      diagnostics.textContent = JSON.stringify(result.diagnostics || [], null, 2);
      const first = Object.keys(result.html || {}).sort()[0];
      if (first) {
        preview.srcdoc = preparePreviewHTML(result, result.html[first]);
      } else {
        clearPreviewObjectURLs();
        preview.srcdoc = "";
      }
      renderGenerated(result);
      status.textContent = (result.diagnostics || []).some((item) => item.severity === "error") ? "errors" : "compiled";
    }
    function preparePreviewHTML(result, html) {
      clearPreviewObjectURLs();
      const files = result.files || {};
      Object.keys(files).sort().forEach((name) => {
        if (!name.startsWith("assets/")) return;
        const url = URL.createObjectURL(new Blob([files[name]], { type: previewMimeType(name) }));
        previewObjectURLs.push(url);
        html = rewritePreviewAssetURL(html, name, url);
      });
      return html;
    }
    function clearPreviewObjectURLs() {
      previewObjectURLs.forEach((url) => URL.revokeObjectURL(url));
      previewObjectURLs = [];
    }
    function previewMimeType(name) {
      if (name.endsWith(".css")) return "text/css";
      if (name.endsWith(".js")) return "text/javascript";
      if (name.endsWith(".wasm")) return "application/wasm";
      if (name.endsWith(".json")) return "application/json";
      return "text/plain";
    }
    function rewritePreviewAssetURL(html, filePath, assetURL) {
      const escapedPath = escapeRegExp(filePath);
      const escapedAbsolutePath = escapeRegExp("/" + filePath);
      const pattern = new RegExp("(\\b(?:href|src)\\s*=\\s*[\"'])(" + escapedAbsolutePath + "|" + escapedPath + ")([\"'])", "gi");
      return html.replace(pattern, "$1" + assetURL + "$3");
    }
    function escapeRegExp(value) {
      return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    }
    function renderGenerated(result) {
      const html = result.html || {};
      const css = result.css || {};
      const files = result.files || {};
      const js = {};
      Object.keys(files).sort().forEach((name) => {
        if (name.endsWith(".js")) js[name] = files[name];
      });
      generatedHTML.textContent = JSON.stringify(html, null, 2);
      generatedCSS.textContent = JSON.stringify(css, null, 2);
      generatedJS.textContent = JSON.stringify(js, null, 2);
      generatedAll.textContent = JSON.stringify(files, null, 2);
      renderGeneratedTabs();
    }
    function renderGeneratedTabs() {
      generatedTabs.innerHTML = "";
      [["html", generatedHTML], ["css", generatedCSS], ["js", generatedJS], ["all", generatedAll]].forEach((item) => {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "secondary";
        button.textContent = item[0].toUpperCase();
        button.addEventListener("click", () => {
          [generatedHTML, generatedCSS, generatedJS, generatedAll].forEach((view) => { view.hidden = view !== item[1]; });
        });
        generatedTabs.appendChild(button);
      });
    }
    document.getElementById("load-starter").addEventListener("click", loadStarter);
    document.getElementById("add-file").addEventListener("click", addFile);
    document.getElementById("save-file").addEventListener("click", saveCurrentFile);
    document.getElementById("delete-file").addEventListener("click", deleteFile);
    document.getElementById("export-project").addEventListener("click", exportProject);
    document.getElementById("share-project").addEventListener("click", shareProject);
    document.getElementById("import-file").addEventListener("change", (event) => importProject(event.target.files[0]));
    document.getElementById("compile").addEventListener("click", compile);
    loadActiveFile();
    renderGeneratedTabs();
    boot().then(compile).catch((error) => { status.textContent = error.message; });
  </script>
</body>
</html>
`
