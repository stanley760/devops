/**
 * 确认框：全局单例，ConfirmHost.vue 消费。
 */
import { reactive } from 'vue'

interface ConfirmRequest {
  open: boolean
  title: string
  text: string
  danger: boolean
  confirmLabel: string
  resolve: ((ok: boolean) => void) | null
}

const state = reactive<ConfirmRequest>({
  open: false,
  title: '',
  text: '',
  danger: false,
  confirmLabel: '确定',
  resolve: null,
})

export function useConfirmState() {
  return state
}

export function confirmAction(opts: {
  title: string
  text?: string
  danger?: boolean
  confirmLabel?: string
}): Promise<boolean> {
  return new Promise((resolve) => {
    state.title = opts.title
    state.text = opts.text ?? '此操作不可撤销，确认继续？'
    state.danger = opts.danger ?? false
    state.confirmLabel = opts.confirmLabel ?? '确定'
    state.resolve = resolve
    state.open = true
  })
}

export function settleConfirm(ok: boolean): void {
  const resolve = state.resolve
  state.resolve = null
  state.open = false
  resolve?.(ok)
}
