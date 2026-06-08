<p align="center">
  <img src="wdk_logo.png" alt="GOWDK logo" width="220">
</p>

# GOWDK

[![CI](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/ci.yml)
[![Release](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml/badge.svg)](https://github.com/cssbruno/GOWDK/actions/workflows/release.yml)
![Go](https://img.shields.io/badge/Go-1.26.4-00ADD8)

GOWDK ships Go web apps as generated Go servers. Write portable `.gwdk`
pages, compile frontend output, and package it into one binary.

**Status:** pre-release. Public contracts can still change. Not production
ready.

Install:

```sh
curl -fsSL https://raw.githubusercontent.com/cssbruno/GoWDK/main/scripts/install.sh | sh
```

Build from source:

```sh
git clone https://github.com/cssbruno/GoWDK.git
cd GoWDK
go build ./cmd/gowdk
./gowdk version
```

## Single-Page Server

Create a one-page app, build its generated server, and run the binary:

```sh
gowdk init --tests --template site ./hello-gowdk
cd ./hello-gowdk
gowdk build
./bin/site
```

Open `http://127.0.0.1:8080/`. A minimal page:

```gwdk
package pages

use widgets "components"

@page home
@route "/"

build {
  => { title: "GOWDK ships apps" }
}

view {
  <main>
    <h1>{title}</h1>
    <p>A .gwdk page compiled into an embedded Go server.</p>
    <widgets.Counter />
  </main>
}
```

Add local reactivity:

```go
// ui/counter.go
package ui

type CounterState struct {
	Count int `json:"count"`
}

func NewCounterState() CounterState {
	return CounterState{Count: 0}
}
```

```gwdk
// components/counter.cmp.gwdk
package components

import ui "github.com/acme/hello-gowdk/ui"

@component Counter

state ui.CounterState = ui.NewCounterState()

client {
  fn Increment() {
    Count = Count + 1
  }

  fn Reset() {
    Count = 0
  }
}

view {
  <section>
    <p class:active={Count > 0}>Count: {Count}</p>
    <button g:on:click={Increment()}>Add</button>
    <button g:if={Count > 0} g:on:click={Reset()}>Reset</button>
  </section>
}
```

Replace `github.com/acme/hello-gowdk/ui` with your app module path.

## Docs

- [Getting started](docs/getting-started.md)
- [Examples](examples/README.md)
