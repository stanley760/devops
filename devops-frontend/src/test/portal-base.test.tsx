import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useHashRoute } from '@/portal/useHashRoute'
import PortalApp from '@/portal/App'
import { PortalCaptchaDialog } from '@/portal/components/PortalCaptchaDialog'
import { portalSession } from '@/lib/portal-session'
import type { CaptchaChallenge } from '@/lib/types'

vi.mock('@/api/portal/auth', () => ({
  portalAuth: { logout: vi.fn().mockResolvedValue(undefined) },
}))

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  window.location.hash = ''
})

function RouteProbe() {
  const { path, navigate } = useHashRoute()
  return (
    <div>
      <span data-testid="path">{path}</span>
      <button onClick={() => navigate('/login')}>go</button>
      <button onClick={() => navigate('/nope')}>go-unknown</button>
    </div>
  )
}

describe('useHashRoute', () => {
  it('parses location.hash and navigates', () => {
    window.location.hash = '#/login'
    render(<RouteProbe />)
    expect(screen.getByTestId('path').textContent).toBe('/login')
    fireEvent.click(screen.getByText('go'))
    expect(window.location.hash).toBe('#/login')
  })
  it('normalizes bare hash to /', () => {
    window.location.hash = ''
    render(<RouteProbe />)
    expect(screen.getByTestId('path').textContent).toBe('/')
  })
  it('falls back to / for unknown hash', () => {
    window.location.hash = '#/nope'
    render(<RouteProbe />)
    expect(screen.getByTestId('path').textContent).toBe('/')
  })
  it('navigate normalizes an unknown route through the whitelist', () => {
    render(<RouteProbe />)
    fireEvent.click(screen.getByText('go-unknown'))
    expect(window.location.hash).toBe('#/')
    expect(screen.getByTestId('path').textContent).toBe('/')
  })
})

describe('PortalApp shell', () => {
  it('shows brand copy and Sign In/Sign Up nav when logged out', () => {
    render(<PortalApp />)
    expect(screen.getAllByText('Design World').length).toBeGreaterThan(0)
    expect(screen.getByText('Explore New Ideas')).toBeInTheDocument()
    expect(screen.getByText('Explore Now')).toBeInTheDocument()
    expect(screen.getAllByText('Sign In').length).toBeGreaterThan(0)
    expect(screen.getByText('Sign Up')).toBeInTheDocument()
  })

  it('Explore Now navigates to #/register', () => {
    render(<PortalApp />)
    fireEvent.click(screen.getByText('Explore Now'))
    expect(window.location.hash).toBe('#/register')
  })

  it('renders docs route', () => {
    window.location.hash = '#/docs'
    render(<PortalApp />)
    expect(screen.getByText('产品文档')).toBeInTheDocument()
  })

  it('logout clears portal_tokens and returns home', async () => {
    portalSession.saveTokens({ access_token: 'a', refresh_token: 'r', expires_in: 1, issued_at: Date.now() })
    window.location.hash = '#/profile'
    render(<PortalApp />)
    fireEvent.click(screen.getByText('退出'))
    await vi.waitFor(() => expect(portalSession.isLoggedIn()).toBe(false))
  })

  it('logout sends the refresh_token so the backend can revoke it', async () => {
    const { portalAuth } = await import('@/api/portal/auth')
    portalSession.saveTokens({ access_token: 'a', refresh_token: 'r', expires_in: 1, issued_at: Date.now() })
    window.location.hash = '#/profile'
    render(<PortalApp />)
    fireEvent.click(screen.getByText('退出'))
    await vi.waitFor(() => expect(portalAuth.logout).toHaveBeenCalledWith('r'))
  })

  it('renders the shell with solid-glass surfaces', () => {
    const view = render(<PortalApp />)
    expect(view.container.querySelector('.glass-panel')).toBeInTheDocument()
    expect(view.container.querySelector('.glass-control')).toBeInTheDocument()
  })
})

