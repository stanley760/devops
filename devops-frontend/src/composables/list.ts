/**
 * 列表页状态：查询 / 分页 / 选中 / 导出 / 竞态防护。
 * ListPage.vue 只消费 ListPageState，不关心数据来源。
 */
import { reactive, ref, shallowRef, type Ref } from 'vue'
import type { PageResult, Query } from '@/api/http'
import { toastError, toastOk } from './toast'

export interface ListPageState<Row> {
  query: Record<string, unknown>
  rows: Ref<Row[]>
  total: Ref<number>
  page: Ref<number>
  pageSize: Ref<number>
  loading: Ref<boolean>
  exporting: Ref<boolean>
  selected: Ref<Array<string | number>>
  load: () => Promise<void>
  search: () => Promise<void>
  reset: () => Promise<void>
  setPage: (p: number) => Promise<void>
  setPageSize: (n: number) => Promise<void>
  exportData: () => Promise<void>
  canExport: boolean
}

export interface ListPageOptions<Row> {
  query: Record<string, unknown>
  fetch: (params: Query) => Promise<PageResult<Row>>
  exporter?: (params: Query) => Promise<Blob>
  exportName?: string
  pageSize?: number
  toParams?: (query: Record<string, unknown>) => Query
  immediate?: boolean
  rowKey?: string
}

function stripEmpty(query: Record<string, unknown>): Query {
  const out: Query = {}
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null || v === '') continue
    out[k] = v as string | number | boolean
  }
  return out
}

export function useListPage<Row>(options: ListPageOptions<Row>): ListPageState<Row> {
  const initial = JSON.parse(JSON.stringify(options.query)) as Record<string, unknown>
  const query = reactive<Record<string, unknown>>({ ...options.query })
  const rows = shallowRef<Row[]>([]) as Ref<Row[]>
  const total = ref(0)
  const page = ref(1)
  const pageSize = ref(options.pageSize ?? 10)
  const loading = ref(false)
  const exporting = ref(false)
  const selected = ref<Array<string | number>>([])
  let requestId = 0

  const params = (): Query => {
    const base = options.toParams ? options.toParams({ ...query }) : stripEmpty({ ...query })
    return base
  }

  async function load(): Promise<void> {
    const rid = ++requestId
    loading.value = true
    try {
      const res = await options.fetch({ ...params(), page_num: page.value, page_size: pageSize.value })
      if (rid !== requestId) return
      rows.value = res.list ?? []
      total.value = res.total ?? 0
      selected.value = []
    } catch (e) {
      if (rid !== requestId) return
      toastError(e, '数据加载失败')
    } finally {
      if (rid === requestId) loading.value = false
    }
  }

  async function search(): Promise<void> {
    page.value = 1
    await load()
  }

  async function reset(): Promise<void> {
    for (const k of Object.keys(query)) delete query[k]
    Object.assign(query, initial)
    page.value = 1
    await load()
  }

  async function setPage(p: number): Promise<void> {
    page.value = p
    await load()
  }

  async function setPageSize(n: number): Promise<void> {
    pageSize.value = n
    page.value = 1
    await load()
  }

  /** 删除末页末行后自动回退一页。 */
  async function afterMutate(removedCount = 0): Promise<void> {
    if (removedCount > 0 && rows.value.length <= removedCount && page.value > 1) {
      page.value = page.value - 1
    }
    await load()
  }

  async function exportData(): Promise<void> {
    if (!options.exporter) return
    exporting.value = true
    try {
      const blob = await options.exporter(params())
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = options.exportName ?? 'export.csv'
      a.click()
      URL.revokeObjectURL(url)
      toastOk('导出完成')
    } catch (e) {
      toastError(e, '导出失败')
    } finally {
      exporting.value = false
    }
  }

  const state: ListPageState<Row> & { afterMutate: typeof afterMutate } = {
    query,
    rows,
    total,
    page,
    pageSize,
    loading,
    exporting,
    selected,
    load,
    search,
    reset,
    setPage,
    setPageSize,
    exportData,
    canExport: Boolean(options.exporter),
    afterMutate,
  }

  if (options.immediate !== false) {
    void load()
  }

  return state
}

/* ---------------- 树列表 ---------------- */

export interface TreeListState<Row> extends ListPageState<Row> {
  tree: true
}

function countNodes<T>(rows: T[], key = 'children'): number {
  let n = 0
  for (const r of rows) {
    n += 1
    const children = (r as Record<string, unknown>)[key]
    if (Array.isArray(children)) n += countNodes(children as T[], key)
  }
  return n
}

export function useTreeListPage<Row extends { children?: Row[] }>(options: Omit<ListPageOptions<Row>, 'pageSize' | 'exporter'>): TreeListState<Row> {
  const base = useListPage<Row>({
    ...options,
    pageSize: Number.MAX_SAFE_INTEGER,
    fetch: async (params) => {
      const { page_num: _p, page_size: _s, ...rest } = params
      return options.fetch(rest as Query)
    },
    immediate: false,
  })

  async function load(): Promise<void> {
    await base.load()
  }

  const state: TreeListState<Row> = {
    ...base,
    load,
    search: load,
    setPage: load,
    setPageSize: load,
    tree: true,
  }

  if (options.immediate !== false) void load()
  return state
}

export function totalCount<Row>(rows: Row[]): number {
  return countNodes(rows)
}
