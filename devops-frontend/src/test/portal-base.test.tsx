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
})
