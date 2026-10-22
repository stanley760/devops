/**
 * devops-system 系统管理接口：用户 / 角色 / 菜单 / 部门 / 岗位 / 租户 /
 * 字典 / 参数 / 登录日志 / 操作日志。
 *
 * 命名约定（与后端 DTO 一致）：
 *   查询与读出多为 camelCase；写入体为 camelCase，少数历史字段 snake_case
 *   （config_key、dict_type、css_class、keep_alive、always_show、expire_time…）。
 * 权限码 system:<模块>:<动作>，403 时 UI 隐藏对应动作（后端仍强制校验）。
 */
import { http, type PageResult, type Query } from './http'

const V1 = '/system/api/v1'

export type { PageResult }

/** 列表查询参数：分页字段 + 业务过滤字段（空值在 http 层被剔除） */
export interface PageQuery extends Query {
  page_num?: number
  page_size?: number
}

/* ---------------- 用户 ---------------- */

export interface SysUser {
  id: string
  username: string
  nickname?: string
  email?: string
  mobile?: string
  gender?: number
  avatar?: string
  deptId?: string
  deptName?: string
  postIds?: string[]
  roleIds?: string[]
  status?: number
  remark?: string
  loginIp?: string
  loginDate?: string
  createTime?: string
}

export interface UserSave {
  username: string
  nickname: string
  deptId: string
  password?: string
  postIds?: string[]
  roleIds?: string[]
  email?: string
  mobile?: string
  gender?: number
  avatar?: string
  remark?: string
  status?: number
}

export const userApi = {
  list: (params: PageQuery) => http.get<PageResult<SysUser>>(`${V1}/users`, params),
  get: (id: string) => http.get<SysUser>(`${V1}/users/${id}`),
  create: (body: UserSave) => http.post<void>(`${V1}/users`, body),
  update: (id: string, body: UserSave) => http.put<void>(`${V1}/users/${id}`, body),
  resetPassword: (id: string, password: string) =>
    http.put<void>(`${V1}/users/${id}/password`, { password }),
  remove: (id: string) => http.del<void>(`${V1}/users/${id}`),
}

/* ---------------- 角色 ---------------- */

export interface SysRole {
  id: string
  name: string
  code: string
  sort: number
  dataScope?: number
  dataScopeDeptIds?: string
  status: number
  type?: number
  remark?: string
  menuIds?: string[]
  createTime?: string
}

export interface RoleSave {
  name: string
  code?: string
  sort?: number
  data_scope?: number
  data_scope_dept_ids?: string
  status?: number
  remark?: string
}

export const roleApi = {
  list: (params: PageQuery) => http.get<PageResult<SysRole>>(`${V1}/roles`, params),
  get: (id: string) => http.get<SysRole>(`${V1}/roles/${id}`),
  create: (body: RoleSave) => http.post<void>(`${V1}/roles`, body),
  update: (id: string, body: RoleSave) => http.put<void>(`${V1}/roles/${id}`, body),
  assignMenus: (id: string, menuIds: string[]) =>
    http.put<void>(`${V1}/roles/${id}/menus`, { menuIds }),
  remove: (id: string) => http.del<void>(`${V1}/roles/${id}`),
}

/* ---------------- 菜单 ---------------- */

export type MenuType = 1 | 2 | 3 // 目录 / 菜单 / 按钮

export interface SysMenu {
  id: string
  name: string
  permission?: string
  type: MenuType
  sort: number
  parentId: string
  path?: string
  icon?: string
  component?: string
  componentName?: string
  status: number
  visible: boolean
  keepAlive: boolean
  alwaysShow: boolean
  createTime?: string
  children?: SysMenu[]
}

export interface MenuSave {
  name: string
  permission?: string
  type: MenuType
  sort?: number
  parentId?: string
  path?: string
  icon?: string
  component?: string
  component_name?: string
  status?: number
  visible?: boolean
  keep_alive?: boolean
  always_show?: boolean
}

export const menuApi = {
  tree: (params?: Query) => http.get<SysMenu[]>(`${V1}/menus`, params),
  get: (id: string) => http.get<SysMenu>(`${V1}/menus/${id}`),
  create: (body: MenuSave) => http.post<void>(`${V1}/menus`, body),
  update: (id: string, body: MenuSave) => http.put<void>(`${V1}/menus/${id}`, body),
  remove: (id: string) => http.del<void>(`${V1}/menus/${id}`),
}

/* ---------------- 部门 ---------------- */

export interface SysDept {
  id: string
  name: string
  parentId: string
  sort: number
  leaderUserId?: string
  leaderName?: string
  phone?: string
  email?: string
  status: number
  remark?: string
  children?: SysDept[]
}

export interface DeptSave {
  name: string
  parentId?: string
  sort?: number
  leader_user_id?: string
  phone?: string
  email?: string
  status?: number
  remark?: string
}

export const deptApi = {
  tree: (params?: Query) => http.get<SysDept[]>(`${V1}/depts`, params),
  get: (id: string) => http.get<SysDept>(`${V1}/depts/${id}`),
  create: (body: DeptSave) => http.post<void>(`${V1}/depts`, body),
  update: (id: string, body: DeptSave) => http.put<void>(`${V1}/depts/${id}`, body),
  remove: (id: string) => http.del<void>(`${V1}/depts/${id}`),
}

/* ---------------- 岗位 ---------------- */

export interface SysPost {
  id: string
  code: string
  name: string
  sort: number
  status: number
  remark?: string
  createTime?: string
}

