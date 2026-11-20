import { useCallback, useRef, useState } from 'react'
import type { PageResult } from '@/lib/types'

export interface ListPageConfig<T> {
  fetcher: (params: { page_num: number; page_size: number; [k: string]: unknown }) => Promise<PageResult<T>>
  defaultQuery?: Record<string, unknown>
  pageSize?: number
  exportPath?: string
}

export interface ListPageState<T> {
  query: Record<string, unknown>
  rows: T[]
  total: number
  page: number
  pageSize: number
  loading: boolean
  exporting: boolean
  selected: string[]
  setSelected: (ids: string[]) => void
  setQuery: (patch: Record<string, unknown>) => void
  load: () => Promise<void>
  search: () => Promise<void>
  reset: () => Promise<void>
  setPage: (p: number) => Promise<void>
  setPageSize: (s: number) => Promise<void>
  exportData: () => Promise<void>
  afterMutate: () => Promise<void>
}

export function useListPage<T extends { id: string }>(cfg: ListPageConfig<T>): ListPageState<T> {
  const [query, setQueryState] = useState<Record<string, unknown>>({ ...(cfg.defaultQuery ?? {}) })
  const [rows, setRows] = useState<T[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPageState] = useState(1)
  const [pageSize, setPageSizeState] = useState(cfg.pageSize ?? 10)
  const [loading, setLoading] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const seq = useRef(0)

  const load = useCallback(async () => {
    const mySeq = ++seq.current
    setLoading(true)
    try {
      const res = await cfg.fetcher({
        page_num: page,
        page_size: Math.min(pageSize, 1000),
        ...query,
      })
      if (mySeq !== seq.current) return // 竞态：过期响应丢弃
      setRows(res.list)
      setTotal(res.total)
      // 删除末页末行后回退一页
      if (res.list.length === 0 && res.total > 0 && page > 1) {
        setPageState(page - 1)
      }
    } finally {
      if (mySeq === seq.current) setLoading(false)
    }
  }, [cfg, page, pageSize, query])

  const search = useCallback(async () => {
    setPageState(1)
  }, [])

  const reset = useCallback(async () => {
    setQueryState({ ...(cfg.defaultQuery ?? {}) })
    setPageState(1)
  }, [cfg])

  const setPage = useCallback(async (p: number) => {
    setPageState(p)
  }, [])

  const setPageSize = useCallback(async (s: number) => {
    setPageSizeState(s)
    setPageState(1)
  }, [])

  const exportData = useCallback(async () => {
    if (!cfg.exportPath) return
    setExporting(true)
    try {
      const params = new URLSearchParams(
        Object.entries({ page_num: '1', page_size: '1000', ...query }).map(([k, v]) => [k, String(v)]),
      )
      const token = localStorage.getItem('devops.access_token')
      const tenantId = localStorage.getItem('devops.tenant_id')
      const res = await fetch(`${cfg.exportPath}?${params}`, {
        headers: {
          Accept: 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          ...(tenantId ? { 'X-Tenant-ID': tenantId } : {}),
        },
      })
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'export.xlsx'
      a.click()
      URL.revokeObjectURL(url)
    } finally {
      setExporting(false)
    }
  }, [cfg, query])

  const afterMutate = useCallback(async () => {
    setSelected([])
    await load()
  }, [load])

  return {
    query, rows, total, page, pageSize, loading, exporting, selected,
    setSelected,
    setQuery: (patch) => setQueryState((q) => ({ ...q, ...patch })),
    load, search, reset, setPage, setPageSize, exportData, afterMutate,
  }
}
