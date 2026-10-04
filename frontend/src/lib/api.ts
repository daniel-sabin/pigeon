import * as App from '../../wailsjs/go/main/App'
import type { Collection, HistoryEntry, Request, Response } from './types'

// The generated bindings type their arguments as classes; our plain objects
// serialize identically, so we cast at this boundary only.
export const sendRequest = (runId: string, req: Request) =>
  App.SendRequest(runId, req as any) as Promise<Response>
export const cancelRequest = (runId: string) => App.CancelRequest(runId)
export const getCollections = async () => ((await App.GetCollections()) ?? []) as Collection[]
export const saveCollections = (cols: Collection[]) => App.SaveCollections(cols as any)
export const getHistory = async () => ((await App.GetHistory()) ?? []) as HistoryEntry[]
export const clearHistory = () => App.ClearHistory()