describe('PortalCaptchaDialog', () => {
  const clickChallenge: CaptchaChallenge = {
    id: 'c1',
    type: 'click',
    image: 'data:image/png;base64,x',
    width: 300,
    height: 150,
    text: 'circle,star',
  }
  const slideChallenge: CaptchaChallenge = {
    id: 'c2',
    type: 'slide',
    image: 'data:image/png;base64,x',
    width: 300,
    height: 220,
    thumb: 'data:image/png;base64,t',
    thumb_x: 10,
    thumb_y: 80,
    thumb_width: 64,
    thumb_height: 64,
  }
  const rotateChallenge: CaptchaChallenge = {
    id: 'c3',
    type: 'rotate',
    image: 'data:image/png;base64,x',
    width: 220,
    height: 220,
    thumb: 'data:image/png;base64,t',
    thumb_size: 140,
  }

  it('collects click points and emits CaptchaAnswer via onAnswer', () => {
    const onAnswer = vi.fn()
    render(
      <PortalCaptchaDialog
        open
        challenge={clickChallenge}
        onAnswer={onAnswer}
        onRefresh={() => {}}
        onCancel={() => {}}
      />,
    )
    expect(screen.getByText(/circle,star/)).toBeInTheDocument()
    fireEvent.click(screen.getByAltText('captcha'))
    fireEvent.click(screen.getByText('提交'))
    expect(onAnswer).toHaveBeenCalledWith({
      points: [expect.objectContaining({ x: expect.any(Number), y: expect.any(Number) })],
    })
  })

  it('slide drags horizontally and keeps the vertical offset at the challenge row', () => {
    const onAnswer = vi.fn()
    render(
      <PortalCaptchaDialog
        open
        challenge={slideChallenge}
        onAnswer={onAnswer}
        onRefresh={() => {}}
        onCancel={() => {}}
      />,
    )
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 20, clientY: 90 })
    // 纵向拖动 40px、横向拖动 60px
    fireEvent.mouseMove(window, { clientX: 80, clientY: 130 })
    fireEvent.mouseUp(window)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { x: number; y: number }
    // 横向累计了 60px
    expect(a.x).toBe(70)
    // 纵向被钳制：slide 只有横向一轴，y 必须仍是切图所在行的 thumb_y
    expect(a.y).toBe(80)
  })

  it('dragdrop allows a vertical move within the image bounds', () => {
    const onAnswer = vi.fn()
    render(
      <PortalCaptchaDialog
        open
        challenge={{ ...slideChallenge, type: 'dragdrop', thumb_y: 5 }}
        onAnswer={onAnswer}
        onRefresh={() => {}}
        onCancel={() => {}}
      />,
    )
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 20, clientY: 20 })
    fireEvent.mouseMove(window, { clientX: 40, clientY: 50 })
    fireEvent.mouseUp(window)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { x: number; y: number }
    expect(a.x).toBe(30)
    expect(a.y).toBe(35)
  })

  it('clamps the drop position to the image bounds', () => {
    const onAnswer = vi.fn()
    render(
      <PortalCaptchaDialog
        open
        challenge={{ ...slideChallenge, type: 'dragdrop', thumb_y: 5 }}
        onAnswer={onAnswer}
        onRefresh={() => {}}
        onCancel={() => {}}
      />,
    )
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 20, clientY: 20 })
    // 拖到图片外面很远
    fireEvent.mouseMove(window, { clientX: 5000, clientY: 5000 })
    fireEvent.mouseUp(window)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { x: number; y: number }
    // 拼图块不能被拖出图片：落点钳制在 [0, width - thumb_width] / [0, height - thumb_height]
    expect(a.x).toBe(300 - 64)
    expect(a.y).toBe(220 - 64)
  })

  it('rotate drag accumulates the delta and submits a non-zero angle', () => {
    const onAnswer = vi.fn()
    render(
      <PortalCaptchaDialog
        open
        challenge={rotateChallenge}
        onAnswer={onAnswer}
        onRefresh={() => {}}
        onCancel={() => {}}
      />,
    )
    const target = screen.getByAltText('rotate target')
    fireEvent.mouseDown(target, { clientX: 100, clientY: 100 })
    fireEvent.mouseMove(window, { clientX: 130, clientY: 100 })
    fireEvent.mouseMove(window, { clientX: 155, clientY: 100 })
    fireEvent.mouseUp(window)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { angle: number }
    // 累计拖过 55px；图宽 220px 对应一整圈（360°），即 55 * 360 / 220 = 90°。
    // 关键是累计值，而不是只看最后一帧的增量（30px + 25px ≠ 25px）。
    expect(a.angle).toBe(90)
    expect(a.angle).not.toBe(0)
  })

  it('rotate slider submits the selected angle', () => {
    const onAnswer = vi.fn()
    render(
      <PortalCaptchaDialog
        open
        challenge={rotateChallenge}
        onAnswer={onAnswer}
        onRefresh={() => {}}
        onCancel={() => {}}
      />,
    )
    fireEvent.change(screen.getByLabelText('旋转角度'), { target: { value: '275' } })
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { angle: number }
    expect(a.angle).toBe(275)
  })
})
