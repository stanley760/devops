import { useState, type FormEvent } from 'react'
import { adminAuth } from '@/api/admin/auth'
import { toastError, toastSuccess } from '@/lib/toast'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

export function PasswordDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [oldPassword, setOld] = useState('')
  const [newPassword, setNew] = useState('')
  const [confirm, setConfirm] = useState('')

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirm) {
      toastError('两次输入的密码不一致')
      return
    }
    try {
      await adminAuth.updatePassword({ old_password: oldPassword, new_password: newPassword })
      toastSuccess('密码修改成功')
      onOpenChange(false)
      setOld('')
      setNew('')
      setConfirm('')
    } catch (err) {
      toastError((err as Error).message)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-sm">
        <DialogHeader><DialogTitle>修改密码</DialogTitle></DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4">
          <Field><FieldLabel>原密码</FieldLabel><Input type="password" value={oldPassword} onChange={(e) => setOld(e.target.value)} required /></Field>
          <Field><FieldLabel>新密码</FieldLabel><Input type="password" value={newPassword} onChange={(e) => setNew(e.target.value)} required /></Field>
          <Field><FieldLabel>确认新密码</FieldLabel><Input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} required /></Field>
          <Button type="submit" className="w-full">提交</Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
