import { useState, type FormEvent } from 'react'
import { portalAuth } from '@/api/portal/auth'
import { portalSession } from '@/lib/portal-session'
import { buildCaptchaAnswer, captchaEnabled } from '@/lib/captcha'
import { toastError } from '@/lib/toast'
import { PortalCaptchaDialog } from '@/portal/components/PortalCaptchaDialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

interface Props {
  navigate: (to: string) => void
  onRegistered: () => void
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function PortalRegisterPage({ navigate, onRegistered }: Props) {
  const [form, setForm] = useState({ username: '', nickname: '', email: '', password: '', confirm: '' })
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [challenge, setChallenge] = useState<CaptchaChallenge | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [payload, setPayload] = useState<Record<string, unknown> | null>(null)

  const requestChallenge = async (): Promise<CaptchaChallenge> => {
    const next = await portalAuth.getCaptcha()
    setChallenge(next)
    return next
  }

  const finish = async (answer?: CaptchaAnswer) => {
    if (!payload) return
    const currentChallenge = challenge
    const body: Record<string, unknown> = { ...payload }
    if (currentChallenge && answer) Object.assign(body, buildCaptchaAnswer(currentChallenge, answer))
    setBusy(true)
    try {
      const tokens = await portalAuth.register(body as { username: string; nickname: string; email: string; password: string })
      portalSession.saveTokens(tokens)
      onRegistered()
      navigate('/profile')
    } catch (err) {
      const status = (err as { status?: number }).status
      if (currentChallenge && answer && status === 400) {
        // 验证码校验失败：弹框内提示并自动换题，旧挑战的作答字段不能复用。
        toastError('验证码校验失败，已为你更换新题')
        setChallenge(null)
        await requestChallenge()
        setDialogOpen(true)
      } else {
        setDialogOpen(false)
        setChallenge(null)
        setPayload(null)
        setError((err as Error).message)
      }
    } finally {
      setBusy(false)
    }
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    if (form.password !== form.confirm) {
      setError('两次输入的密码不一致')
      return
    }
    if (!EMAIL_RE.test(form.email)) {
      setError('邮箱格式不正确')
      return
    }
    const base: Record<string, unknown> = {
      username: form.username, nickname: form.nickname, email: form.email, password: form.password,
    }
    setBusy(true)
    try {
      const c = await requestChallenge()
      if (captchaEnabled(c)) {
        setPayload(base)
        setDialogOpen(true)
        return
      }
      const tokens = await portalAuth.register(base as { username: string; nickname: string; email: string; password: string })
      portalSession.saveTokens(tokens)
      onRegistered()
      navigate('/profile')
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card className="glass-panel mx-auto my-10 max-w-sm rounded-xl">
      <CardHeader><CardTitle>Sign Up</CardTitle></CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4">
          {error && <p className="text-destructive text-sm">{error}</p>}
          <Field><FieldLabel htmlFor="portal-register-username">用户名</FieldLabel><Input id="portal-register-username" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} required /></Field>
          <Field><FieldLabel htmlFor="portal-register-nickname">昵称</FieldLabel><Input id="portal-register-nickname" value={form.nickname} onChange={(e) => setForm({ ...form, nickname: e.target.value })} required /></Field>
          <Field><FieldLabel htmlFor="portal-register-email">邮箱</FieldLabel><Input id="portal-register-email" type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required /></Field>
          <Field><FieldLabel htmlFor="portal-register-password">密码</FieldLabel><Input id="portal-register-password" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required /></Field>
          <Field><FieldLabel htmlFor="portal-register-confirm">确认密码</FieldLabel><Input id="portal-register-confirm" type="password" value={form.confirm} onChange={(e) => setForm({ ...form, confirm: e.target.value })} required /></Field>
          <Button type="submit" className="glass-control w-full" disabled={busy}>立即注册</Button>
        </form>
      </CardContent>
      <PortalCaptchaDialog
        open={dialogOpen} challenge={challenge}
        onAnswer={(a) => void finish(a)}
        onRefresh={() => void requestChallenge()}
        onCancel={() => {
          setDialogOpen(false)
          setChallenge(null)
          setPayload(null)
        }}
      />
    </Card>
  )
}
