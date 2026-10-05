<script lang="ts">
  import * as api from '../lib/api'
  import type { Collection } from '../lib/types'

  interface Props {
    onimport: (col: Collection) => void
    onclose: () => void
  }
  let { onimport, onclose }: Props = $props()

  let url = $state('')
  let loading = $state(false)
  let error = $state('')

  async function run(load: () => Promise<Collection | null>) {
    loading = true
    error = ''
    try {
      const col = await load()
      if (col) onimport(col)
    } catch (e) {
      error = String(e)
    } finally {
      loading = false
    }
  }

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (url.trim() && !loading) run(() => api.importFromUrl(url.trim()))
  }

  function focus(el: HTMLInputElement) {
    el.focus()
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions, a11y_no_noninteractive_element_interactions -->
<div class="backdrop" onclick={() => !loading && onclose()}>
  <form class="dialog" onsubmit={submit} onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && !loading && onclose()}>
    <h3>Import from OpenAPI / Swagger</h3>
    <p class="help">Creates a collection with one request per operation. Swagger 2.0 and OpenAPI 3.x, JSON or YAML.</p>

    <label>
      <span>Document or Swagger UI URL</span>
      <input
        class="mono"
        placeholder="https://petstore3.swagger.io/api/v3/openapi.json"
        bind:value={url}
        use:focus
        spellcheck="false"
        autocomplete="off"
        disabled={loading}
      />
    </label>

    {#if error}<p class="error">{error}</p>{/if}

    <div class="actions">
      <button type="button" class="btn" disabled={loading} onclick={() => run(api.importFromFile)}>Choose a file…</button>
      <span class="spacer"></span>
      <button type="button" class="btn" disabled={loading} onclick={onclose}>Cancel</button>
      <button type="submit" class="btn primary" disabled={loading || !url.trim()}>
        {loading ? 'Importing…' : 'Import'}
      </button>
    </div>
    <p class="hint">Tip: you can also drop a .json or .yaml file on the window.</p>
  </form>
</div>

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.45);
    display: flex;
    align-items: flex-start;
    justify-content: center;
    padding-top: 120px;
    z-index: 10;
  }
  .dialog {
    width: 520px;
    background: var(--bg-side);
    border: 1px solid var(--border);
    border-radius: 10px;
    padding: 18px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    box-shadow: 0 20px 50px rgba(0, 0, 0, 0.5);
  }
  h3 {
    margin: 0;
    font-size: 14px;
  }
  .help,
  .hint {
    margin: 0;
    font-size: 12px;
    color: var(--text-dim);
    line-height: 1.5;
  }
  .hint {
    opacity: 0.75;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 5px;
    font-size: 12px;
    color: var(--text-dim);
  }
  .error {
    margin: 0;
    padding: 8px 10px;
    border-radius: 6px;
    background: #2a1717;
    border: 1px solid #6b2a2a;
    color: #f19999;
    font-size: 12px;
    font-family: var(--mono);
    white-space: pre-wrap;
    user-select: text;
  }
  .actions {
    display: flex;
    gap: 8px;
    margin-top: 4px;
  }
  .spacer {
    flex: 1;
  }
  .btn:disabled {
    opacity: 0.55;
    cursor: default;
  }
</style>
