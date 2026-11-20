import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Login from '@/views/Login'
import { session } from '@/lib/session'

function ok(data: unknown) {
  return new Response(JSON.stringify({ code: 200, data }), { status: 200 })
}
function bizError(code: number, msg: string) {
  return new Response(JSON.stringify({ code, msg }), { status: 200 })
}

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

function renderLogin(initial = '/login') {
  return render(
    <MemoryRouter initialEntries={[initial]}>
      <Login />
    </MemoryRouter>,
  )
}

describe('Login page', () => {
  it('renders tenant input when unlocked (no query/path/subdomain/last)', () => {
    renderLogin('/login')
    expect(screen.getByLabelText('租户编码')).toBeInTheDocument()
    expect(screen.getByText('登录 DevOps 管理平台')).toBeInTheDocument()
  })

  it('locks tenant input when ?tenantCode= present', () => {
    renderLogin('/login?tenantCode=acme')
    expect(screen.queryByLabelText('租户编码')).not.toBeInTheDocument()
  })

  it('shows mapped credential error on login card (closes captcha dialog)', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/captcha'))
        return ok({ id: 'c', type: 'slide', image: 'i', width: 1, height: 1, enabled: false })
      if ((url as string).includes('/login')) return bizError(401, 'incorrect username or password')
      throw new Error('unexpected')
    })
    renderLogin('/login')
    fireEvent.change(screen.getByLabelText('账号'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByText('登录'))
    await waitFor(() => expect(screen.getByText('账号或密码错误')).toBeInTheDocument())
    expect(screen.queryByText('安全验证')).not.toBeInTheDocument()
  })

  it('skips captcha when enabled=false and logs in directly', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/captcha'))
        return ok({ id: 'c', type: 'slide', image: 'i', width: 1, height: 1, enabled: false })
      if ((url as string).includes('/login'))
        return ok({
          access_token: 'a', refresh_token: 'r', expires_in: 1,
          tenant: { id: '1', name: 'ACME', code: 'acme' },
        })
      throw new Error('unexpected')
    })
    renderLogin('/login?tenantCode=acme')
    fireEvent.change(screen.getByLabelText('账号'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByText('登录'))
    await waitFor(() => expect(session.isLoggedIn()).toBe(true))
    expect(localStorage.getItem('devops.last_tenant')).toBe('acme')
    expect(screen.queryByText('安全验证')).not.toBeInTheDocument()
  })

  it('pulls captcha on submit and shows dialog when enabled', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/captcha'))
        return ok({ id: 'c', type: 'click', image: 'i', width: 100, height: 50, text: 'circle' })
      throw new Error('unexpected')
    })
    renderLogin('/login')
    fireEvent.change(screen.getByLabelText('账号'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByText('登录'))
    await waitFor(() => expect(screen.getByText('安全验证')).toBeInTheDocument())
  })
})
