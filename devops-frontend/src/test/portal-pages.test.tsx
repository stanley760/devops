import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { PortalLoginPage } from '@/portal/pages/Login'
import { PortalRegisterPage } from '@/portal/pages/Register'
import { ProfilePage } from '@/portal/pages/Profile'
import { portalAuth } from '@/api/portal/auth'
import { portalSession, PortalApiError } from '@/lib/portal-session'
import { toastError, toastSuccess } from '@/lib/toast'

vi.mock('@/lib/toast', () => ({
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}))

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
  vi.mocked(portalAuth.getProfile).mockResolvedValue({ ...profileFixture })
  vi.mocked(portalAuth.updateProfile).mockResolvedValue(undefined)
  vi.mocked(portalAuth.updatePassword).mockResolvedValue(undefined)
  vi.mocked(portalAuth.logout).mockResolvedValue(undefined)
  vi.mocked(portalAuth.login).mockResolvedValue(tokens())
  vi.mocked(portalAuth.register).mockResolvedValue(tokens())
})

const tokens = () => ({ access_token: 'a', refresh_token: 'r', expires_in: 1, issued_at: Date.now() })

const profileFixture = {
  id: 42, username: 'legacy-user', nickname: '昵称', email: 'a@b.co', mobile: '13800138000',
  gender: 1, avatar: '', created_at: 1730000000000,
}

type CaptchaInteraction = {
  points?: ReadonlyArray<{ x: number; y: number }>
  x?: number
  y?: number
  angle?: number
}

