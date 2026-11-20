import { request } from '@/lib/http'
import type {
  DictItem, DictType, LoginLog, OperationLog, PageResult,
  SystemConfig, SystemDept, SystemMenu, SystemPost, SystemRole, SystemTenant, SystemUser,
} from '@/lib/types'

// 角色 data_scope_dept_ids：JSON 数组字符串（非逗号分隔）
export function parseDeptIDsJson(s?: string): string[] {
  if (!s) return []
  try {
    const v = JSON.parse(s)
    return Array.isArray(v) ? v.map(String) : []
  } catch {
    return []
  }
}

export function encodeDeptIDsJson(ids: string[]): string {
  return JSON.stringify(ids)
}

export type ListQuery = { page_num?: number; page_size?: number } & Record<string, unknown>

export interface UserWrite {
  username: string
  nickname: string
  dept_id: string
  post_ids: string[]
  role_ids: string[]
  email?: string
  mobile?: string
  gender?: string
  avatar?: string
  remark?: string
  status?: number
}

export interface RoleWrite {
  name: string
  code: string
  sort: number
  data_scope: number
  data_scope_dept_ids?: string
  status: number
  remark?: string
}

export interface MenuWrite {
  name: string
  permission?: string
  type: number
  sort: number
  parent_id: string
  path?: string
  icon?: string
  component?: string
  component_name?: string
  status: number
  visible: boolean
  keep_alive?: boolean
  always_show?: boolean
}

export interface DeptWrite {
  name: string
  parent_id: string
  sort: number
  leader_user_id?: string
  phone?: string
  email?: string
  status: number
}

export interface PostWrite {
  code: string
  name: string
  sort: number
  status: number
  remark?: string
}

export interface TenantWrite {
  name: string
  code: string
  contact_name: string
  contact_mobile: string
  expire_time?: string
  remark?: string
  admin_username?: string
  admin_password?: string
}

export interface DictTypeWrite {
  name: string
  type: string
  status: number
  remark?: string
}

export interface DictItemWrite {
  dict_type: string
  label: string
  value: string
  sort: number
}

export interface ConfigWrite {
  config_key: string
  config_name: string
  config_value: string
  type: number
  visible: boolean
  remark?: string
}

