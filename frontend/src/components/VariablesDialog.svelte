<script lang="ts">
  import { onMount } from 'svelte'
  import KeyValueEditor from './KeyValueEditor.svelte'
  import * as api from '../lib/api'
  import type { KeyValue } from '../lib/types'

  interface Props {
    onclose: () => void
    onerror: (msg: string) => void
  }
  let { onclose, onerror }: Props = $props()

  let vars = $state<KeyValue[]>([])

  onMount(async () => {
    try {
      vars = await api.getVariables()
    } catch (e) {
      onerror(`Could not load variables: ${e}`)
    }
  })

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    try {
      await api.saveVariables($state.snapshot(vars).filter((v) => v.key.trim() !== ''))
      onclose()
    } catch (err) {
      onerror(`Could not save variables: ${err}`)
    }
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions, a11y_no_noninteractive_element_interactions -->
<div class="backdrop" onclick={onclose}>
  <form class="dialog" onsubmit={submit} onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && onclose()}>
    <h3>Variables</h3>
    <p class="hint">
      Write <code>{'{{name}}'}</code> in the URL, params, headers, body or auth of any request: it is replaced when the
      request is sent. The history keeps the placeholder, not the value.
    </p>
    <div class="table">
      <KeyValueEditor bind:rows={vars} keyPlaceholder="name" valuePlaceholder="value" />
    </div>
    <div class="actions">
      <button type="button" class="btn" onclick={onclose}>Cancel</button>
      <button type="submit" class="btn primary">Save</button>
    </div>
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
  .hint {
    margin: 0;
    font-size: 12px;
    color: var(--text-dim);
    line-height: 1.5;
  }
  code {
    font-family: var(--mono);
    color: var(--text);
  }
  .table {
    max-height: 320px;
    overflow: auto;
    border-top: 1px solid var(--border);
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }
</style>
