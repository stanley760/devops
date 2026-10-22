/**
 * 轻提示：全局单例队列，ToastHost.vue 消费。
 */
import { reactive } from 'vue'

export type ToastKind = 'ok' | 'info' | 'warn' | 'crit'

export interface ToastItem {
  id: number
  kind: ToastKind
  text: string
}

const state = reactive<{ items: ToastItem[] }>({ items: [] })
let seq = 0

export function useToastState() {
  return state
}

export function pushToast(text: string, kind: ToastKind = 'ok', duration = 2600): void {
  const id = ++seq
  state.items.push({ id, kind, text })
  window.setTimeout(() => removeToast(id), duration)
}

export function removeToast(id: number): void {
  const i = state.items.findIndex((t) => t.id === id)
  if (i >= 0) state.items.splice(i, 1)
}

export function toastOk(text: string): void {
  pushToast(text, 'ok')
}

export function toastError(error: unknown, fallback = '操作失败'): void {
  const msg = error instanceof Error ? error.message : fallback
  pushToast(msg || fallback, 'crit', 3400)
}
