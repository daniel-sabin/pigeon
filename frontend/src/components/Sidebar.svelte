<script lang="ts">
  import type { Collection, HistoryEntry, Origin, Request } from '../lib/types'
  import { folders, uid } from '../lib/types'
  import { statusClass, timeAgo } from '../lib/format'

  interface Props {
    collections: Collection[]
    history: HistoryEntry[]
    origin: Origin | null
    onopen: (req: Request, origin: Origin | null) => void
    onchange: () => void
    onclearhistory: () => void
    onnew: () => void
    onimport: () => void
    onvariables: () => void
  }
  let { collections = $bindable(), history, origin, onopen, onchange, onclearhistory, onnew, onimport, onvariables }: Props = $props()

  let view = $state<'collections' | 'history'>('collections')
  let collapsed = $state<Record<string, boolean>>({})
  let renaming = $state<string | null>(null)
  let armed = $state<string | null>(null) // id awaiting a second click to delete
  let filter = $state('')

  const matches = (r: Request) => {
    const f = filter.trim().toLowerCase()
    return !f || r.name.toLowerCase().includes(f) || r.url.toLowerCase().includes(f)
  }

  function addCollection() {
    const c = { id: uid(), name: 'New collection', requests: [] }
    collections.push(c)
    renaming = c.id
    onchange()
  }

  function confirmDelete(id: string, action: () => void) {
    if (armed === id) {
      armed = null
      action()
      onchange()
    } else {
      armed = id
      setTimeout(() => armed === id && (armed = null), 2500)
    }
  }

  function finishRename(e: Event, setName: (n: string) => void) {
    const name = (e.target as HTMLInputElement).value.trim()
    if (name) setName(name)
    renaming = null
    onchange()
  }

  function renameKeys(e: KeyboardEvent) {
    if (e.key === 'Enter') (e.target as HTMLInputElement).blur()
    if (e.key === 'Escape') renaming = null
  }

  function focus(el: HTMLInputElement) {
    el.focus()
    el.select()
  }
</script>

