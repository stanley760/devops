import { useEffect, useState, type FormEvent } from 'react'
import { portalAuth } from '@/api/portal/auth'
import { PortalApiError } from '@/lib/portal-session'
import { toastError, toastSuccess } from '@/lib/toast'
import { formatDateTime } from '@/lib/datetime'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import type { PublicProfile } from '@/lib/types'

interface Props {
  navigate: (to: string) => void
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const MOBILE_RE = /^1\d{10}$/
const GENDER_TEXT: Record<string, string> = { '0': '未知', '1': '男', '2': '女' }

export function ProfilePage({ navigate }: Props) {
  const [profile, setProfile] = useState<PublicProfile | null>(null)
  const [editForm, setEditForm] = useState({ nickname: '', email: '', mobile: '', gender: 0 })
  const [editError, setEditError] = useState<string | null>(null)
  const [editSaved, setEditSaved] = useState(false)
  const [pwdForm, setPwdForm] = useState({ old_password: '', new_password: '', confirm: '' })
  const [pwdError, setPwdError] = useState<string | null>(null)
  const [pwdSaved, setPwdSaved] = useState(false)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    portalAuth
      .getProfile()
      .then((p) => {
        setProfile(p)
        setEditForm({ nickname: p.nickname, email: p.email, mobile: p.mobile, gender: p.gender ?? 0 })
      })
      .catch((e) => {
        if (e instanceof PortalApiError && e.status === 401) {
          toastError('登录已过期，请重新登录')
          navigate('/login')
          return
        }
        toastError((e as Error).message)
      })
  }, [navigate])

  const submitProfile = async (e: FormEvent) => {
    e.preventDefault()
    setEditError(null)
    if (!editForm.nickname) {
      setEditError('昵称必填')
      return
    }
    if (editForm.email && !EMAIL_RE.test(editForm.email)) {
      setEditError('邮箱格式不正确')
      return
    }
    if (editForm.mobile && !MOBILE_RE.test(editForm.mobile)) {
      setEditError('手机号格式不正确')
      return
    }
    setBusy(true)
    try {
      await portalAuth.updateProfile(editForm)
      toastSuccess('已保存')
      setProfile(await portalAuth.getProfile())
      setEditSaved(true)
    } catch (err) {
      setEditSaved(false)
      setEditError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const submitPassword = async (e: FormEvent) => {
    e.preventDefault()
    setPwdError(null)
    if (pwdForm.new_password !== pwdForm.confirm) {
      setPwdSaved(false)
      setPwdError('两次输入的新密码不一致')
      return
    }
    setBusy(true)
    try {
      await portalAuth.updatePassword({ old_password: pwdForm.old_password, new_password: pwdForm.new_password })
      toastSuccess('密码已更新')
      setPwdForm({ old_password: '', new_password: '', confirm: '' })
      setPwdSaved(true)
    } catch (err) {
      setPwdSaved(false)
      setPwdError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  if (!profile) return <Skeleton className="glass-panel mx-auto my-10 h-64 w-full max-w-md" />

  const genderText = GENDER_TEXT[String(profile.gender)] ?? '未知'

  return (
    <div className="mx-auto my-10 max-w-md space-y-4">
      <Card className="glass-panel">
        <CardHeader><CardTitle>个人资料</CardTitle></CardHeader>
        <CardContent className="space-y-1 text-sm">
          <div>用户名：{profile.username}</div>
          <div>昵称：{profile.nickname}</div>
          <div>邮箱：{profile.email || '未绑定'}</div>
          <div>手机号：{profile.mobile || '未绑定'}</div>
          <div>性别：{genderText}</div>
          <div>注册时间：{formatDateTime(profile.created_at)}</div>
        </CardContent>
      </Card>
      <Card className="glass-panel">
        <CardHeader><CardTitle>编辑资料</CardTitle></CardHeader>
        <CardContent>
          <form onSubmit={submitProfile} className="space-y-4">
            {editError && <p className="text-destructive text-sm">{editError}</p>}
            {editSaved && <p className="text-sm text-emerald-600">已保存</p>}
            <Field><FieldLabel htmlFor="portal-profile-nickname">昵称</FieldLabel><Input id="portal-profile-nickname" value={editForm.nickname} onChange={(e) => { setEditForm({ ...editForm, nickname: e.target.value }); setEditSaved(false) }} required /></Field>
            <Field><FieldLabel htmlFor="portal-profile-email">邮箱</FieldLabel><Input id="portal-profile-email" type="email" value={editForm.email} onChange={(e) => { setEditForm({ ...editForm, email: e.target.value }); setEditSaved(false) }} /></Field>
            <Field><FieldLabel htmlFor="portal-profile-mobile">手机号</FieldLabel><Input id="portal-profile-mobile" value={editForm.mobile} onChange={(e) => { setEditForm({ ...editForm, mobile: e.target.value }); setEditSaved(false) }} /></Field>
            <Field>
              <FieldLabel htmlFor="portal-profile-gender">性别</FieldLabel>
              <Select value={String(editForm.gender)} onValueChange={(v) => { setEditForm({ ...editForm, gender: Number(v) }); setEditSaved(false) }}>
                <SelectTrigger id="portal-profile-gender" className="glass-control"><SelectValue /></SelectTrigger>
                <SelectContent className="glass-panel">
                  <SelectItem value="1">男</SelectItem>
                  <SelectItem value="2">女</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Button type="submit" className="glass-control" disabled={busy}>保存资料</Button>
          </form>
        </CardContent>
      </Card>
      <Separator />
      <Card className="glass-panel">
        <CardHeader><CardTitle>修改密码</CardTitle></CardHeader>
        <CardContent>
          <form onSubmit={submitPassword} className="space-y-4">
            {pwdError && <p className="text-destructive text-sm">{pwdError}</p>}
            {pwdSaved && <p className="text-sm text-emerald-600">密码已更新</p>}
            <Field><FieldLabel htmlFor="portal-profile-old-password">原密码</FieldLabel><Input id="portal-profile-old-password" type="password" value={pwdForm.old_password} onChange={(e) => { setPwdForm({ ...pwdForm, old_password: e.target.value }); setPwdSaved(false) }} required /></Field>
            <Field><FieldLabel htmlFor="portal-profile-new-password">新密码</FieldLabel><Input id="portal-profile-new-password" type="password" value={pwdForm.new_password} onChange={(e) => { setPwdForm({ ...pwdForm, new_password: e.target.value }); setPwdSaved(false) }} required /></Field>
            <Field><FieldLabel htmlFor="portal-profile-confirm-password">确认新密码</FieldLabel><Input id="portal-profile-confirm-password" type="password" value={pwdForm.confirm} onChange={(e) => { setPwdForm({ ...pwdForm, confirm: e.target.value }); setPwdSaved(false) }} required /></Field>
            <Button type="submit" className="glass-control" disabled={busy}>修改密码</Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
