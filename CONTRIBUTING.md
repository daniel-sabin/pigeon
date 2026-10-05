# Contributing to Pigeon

Thanks for your interest in Pigeon! Bug reports, ideas and pull requests are all welcome.

## Ground rules

- **Open an issue first** for anything bigger than a small fix, so we can agree on the approach before you spend time on it.
- **Keep it lightweight.** Pigeon aims to be a fast, focused HTTP client — not a Postman clone with every feature. Proposals that add accounts, cloud sync or telemetry are out of scope.
- **Local-first.** All user data stays in plain JSON files on the user's machine.

## Development setup

### Prerequisites

| Tool | Version | Install |
|---|---|---|
| macOS | 11+ | — |
| Xcode Command Line Tools | latest | `xcode-select --install` |
| Go | ≥ 1.25 | <https://go.dev/dl/> or `brew install go` |
| Node.js | ≥ 20 (see `.nvmrc`) | `nvm install && nvm use` |
| Wails CLI | v2 | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |

Make sure `$(go env GOPATH)/bin` is on your `PATH` so the `wails` command is found. `wails doctor` checks that everything is in place.

### Run the app in dev mode

```sh
git clone git@github.com:daniel-sabin/pigeon.git
cd pigeon
nvm use
wails dev
```

`wails dev` starts the Vite dev server with hot reload for the frontend and rebuilds the Go side when `.go` files change. It also regenerates the TypeScript bindings in `frontend/wailsjs/`.

### Build

```sh
wails build                               # build/bin/Pigeon.app (current arch)
wails build -platform darwin/universal    # Intel + Apple Silicon
```

## Project layout

```
.
├── main.go                 # Wails bootstrap: window, menu, bindings
├── app.go                  # Methods exposed to the frontend (send, cancel, collections, history)
├── internal/
│   ├── engine/             # Builds and sends HTTP requests, reads responses
│   ├── openapi/            # Swagger 2.0 / OpenAPI 3.x → collection (fetch, parse, examples)
│   └── storage/            # JSON persistence for collections and history
├── frontend/
│   ├── src/
│   │   ├── App.svelte      # App state and layout
│   │   ├── components/     # Sidebar, request/response panels, editors
│   │   └── lib/            # Types, API wrapper, URL ↔ params sync, formatting
│   └── wailsjs/            # Generated bindings — do not edit by hand
└── build/                  # App icon and macOS Info.plist
```

Design principles:

- **The Go side does all network I/O.** The webview never makes HTTP calls itself, which avoids CORS and gives full control over headers, TLS and timeouts.
- **`internal/` packages don't depend on Wails**, so they can be tested with plain `go test`.
- **Errors from requests are data, not exceptions:** `engine.Send` always returns a `Response`, with `Error` set when something went wrong, so the UI can display it.

## Checks

Run these before opening a pull request — CI runs the same ones:

```sh
gofmt -l .                   # must print nothing
go vet ./internal/...
go test -race ./internal/...
cd frontend && npm run check # svelte-check / TypeScript
```

New behavior in `internal/` should come with tests. `httptest.NewServer` makes it easy to test the engine against a real HTTP server.

UI changes are not covered by automated tests: please describe what you checked manually and add a screenshot to the pull request.

## Code style

- **Go:** `gofmt`, standard library first, small focused packages. Comments explain *why*, not *what*.
- **Frontend:** Svelte 5 runes (`$state`, `$derived`, `$props`), TypeScript, component-scoped styles. Shared colors live as CSS variables in `frontend/src/style.css`.
- Avoid adding dependencies unless they clearly pay for themselves.

## Commits and pull requests

- Use clear, imperative commit messages: `Add environment variables`, `Fix params sync when URL has a fragment`.
- Keep pull requests focused on one change.
- Update the README when you change user-facing behavior.

## Reporting bugs

Please include:

- your macOS version and chip (Intel / Apple Silicon),
- the Pigeon version (*Pigeon → About Pigeon*),
- steps to reproduce, what you expected and what happened.

If the problem is about a specific request, a minimal `curl` command that reproduces it helps a lot (remove any secrets first!).

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
