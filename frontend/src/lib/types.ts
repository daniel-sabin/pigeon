export interface KeyValue {
  key: string
  value: string
  enabled: boolean
}

export type BodyType = 'none' | 'json' | 'text' | 'xml' | 'form'
export type AuthType = 'none' | 'bearer' | 'basic'

export interface Request {
  id: string
  name: string
  method: string
  url: string
  params: KeyValue[]
  headers: KeyValue[]
  body: { type: BodyType; raw: string; form: KeyValue[] }
  auth: { type: AuthType; token: string; username: string; password: string }
}

export interface Response {
  status: number
  statusText: string
  proto: string
  headers: KeyValue[]
  contentType: string
  body: string
  bodyPretty: string
  bodyBase64: string
  isBinary: boolean
  truncated: boolean
  size: number
  durationMs: number
  url: string
  error: string
}

export interface Collection {
  id: string
  name: string
  requests: Request[]
}

export interface HistoryEntry {
  id: string
  request: Request
  status: number
  error: string
  durationMs: number
  timestamp: number
}

/** Where the request being edited was opened from, so Save can overwrite it. */
export interface Origin {
  collectionId: string
  requestId: string
}

export const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS']

export function uid(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8)
}

export function newRequest(): Request {
  return {
    id: uid(),
    name: '',
    method: 'GET',
    url: '',
    params: [],
    headers: [],
    body: { type: 'none', raw: '', form: [] },
    auth: { type: 'none', token: '', username: '', password: '' },
  }
}

const isBlank = (kv: KeyValue) => kv.key === '' && kv.value === ''

/** Deep copy without the empty trailing rows the editors add. */
export function cleanRequest(r: Request): Request {
  const c: Request = JSON.parse(JSON.stringify(r))
  c.params = c.params.filter((kv) => !isBlank(kv))
  c.headers = c.headers.filter((kv) => !isBlank(kv))
  c.body.form = c.body.form.filter((kv) => !isBlank(kv))
  return c
}

/** Fills fields that may be missing from older saved data. */
export function normalizeRequest(r: Partial<Request>): Request {
  const base = newRequest()
  return {
    ...base,
    ...r,
    params: r.params ?? [],
    headers: r.headers ?? [],
    body: { ...base.body, ...r.body, form: r.body?.form ?? [] },
    auth: { ...base.auth, ...r.auth },
  }
}
