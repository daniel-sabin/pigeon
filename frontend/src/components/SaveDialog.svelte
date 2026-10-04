<script lang="ts">
  import type { Collection } from '../lib/types'

  interface Props {
    collections: Collection[]
    defaultName: string
    onsave: (name: string, collectionId: string | null, newCollectionName: string) => void
    onclose: () => void
  }
  let { collections, defaultName, onsave, onclose }: Props = $props()

  // Initial values only: the dialog is recreated each time it opens.
  // svelte-ignore state_referenced_locally
  let name = $state(defaultName)
  // svelte-ignore state_referenced_locally
  let target = $state(collections[0]?.id ?? '__new')
  let newCollection = $state('My API')

  function submit(e: SubmitEvent) {
    e.preventDefault()
    if (!name.trim()) return
    if (target === '__new' && !newCollection.trim()) return
    onsave(name.trim(), target === '__new' ? null : target, newCollection.trim())
  }

  function focus(el: HTMLInputElement) {
    el.focus()
    el.select()
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions, a11y_no_noninteractive_element_interactions -->
<div class="backdrop" onclick={onclose}>
  <form class="dialog" onsubmit={submit} onclick={(e) => e.stopPropagation()} onkeydown={(e) => e.key === 'Escape' && onclose()}>
    <h3>Save request</h3>
    <label>
      <span>Name</span>
      <input bind:value={name} use:focus />
    </label>
    <label>
      <span>Collection</span>
      <select bind:value={target}>
        {#each collections as c}<option value={c.id}>{c.name}</option>{/each}
        <option value="__new">New collection…</option>
      </select>
    </label>
    {#if target === '__new'}
      <label>
        <span>New collection</span>
        <input bind:value={newCollection} />
      </label>
    {/if}
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
    width: 400px;
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
    margin: 0 0 4px;
    font-size: 14px;
  }
  label {
    display: flex;
    flex-direction: column;
    gap: 5px;
    font-size: 12px;
    color: var(--text-dim);
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 6px;
  }
</style>
