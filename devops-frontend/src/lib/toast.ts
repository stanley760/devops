import { toast as sonnerToast } from 'sonner'

export function toast(msg: string): void {
  sonnerToast(msg)
}

export function toastSuccess(msg: string): void {
  sonnerToast.success(msg)
}

export function toastError(msg: string): void {
  sonnerToast.error(msg)
}
