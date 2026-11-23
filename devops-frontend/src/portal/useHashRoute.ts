import { useCallback, useEffect, useState } from 'react'

export const PORTAL_ROUTES = ['/', '/login', '/register', '/profile', '/docs'] as const

/**
 * 归一化 hash：去掉前导 #，补上缺失的前导 /，再过一遍白名单。
 *
 * 白名单在归一化之后统一执行，所以直接改写 location.hash（用户手敲地址、
 * 浏览器前进后退）得到的路径和 navigate() 走的是同一条路：未知路径一律落 `/`。
 */
export function parseHash(hash: string): string {
  const h = hash.replace(/^#/, '')
  if (!h || h === '/') return '/'
  const path = h.startsWith('/') ? h : `/${h}`
  return (PORTAL_ROUTES as readonly string[]).includes(path) ? path : '/'
}

export function useHashRoute(): { path: string; navigate: (to: string) => void } {
  const [path, setPath] = useState(() => parseHash(window.location.hash))

  useEffect(() => {
    const onHash = () => setPath(parseHash(window.location.hash))
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  const navigate = useCallback((to: string) => {
    // 同样过白名单：navigate 到未知路由时不能把地址栏写成非法 hash。
    const normalized = parseHash(to)
    window.location.hash = `#${normalized}`
    setPath(normalized)
  }, [])

  return { path, navigate }
}
