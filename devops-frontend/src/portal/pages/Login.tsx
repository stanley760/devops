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
  onLoggedIn: () => void
}

export function PortalLoginPage({ navigate, onLoggedIn }: Props) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [challenge, setChallenge] = useState<CaptchaChallenge | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)

  const requestChallenge = async (): Promise<CaptchaChallenge> => {
    const next = await portalAuth.getCaptcha()
    setChallenge(next)
    return next
  }

  const doLogin = async (answer?: CaptchaAnswer) => {
    setError(null)
    setBusy(true)
    try {
      const currentChallenge = challenge
      const body: Record<string, unknown> = { username, password }
      if (currentChallenge && answer) Object.assign(body, buildCaptchaAnswer(currentChallenge, answer))
      const tokens = await portalAuth.login(body as { username: string; password: string })
      portalSession.saveTokens(tokens)
      onLoggedIn()
      navigate('/profile')
    } catch (err) {
      const code = (err as { status?: number }).status
      if (challenge && answer && code === 400) {
        // 验证码校验失败：弹框内提示并自动换题；旧挑战作答不再复用。
        toastError('验证码校验失败，已为你更换新题')
        setChallenge(null)
        await requestChallenge()
        setDialogOpen(true)
      } else {
        setDialogOpen(false)
        setChallenge(null)
        setError((err as Error).message)
      }
    } finally {
      setBusy(false)
    }
  }

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    try {
      const c = await requestChallenge()
      if (captchaEnabled(c)) {
        setDialogOpen(true)
        return
      }
      await doLogin()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <Card className="glass-panel mx-auto my-10 max-w-sm rounded-xl">
      <CardHeader><CardTitle>Sign In</CardTitle></CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} className="space-y-4">
          {error && <p className="text-destructive text-sm">{error}</p>}
          <Field><FieldLabel htmlFor="portal-login-username">用户名</FieldLabel><Input id="portal-login-username" value={username} onChange={(e) => setUsername(e.target.value)} required /></Field>
          <Field><FieldLabel htmlFor="portal-login-password">密码</FieldLabel><Input id="portal-login-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required /></Field>
          <Button type="submit" className="glass-control w-full" disabled={busy}>Sign In</Button>
        </form>
      </CardContent>
      <PortalCaptchaDialog
        open={dialogOpen} challenge={challenge}
        onAnswer={(a) => void doLogin(a)}
        onRefresh={() => void requestChallenge()}
        onCancel={() => { setDialogOpen(false); setChallenge(null) }}
      />
    </Card>
  )
}
