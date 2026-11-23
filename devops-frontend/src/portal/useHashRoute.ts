import { useCallback, useEffect, useState } from 'react'

export const PORTAL_ROUTES = ['/', '/login', '/register', '/profile', '/docs'] as const

function parseHash(): string {
  const h = window.location.hash.replace(/^#/, '')
  if (!h || h === '/') return '/'
  const path = h.startsWith('/') ? h : `/${h}`
  return (PORTAL_ROUTES as readonly string[]).includes(path) ? path : '/'
}

export function useHashRoute(): { path: string; navigate: (to: string) => void } {
  const [path, setPath] = useState(parseHash)

  useEffect(() => {
    const onHash = () => setPath(parseHash())
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  const navigate = useCallback((to: string) => {
    const normalized = to.startsWith('/') ? to : `/${to}`
    window.location.hash = `#${normalized}`
    setPath(normalized)
  }, [])

  return { path, navigate }
}
