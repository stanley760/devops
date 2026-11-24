import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { PortalLoginPage } from '@/portal/pages/Login'
import { PortalRegisterPage } from '@/portal/pages/Register'
import { ProfilePage } from '@/portal/pages/Profile'
import { portalAuth } from '@/api/portal/auth'
import { portalSession, PortalApiError } from '@/lib/portal-session'

vi.mock('@/api/portal/auth', () => ({
  portalAuth: {
    getCaptcha: vi.fn(),
    login: vi.fn(),
    register: vi.fn().mockResolvedValue({ access_token: 'a', refresh_token: 'r', expires_in: 1, issued_at: Date.now() }),
    getProfile: vi.fn(),
    updateProfile: vi.fn().mockResolvedValue(undefined),
    updatePassword: vi.fn().mockResolvedValue(undefined),
    logout: vi.fn().mockResolvedValue(undefined),
  },
}))

const navigate = vi.fn()
const noop = () => {}

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(portalAuth.getCaptcha).mockResolvedValue({
    id: 'c', type: 'slide', image: 'i', width: 100, height: 50, enabled: false,
  })
})

describe('Portal Login', () => {
  it('renders Sign In and skips captcha when disabled', async () => {
    vi.mocked(portalAuth.login).mockResolvedValue({ access_token: 'a', refresh_token: 'r', expires_in: 1, issued_at: Date.now() })
    render(<PortalLoginPage navigate={navigate} onLoggedIn={noop} />)
    expect(screen.getAllByText('Sign In').length).toBeGreaterThan(0)
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))
    await waitFor(() => expect(portalSession.isLoggedIn()).toBe(true))
    expect(portalAuth.login).toHaveBeenCalledWith({ username: 'u', password: 'p' })
    expect(navigate).toHaveBeenCalledWith('/profile')
  })

  it('opens captcha dialog and submits a built answer when enabled', async () => {
    vi.mocked(portalAuth.getCaptcha).mockResolvedValue({
      id: 'c1', type: 'click', image: 'i', width: 100, height: 50, text: 'star', enabled: true,
    })
    vi.mocked(portalAuth.login).mockResolvedValue({ access_token: 'a', refresh_token: 'r', expires_in: 1, issued_at: Date.now() })
    render(<PortalLoginPage navigate={navigate} onLoggedIn={noop} />)
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))
    await screen.findByText('安全验证')
    fireEvent.click(screen.getByAltText('captcha'))
    fireEvent.click(screen.getByText('提交'))
    await waitFor(() => expect(portalAuth.login).toHaveBeenCalledWith(
      expect.objectContaining({ captcha_id: 'c1', captcha_points: [expect.any(Object)] }),
    ))
  })
})

describe('Portal Register', () => {
  it('rejects mismatched passwords with spec message', async () => {
    render(<PortalRegisterPage navigate={navigate} onRegistered={noop} />)
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'a' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'b' } })
    fireEvent.submit(screen.getByRole('button', { name: '立即注册' }).closest('form')!)
    await waitFor(() => expect(screen.getByText('两次输入的密码不一致')).toBeInTheDocument())
    expect(portalAuth.register).not.toHaveBeenCalled()
  })

  it('registers and logs in immediately', async () => {
    render(<PortalRegisterPage navigate={navigate} onRegistered={noop} />)
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('昵称'), { target: { value: 'n' } })
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: 'a@b.co' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByText('立即注册'))
    await waitFor(() => expect(portalAuth.register).toHaveBeenCalled())
    await waitFor(() => expect(portalSession.isLoggedIn()).toBe(true))
    expect(navigate).toHaveBeenCalledWith('/profile')
  })

  it('rejects an invalid email before calling register', async () => {
    render(<PortalRegisterPage navigate={navigate} onRegistered={noop} />)
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('昵称'), { target: { value: 'n' } })
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: 'not-an-email' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'p' } })
    fireEvent.submit(screen.getByRole('button', { name: '立即注册' }).closest('form')!)
    await waitFor(() => expect(screen.getByText('邮箱格式不正确')).toBeInTheDocument())
    expect(portalAuth.register).not.toHaveBeenCalled()
  })
})

