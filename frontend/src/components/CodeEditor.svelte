<script lang="ts">
  import { onMount } from 'svelte'
  import { basicSetup } from 'codemirror'
  import { EditorView, keymap } from '@codemirror/view'
  import { Compartment, EditorState, Prec, type Extension } from '@codemirror/state'
  import { json } from '@codemirror/lang-json'
  import { xml } from '@codemirror/lang-xml'
  import { html } from '@codemirror/lang-html'
  import { oneDark } from '@codemirror/theme-one-dark'
  import type { Lang } from '../lib/format'

  interface Props {
    value: string
    lang?: Lang
    readonly?: boolean
    onchange?: (value: string) => void
  }
  let { value, lang = 'text', readonly = false, onchange }: Props = $props()

  let host: HTMLDivElement
  let view: EditorView
  const language = new Compartment()

  const langExt = (l: Lang): Extension =>
    l === 'json' ? json() : l === 'xml' ? xml() : l === 'html' ? html() : []

  const theme = EditorView.theme({
    '&': { height: '100%', fontSize: '12.5px', backgroundColor: 'transparent' },
    '.cm-scroller': { fontFamily: 'var(--mono)', lineHeight: '1.55' },
    '.cm-gutters': { backgroundColor: 'transparent', border: 'none' },
    '&.cm-focused': { outline: 'none' },
  })

  onMount(() => {
    view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: value,
        extensions: [
          theme, // before oneDark so it takes precedence
          basicSetup,
          oneDark,
          EditorView.lineWrapping,
          language.of(langExt(lang)),
          EditorState.readOnly.of(readonly),
          // Let the app-level shortcuts (Cmd+Enter, Cmd+S) through.
          Prec.highest(keymap.of([{ key: 'Mod-Enter', run: () => true, preventDefault: false }])),
          EditorView.updateListener.of((u) => {
            if (u.docChanged) onchange?.(u.state.doc.toString())
          }),
        ],
      }),
    })
    return () => view.destroy()
  })

  $effect(() => {
    if (view && value !== view.state.doc.toString()) {
      view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: value } })
    }
  })

  $effect(() => {
    view?.dispatch({ effects: language.reconfigure(langExt(lang)) })
  })
</script>

<div class="editor" bind:this={host}></div>

<style>
  .editor {
    height: 100%;
    overflow: hidden;
  }
  .editor :global(.cm-editor) {
    height: 100%;
  }
</style>
