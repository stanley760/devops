import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { PermissionsProvider, usePermissions } from '@/lib/permission'

function Probe({ code }: { code: string }) {
  const { has } = usePermissions()
  return <div>{has(code) ? 'granted' : 'denied'}</div>
}

describe('permissions (exact match)', () => {
  it('matches exact permission code', () => {
    render(
      <PermissionsProvider permissions={['system:user:create']}>
        <Probe code="system:user:create" />
      </PermissionsProvider>,
    )
    expect(screen.getByText('granted')).toBeInTheDocument()
  })
  it('denies non-exact codes (no wildcard)', () => {
    render(
      <PermissionsProvider permissions={['system:user:create']}>
        <Probe code="system:user:*" />
        <Probe code="system:user:create:extra" />
      </PermissionsProvider>,
    )
    expect(screen.getAllByText('denied')).toHaveLength(2)
  })
})