describe('Portal Profile', () => {
  const profile = {
    id: 42, username: 'u', nickname: '昵称', email: 'a@b.co', mobile: '13800138000',
    gender: '1', avatar: '', created_at: 1730000000000,
  }

  it('loads profile with numeric id preserved and shows created_at', async () => {
    vi.mocked(portalAuth.getProfile).mockResolvedValue(profile)
    render(<ProfilePage navigate={navigate} onLoggedOut={noop} />)
    await screen.findByText('昵称')
    expect(screen.getByText(/注册时间/)).toBeInTheDocument()
    expect(portalAuth.getProfile).toHaveBeenCalled()
    const result = (await vi.mocked(portalAuth.getProfile).mock.results[0]!.value) as { id: number }
    expect(result.id).toBe(42)
  })

  it('redirects to login on 401', async () => {
    vi.mocked(portalAuth.getProfile).mockRejectedValue(new PortalApiError(401, '登录已过期，请重新登录'))
    render(<ProfilePage navigate={navigate} onLoggedOut={noop} />)
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/login'))
  })

  it('updates profile', async () => {
    vi.mocked(portalAuth.getProfile).mockResolvedValue(profile)
    render(<ProfilePage navigate={navigate} onLoggedOut={noop} />)
    await screen.findByText('昵称')
    fireEvent.click(screen.getByText('保存资料'))
    await waitFor(() =>
      expect(portalAuth.updateProfile).toHaveBeenCalledWith(expect.objectContaining({ nickname: '昵称' })),
    )
    await waitFor(() => expect(screen.getByText('已保存')).toBeInTheDocument())
  })

  it('validates profile edits', async () => {
    vi.mocked(portalAuth.getProfile).mockResolvedValue(profile)
    render(<ProfilePage navigate={navigate} onLoggedOut={noop} />)
    await screen.findByText('昵称')
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: 'bad' } })
    fireEvent.submit(screen.getByRole('button', { name: '保存资料' }).closest('form')!)
    await waitFor(() => expect(screen.getByText('邮箱格式不正确')).toBeInTheDocument())
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: '' } })
    fireEvent.change(screen.getByLabelText('手机号'), { target: { value: '123' } })
    fireEvent.submit(screen.getByRole('button', { name: '保存资料' }).closest('form')!)
    await waitFor(() => expect(screen.getByText('手机号格式不正确')).toBeInTheDocument())
    expect(portalAuth.updateProfile).not.toHaveBeenCalled()
  })

  it('rejects mismatched password confirmation', async () => {
    vi.mocked(portalAuth.getProfile).mockResolvedValue(profile)
    render(<ProfilePage navigate={navigate} onLoggedOut={noop} />)
    await screen.findByText('昵称')
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'a' } })
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'b' } })
    fireEvent.submit(screen.getByRole('button', { name: '修改密码' }).closest('form')!)
    await waitFor(() => expect(screen.getByText('两次输入的密码不一致')).toBeInTheDocument())
    expect(portalAuth.updatePassword).not.toHaveBeenCalled()
  })

  it('updates the password and clears the form', async () => {
    vi.mocked(portalAuth.getProfile).mockResolvedValue(profile)
    render(<ProfilePage navigate={navigate} onLoggedOut={noop} />)
    await screen.findByText('昵称')
    fireEvent.change(screen.getByLabelText('原密码'), { target: { value: 'old' } })
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'new' } })
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'new' } })
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }))
    await waitFor(() => expect(portalAuth.updatePassword).toHaveBeenCalledWith({ old_password: 'old', new_password: 'new' }))
    await waitFor(() => expect(screen.getByText('密码已更新')).toBeInTheDocument())
    expect(screen.getByLabelText('新密码')).toHaveValue('')
  })

  it('logs out locally even when the API call fails', async () => {
    vi.mocked(portalAuth.getProfile).mockResolvedValue(profile)
    vi.mocked(portalAuth.logout).mockRejectedValueOnce(new Error('offline'))
    portalSession.saveTokens({ access_token: 'a', refresh_token: 'r', expires_in: 1, issued_at: Date.now() })
    const onLoggedOut = vi.fn()
    render(<ProfilePage navigate={navigate} onLoggedOut={onLoggedOut} />)
    await screen.findByText('昵称')
    fireEvent.click(screen.getByText('退出'))
    await waitFor(() => expect(portalSession.isLoggedIn()).toBe(false))
    expect(onLoggedOut).toHaveBeenCalled()
    expect(navigate).toHaveBeenCalledWith('/')
  })
})
