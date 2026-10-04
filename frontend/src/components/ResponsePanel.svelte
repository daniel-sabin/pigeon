<script lang="ts">
  import Tabs from './Tabs.svelte'
  import CodeEditor from './CodeEditor.svelte'
  import type { Response } from '../lib/types'
  import { formatDuration, formatSize, langFor, statusClass } from '../lib/format'

  let { response, sending }: { response: Response | null; sending: boolean } = $props()

  let tab = $state('body')
  let pretty = $state(true)
  let copied = $state(false)

  const tabs = $derived([
    { id: 'body', label: 'Body' },
    { id: 'headers', label: 'Headers', count: response?.headers?.length ?? 0 },
  ])
  const canPretty = $derived(!!response?.bodyPretty)
  const shown = $derived(response ? (pretty && response.bodyPretty ? response.bodyPretty : response.body) : '')

  async function copy() {
    await navigator.clipboard.writeText(shown)
    copied = true
    setTimeout(() => (copied = false), 1200)
  }
</script>

<section class="response">
  {#if sending}
    <div class="placeholder"><span class="spinner"></span> Sending…</div>
  {:else if !response}
    <div class="placeholder">
      <p>Send a request to see the response.</p>
      <p class="hint">⌘↵ send · ⌘S save · ⌘N new request</p>
    </div>
  {:else if response.error}
    <div class="error">
      <strong>Could not get a response</strong>
      <code>{response.error}</code>
    </div>
  {:else}
    <div class="meta">
      <span class="status {statusClass(response.status)}">{response.status} {response.statusText}</span>
      <span>{formatDuration(response.durationMs)}</span>
      <span>{formatSize(response.size)}</span>
      {#if response.truncated}<span class="warn">truncated</span>{/if}
      <span class="spacer"></span>
      {#if tab === 'body' && !response.isBinary}
        {#if canPretty}
          <button class="link" onclick={() => (pretty = !pretty)}>{pretty ? 'Raw' : 'Pretty'}</button>
        {/if}
        <button class="link" onclick={copy}>{copied ? 'Copied' : 'Copy'}</button>
      {/if}
    </div>

    <Tabs {tabs} bind:active={tab} />

    <div class="content">
      {#if tab === 'body'}
        {#if response.bodyBase64}
          <div class="image">
            <img alt="response" src="data:{response.contentType};base64,{response.bodyBase64}" />
          </div>
        {:else if response.isBinary}
          <p class="empty">Binary content ({formatSize(response.size)}, {response.contentType || 'unknown type'})</p>
        {:else if response.body === ''}
          <p class="empty">Empty body</p>
        {:else}
          <CodeEditor value={shown} lang={langFor(response.contentType)} readonly />
        {/if}
      {:else}
        <table class="headers">
          <tbody>
            {#each response.headers ?? [] as h}
              <tr><td class="k">{h.key}</td><td class="v">{h.value}</td></tr>
            {/each}
          </tbody>
        </table>
      {/if}
    </div>
  {/if}
</section>

<style>
  .response {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .placeholder {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 6px;
    color: var(--text-dim);
    font-size: 13px;
  }
  .placeholder p {
    margin: 0;
  }
  .hint {
    font-size: 12px;
    opacity: 0.7;
  }
  .spinner {
    width: 18px;
    height: 18px;
    border: 2px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
  .error {
    margin: 16px;
    padding: 12px 14px;
    border: 1px solid #6b2a2a;
    background: #2a1717;
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    gap: 6px;
    font-size: 13px;
  }
  .error code {
    font-family: var(--mono);
    color: #f19999;
    white-space: pre-wrap;
  }
  .meta {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 10px 12px 4px;
    font-size: 12.5px;
    color: var(--text-dim);
    font-family: var(--mono);
  }
  .status {
    font-weight: 600;
  }
  .warn {
    color: var(--yellow);
  }
  .spacer {
    flex: 1;
  }
  .link {
    background: none;
    border: none;
    color: var(--accent);
    cursor: pointer;
    font-size: 12.5px;
  }
  .content {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .empty {
    color: var(--text-dim);
    padding: 12px;
    font-size: 13px;
  }
  .image {
    padding: 16px;
  }
  .image img {
    max-width: 100%;
    background: repeating-conic-gradient(#333 0% 25%, #2a2a2a 0% 50%) 50% / 16px 16px;
  }
  .headers {
    width: 100%;
    border-collapse: collapse;
    font-family: var(--mono);
    font-size: 12.5px;
  }
  .headers td {
    padding: 6px 12px;
    border-bottom: 1px solid var(--border);
    vertical-align: top;
    word-break: break-all;
    user-select: text;
  }
  .headers .k {
    width: 30%;
    color: var(--text-dim);
  }
</style>
