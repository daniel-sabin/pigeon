<script lang="ts">
  interface Tab {
    id: string
    label: string
    count?: number
  }
  let { tabs, active = $bindable() }: { tabs: Tab[]; active: string } = $props()
</script>

<div class="tabs">
  {#each tabs as tab}
    <button class="tab" class:active={active === tab.id} onclick={() => (active = tab.id)}>
      {tab.label}
      {#if tab.count}<span class="count">{tab.count}</span>{/if}
    </button>
  {/each}
</div>

<style>
  .tabs {
    display: flex;
    gap: 2px;
    padding: 0 12px;
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
  }
  .tab {
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    color: var(--text-dim);
    padding: 9px 10px 7px;
    font-size: 12.5px;
    cursor: pointer;
  }
  .tab:hover {
    color: var(--text);
  }
  .tab.active {
    color: var(--text);
    border-bottom-color: var(--accent);
  }
  .count {
    color: var(--accent);
    font-size: 11px;
    margin-left: 3px;
  }
</style>
