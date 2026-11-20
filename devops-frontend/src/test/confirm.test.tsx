import { render, screen, fireEvent, act } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { ConfirmProvider, useConfirm } from '@/lib/confirm'

function Probe() {
  const confirm = useConfirm()
  return (
    <button
      onClick={async () => {
        const ok = await confirm({ title: '确认删除？', danger: true })
        document.body.dataset.result = String(ok)
      }}
    >
      ask
    </button>
  )
}

describe('useConfirm', () => {
  it('resolves true on confirm', async () => {
    render(
      <ConfirmProvider>
        <Probe />
      </ConfirmProvider>,
    )
    fireEvent.click(screen.getByText('ask'))
    expect(screen.getByText('确认删除？')).toBeInTheDocument()
    await act(async () => { fireEvent.click(screen.getByText('确定')) })
    expect(document.body.dataset.result).toBe('true')
  })
  it('resolves false on cancel', async () => {
    render(
      <ConfirmProvider>
        <Probe />
      </ConfirmProvider>,
    )
    fireEvent.click(screen.getByText('ask'))
    await act(async () => { fireEvent.click(screen.getByText('取消')) })
    expect(document.body.dataset.result).toBe('false')
  })
})
