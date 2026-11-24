export type { Appearance, Density, Theme } from '@/lib/appearance'

// ── 通用 ──
export interface PageResult<T> {
  list: T[]
  total: number
  page_num: number
  page_size: number
  pages: number
}

// ── Admin 认证 ──
export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_in: number
}

export interface TenantBrief {
  id: string
  name: string
  code: string
}

export interface LoginResult extends TokenPair {
  tenant: TenantBrief
}

export interface AuthUser {
  id: string
  username: string
  nickname?: string
  email?: string
  mobile?: string
  /** Backend int8: 0 unknown, 1 male, 2 female. */
  gender?: number
  avatar?: string
  deptId?: string
  postIds?: string[]
  remark?: string
  loginIp?: string
  loginDate?: string
}

export interface AuthInfo {
  user: AuthUser
  tenant: TenantBrief
  roles: string[]
  permissions: string[]
}

export interface RouterNode {
  id: string
  name: string
  path: string
  component: string
  componentName?: string
  icon?: string
  sort: number
  visible: boolean
  keepAlive?: boolean
  alwaysShow?: boolean
  children?: RouterNode[]
}

// ── 行为式验证码 ──
export type CaptchaType = 'click' | 'slide' | 'dragdrop' | 'rotate'

export interface CaptchaChallenge {
  id: string
  type: CaptchaType
  image: string
  thumb?: string
  width: number
  height: number
  text?: string
  thumb_x?: number
  thumb_y?: number
  thumb_width?: number
  thumb_height?: number
  thumb_size?: number
  enabled?: boolean
}

export interface CaptchaAnswer {
  points?: Array<{ x: number; y: number }>
  x?: number
  y?: number
  angle?: number
}

// ── Portal ──
export interface PortalTokenPair {
  access_token: string
  refresh_token: string
  expires_in: number
  issued_at: number
}

export interface PublicProfile {
  id: number
  username: string
  nickname: string
  email: string
  mobile: string
  /** Backend int8: 0 unknown, 1 male, 2 female. */
  gender: number
  avatar: string
  created_at: number
}

// ── Admin system 模块（读 camelCase；写 snake_case 由 api 层映射） ──
export interface SystemUser {
  id: string
  username: string
  nickname: string
  deptId: string
  postIds: string[]
  roleIds: string[]
  email?: string
  mobile?: string
  /** Backend int8: 0 unknown, 1 male, 2 female. */
  gender?: number
  avatar?: string
  remark?: string
  status: number
  createdAt?: string
}

export interface SystemRole {
  id: string
  name: string
  code: string
  sort: number
  dataScope: number
  /** Backend stores this as a JSON array string (e.g. '["101"]'); decode with parseDeptIDsJson. */
  dataScopeDeptIds: string
  /** Backend type: 1 builtin, 2 custom. */
  type: number
  status: number
  remark?: string
}

export interface SystemMenu extends RouterNode {
  type: number // 1目录 2菜单 3按钮
  parentId: string
  permission?: string
  status: number
  children?: SystemMenu[]
}

export interface SystemDept {
  id: string
  name: string
  parentId: string
  sort: number
  leaderUserId?: string
  phone?: string
  email?: string
  status: number
}

export interface SystemPost {
  id: string
  code: string
  name: string
  sort: number
  status: number
  remark?: string
}

export interface SystemTenant {
  id: string
  name: string
  code: string
  contactName: string
  contactMobile: string
  expireTime?: string
  remark?: string
}

export interface DictType {
  id: string
  name: string
  type: string
  status: number
  remark?: string
}

export interface DictItem {
  id: string
  dictType: string
  label: string
  value: string
  sort: number
}

export interface SystemConfig {
  id: string
  configKey: string
  configName: string
  configValue: string
  type: number // 1内置 2自定义
  visible: boolean
  remark?: string
}

/** Backend types.LoginLogDTO — camelCase wire fields, IDs serialized as strings. */
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
  /** 0 success, 1 failure. */
  result: number
  /** Auth business code; the error text is not part of the DTO. */
  code: number
  createTime: string
}

/** Backend types.OperationLogDTO — camelCase wire fields. */
export interface OperationLog {
  id: string
  traceId: string
  userId: string
  userName: string
  userType: number
  userTypeName: string
  module: string
  subModule: string
  bizId: string
  /** Change summary; no dedicated action/title fields exist. */
  contents: string
  /** Extra JSON text (e.g. duration); no dedicated request/response fields exist. */
  extra: string
  requestMethod: string
  requestUrl: string
  userIp: string
  userAgent: string
  geoCountry: string
  geoProvince: string
  geoCity: string
  geoIsp: string
  /** 0 success, 1 failure. */
  result: number
  /** HTTP status code. */
  code: number
  createTime: string
}
