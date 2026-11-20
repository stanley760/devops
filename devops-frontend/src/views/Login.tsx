import { useState, type FormEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { adminAuth } from '@/api/admin/auth'
import { session } from '@/lib/session'
import { mapLoginError } from '@/lib/errors'
import { resolveTenantEntry } from '@/lib/tenant'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

export default function Login() {
  const [search] = useSearchParams()
  const navigate = useNavigate()
  const entry = resolveTenantEntry(window.location.search, window.location.pathname, window.location.hostname)
  const tenantCode = search.get('tenantCode') ?? entry.code
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const redirect = search.get('redirect') || '/'

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      const result = await adminAuth.login({ username, password, tenant_code: tenantCode })
      session.saveSession(result, result.tenant)
      navigate(redirect, { replace: true })
    } catch (err) {
      setError(mapLoginError((err as Error).message))
    } finally {
      setBusy(false)
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
            <Field>
              <FieldLabel>账号</FieldLabel>
              <Input value={username} onChange={(e) => setUsername(e.target.value)} required />
            </Field>
            <Field>
              <FieldLabel>密码</FieldLabel>
              <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
            </Field>
            <Button type="submit" className="w-full" disabled={busy}>登录</Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
