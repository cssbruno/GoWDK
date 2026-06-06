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
    :root { color-scheme: light; font-family: system-ui, sans-serif; }
    body { margin: 0; background: #f7f7f5; color: #151515; }
    main { display: grid; grid-template-columns: minmax(18rem, 1fr) minmax(22rem, 1fr); min-height: 100vh; }
    section { padding: 1rem; border-right: 1px solid #ddd; }
    textarea, pre, iframe { width: 100%; box-sizing: border-box; border: 1px solid #ccc; border-radius: 6px; background: #fff; }
    textarea { min-height: 24rem; font: 13px/1.45 ui-monospace, SFMono-Regular, Menlo, monospace; padding: .75rem; }
    pre { min-height: 8rem; overflow: auto; padding: .75rem; }
    iframe { height: 24rem; }
    button { border: 1px solid #222; border-radius: 6px; background: #222; color: #fff; padding: .55rem .8rem; cursor: pointer; }
    .toolbar { display: flex; gap: .5rem; align-items: center; margin: .75rem 0; }
    .status { color: #555; font-size: .9rem; }
    @media (max-width: 800px) { main { grid-template-columns: 1fr; } section { border-right: 0; border-bottom: 1px solid #ddd; } }
  </style>
</head>
<body>
  <main>
    <section>
      <h1>GOWDK Playground</h1>
      <div class="toolbar">
        <button id="compile">Compile</button>
        <span class="status" id="status">loading compiler</span>
      </div>
      <textarea id="source">package app

@page home
@route "/"

view {
  <main><h1>Hello GOWDK</h1></main>
}
</textarea>
    </section>
    <section>
      <h2>Preview</h2>
      <iframe id="preview" sandbox="allow-scripts"></iframe>
      <h2>Diagnostics</h2>
      <pre id="diagnostics">[]</pre>
      <h2>Generated Files</h2>
      <pre id="files">{}</pre>
    </section>
  </main>
  <script src="wasm_exec.js"></script>
  <script>
    const status = document.getElementById("status");
    const source = document.getElementById("source");
    const diagnostics = document.getElementById("diagnostics");
    const files = document.getElementById("files");
    const preview = document.getElementById("preview");
    async function boot() {
      const go = new Go();
      const result = await WebAssembly.instantiateStreaming(fetch("gowdk.wasm"), go.importObject);
      go.run(result.instance);
      status.textContent = "ready";
    }
    function compile() {
      if (typeof window.gowdkCompile !== "function") {
        status.textContent = "compiler unavailable";
        return;
      }
      const request = { files: { "home.page.gwdk": source.value } };
      const result = JSON.parse(window.gowdkCompile(JSON.stringify(request)));
      diagnostics.textContent = JSON.stringify(result.diagnostics || [], null, 2);
      files.textContent = JSON.stringify(Object.keys(result.files || {}).sort(), null, 2);
      const first = Object.keys(result.html || {}).sort()[0];
      preview.srcdoc = first ? result.html[first] : "";
      status.textContent = (result.diagnostics || []).some((item) => item.severity === "error") ? "errors" : "compiled";
    }
    document.getElementById("compile").addEventListener("click", compile);
    boot().then(compile).catch((error) => { status.textContent = error.message; });
  </script>
</body>
</html>
`
