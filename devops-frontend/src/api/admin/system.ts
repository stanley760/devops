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

/** Backend UserCreateReq/UserUpdateReq: camelCase keys, numeric gender, string IDs. */
export interface UserWrite {
  username: string
  nickname: string
  deptId: string
  postIds: string[]
  roleIds: string[]
  /** Required on create; ignored by the update endpoint. */
  password?: string
  email?: string
  mobile?: string
  /** 0 unknown, 1 male, 2 female. */
  gender?: number
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
  listUsers: (q: ListQuery) => request<PageResult<SystemUser>>('/system/api/v1/users', { query: q }),
  getUser: (id: string) => request<SystemUser>(`/system/api/v1/users/${id}`),
  createUser: (u: UserWrite) =>
    request<void>('/system/api/v1/users', { method: 'POST', body: JSON.stringify(u) }),
  updateUser: (id: string, u: Partial<UserWrite>) =>
    request<void>(`/system/api/v1/users/${id}`, { method: 'PUT', body: JSON.stringify(u) }),
  updateUserPassword: (id: string, p: { password: string }) =>
    request<void>(`/system/api/v1/users/${id}/password`, { method: 'PUT', body: JSON.stringify(p) }),
  deleteUser: (id: string) => request<void>(`/system/api/v1/users/${id}`, { method: 'DELETE' }),

  // ── 角色 ──
  listRoles: (q: ListQuery) => request<PageResult<SystemRole>>('/system/api/v1/roles', { query: q }),
  /** Backend has no /roles/all; fetch a full page via the list endpoint. */
  listAllRoles: () =>
    request<PageResult<SystemRole>>('/system/api/v1/roles', {
      query: { page_num: 1, page_size: 1000 },
    }).then((res) => res.list),
  createRole: (r: RoleWrite) =>
    request<void>('/system/api/v1/roles', { method: 'POST', body: JSON.stringify(r) }),
  updateRole: (id: string, r: Partial<RoleWrite>) =>
    request<void>(`/system/api/v1/roles/${id}`, { method: 'PUT', body: JSON.stringify(r) }),
  getRole: (id: string) => request<SystemRole & { menuIds: string[] }>(`/system/api/v1/roles/${id}`),
  assignRoleMenus: (id: string, menuIds: string[]) =>
    request<void>(`/system/api/v1/roles/${id}/menus`, { method: 'PUT', body: JSON.stringify({ menuIds }) }),
  deleteRole: (id: string) => request<void>(`/system/api/v1/roles/${id}`, { method: 'DELETE' }),

  // ── 菜单 ──
  listMenus: () => request<SystemMenu[]>('/system/api/v1/menus'),
  listMenuTree: () => request<SystemMenu[]>('/system/api/v1/menus/tree'),
  createMenu: (m: MenuWrite) =>
    request<void>('/system/api/v1/menus', { method: 'POST', body: JSON.stringify(m) }),
  updateMenu: (id: string, m: Partial<MenuWrite>) =>
    request<void>(`/system/api/v1/menus/${id}`, { method: 'PUT', body: JSON.stringify(m) }),
  deleteMenu: (id: string) => request<void>(`/system/api/v1/menus/${id}`, { method: 'DELETE' }),

