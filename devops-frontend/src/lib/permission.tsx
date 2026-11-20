import { createContext, useContext, useMemo, type ReactNode } from 'react'

interface PermissionsContextValue {
  permissions: string[]
  has: (p: string) => boolean
}

// 权限码精确匹配，不支持通配符
const PermissionsContext = createContext<PermissionsContextValue>({
  permissions: [],
  has: () => false,
})

export function PermissionsProvider({
  permissions,
  children,
}: {
  permissions: string[]
  children: ReactNode
}) {
  const value = useMemo<PermissionsContextValue>(
    () => ({
      permissions,
      has: (p: string) => permissions.includes(p),
    }),
    [permissions],
  )
  return <PermissionsContext.Provider value={value}>{children}</PermissionsContext.Provider>
}

export function usePermissions(): PermissionsContextValue {
  return useContext(PermissionsContext)
}
