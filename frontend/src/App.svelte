<script lang="ts">
  import { onMount } from 'svelte'
  import Sidebar from './components/Sidebar.svelte'
  import RequestPanel from './components/RequestPanel.svelte'
  import ResponsePanel from './components/ResponsePanel.svelte'
  import SaveDialog from './components/SaveDialog.svelte'
  import * as api from './lib/api'
  import { paramsFromUrl } from './lib/url'
  import {
    cleanRequest, newRequest, normalizeRequest, uid,
    type Collection, type HistoryEntry, type Origin, type Request, type Response,
  } from './lib/types'

  let request = $state<Request>(newRequest())
  let origin = $state<Origin | null>(null)
  // svelte-ignore state_referenced_locally
  let saved = $state(JSON.stringify(cleanRequest(request)))
  let response = $state<Response | null>(null)
  let sending = $state(false)
  let runId = ''

  let collections = $state<Collection[]>([])
  let history = $state<HistoryEntry[]>([])
  let showSave = $state(false)
  let notice = $state('')

  let split = $state(45) // request panel height, in % of the main area
  let main: HTMLElement

  const dirty = $derived(JSON.stringify(cleanRequest(request)) !== saved)
  const title = $derived(request.name || request.url || 'New request')

  onMount(async () => {
    try {
      collections = (await api.getCollections()).map((c) => ({ ...c, requests: (c.requests ?? []).map(normalizeRequest) }))
      history = await api.getHistory()
    } catch (e) {
      flash(`Could not load data: ${e}`)
    }
  })

  function flash(msg: string) {
    notice = msg
    setTimeout(() => notice === msg && (notice = ''), 3000)
  }

  function load(req: Request, from: Origin | null) {
    if (sending) cancel()
    request = normalizeRequest(JSON.parse(JSON.stringify(req)))
    request.params = paramsFromUrl(request.url, request.params)
    if (!from) request.id = uid()
    origin = from
    saved = from ? JSON.stringify(cleanRequest(request)) : ''
    response = null
  }

  function newTab() {
    load(newRequest(), null)
    saved = JSON.stringify(cleanRequest(request))
  }

  async function send() {
    if (sending || !request.url.trim()) return
    sending = true
    runId = uid()
    try {
      response = await api.sendRequest(runId, cleanRequest(request))
    } catch (e) {
      response = { error: String(e) } as Response
    } finally {
      sending = false
    }
    history = await api.getHistory().catch(() => history)
  }

  function cancel() {
    api.cancelRequest(runId)
  }

  async function persist() {
    try {
      await api.saveCollections($state.snapshot(collections))
    } catch (e) {
      flash(`Could not save: ${e}`)
    }
  }

  function save() {
    const col = origin && collections.find((c) => c.id === origin!.collectionId)
    const idx = col ? col.requests.findIndex((r) => r.id === origin!.requestId) : -1
    if (!col || idx < 0) {
      showSave = true
      return
    }
    const r = cleanRequest(request)
    r.name = col.requests[idx].name
    col.requests[idx] = r
    saved = JSON.stringify(cleanRequest(request))
    persist()
    flash('Saved')
  }

  function saveAs(name: string, collectionId: string | null, newCollectionName: string) {
    let col = collections.find((c) => c.id === collectionId)
    if (!col) {
      collections.push({ id: uid(), name: newCollectionName, requests: [] })
      col = collections[collections.length - 1]
    }
    request.name = name
    request.id = uid()
    col.requests.push(cleanRequest(request))
    origin = { collectionId: col.id, requestId: request.id }
    saved = JSON.stringify(cleanRequest(request))
    showSave = false
    persist()
  }

  async function clearHistory() {
    await api.clearHistory()
    history = []
  }

  function onKeydown(e: KeyboardEvent) {
    if (!e.metaKey || showSave) return
    if (e.key === 'Enter') {
      e.preventDefault()
      send()
    } else if (e.key === 's') {
      e.preventDefault()
      save()
    } else if (e.key === 'n') {
      e.preventDefault()
      newTab()
    }
  }

  function startResize(e: PointerEvent) {
    const el = e.currentTarget as HTMLElement
    el.setPointerCapture(e.pointerId)
    const rect = main.getBoundingClientRect()
    const move = (ev: PointerEvent) => {
      split = Math.min(80, Math.max(20, ((ev.clientY - rect.top) / rect.height) * 100))
    }
    el.addEventListener('pointermove', move)
    el.addEventListener('pointerup', () => el.removeEventListener('pointermove', move), { once: true })
  }
</script>

<svelte:window onkeydown={onKeydown} />

<div class="app">
  <header class="titlebar" style="--wails-draggable:drag">
    <span class="title">{title}</span>
    {#if origin && dirty}<span class="dot" title="Unsaved changes">●</span>{/if}
    {#if notice}<span class="notice">{notice}</span>{/if}
  </header>

  <div class="body">
    <Sidebar
      bind:collections
      {history}
      {origin}
      onopen={load}
      onchange={persist}
      onclearhistory={clearHistory}
      onnew={newTab}
    />

    <main bind:this={main}>
      <div class="pane" style="height: {split}%">
        <RequestPanel bind:request {sending} onsend={send} oncancel={cancel} onsave={save} />
      </div>
      <div class="divider" role="separator" aria-orientation="horizontal" onpointerdown={startResize}></div>
      <div class="pane grow">
        <ResponsePanel {response} {sending} />
      </div>
    </main>
  </div>
</div>

{#if showSave}
  <SaveDialog {collections} defaultName={request.name || request.url || 'New request'} onsave={saveAs} onclose={() => (showSave = false)} />
{/if}

<style>
  .app {
    display: flex;
    flex-direction: column;
    height: 100vh;
  }
  .titlebar {
    height: 38px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 0 80px;
    border-bottom: 1px solid var(--border);
    background: var(--bg-side);
    font-size: 12.5px;
    color: var(--text-dim);
    user-select: none;
  }
  .title {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dot {
    color: var(--accent);
    font-size: 9px;
  }
  .notice {
    position: absolute;
    right: 14px;
    color: var(--green);
  }
  .body {
    flex: 1;
    min-height: 0;
    display: grid;
    grid-template-columns: 270px 1fr;
  }
  main {
    display: flex;
    flex-direction: column;
    min-width: 0;
    min-height: 0;
  }
  .pane {
    min-height: 0;
    overflow: hidden;
  }
  .grow {
    flex: 1;
  }
  .divider {
    height: 5px;
    margin: -2px 0;
    position: relative;
    z-index: 1;
    cursor: row-resize;
    border-top: 2px solid transparent;
    border-bottom: 2px solid transparent;
    background: var(--border);
    background-clip: content-box;
  }
  .divider:hover {
    background-color: var(--accent);
  }
</style>