export interface PostSave {
  code: string
  name: string
  sort?: number
  status?: number
  remark?: string
}

export const postApi = {
  list: (params: PageQuery) => http.get<PageResult<SysPost>>(`${V1}/posts`, params),
  get: (id: string) => http.get<SysPost>(`${V1}/posts/${id}`),
  create: (body: PostSave) => http.post<void>(`${V1}/posts`, body),
  update: (id: string, body: PostSave) => http.put<void>(`${V1}/posts/${id}`, body),
  remove: (id: string) => http.del<void>(`${V1}/posts/${id}`),
}

/* ---------------- 租户（平台级） ---------------- */

export interface SysTenant {
  id: string
  name: string
  code: string
  contactName?: string
  contactMobile?: string
  expireTime?: string
  status: number
  remark?: string
  createTime?: string
}

export interface TenantSave {
  name: string
  code?: string
  contact_name?: string
  contact_mobile?: string
  expire_time?: string
  remark?: string
  admin_username?: string
  admin_password?: string
}

export const tenantApi = {
  list: (params: PageQuery) => http.get<PageResult<SysTenant>>(`${V1}/tenants`, params),
  get: (id: string) => http.get<SysTenant>(`${V1}/tenants/${id}`),
  create: (body: TenantSave) => http.post<void>(`${V1}/tenants`, body),
  update: (id: string, body: TenantSave) => http.put<void>(`${V1}/tenants/${id}`, body),
  updateStatus: (id: string, status: number) =>
    http.put<void>(`${V1}/tenants/${id}/status`, { status }),
  remove: (id: string) => http.del<void>(`${V1}/tenants/${id}`),
}

/* ---------------- 字典 ---------------- */

export interface DictType {
  id: string
  name: string
  type: string
  status: number
  remark?: string
  createTime?: string
}

export interface DictTypeSave {
  name: string
  type?: string
  status?: number
  remark?: string
}

export interface DictData {
  id: string
  dictType: string
  label: string
  value: string
  sort: number
  status: number
  cssClass?: string
  remark?: string
  createTime?: string
}

export interface DictDataSave {
  dict_type: string
  label: string
  value: string
  sort?: number
  status?: number
  css_class?: string
  remark?: string
}

export const dictApi = {
  type: {
    list: (params: PageQuery) => http.get<PageResult<DictType>>(`${V1}/dict-types`, params),
    get: (id: string) => http.get<DictType>(`${V1}/dict-types/${id}`),
    create: (body: DictTypeSave) => http.post<void>(`${V1}/dict-types`, body),
    update: (id: string, body: DictTypeSave) => http.put<void>(`${V1}/dict-types/${id}`, body),
    remove: (id: string) => http.del<void>(`${V1}/dict-types/${id}`),
  },
  data: {
    list: (params: PageQuery) => http.get<PageResult<DictData>>(`${V1}/dict-data`, params),
    simple: (dictType: string) => http.get<DictData[]>(`${V1}/dict-data/simple`, { dict_type: dictType }),
    get: (id: string) => http.get<DictData>(`${V1}/dict-data/${id}`),
    create: (body: DictDataSave) => http.post<void>(`${V1}/dict-data`, body),
    update: (id: string, body: DictDataSave) => http.put<void>(`${V1}/dict-data/${id}`, body),
    remove: (id: string) => http.del<void>(`${V1}/dict-data/${id}`),
  },
}

/* ---------------- 参数配置 ---------------- */

export interface SysConfig {
  id: string
  configKey: string
  configName: string
  configValue: string
  type: number
  visible: boolean
  remark?: string
  createTime?: string
}

export interface ConfigSave {
  config_key?: string
  config_name: string
  config_value?: string
  type?: number
  visible?: boolean
  remark?: string
}

export const configApi = {
  list: (params: PageQuery) => http.get<PageResult<SysConfig>>(`${V1}/configs`, params),
  get: (id: string) => http.get<SysConfig>(`${V1}/configs/${id}`),
  byKey: (key: string) => http.get<SysConfig>(`${V1}/configs/key/${encodeURIComponent(key)}`),
  create: (body: ConfigSave) => http.post<void>(`${V1}/configs`, body),
  update: (id: string, body: ConfigSave) => http.put<void>(`${V1}/configs/${id}`, body),
  remove: (id: string) => http.del<void>(`${V1}/configs/${id}`),
}

/* ---------------- 日志 ---------------- */

export interface LoginLog {
  id: string
  username: string
  ip?: string
  location?: string
  browser?: string
  os?: string
  status?: number
  msg?: string
  loginTime?: string
}

export interface OperationLog {
  id: string
  title?: string
  action?: string
  method?: string
  operator?: string
  operatorName?: string
  ip?: string
  status?: number
  errorMsg?: string
  costMs?: number
  operTime?: string
}

export const loginLogApi = {
  list: (params: PageQuery) => http.get<PageResult<LoginLog>>(`${V1}/login-logs`, params),
  remove: (ids: string[]) => http.del<void>(`${V1}/login-logs`, { ids }),
  clean: () => http.del<void>(`${V1}/login-logs/clean`),
}

export const operationLogApi = {
  list: (params: PageQuery) => http.get<PageResult<OperationLog>>(`${V1}/operation-logs`, params),
  remove: (ids: string[]) => http.del<void>(`${V1}/operation-logs`, { ids }),
  clean: () => http.del<void>(`${V1}/operation-logs/clean`),
}
