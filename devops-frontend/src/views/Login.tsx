import { useCallback, useState, type FormEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { adminAuth } from '@/api/admin/auth'
import { session } from '@/lib/session'
import { buildCaptchaAnswer, captchaEnabled } from '@/lib/captcha'
import { mapLoginError } from '@/lib/errors'
import { resolveTenantEntry } from '@/lib/tenant'
import { toastError } from '@/lib/toast'
import { useSession } from '@/store/session'
import { CaptchaDialog } from '@/components/business/CaptchaDialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

export default function Login() {
  const [search] = useSearchParams()
  const navigate = useNavigate()
  const sessionStore = useSession(true)
  const entry = resolveTenantEntry(window.location.search, window.location.pathname, window.location.hostname)
  const tenantCode = search.get('tenantCode') ?? entry.code
  // 前三项命中即锁定：不渲染租户输入，仅账号 + 密码
  const locked = entry.locked || !!search.get('tenantCode')
  const [manualTenant, setManualTenant] = useState(tenantCode)

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [challenge, setChallenge] = useState<CaptchaChallenge | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)

  const redirect = search.get('redirect') || '/'
  const effectiveTenant = locked ? tenantCode : manualTenant || 'default'

  const doLogin = useCallback(
    async (answer?: CaptchaAnswer) => {
      setError(null)
      setBusy(true)
      try {
        const body: Record<string, unknown> = {
          username,
          password,
          tenant_code: effectiveTenant,
        }
        if (challenge && answer) {
          Object.assign(body, buildCaptchaAnswer(challenge, answer))
        }
        const result = await adminAuth.login(body as { username: string; password: string; tenant_code: string })
        session.saveSession(result, result.tenant)
        await sessionStore?.reload()
        session.setLastUser(username)
        navigate(redirect, { replace: true })
      } catch (err) {
        const msg = (err as Error).message
        // 400 = 参数/验证码错误：留在弹框内提示并自动换题（挑战一次性消费）
        if (challenge && answer && (err as { code?: number }).code === 400) {
          toastError('验证码校验失败，已为你更换新题')
          const next = await adminAuth.getCaptcha(effectiveTenant)
          setChallenge(next)
          setDialogOpen(true)
        } else {
          // 其他失败（凭据错误、限流等）关闭弹框，错误显示在登录卡
          setDialogOpen(false)
          setChallenge(null)
          setError(mapLoginError(msg))
        }
      } finally {
        setBusy(false)
      }
    },
    [username, password, effectiveTenant, challenge, redirect, navigate, sessionStore],
  )

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    try {
      const c = await adminAuth.getCaptcha(effectiveTenant)
      if (captchaEnabled(c)) {
        setChallenge(c)
        setDialogOpen(true)
        return
      }
      await doLogin()
    } catch (err) {
      setError(mapLoginError((err as Error).message))
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>登录 DevOps 管理平台</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} className="space-y-4">
            {error && <p className="text-destructive text-sm">{error}</p>}
            {!locked && (
              <Field>
                <FieldLabel htmlFor="login-tenant">租户编码</FieldLabel>
                <Input id="login-tenant" value={manualTenant} onChange={(e) => setManualTenant(e.target.value)} />
              </Field>
            )}
            <Field>
              <FieldLabel htmlFor="login-username">账号</FieldLabel>
              <Input id="login-username" value={username} onChange={(e) => setUsername(e.target.value)} required />
            </Field>
            <Field>
              <FieldLabel htmlFor="login-password">密码</FieldLabel>
              <Input id="login-password" type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
            </Field>
            <Button type="submit" className="w-full" disabled={busy}>登录</Button>
          </form>
        </CardContent>
      </Card>
      <CaptchaDialog
        open={dialogOpen}
        challenge={challenge}
        onAnswer={(a) => void doLogin(a)}
        onRefresh={() => {
          void adminAuth.getCaptcha(effectiveTenant).then(setChallenge)
        }}
        onCancel={() => {
          setDialogOpen(false)
          setChallenge(null)
        }}
      />
    </div>
  )
}
