<script lang="ts">
  import type { KeyValue } from '../lib/types'

  interface Props {
    rows: KeyValue[]
    keyPlaceholder?: string
    valuePlaceholder?: string
    onchange?: () => void
  }
  let { rows = $bindable(), keyPlaceholder = 'Key', valuePlaceholder = 'Value', onchange }: Props = $props()

  // Always keep one empty row at the end to type into.
  $effect(() => {
    const last = rows[rows.length - 1]
    if (!last || last.key !== '' || last.value !== '') {
      rows.push({ key: '', value: '', enabled: true })
    }
  })

  function remove(i: number) {
    rows.splice(i, 1)
    onchange?.()
  }
</script>

<table class="kv">
  <tbody>
    {#each rows as row, i}
      {@const isDraft = i === rows.length - 1}
      <tr class:disabled={!row.enabled && !isDraft}>
        <td class="check">
          {#if !isDraft}
            <input type="checkbox" bind:checked={row.enabled} onchange={() => onchange?.()} />
          {/if}
        </td>
        <td><input class="cell" placeholder={keyPlaceholder} bind:value={row.key} oninput={() => onchange?.()} spellcheck="false" /></td>
        <td><input class="cell" placeholder={valuePlaceholder} bind:value={row.value} oninput={() => onchange?.()} spellcheck="false" /></td>
        <td class="del">
          {#if !isDraft}
            <button class="icon" title="Remove" onclick={() => remove(i)}>×</button>
          {/if}
        </td>
      </tr>
    {/each}
  </tbody>
</table>

<style>
  .kv {
    width: 100%;
    border-collapse: collapse;
    table-layout: fixed;
  }
  td {
    border-bottom: 1px solid var(--border);
    padding: 0;
  }
  td.check {
    width: 30px;
    text-align: center;
  }
  td.del {
    width: 30px;
    text-align: center;
  }
  tr:not(:hover) .del button {
    visibility: hidden;
  }
  tr.disabled .cell {
    color: var(--text-dim);
    text-decoration: line-through;
  }
  .cell {
    width: 100%;
    background: transparent;
    border: none;
    color: var(--text);
    font-family: var(--mono);
    font-size: 12.5px;
    padding: 7px 8px;
    outline: none;
  }
  .cell:focus {
    background: var(--bg-hover);
  }
</style>
