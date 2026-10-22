/**
 * devops-system 系统管理接口：用户 / 角色 / 菜单 / 部门 / 岗位 / 租户 /
 * 字典 / 参数 / 登录日志 / 操作日志。
 *
 * 命名约定（与后端 DTO 严格一致，改动前请先核对 Go 侧 types 包）：
 *   - **读**接口（DTO）统一 camelCase：deptName / leaderName / dataScope / configKey…
 *   - **写**接口（Save/Req）统一 snake_case：config_key、dict_type、css_class、
 *     keep_alive、always_show、data_scope、leader_user_id、contact_name、expire_time…
 * 改任何字段前务必对照 devops-system/internal/types/*.go。
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

/**
 * 用户读模型。
 * 注意字段来自两个不同的后端响应：列表行（GET /users）不含 roleIds / deptName，
 * 详情（GET /users/{id}）含 roleIds；deptName 现已由后端一并返回。
 */
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
  /** 仅详情接口返回 */
  roleIds?: string[]
  remark?: string
  status?: number
  loginIp?: string
  loginDate?: string
}

/** 写接口：deptId 必填（后端 required + ID 解析），空串会返回 400。 */
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
  /** JSON 数组字符串（如 '["101"]'），用 parseDeptIDsJson 解析 */
  dataScopeDeptIds?: string
  status: number
  type?: number
  remark?: string
  menuIds?: string[]
  createTime?: string
}

/**
 * 解析 dataScopeDeptIds（JSON 数组字符串）为 id 数组。
 * 后端也可能写入裸数字字符串（脏数据），这里一并兼容，避免解析失败让编辑表单崩掉。
 */
export function parseDeptIDsJson(raw?: string): string[] {
  if (!raw) return []
  try {
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) return parsed.map(String).filter(Boolean)
  } catch {
    // 非 JSON：按逗号分隔兜底
    return raw
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
  }
  return []
}

/** 把 id 数组编码为后端期望的 JSON 数组字符串。 */
export function encodeDeptIDsJson(ids: string[]): string {
  return JSON.stringify(ids.map(Number).filter((n) => Number.isFinite(n) && n > 0))
}

/**
 * 写接口：
 *  - data_scope_dept_ids 是 **JSON 数组字符串**（如 '["101"]'），不是逗号分隔；
 *    与 post_ids 同一种存储格式，传错会被后端判为"未提供部门集合"（400）。
 *  - 该字段在读取侧同样是 JSON 数组字符串，需要 JSON.parse 才能拿到 id 列表。
 */
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
  createTime?: string
  children?: SysDept[]
}

/** 写接口：无 remark 字段（后端 DeptSaveReq 未提供），顶级部门 parentId 传 '0' 而非空串。 */
export interface DeptSave {
  name: string
  parentId?: string
  sort?: number
  leader_user_id?: string
  phone?: string
  email?: string
  status?: number
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

/**
 * 写接口：expire_time 需为 RFC3339；编辑时传 null 显式清空（永不过期），
 * 不能靠省略字段——后端用 *time.Time 区分「清空」与「不修改」。
 */
export interface TenantSave {
  name: string
  code?: string
  contact_name?: string
  contact_mobile?: string
  expire_time?: string | null
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

/**
 * 字典类型编码约定（与后端常量保持一致）：
 *   sys_captcha_type — 验证码类型，sys.captcha.type 的取值域
 *   sys_status       — 通用启用/停用状态
 */
export const DICT_CAPTCHA_TYPE = 'sys_captcha_type'
export const DICT_STATUS = 'sys_status'

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
  userId: string
  ip: string
  userAgent: string
  geoCountry: string
  geoProvince: string
  geoCity: string
  geoIsp: string
  result: number
  code: number
  createTime: string
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
