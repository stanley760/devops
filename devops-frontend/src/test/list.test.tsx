import { renderHook, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { useListPage } from '@/lib/list'
import type { PageResult } from '@/lib/types'

function page(list: Array<{ id: string }>, total: number, page: number, pageSize: number): PageResult<{ id: string }> {
  return { list, total, page_num: page, page_size: pageSize, pages: Math.ceil(total / pageSize) }
}

describe('useListPage', () => {
  it('loads page and exposes state', async () => {
    const fetcher = vi.fn().mockResolvedValue(page([{ id: '1' }], 1, 1, 10))
    const { result } = renderHook(() => useListPage({ fetcher }))
    await act(async () => { await result.current.load() })
    await waitFor(() => expect(result.current.rows).toEqual([{ id: '1' }]))
    expect(result.current.total).toBe(1)
    expect(fetcher).toHaveBeenCalledWith(expect.objectContaining({ page_num: 1, page_size: 10 }))
  })

  it('drops stale response on race', async () => {
    let resolveSecond: ((v: PageResult<{ id: string }>) => void) | null = null
    const fetcher = vi
      .fn()
      .mockImplementationOnce(
        () => new Promise((r) => setTimeout(() => r(page([{ id: 'slow' }], 1, 1, 10)), 50)),
      )
      .mockImplementationOnce(
        () =>
          new Promise<PageResult<{ id: string }>>((resolve) => {
            resolveSecond = resolve
          }),
      )
    const { result } = renderHook(() => useListPage({ fetcher }))
    const first = act(() => { void result.current.load() })
    await act(async () => { await result.current.setPage(2) })
    const second = act(() => { void result.current.load() })
    await first
    resolveSecond!(page([{ id: 'fast' }], 1, 2, 10))
    await second
    await waitFor(() => expect(result.current.rows).toEqual([{ id: 'fast' }]))
  })

  it('caps page_size at 1000', async () => {
    const fetcher = vi.fn().mockResolvedValue(page([], 0, 1, 10))
    const { result } = renderHook(() => useListPage({ fetcher, pageSize: 5000 }))
    await act(async () => { await result.current.load() })
    expect(fetcher).toHaveBeenCalledWith(expect.objectContaining({ page_size: 1000 }))
  })
})
