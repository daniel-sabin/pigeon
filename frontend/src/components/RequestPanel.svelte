<script lang="ts">
  import Tabs from './Tabs.svelte'
  import KeyValueEditor from './KeyValueEditor.svelte'
  import CodeEditor from './CodeEditor.svelte'
  import { METHODS, type BodyType, type Request } from '../lib/types'
  import { paramsFromUrl, urlWithParams } from '../lib/url'

  interface Props {
    request: Request
    sending: boolean
    onsend: () => void
    oncancel: () => void
    onsave: () => void
  }
  let { request = $bindable(), sending, onsend, oncancel, onsave }: Props = $props()

  let tab = $state('params')

  const filled = (rows: { key: string; enabled: boolean }[]) => rows.filter((r) => r.enabled && r.key).length

  const tabs = $derived([
    { id: 'params', label: 'Params', count: filled(request.params) },
    { id: 'headers', label: 'Headers', count: filled(request.headers) },
    { id: 'body', label: 'Body', count: request.body.type !== 'none' ? 1 : 0 },
    { id: 'auth', label: 'Auth', count: request.auth.type !== 'none' ? 1 : 0 },
  ])

  const bodyTypes: { id: BodyType; label: string }[] = [
    { id: 'none', label: 'None' },
    { id: 'json', label: 'JSON' },
    { id: 'text', label: 'Text' },
    { id: 'xml', label: 'XML' },
    { id: 'form', label: 'Form URL-encoded' },
  ]

  function onUrlInput() {
    request.params = paramsFromUrl(request.url, request.params)
  }

  function onParamsChange() {
    request.url = urlWithParams(request.url, request.params)
  }

  function formatJson() {
    try {
      request.body.raw = JSON.stringify(JSON.parse(request.body.raw), null, 2)
    } catch {
      // leave invalid JSON untouched
    }
  }
</script>

<section class="request">
  <div class="bar">
    <select class="method m-{request.method}" bind:value={request.method}>
      {#each METHODS as m}<option value={m}>{m}</option>{/each}
    </select>
    <input
      class="url"
      placeholder="https://api.example.com/resource"
      bind:value={request.url}
      oninput={onUrlInput}
      spellcheck="false"
      autocomplete="off"
    />
    {#if sending}
      <button class="btn primary" onclick={oncancel}>Cancel</button>
    {:else}
      <button class="btn primary" onclick={onsend} title="⌘↵">Send</button>
    {/if}
    <button class="btn" onclick={onsave} title="⌘S">Save</button>
  </div>

  <Tabs {tabs} bind:active={tab} />

  <div class="content">
    {#if tab === 'params'}
      <KeyValueEditor bind:rows={request.params} onchange={onParamsChange} />
    {:else if tab === 'headers'}
      <KeyValueEditor bind:rows={request.headers} keyPlaceholder="Header" />
    {:else if tab === 'body'}
      <div class="body">
        <div class="subbar">
          {#each bodyTypes as bt}
            <label class="radio">
              <input type="radio" bind:group={request.body.type} value={bt.id} />
              {bt.label}
            </label>
          {/each}
          {#if request.body.type === 'json'}
            <button class="link" onclick={formatJson}>Format</button>
          {/if}
        </div>
        {#if request.body.type === 'form'}
          <KeyValueEditor bind:rows={request.body.form} />
        {:else if request.body.type !== 'none'}
          <div class="code">
            <CodeEditor
              value={request.body.raw}
              lang={request.body.type === 'json' ? 'json' : request.body.type === 'xml' ? 'xml' : 'text'}
              onchange={(v) => (request.body.raw = v)}
            />
          </div>
        {:else}
          <p class="empty">This request has no body.</p>
        {/if}
      </div>
    {:else if tab === 'auth'}
      <div class="auth">
        <label class="field">
          <span>Type</span>
          <select bind:value={request.auth.type}>
            <option value="none">No auth</option>
            <option value="bearer">Bearer token</option>
            <option value="basic">Basic auth</option>
          </select>
        </label>
        {#if request.auth.type === 'bearer'}
          <label class="field">
            <span>Token</span>
            <input class="mono" bind:value={request.auth.token} spellcheck="false" />
          </label>
        {:else if request.auth.type === 'basic'}
          <label class="field">
            <span>Username</span>
            <input class="mono" bind:value={request.auth.username} spellcheck="false" />
          </label>
          <label class="field">
            <span>Password</span>
            <input class="mono" type="password" bind:value={request.auth.password} />
          </label>
        {/if}
      </div>
    {/if}
  </div>
</section>

<style>
  .request {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: 100%;
  }
  .bar {
    display: flex;
    gap: 8px;
    padding: 12px;
  }
  .method {
    width: 104px;
    font-weight: 600;
    font-family: var(--mono);
  }
  .url {
    flex: 1;
    font-family: var(--mono);
    font-size: 13px;
  }
  .content {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .body {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .subbar {
    display: flex;
    align-items: center;
    gap: 14px;
    padding: 8px 12px;
    font-size: 12.5px;
    color: var(--text-dim);
  }
  .radio {
    display: flex;
    align-items: center;
    gap: 5px;
    cursor: pointer;
  }
  .link {
    margin-left: auto;
    background: none;
    border: none;
    color: var(--accent);
    cursor: pointer;
    font-size: 12.5px;
  }
  .code {
    flex: 1;
    min-height: 0;
    border-top: 1px solid var(--border);
  }
  .empty {
    color: var(--text-dim);
    padding: 12px;
    font-size: 13px;
  }
  .auth {
    padding: 14px 12px;
    display: flex;
    flex-direction: column;
    gap: 10px;
    max-width: 520px;
  }
  .field {
    display: grid;
    grid-template-columns: 90px 1fr;
    align-items: center;
    font-size: 12.5px;
    color: var(--text-dim);
  }
</style>