{#snippet item(col: Collection, req: Request, nested: boolean)}
  <div class="row item" class:nested class:selected={origin?.requestId === req.id} role="treeitem" aria-selected={origin?.requestId === req.id} tabindex="0"
    onclick={() => onopen(req, { collectionId: col.id, requestId: req.id })}
    onkeydown={(e) => e.key === 'Enter' && onopen(req, { collectionId: col.id, requestId: req.id })}
    ondblclick={() => (renaming = req.id)}>
    <span class="verb m-{req.method}">{req.method}</span>
    {#if renaming === req.id}
      <input class="rename" value={req.name} use:focus onclick={(e) => e.stopPropagation()}
        onblur={(e) => finishRename(e, (n) => (req.name = n))} onkeydown={renameKeys} />
    {:else}
      <span class="name" title={req.url}>{req.name || req.url || 'Untitled'}</span>
      <button class="icon del" class:armed={armed === req.id} title="Delete request"
        onclick={(e) => { e.stopPropagation(); confirmDelete(req.id, () => (col.requests = col.requests.filter((r) => r.id !== req.id))) }}>
        {armed === req.id ? 'Delete?' : '×'}
      </button>
    {/if}
  </div>
{/snippet}

<aside class="sidebar">
  <div class="switch">
    <button class:active={view === 'collections'} onclick={() => (view = 'collections')}>Collections</button>
    <button class:active={view === 'history'} onclick={() => (view = 'history')}>History</button>
  </div>

  {#if view === 'collections'}
    <div class="tools">
      <input class="filter" placeholder="Filter" bind:value={filter} />
      <button class="icon" title="New request (⌘N)" onclick={onnew}>＋</button>
      <button class="icon" title="New collection" onclick={addCollection}>⊞</button>
      <button class="icon" title="Import from OpenAPI / Swagger" onclick={onimport}>⇣</button>
      <button class="icon" title="Variables, used as {'{{name}}'} in any request" onclick={onvariables}>{'{x}'}</button>
    </div>
    <div class="list">
      {#if collections.length === 0}
        <p class="empty">
          No collections yet. Save a request with ⌘S, or
          <button class="inline-link" onclick={onimport}>import an OpenAPI / Swagger spec</button>.
        </p>
      {/if}
      {#each collections as col (col.id)}
        <div class="row folder" ondblclick={() => (renaming = col.id)} role="treeitem" aria-selected="false" aria-expanded={!collapsed[col.id]} tabindex="0"
          onclick={() => (collapsed[col.id] = !collapsed[col.id])} onkeydown={(e) => e.key === 'Enter' && (collapsed[col.id] = !collapsed[col.id])}>
          <span class="chev">{collapsed[col.id] ? '▸' : '▾'}</span>
          {#if renaming === col.id}
            <input class="rename" value={col.name} use:focus onclick={(e) => e.stopPropagation()}
              onblur={(e) => finishRename(e, (n) => (col.name = n))} onkeydown={renameKeys} />
          {:else}
            <span class="name" title={col.source ? `Imported from ${col.source}` : col.name}>{col.name}</span>
            <span class="muted">{col.requests.length}</span>
            <button class="icon del" class:armed={armed === col.id} title="Delete collection"
              onclick={(e) => { e.stopPropagation(); confirmDelete(col.id, () => (collections = collections.filter((c) => c.id !== col.id))) }}>
              {armed === col.id ? 'Delete?' : '×'}
            </button>
          {/if}
        </div>
        {#if !collapsed[col.id] || filter}
          {#each folders(col.requests.filter(matches)) as f (f.name)}
            {#if f.name}
              {@const key = `${col.id}/${f.name}`}
              <div class="row folder sub" role="treeitem" aria-selected="false" aria-expanded={!collapsed[key]} tabindex="0"
                onclick={() => (collapsed[key] = !collapsed[key])} onkeydown={(e) => e.key === 'Enter' && (collapsed[key] = !collapsed[key])}>
                <span class="chev">{collapsed[key] ? '▸' : '▾'}</span>
                <span class="name" title={f.name}>{f.name}</span>
                <span class="muted">{f.requests.length}</span>
              </div>
              {#if !collapsed[key] || filter}
                {#each f.requests as req (req.id)}{@render item(col, req, true)}{/each}
              {/if}
            {:else}
              {#each f.requests as req (req.id)}{@render item(col, req, false)}{/each}
            {/if}
          {/each}
        {/if}
      {/each}
    </div>
  {:else}
    <div class="tools">
      <input class="filter" placeholder="Filter" bind:value={filter} />
      <button class="icon" class:armed={armed === 'history'} title="Clear history"
        onclick={() => confirmDelete('history', onclearhistory)}>{armed === 'history' ? 'Clear?' : '⌫'}</button>
    </div>
    <div class="list">
      {#if history.length === 0}
        <p class="empty">Requests you send will appear here.</p>
      {/if}
      {#each history.filter((h) => matches(h.request)) as h (h.id)}
        <div class="row item hist" role="button" tabindex="0" onclick={() => onopen(h.request, null)}
          onkeydown={(e) => e.key === 'Enter' && onopen(h.request, null)}>
          <span class="verb m-{h.request.method}">{h.request.method}</span>
          <span class="name" title={h.request.url}>{h.request.url}</span>
          <span class="status {h.error ? 's5' : statusClass(h.status)}">{h.error ? 'ERR' : h.status}</span>
          <span class="when">{timeAgo(h.timestamp)}</span>
        </div>
      {/each}
    </div>
  {/if}
</aside>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    background: var(--bg-side);
    border-right: 1px solid var(--border);
  }
  .switch {
    display: flex;
    margin: 10px;
    background: var(--bg);
    border-radius: 6px;
    padding: 2px;
  }
  .switch button {
    flex: 1;
    background: none;
    border: none;
    color: var(--text-dim);
    padding: 5px;
    border-radius: 5px;
    font-size: 12px;
    cursor: pointer;
  }
  .switch button.active {
    background: var(--bg-hover);
    color: var(--text);
  }
  .tools {
    display: flex;
    gap: 4px;
    padding: 0 10px 8px;
  }
  .filter {
    flex: 1;
    min-width: 0;
    padding: 4px 8px;
    font-size: 12px;
  }
  .list {
    flex: 1;
    overflow-y: auto;
    padding-bottom: 10px;
  }
  .empty {
    color: var(--text-dim);
    font-size: 12px;
    padding: 6px 14px;
    line-height: 1.5;
  }
  .inline-link {
    background: none;
    border: none;
    padding: 0;
    color: var(--accent);
    font: inherit;
    cursor: pointer;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 5px 10px;
    font-size: 12.5px;
    cursor: pointer;
    user-select: none;
    outline: none;
  }
  .row:hover,
  .row:focus-visible {
    background: var(--bg-hover);
  }
  .row.selected {
    background: var(--bg-active);
  }
  .item {
    padding-left: 22px;
  }
  .folder.sub {
    padding-left: 22px;
  }
  .folder.sub .name {
    font-weight: normal;
    color: var(--text-dim);
  }
  .item.nested {
    padding-left: 38px;
  }
  .hist {
    padding-left: 10px;
  }
  .chev {
    width: 10px;
    color: var(--text-dim);
    font-size: 10px;
  }
  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .folder .name {
    font-weight: 500;
  }
  .muted,
  .when {
    color: var(--text-dim);
    font-size: 11px;
    white-space: nowrap;
  }
  .verb {
    font-family: var(--mono);
    font-size: 10px;
    font-weight: 700;
    width: 44px;
    flex-shrink: 0;
  }
  .status {
    font-family: var(--mono);
    font-size: 11px;
  }
  .del {
    visibility: hidden;
  }
  .row:hover .del,
  .del.armed {
    visibility: visible;
  }
  .armed {
    color: var(--red) !important;
    font-size: 11px;
  }
  .rename {
    flex: 1;
    min-width: 0;
    padding: 2px 6px;
    font-size: 12.5px;
  }
</style>
