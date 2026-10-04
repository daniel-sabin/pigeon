import type { KeyValue } from './types'

function decode(s: string): string {
  try {
    return decodeURIComponent(s.replace(/\+/g, ' '))
  } catch {
    return s
  }
}

// Only escape what would break re-parsing; the backend does the real encoding.
const encode = (s: string) => s.replace(/%/g, '%25').replace(/&/g, '%26').replace(/#/g, '%23').replace(/\+/g, '%2B')

export function splitUrl(url: string): { base: string; query: string } {
  const noHash = url.split('#')[0]
  const i = noHash.indexOf('?')
  return i < 0 ? { base: noHash, query: '' } : { base: noHash.slice(0, i), query: noHash.slice(i + 1) }
}

/** Params from the URL's query string, followed by the existing disabled ones. */
export function paramsFromUrl(url: string, current: KeyValue[]): KeyValue[] {
  const { query } = splitUrl(url)
  const parsed = query
    .split('&')
    .filter((p) => p !== '')
    .map((p) => {
      const i = p.indexOf('=')
      return i < 0
        ? { key: decode(p), value: '', enabled: true }
        : { key: decode(p.slice(0, i)), value: decode(p.slice(i + 1)), enabled: true }
    })
  return [...parsed, ...current.filter((p) => !p.enabled && (p.key || p.value))]
}

export function urlWithParams(url: string, params: KeyValue[]): string {
  const { base } = splitUrl(url)
  const query = params
    .filter((p) => p.enabled && p.key !== '')
    .map((p) => `${encode(p.key).replace(/=/g, '%3D')}=${encode(p.value)}`)
    .join('&')
  return query ? `${base}?${query}` : base
}