export const adminSystem = {
  // ── 用户 ──
  listUsers: (q: ListQuery) => request<PageResult<SystemUser>>('/system/api/v1/system/users', { query: q }),
  createUser: (u: UserWrite) =>
    request<void>('/system/api/v1/system/users', { method: 'POST', body: JSON.stringify(u) }),
  updateUser: (id: string, u: Partial<UserWrite>) =>
    request<void>(`/system/api/v1/system/users/${id}`, { method: 'PUT', body: JSON.stringify(u) }),
  updateUserPassword: (id: string, p: { password: string }) =>
    request<void>(`/system/api/v1/system/users/${id}/password`, { method: 'PUT', body: JSON.stringify(p) }),
  deleteUsers: (ids: string[]) =>
    request<void>('/system/api/v1/system/users', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 角色 ──
  listRoles: (q: ListQuery) => request<PageResult<SystemRole>>('/system/api/v1/system/roles', { query: q }),
  listAllRoles: () => request<SystemRole[]>('/system/api/v1/system/roles/all'),
  createRole: (r: RoleWrite) =>
    request<void>('/system/api/v1/system/roles', { method: 'POST', body: JSON.stringify(r) }),
  updateRole: (id: string, r: Partial<RoleWrite>) =>
    request<void>(`/system/api/v1/system/roles/${id}`, { method: 'PUT', body: JSON.stringify(r) }),
  assignRoleMenus: (id: string, menu_ids: string[]) =>
    request<void>(`/system/api/v1/system/roles/${id}/menus`, { method: 'PUT', body: JSON.stringify({ menu_ids }) }),
  deleteRoles: (ids: string[]) =>
    request<void>('/system/api/v1/system/roles', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 菜单 ──
  listMenus: () => request<SystemMenu[]>('/system/api/v1/system/menus'),
  createMenu: (m: MenuWrite) =>
    request<void>('/system/api/v1/system/menus', { method: 'POST', body: JSON.stringify(m) }),
  updateMenu: (id: string, m: Partial<MenuWrite>) =>
    request<void>(`/system/api/v1/system/menus/${id}`, { method: 'PUT', body: JSON.stringify(m) }),
  deleteMenu: (id: string) => request<void>(`/system/api/v1/system/menus/${id}`, { method: 'DELETE' }),

  // ── 部门 ──
  listDepts: () => request<SystemDept[]>('/system/api/v1/system/depts'),
  createDept: (d: DeptWrite) =>
    request<void>('/system/api/v1/system/depts', { method: 'POST', body: JSON.stringify(d) }),
  updateDept: (id: string, d: Partial<DeptWrite>) =>
    request<void>(`/system/api/v1/system/depts/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
  deleteDept: (id: string) => request<void>(`/system/api/v1/system/depts/${id}`, { method: 'DELETE' }),

  // ── 岗位 ──
  listPosts: (q: ListQuery) => request<PageResult<SystemPost>>('/system/api/v1/system/posts', { query: q }),
  listAllPosts: () => request<SystemPost[]>('/system/api/v1/system/posts/all'),
  createPost: (p: PostWrite) =>
    request<void>('/system/api/v1/system/posts', { method: 'POST', body: JSON.stringify(p) }),
  updatePost: (id: string, p: Partial<PostWrite>) =>
    request<void>(`/system/api/v1/system/posts/${id}`, { method: 'PUT', body: JSON.stringify(p) }),
  deletePosts: (ids: string[]) =>
    request<void>('/system/api/v1/system/posts', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 租户 ──
  listTenants: (q: ListQuery) => request<PageResult<SystemTenant>>('/system/api/v1/platform/tenants', { query: q }),
  createTenant: (t: TenantWrite) =>
    request<void>('/system/api/v1/platform/tenants', { method: 'POST', body: JSON.stringify(t) }),
  updateTenant: (id: string, t: Partial<TenantWrite>) =>
    request<void>(`/system/api/v1/platform/tenants/${id}`, { method: 'PUT', body: JSON.stringify(t) }),
  deleteTenants: (ids: string[]) =>
    request<void>('/system/api/v1/platform/tenants', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 字典 ──
  listDictTypes: (q: ListQuery) => request<PageResult<DictType>>('/system/api/v1/system/dict/types', { query: q }),
  createDictType: (d: DictTypeWrite) =>
    request<void>('/system/api/v1/system/dict/types', { method: 'POST', body: JSON.stringify(d) }),
  updateDictType: (id: string, d: Partial<DictTypeWrite>) =>
    request<void>(`/system/api/v1/system/dict/types/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
  deleteDictTypes: (ids: string[]) =>
    request<void>('/system/api/v1/system/dict/types', { method: 'DELETE', body: JSON.stringify({ ids }) }),
  listDictItems: (dictType: string) =>
    request<DictItem[]>('/system/api/v1/system/dict/items', { query: { dict_type: dictType } }),
  createDictItem: (i: DictItemWrite) =>
    request<void>('/system/api/v1/system/dict/items', { method: 'POST', body: JSON.stringify(i) }),
  updateDictItem: (id: string, i: Partial<DictItemWrite>) =>
    request<void>(`/system/api/v1/system/dict/items/${id}`, { method: 'PUT', body: JSON.stringify(i) }),
  deleteDictItems: (ids: string[]) =>
    request<void>('/system/api/v1/system/dict/items', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 参数配置 ──
  listConfigs: (q: ListQuery) => request<PageResult<SystemConfig>>('/system/api/v1/system/configs', { query: q }),
  createConfig: (c: ConfigWrite) =>
    request<void>('/system/api/v1/system/configs', { method: 'POST', body: JSON.stringify(c) }),
  updateConfig: (id: string, c: Partial<ConfigWrite>) =>
    request<void>(`/system/api/v1/system/configs/${id}`, { method: 'PUT', body: JSON.stringify(c) }),
  deleteConfigs: (ids: string[]) =>
    request<void>('/system/api/v1/system/configs', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 日志 ──
  listLoginLogs: (q: ListQuery) => request<PageResult<LoginLog>>('/system/api/v1/system/loginlogs', { query: q }),
  cleanLoginLogs: () => request<void>('/system/api/v1/system/loginlogs', { method: 'DELETE' }),
  removeLoginLogs: (ids: string[]) =>
    request<void>('/system/api/v1/system/loginlogs/batch', { method: 'DELETE', body: JSON.stringify({ ids }) }),
  listOperationLogs: (q: ListQuery) =>
    request<PageResult<OperationLog>>('/system/api/v1/system/operationlogs', { query: q }),
  cleanOperationLogs: () => request<void>('/system/api/v1/system/operationlogs', { method: 'DELETE' }),
  removeOperationLogs: (ids: string[]) =>
    request<void>('/system/api/v1/system/operationlogs/batch', { method: 'DELETE', body: JSON.stringify({ ids }) }),
}
