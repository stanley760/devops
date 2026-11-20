import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { RequireAuth } from '@/router/guards'
import { session } from '@/lib/session'
import { SessionProvider } from '@/store/session'

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <SessionProvider>
        <Routes>
          <Route
            path="/protected"
            element={
              <RequireAuth>
                <div>secret</div>
              </RequireAuth>
            }
          />
          <Route path="/login" element={<div>login page</div>} />
        </Routes>
      </SessionProvider>
    </MemoryRouter>,
  )
}

describe('RequireAuth', () => {
  it('renders children when logged in', async () => {
    session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
    renderAt('/protected')
    await waitFor(() => expect(screen.getByText('secret')).toBeInTheDocument())
  })

  it('redirects to /login with tenantCode+redirect when not logged in', async () => {
    renderAt('/protected')
    await waitFor(() => expect(screen.getByText('login page')).toBeInTheDocument())
  })
})