  // ── 部门 ──
  listDepts: () => request<SystemDept[]>('/system/api/v1/depts'),
  createDept: (d: DeptWrite) =>
    request<void>('/system/api/v1/depts', { method: 'POST', body: JSON.stringify(d) }),
  updateDept: (id: string, d: Partial<DeptWrite>) =>
    request<void>(`/system/api/v1/depts/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
  deleteDept: (id: string) => request<void>(`/system/api/v1/depts/${id}`, { method: 'DELETE' }),

  // ── 岗位 ──
  listPosts: (q: ListQuery) => request<PageResult<SystemPost>>('/system/api/v1/posts', { query: q }),
  /** Backend has no /posts/all; fetch a full page via the list endpoint. */
  listAllPosts: () =>
    request<PageResult<SystemPost>>('/system/api/v1/posts', {
      query: { page_num: 1, page_size: 1000 },
    }).then((res) => res.list),
  createPost: (p: PostWrite) =>
    request<void>('/system/api/v1/posts', { method: 'POST', body: JSON.stringify(p) }),
  updatePost: (id: string, p: Partial<PostWrite>) =>
    request<void>(`/system/api/v1/posts/${id}`, { method: 'PUT', body: JSON.stringify(p) }),
  deletePost: (id: string) => request<void>(`/system/api/v1/posts/${id}`, { method: 'DELETE' }),

  // ── 租户 ──
  listTenants: (q: ListQuery) => request<PageResult<SystemTenant>>('/system/api/v1/tenants', { query: q }),
  createTenant: (t: TenantWrite) =>
    request<void>('/system/api/v1/tenants', { method: 'POST', body: JSON.stringify(t) }),
  updateTenant: (id: string, t: Partial<TenantWrite>) =>
    request<void>(`/system/api/v1/tenants/${id}`, { method: 'PUT', body: JSON.stringify(t) }),
  deleteTenant: (id: string) => request<void>(`/system/api/v1/tenants/${id}`, { method: 'DELETE' }),

  // ── 字典 ──
  listDictTypes: (q: ListQuery) => request<PageResult<DictType>>('/system/api/v1/dict-types', { query: q }),
  createDictType: (d: DictTypeWrite) =>
    request<void>('/system/api/v1/dict-types', { method: 'POST', body: JSON.stringify(d) }),
  updateDictType: (id: string, d: Partial<DictTypeWrite>) =>
    request<void>(`/system/api/v1/dict-types/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
  deleteDictType: (id: string) => request<void>(`/system/api/v1/dict-types/${id}`, { method: 'DELETE' }),
  listDictItems: (dictType: string) =>
    request<DictItem[]>('/system/api/v1/dict-data', { query: { dict_type: dictType } }),
  createDictItem: (i: DictItemWrite) =>
    request<void>('/system/api/v1/dict-data', { method: 'POST', body: JSON.stringify(i) }),
  updateDictItem: (id: string, i: Partial<DictItemWrite>) =>
    request<void>(`/system/api/v1/dict-data/${id}`, { method: 'PUT', body: JSON.stringify(i) }),
  deleteDictItem: (id: string) => request<void>(`/system/api/v1/dict-data/${id}`, { method: 'DELETE' }),

  // ── 参数配置 ──
  listConfigs: (q: ListQuery) => request<PageResult<SystemConfig>>('/system/api/v1/configs', { query: q }),
  createConfig: (c: ConfigWrite) =>
    request<void>('/system/api/v1/configs', { method: 'POST', body: JSON.stringify(c) }),
  updateConfig: (id: string, c: Partial<ConfigWrite>) =>
    request<void>(`/system/api/v1/configs/${id}`, { method: 'PUT', body: JSON.stringify(c) }),
  deleteConfig: (id: string) => request<void>(`/system/api/v1/configs/${id}`, { method: 'DELETE' }),

  // ── 日志 ──
  listLoginLogs: (q: ListQuery) => request<PageResult<LoginLog>>('/system/api/v1/login-logs', { query: q }),
  cleanLoginLogs: () => request<void>('/system/api/v1/login-logs', { method: 'DELETE' }),
  removeLoginLogs: (ids: string[]) =>
    request<void>('/system/api/v1/login-logs', { query: { ids: ids.join(',') }, method: 'DELETE' }),
  listOperationLogs: (q: ListQuery) =>
    request<PageResult<OperationLog>>('/system/api/v1/operation-logs', { query: q }),
  cleanOperationLogs: () => request<void>('/system/api/v1/operation-logs', { method: 'DELETE' }),
  removeOperationLogs: (ids: string[]) =>
    request<void>('/system/api/v1/operation-logs', { query: { ids: ids.join(',') }, method: 'DELETE' }),
}