async function submitCaptchaAnswer(answer: CaptchaInteraction) {
  const challengeType = screen.queryByLabelText('旋转角度', { selector: 'input' })
    ? 'rotate'
    : screen.queryByAltText('puzzle piece')
      ? 'drag'
      : 'click'
  if (challengeType === 'click') {
    fireEvent.click(screen.getByAltText('captcha'))
  } else if (challengeType === 'rotate') {
    fireEvent.change(screen.getByLabelText('旋转角度'), { target: { value: String(answer.angle ?? 0) } })
  } else {
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 0, clientY: 0 })
    fireEvent.mouseMove(window, { clientX: answer.x ?? 0, clientY: answer.y ?? 0 })
    fireEvent.mouseUp(window)
  }
  fireEvent.click(screen.getByText('提交'))
}

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
    render(<PortalLoginPage navigate={navigate} onLoggedIn={noop} />)
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))
    await screen.findByText('安全验证')
    await submitCaptchaAnswer({ points: [{ x: 1, y: 2 }] })
    await waitFor(() => expect(portalAuth.login).toHaveBeenCalledWith(
      expect.objectContaining({ captcha_id: 'c1', captcha_points: [expect.any(Object)] }),
    ))
  })

  it.each([
    ['click', { points: [] }, { captcha_id: 'c2', captcha_points: [expect.any(Object)] }, ['captcha_x', 'captcha_y', 'captcha_angle']],
    ['slide', { x: 7, y: 0 }, { captcha_id: 'c2', captcha_x: 7, captcha_y: 0 }, ['captcha_points', 'captcha_angle']],
    ['dragdrop', { x: 5, y: 6 }, { captcha_id: 'c2', captcha_x: 5, captcha_y: 6 }, ['captcha_points', 'captcha_angle']],
    ['rotate', { angle: 90 }, { captcha_id: 'c2', captcha_angle: 90 }, ['captcha_points', 'captcha_x', 'captcha_y']],
  ] as const)('maps %s login answer to the current challenge without stale fields', async (type, answer, expected, forbidden) => {
    vi.mocked(portalAuth.getCaptcha)
      .mockResolvedValueOnce({ id: 'old', type: 'click', image: 'i', width: 100, height: 50, enabled: true })
      .mockResolvedValue({
        id: 'c2', type, image: 'i', width: 100, height: 50, thumb: 't', text: 'star', enabled: true,
      })
    vi.mocked(portalAuth.login)
      .mockRejectedValueOnce(Object.assign(new Error('bad captcha'), { status: 400 }))
      .mockResolvedValue(tokens())
    render(<PortalLoginPage navigate={navigate} onLoggedIn={noop} />)
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))
    await screen.findByText('安全验证')
    await submitCaptchaAnswer({ points: [{ x: 1, y: 1 }] })
    await waitFor(() => expect(toastError).toHaveBeenCalledWith('验证码校验失败，已为你更换新题'))
    await screen.findByText(type === 'click' ? '请依次点击：star' : type === 'slide' ? '拖动滑块完成拼图' : type === 'dragdrop' ? '将拼图块拖到正确位置' : '拖动或用滑杆旋转图片至正向')
    await submitCaptchaAnswer(answer)
    await waitFor(() => expect(portalAuth.login).toHaveBeenCalledTimes(2))
    const second = vi.mocked(portalAuth.login).mock.calls[1]![0]
    expect(second).toMatchObject(expected)
    for (const field of forbidden) expect(second).not.toHaveProperty(field)
  })

  it('clears captcha state when refresh is requested', async () => {
    vi.mocked(portalAuth.getCaptcha).mockResolvedValue({
      id: 'refreshed', type: 'rotate', image: 'i', width: 100, height: 50, thumb: 't', enabled: true,
    })
    render(<PortalLoginPage navigate={navigate} onLoggedIn={noop} />)
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign In' }))
    await screen.findByText('安全验证')
    fireEvent.click(screen.getByText('换一题'))
    fireEvent.change(screen.getByLabelText('旋转角度'), { target: { value: '45' } })
    fireEvent.click(screen.getByText('提交'))
    await waitFor(() => expect(portalAuth.login).toHaveBeenCalledWith(
      expect.objectContaining({ captcha_id: 'refreshed', captcha_angle: 45 }),
    ))
    const sent = vi.mocked(portalAuth.login).mock.calls[0]![0]
    expect(sent).not.toHaveProperty('captcha_points')
    expect(sent).not.toHaveProperty('captcha_x')
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

  it.each([
    ['click', { points: [] }, { captcha_id: 'c2', captcha_points: [expect.any(Object)] }, ['captcha_x', 'captcha_y', 'captcha_angle']],
    ['slide', { x: 7, y: 0 }, { captcha_id: 'c2', captcha_x: 7, captcha_y: 0 }, ['captcha_points', 'captcha_angle']],
    ['dragdrop', { x: 5, y: 6 }, { captcha_id: 'c2', captcha_x: 5, captcha_y: 6 }, ['captcha_points', 'captcha_angle']],
    ['rotate', { angle: 90 }, { captcha_id: 'c2', captcha_angle: 90 }, ['captcha_points', 'captcha_x', 'captcha_y']],
  ] as const)('maps %s register answer to the current challenge without stale fields', async (type, answer, expected, forbidden) => {
    vi.mocked(portalAuth.getCaptcha)
      .mockResolvedValueOnce({ id: 'old', type: 'click', image: 'i', width: 100, height: 50, enabled: true })
      .mockResolvedValue({
        id: 'c2', type, image: 'i', width: 100, height: 50, thumb: 't', text: 'star', enabled: true,
      })
    vi.mocked(portalAuth.register)
      .mockRejectedValueOnce(Object.assign(new Error('bad captcha'), { status: 400 }))
      .mockResolvedValue(tokens())
    render(<PortalRegisterPage navigate={navigate} onRegistered={noop} />)
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('昵称'), { target: { value: 'n' } })
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: 'a@b.co' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByRole('button', { name: '立即注册' }))
    await screen.findByText('安全验证')
    await submitCaptchaAnswer({ points: [{ x: 1, y: 1 }] })
    await waitFor(() => expect(toastError).toHaveBeenCalledWith('验证码校验失败，已为你更换新题'))
    await screen.findByText(type === 'click' ? '请依次点击：star' : type === 'slide' ? '拖动滑块完成拼图' : type === 'dragdrop' ? '将拼图块拖到正确位置' : '拖动或用滑杆旋转图片至正向')
    await submitCaptchaAnswer(answer)
    await waitFor(() => expect(portalAuth.register).toHaveBeenCalledTimes(2))
    const second = vi.mocked(portalAuth.register).mock.calls[1]![0]
    expect(second).toMatchObject(expected)
    for (const field of forbidden) expect(second).not.toHaveProperty(field)
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
  it('shows all public profile fields in the summary', async () => {
    render(<ProfilePage navigate={navigate} />)
    await screen.findByText('个人资料')
    expect(screen.getByText('昵称')).toBeInTheDocument()
    expect(screen.getByText(/手机号：13800138000/)).toBeInTheDocument()
    expect(screen.getByText(/性别：男/)).toBeInTheDocument()
    expect(screen.getByText(/邮箱：a@b.co/)).toBeInTheDocument()
    expect(screen.getByText(/注册时间：2024-10-27/)).toBeInTheDocument()
    expect((await vi.mocked(portalAuth.getProfile).mock.results[0]!.value as { id: number }).id).toBe(42)
  })

  it('displays gender 0 as 未知 and submits a numeric gender when the select changes', async () => {
    vi.mocked(portalAuth.getProfile).mockResolvedValue({ ...profileFixture, gender: 0 })
    render(<ProfilePage navigate={navigate} />)
    await screen.findByText('个人资料')
    expect(screen.getByText(/性别：未知/)).toBeInTheDocument()

    // 编辑表单中的性别下拉框发出字符串值，提交前必须转成后端要求的数字。
    fireEvent.click(screen.getByLabelText('性别'))
    const female = await screen.findByRole('option', { name: '女' })
    fireEvent.pointerDown(female, { pointerType: 'mouse', button: 0 })
    fireEvent.click(female)
    fireEvent.click(screen.getByRole('button', { name: '保存资料' }))
    await waitFor(() =>
      expect(portalAuth.updateProfile).toHaveBeenCalledWith(expect.objectContaining({ gender: 2 })),
    )
    const sent = vi.mocked(portalAuth.updateProfile).mock.calls[0]![0] as { gender: unknown }
    expect(typeof sent.gender).toBe('number')
  })

  it('redirects to login with a user-facing expiry message on 401', async () => {
    vi.mocked(portalAuth.getProfile).mockRejectedValue(new PortalApiError(401, '登录已过期，请重新登录'))
    render(<ProfilePage navigate={navigate} />)
    await waitFor(() => expect(toastError).toHaveBeenCalledWith('登录已过期，请重新登录'))
    expect(navigate).toHaveBeenCalledWith('/login')
  })

  it('updates profile with transient success copy and toasts', async () => {
    render(<ProfilePage navigate={navigate} />)
    await screen.findByText('个人资料')
    fireEvent.click(screen.getByText('保存资料'))
    await waitFor(() =>
      expect(portalAuth.updateProfile).toHaveBeenCalledWith(expect.objectContaining({ nickname: '昵称' })),
    )
    await waitFor(() => expect(screen.getByText('已保存')).toBeInTheDocument())
    expect(toastSuccess).toHaveBeenCalledWith('已保存')
  })

  it('clears success copy on subsequent edit and failure', async () => {
    render(<ProfilePage navigate={navigate} />)
    await screen.findByText('个人资料')
    fireEvent.click(screen.getByText('保存资料'))
    await screen.findByText('已保存')
    fireEvent.change(screen.getByLabelText('昵称'), { target: { value: '新昵称' } })
    expect(screen.queryByText('已保存')).not.toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: 'bad' } })
    fireEvent.submit(screen.getByRole('button', { name: '保存资料' }).closest('form')!)
    await screen.findByText('邮箱格式不正确')
    expect(screen.queryByText('已保存')).not.toBeInTheDocument()
  })

  it('validates profile edits', async () => {
    render(<ProfilePage navigate={navigate} />)
    await screen.findByText('个人资料')
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: 'bad' } })
    fireEvent.submit(screen.getByRole('button', { name: '保存资料' }).closest('form')!)
    await waitFor(() => expect(screen.getByText('邮箱格式不正确')).toBeInTheDocument())
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: '' } })
    fireEvent.change(screen.getByLabelText('手机号'), { target: { value: '' } })
    fireEvent.change(screen.getByLabelText('手机号'), { target: { value: '123' } })
    fireEvent.submit(screen.getByRole('button', { name: '保存资料' }).closest('form')!)
    await waitFor(() => expect(screen.getByText('手机号格式不正确')).toBeInTheDocument())
    expect(portalAuth.updateProfile).not.toHaveBeenCalled()
  })

  it('rejects mismatched password confirmation with legacy copy', async () => {
    render(<ProfilePage navigate={navigate} />)
    await screen.findByText('个人资料')
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'a' } })
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'b' } })
    fireEvent.submit(screen.getByRole('button', { name: '修改密码' }).closest('form')!)
    await waitFor(() => expect(screen.getByText('两次输入的新密码不一致')).toBeInTheDocument())
    expect(screen.queryByText('两次输入的密码不一致')).not.toBeInTheDocument()
    expect(portalAuth.updatePassword).not.toHaveBeenCalled()
  })

  it('updates the password, clears the form, and shows a transient success', async () => {
    render(<ProfilePage navigate={navigate} />)
    await screen.findByText('个人资料')
    fireEvent.change(screen.getByLabelText('原密码'), { target: { value: 'old' } })
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'new' } })
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'new' } })
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }))
    await waitFor(() => expect(portalAuth.updatePassword).toHaveBeenCalledWith({ old_password: 'old', new_password: 'new' }))
    await waitFor(() => expect(screen.getByText('密码已更新')).toBeInTheDocument())
    expect(toastSuccess).toHaveBeenCalledWith('密码已更新')
    expect(screen.getByLabelText('新密码')).toHaveValue('')
  })

  it('clears stale password success on a later validation failure', async () => {
    render(<ProfilePage navigate={navigate} />)
    await screen.findByText('个人资料')
    fireEvent.change(screen.getByLabelText('原密码'), { target: { value: 'old' } })
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'new1' } })
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'new1' } })
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }))
    await screen.findByText('密码已更新')
    fireEvent.change(screen.getByLabelText('原密码'), { target: { value: 'old' } })
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'new2' } })
    expect(screen.queryByText('密码已更新')).not.toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'different' } })
    fireEvent.click(screen.getByRole('button', { name: '修改密码' }))
    expect(screen.getByText('两次输入的新密码不一致')).toBeInTheDocument()
    expect(screen.queryByText('密码已更新')).not.toBeInTheDocument()
  })

  it('does not render a duplicate logout control inside the profile', async () => {
    render(<ProfilePage navigate={navigate} />)
    await screen.findByText('个人资料')
    expect(screen.queryByText('退出')).not.toBeInTheDocument()
    expect(screen.queryByText('退出登录')).not.toBeInTheDocument()
  })
})
