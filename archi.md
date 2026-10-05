# Architecture de Pigeon

Pigeon est une app **[Wails](https://wails.io)** : un seul binaire Go qui ouvre une fenêtre macOS native contenant une **webview** (le moteur de Safari, WKWebView). Cette webview affiche une interface écrite en **Svelte 5 + TypeScript**.

Il n'y a **pas de serveur HTTP** entre le front et le back : le JS appelle directement des méthodes Go, et Wails fait le pont.

---

## 1. Vue d'ensemble

```
┌──────────────────────── Pigeon.app (1 binaire Go) ─────────────────────────┐
│                                                                             │
│  ┌──────────── Webview (WKWebView) ────────────┐                            │
│  │  frontend/dist  (HTML/JS/CSS embarqués)     │                            │
│  │                                              │                            │
│  │  App.svelte ──► lib/api.ts ──► wailsjs/go/main/App.js                    │
│  │                                   │  window.go.main.App.SendRequest(...)  │
│  └───────────────────────────────────┼──────────┘                            │
│                       pont IPC Wails │  (JSON aller / retour, Promise)       │
│  ┌───────────────────────────────────▼──────────────────────────────────┐  │
│  │ main.go   : câblage (store, menu, fenêtre, Bind: app)                 │  │
│  │ app.go    : type App = la "façade" exposée au JS                      │  │
│  │   ├── internal/engine   : construit et exécute la requête HTTP        │  │
│  │   ├── internal/storage  : collections + historique en fichiers JSON   │  │
│  │   └── internal/openapi  : import Swagger/OpenAPI → Collection         │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Comment le front et Go se parlent

### a) Côté Go : `Bind`

Dans [`main.go`](main.go) :

```go
Bind: []interface{}{app},
```

Wails parcourt par réflexion **toutes les méthodes exportées** de `*App` et les expose au JS sous `window.go.main.App.<NomMéthode>`. Le `main` vient du nom du package Go.

### b) La sérialisation, c'est du JSON

- Les arguments JS sont convertis en JSON, puis désérialisés dans les types Go. Ce sont les tags `json:"..."` (par exemple dans `engine.Request`) qui décident des noms de champs.
- La valeur de retour fait le chemin inverse.
- Côté JS, **tout appel renvoie une `Promise`**.
- Si la méthode Go renvoie `(T, error)` avec une `error` non nil, la Promise est **rejetée**. C'est ce que capture le `try/catch` du front.

### c) Le code généré : `frontend/wailsjs/`

Au lancement de `wails dev` ou `wails build`, Wails génère :

- `wailsjs/go/main/App.js` : une fonction par méthode Go, qui appelle simplement `window['go']['main']['App']['SendRequest'](arg1, arg2)`.
- `App.d.ts` et `models.ts` : les types TypeScript correspondants, déduits des structs Go.
- `wailsjs/runtime/` : les fonctions du runtime Wails (événements, drag & drop, etc.).

**On ne modifie jamais ces fichiers à la main.** Ils sont régénérés.

### d) La couche d'adaptation : `frontend/src/lib/api.ts`

Les composants n'importent jamais `wailsjs` directement : ils passent par [`lib/api.ts`](frontend/src/lib/api.ts), qui ré-exporte chaque appel avec les types écrits à la main dans [`lib/types.ts`](frontend/src/lib/types.ts). Les classes générées dans `models.ts` ne sont pas utilisées, d'où le `as any` à cette frontière.

> ⚠️ `types.ts` est une **copie manuelle** des structs Go. Si tu ajoutes un champ dans `engine.Request`, il faut aussi l'ajouter dans `types.ts`, avec exactement le même nom que le tag json.

### e) Dans l'autre sens, de Go vers le JS : le runtime

- `runtime.OpenFileDialog` (dans `App.ImportOpenAPIFile`) : Go ouvre une boîte de dialogue native.
- `OnFileDrop` (dans `App.svelte`) : le front s'abonne à un événement natif, le dépôt de fichiers sur la fenêtre.
- Ces appels runtime ont besoin du `ctx` reçu dans `startup()`. C'est pour ça que `App` le garde.

### Exemple complet : un clic sur « Send »

1. `RequestPanel.svelte` : le bouton appelle `onsend()`, un callback reçu du parent.
2. `send()` dans `App.svelte` génère un `runId`, puis appelle `api.sendRequest(runId, cleanRequest(request))`.
3. Le pont Wails sérialise la requête en JSON, et Go exécute `App.SendRequest` dans **sa propre goroutine**.
4. `App.SendRequest` crée un contexte annulable, le range dans `inflight[runID]`, puis appelle `engine.Send(...)`.
5. Il enregistre l'entrée dans l'historique (`store.AddHistory`) et renvoie la `Response`.
6. La Promise se résout côté JS. L'instruction `response = ...` met à jour l'état, et `ResponsePanel` se ré-affiche tout seul.
7. Le front recharge ensuite l'historique avec `api.getHistory()`.

**Annuler** : `cancelRequest(runId)` appelle Go, qui retrouve la `CancelFunc` dans la map et l'exécute. `client.Do` échoue avec `context.Canceled`, et `describeError` renvoie `"Request cancelled"`.

---

## 3. La partie Go

| Fichier | Rôle | Dépend de Wails ? |
|---|---|---|
| [`main.go`](main.go) | Point d'entrée : crée le `Store`, l'`App`, le menu, lance `wails.Run`. `//go:embed all:frontend/dist` embarque le front compilé dans le binaire | oui |
| [`app.go`](app.go) | **Contrôleur fin** : chaque méthode publique est un point d'entrée appelable depuis le JS. Pas de logique métier, il délègue | oui |
| [`internal/engine`](internal/engine) | Logique HTTP pure : `buildURL`, `buildHTTPRequest` (headers, body, auth), `Send` (lecture limitée à 20 Mo, détection binaire/image, mise en forme du JSON) | non |
| [`internal/storage`](internal/storage) | `collections.json` et `history.json` dans `~/Library/Application Support/Pigeon`. Écriture atomique (fichier temporaire puis `Rename`), historique plafonné à 300 entrées | non |
| [`internal/openapi`](internal/openapi) | `decode.go` lit du JSON ou du YAML **en conservant l'ordre des clés** (type `Map`), `convert.go` transforme une spec v2/v3 en `Collection`, `fetch.go` découvre l'URL de la spec depuis une page Swagger UI | non |

Graphe des dépendances internes : `openapi → storage → engine`. `engine` ne dépend de rien d'interne.

### Principes de conception

Voir aussi [`CONTRIBUTING.md`](CONTRIBUTING.md).

- **`internal/` ne connaît pas Wails.** Tout se teste avec `go test -race ./internal/...`, et `httptest.NewServer` sert à tester l'engine contre un vrai serveur.
- **Les erreurs de requête sont des données** : `engine.Send` renvoie toujours une `Response` avec un champ `Error`, jamais une `error` Go. L'UI affiche ainsi « connection refused » comme un résultat normal.
- **Concurrence** : Wails exécute chaque appel dans une goroutine différente. D'où le `sync.Mutex` dans `Store` et dans `App.inflight`.

---

## 4. Le front

### Les outils

- **Vite** ([`vite.config.ts`](frontend/vite.config.ts)) : en dev, un serveur avec rechargement à chaud ; en build, il compile tout dans `frontend/dist/`. C'est ce dossier que Go embarque.
- **Svelte 5** : un *compilateur*. Chaque fichier `.svelte` est transformé en JS qui met à jour le DOM directement, sans DOM virtuel.
- **TypeScript** : du JS typé. `npm run check` joue le rôle de `go vet` et vérifie les types.
- **CodeMirror** : l'éditeur de code (body JSON, affichage de la réponse).

### Le démarrage

`index.html` contient un `<div id="app">`. [`main.ts`](frontend/src/main.ts) fait `mount(App, { target: ... })`, et tout part de [`App.svelte`](frontend/src/App.svelte).

### Anatomie d'un fichier `.svelte`

Un fichier contient trois blocs :

```svelte
<script lang="ts">  ...logique...  </script>

...balisage HTML avec {expressions}...

<style>  ...CSS limité à CE composant...  </style>
```

### Les « runes » : la réactivité de Svelte 5

| Rune | Ce que ça fait | Exemple dans le code |
|---|---|---|
| `$state(v)` | Variable **réactive** : quand elle change, l'UI qui l'utilise se redessine. Fonctionne aussi pour les mutations profondes (`col.requests.push(...)`) | `let response = $state<Response \| null>(null)` |
| `$derived(expr)` | Valeur **calculée** à partir d'autres états, recalculée automatiquement | `const dirty = $derived(JSON.stringify(...) !== saved)` |
| `$effect(fn)` | Code exécuté chaque fois que l'état qu'il lit change (effet de bord) | `KeyValueEditor` : garde toujours une ligne vide à la fin |
| `$props()` | Les paramètres du composant, comme les arguments d'une fonction | `let { request = $bindable(), sending, onsend } = $props()` |
| `$bindable()` | Autorise le parent à faire un `bind:`, c'est-à-dire un lien bidirectionnel | `<RequestPanel bind:request ...>` |

### Le balisage

- `{expr}` affiche une valeur.
- `{#if cond}...{:else}...{/if}` et `{#each list as item}...{/each}` gèrent conditions et boucles.
- `onclick={fn}` branche un événement.
- `bind:value={x}` lie un input à une variable dans les deux sens.

### Le flux de données

```
App.svelte  ← possède TOUT l'état (request, response, collections, history…)
 ├─ Sidebar        bind:collections, {history}, onopen={load}, onchange={persist}
 ├─ RequestPanel   bind:request, onsend={send}, oncancel, onsave
 │    ├─ Tabs, KeyValueEditor (bind:rows), CodeEditor
 ├─ ResponsePanel  {response} {sending}   (lecture seule)
 ├─ SaveDialog / ImportDialog  (affichés via {#if ...})
```

La règle : **les données descendent par les props, les actions remontent par des callbacks** (`onsend`, `onchange`). `bind:` est un raccourci quand l'enfant modifie directement l'objet du parent.

### Persistance

- Le front garde **toute** la liste des collections en mémoire. À chaque modification, `persist()` renvoie le tableau entier via `SaveCollections`.
- `$state.snapshot(...)` convertit l'objet réactif (un *Proxy* JS) en objet simple avant l'envoi à Go.
- L'historique, lui, est écrit côté Go pendant `SendRequest`. Le front se contente de le relire.

### Intégrer une bibliothèque non-Svelte

[`CodeEditor.svelte`](frontend/src/components/CodeEditor.svelte) montre la marche à suivre :

1. Créer l'objet dans `onMount`.
2. Le détruire dans le `return`.
3. Synchroniser les props avec des `$effect`.

---

## 5. Faire évoluer le code

### A) Exposer une nouvelle méthode Go au front

1. Ajouter une méthode **exportée** sur `*App` dans `app.go`, de préférence fine, qui délègue à un package `internal/`.
2. Lancer `wails dev` : il régénère `wailsjs/go/main/App.js` et `.d.ts`.
3. Ajouter un wrapper typé dans `lib/api.ts`.
4. L'appeler depuis un composant avec `await api.maFonction(...)` dans un `try/catch`.

### B) Ajouter un champ ou une fonctionnalité qui traverse toutes les couches

Exemple : un type d'auth « API key ».

1. **Go** : ajouter `Key`/`In` dans `engine.Auth` (avec les tags json), un `case "apikey":` dans `buildHTTPRequest`, et un test dans `request_test.go`.
2. **TS** : mettre à jour `types.ts`, c'est-à-dire `AuthType`, l'interface `Request.auth` et les valeurs par défaut de `newRequest()`. `normalizeRequest` complète déjà les anciennes données sauvegardées sans ce champ.
3. **UI** : ajouter l'onglet Auth dans `RequestPanel.svelte`.
4. **Vérifications** : `go test -race ./internal/...` puis `cd frontend && npm run check`.

---

## 6. Les pièges à connaître

- **slice nil en Go → `null` en JS**, pas `[]`. D'où les `?? []` dans `api.ts` et `App.svelte`. Renvoyer `[]T{}` plutôt que `nil` quand c'est possible.
- **Désynchronisation `types.ts` ↔ structs Go** : le compilateur ne la détecte pas. Si un champ arrive vide côté front, vérifier d'abord le tag json.
- **`go build` échoue si `frontend/dist` n'existe pas**, à cause de `go:embed`. Passer par `wails build`. Les tests de `internal/` n'en ont pas besoin.
- **`npm run dev` seul dans un navigateur** : `window.go` n'existe pas, donc les appels Go échouent. C'est pour ça qu'il y a un `try/catch` autour de `OnFileDrop`. Développer avec `wails dev`.
- **Une méthode non exportée, ou sur un autre type que `App`**, n'est pas visible côté JS.

---

## 7. Parcours de lecture conseillé

1. `main.go` puis `app.go` : le contrat complet entre front et back.
2. `internal/engine/request.go` et son test.
3. `frontend/src/lib/types.ts` et `lib/api.ts` : la frontière vue depuis le JS.
4. `App.svelte` : l'état central et les fonctions `send`, `save`, `load`.
5. `KeyValueEditor.svelte` : le plus petit composant, idéal pour comprendre `$props`, `$bindable`, `$effect` et `{#each}`.
6. `RequestPanel.svelte`, puis `Sidebar.svelte`.
7. `internal/openapi`.
