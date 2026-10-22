/**
 * 共享选项数据源：把「列表类接口」摊平成下拉选项，供各表单页复用。
 *
 * 为什么要这么写（几个反复踩到的点）：
 *  1. **不能假设一定有权限**。当前用户可能只有菜单的权限码而没有对应列表权限
 *     （例如能进用户页但没有 system:dept:list）。所以每个 loader 都吞掉异常并
 *     返回空列表——下拉为空好过整个页面白屏报错，用户仍可手填其余字段。
 *  2. **ID 一律字符串**。后端 ID 是 int64 序列化的字符串，选项 value 必须用字符串
 *     原样透传，不能转成 number（雪花 ID 有精度丢失风险）。
 *  3. **树要拉平**。部门/菜单是树形接口，下拉需要带缩进的一维选项。
 *  4. **同一份数据只拉一次**。同一页面可能同时用部门树做「上级部门」和
 *     「自定义数据范围」，用模块级缓存 + in-flight 去重避免重复请求。
 */
import { ref, computed, type Ref } from 'vue'
import {
  deptApi,
  postApi,
  roleApi,
  dictApi,
  type SysDept,
  type DictData,
} from '@/api/system'
import type { SelectOption } from '@/components/ui/types'

/** 通用缓存：key → { data, inFlight }，data 为 null 表示加载失败（降级为空列表）。 */
interface CacheEntry<T> {
  data: Ref<T[]> | null
  inFlight: Promise<T[]> | null
}

const caches = new Map<string, CacheEntry<unknown>>()

function cached<T>(key: string, loader: () => Promise<T[]>): Ref<T[]> {
  let entry = caches.get(key) as CacheEntry<T> | undefined
  if (!entry) {
    const data = ref<T[]>([]) as Ref<T[]>
    entry = { data, inFlight: null }
    caches.set(key, entry as CacheEntry<unknown>)
  }
  // 已有成功数据则直接复用；正在加载则复用同一个 Promise
  if (entry.data!.value.length > 0 || entry.inFlight) return entry.data!
  const p = loader()
    .then((list) => {
      entry!.data!.value = list
      return list
    })
    .catch(() => {
      // 降级：保留空列表，不打断页面
      entry!.data!.value = []
      return [] as T[]
    })
    .finally(() => {
      entry!.inFlight = null
    })
  entry.inFlight = p
  return entry.data!
}

/** 清空全部选项缓存（切换租户 / 重新登录后调用，避免跨租户脏数据）。 */
export function resetOptionsCache(): void {
  caches.clear()
}

/** 把树形节点拉平成带缩进的一维选项（部门/菜单共用）。 */
export function flattenTreeOptions<T extends { id: string; name: string; children?: T[] }>(
  nodes: T[],
  depth = 0,
  out: SelectOption[] = [],
): SelectOption[] {
  for (const n of nodes) {
    out.push({ value: n.id, label: `${'　'.repeat(depth)}${n.name}` })
    if (n.children?.length) flattenTreeOptions(n.children as T[], depth + 1, out)
  }
  return out
}

/* ---------------- 部门 ---------------- */

interface DeptNode extends SysDept {
  children?: DeptNode[]
}

/** 部门下拉选项（树形拉平，带缩进）。无权限时返回空数组。 */
export function useDeptOptions(): Ref<SelectOption[]> {
  const raw = cached<DeptNode>('depts', async () => (await deptApi.tree()) as DeptNode[])
  return computed(() => flattenTreeOptions(raw.value))
}

/** 部门原始树，供需要层级语义的地方使用。 */
export function useDeptTree(): Ref<DeptNode[]> {
  return cached<DeptNode>('depts', async () => (await deptApi.tree()) as DeptNode[])
}

/* ---------------- 岗位 ---------------- */

export function usePostOptions(): Ref<SelectOption[]> {
  const raw = cached<{ id: string; name: string; status: number }>('posts', async () => {
    const res = await postApi.list({ page_num: 1, page_size: 200 })
    return res?.list ?? []
  })
  return computed(() =>
    raw.value
      .filter((p) => p.status === 1)
      .map((p) => ({ value: p.id, label: p.name })),
  )
}

/* ---------------- 角色 ---------------- */

export function useRoleOptions(): Ref<SelectOption[]> {
  const raw = cached<{ id: string; name: string; code: string; status: number }>('roles', async () => {
    const res = await roleApi.list({ page_num: 1, page_size: 200 })
    return res?.list ?? []
  })
  return computed(() =>
    raw.value
      .filter((r) => r.status === 1)
      .map((r) => ({ value: r.id, label: r.name })),
  )
}

/* ---------------- 字典 ---------------- */

/**
 * 字典下拉选项（走 GET /dict-data/simple，后端只返回启用中的项）。
 * @param dictType 字典类型编码，如 sys_captcha_type
 */
export function useDictOptions(dictType: string): Ref<SelectOption[]> {
  const raw = cached<DictData>(`dict:${dictType}`, async () => dictApi.data.simple(dictType))
  return computed(() =>
    raw.value
      .filter((d) => d.status === 1)
      .map((d) => ({ value: d.value, label: d.label })),
  )
}

/** 字典原始项，供需要 cssClass / remark 的地方使用。 */
export function useDictItems(dictType: string): Ref<DictData[]> {
  return cached<DictData>(`dict:${dictType}`, async () => dictApi.data.simple(dictType))
}