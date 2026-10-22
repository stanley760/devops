/**
 * devops-system 认证接口：验证码 / 登录 / 双 token 刷新 / 会话信息 / 动态菜单。
 */
import { http, type Query } from './http'
import { session, type TokenPair, type TenantBrief } from './session'

const BASE = '/system/api/v1/auth'

export type CaptchaType = 'click' | 'slide' | 'dragdrop' | 'rotate'

export interface CaptchaChallenge {
  id: string
  type: CaptchaType
  image: string
  thumb?: string
  width: number
  height: number
  /** click 型的目标形状提示，如 "circle,star" */
  text?: string
  /** slide/dragdrop：拼图块在主图内的初始左上角坐标（px） */
  thumb_x?: number
  thumb_y?: number
  /** slide/dragdrop：拼图块图片实际尺寸（px），60–70 随机 */
  thumb_width?: number
  thumb_height?: number
  /** rotate：待旋转缩略图边长（px），140–170 */
  thumb_size?: number
  /**
   * 后端是否开启了验证码校验。
   * false 时登录页跳过验证步骤直接提交（服务端 Verify 是空操作）；
   * 缺失时按开启处理（旧后端没有这个字段）。
   */
  enabled?: boolean
}

/** 登录验证码作答（按挑战类型使用不同字段）。 */
export interface CaptchaAnswer {
  captcha_id: string
  captcha_points?: Array<{ x: number; y: number }>
  captcha_x?: number
  captcha_y?: number
  captcha_angle?: number
}

export interface LoginPayload {
  tenant_code?: string
  username: string
  password: string
  captcha?: CaptchaAnswer
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

export const authApi = {
  captcha(tenantCode?: string): Promise<CaptchaChallenge> {
    return http.get<CaptchaChallenge>(`${BASE}/captcha`, { tenant_code: tenantCode } as Query, { auth: false })
  },
  login(payload: LoginPayload): Promise<LoginResult> {
    return http.post<LoginResult>(`${BASE}/login`, payload, { auth: false })
  },
  refresh(refreshToken: string): Promise<TokenPair> {
    return http.post<TokenPair>(`${BASE}/refresh`, { refresh_token: refreshToken }, { auth: false })
  },
  logout(): Promise<void> {
    return http.post<void>(`${BASE}/logout`)
  },
  info(): Promise<AuthInfo> {
    return http.get<AuthInfo>(`${BASE}/info`)
  },
  routers(): Promise<RouterNode[]> {
    return http.get<RouterNode[]>(`${BASE}/routers`)
  },
  updateProfile(payload: Partial<Pick<AuthUser, 'nickname' | 'email' | 'mobile'>>): Promise<void> {
    return http.put<void>(`${BASE}/profile`, payload)
  },
  changePassword(payload: { old_password: string; new_password: string }): Promise<void> {
    return http.put<void>(`${BASE}/profile/password`, payload)
  },
}

/** 组装验证码答案：按挑战类型映射到后端字段。 */
export function solveCaptcha(ch: CaptchaChallenge, solve: { points?: Array<{ x: number; y: number }>; x?: number; y?: number; angle?: number }): CaptchaAnswer {
  const a: CaptchaAnswer = { captcha_id: ch.id }
  if (ch.type === 'click') a.captcha_points = solve.points ?? []
  if (ch.type === 'slide' || ch.type === 'dragdrop') {
    a.captcha_x = solve.x ?? 0
    a.captcha_y = solve.y ?? 0
  }
  if (ch.type === 'rotate') a.captcha_angle = solve.angle ?? 0
  return a
}

export type { TokenPair, TenantBrief }

export function isLoggedIn(): boolean {
  return session.loggedIn
}
