/**
 * 会话存储：双 token（access + refresh）与租户上下文。
 * 约定：页面与组件不得直接改这些键，只经 auth.ts / 本模块的清理入口。
 */
import { ref } from 'vue'

const K_ACCESS = 'devops.access_token'
const K_REFRESH = 'devops.refresh_token'
const K_TENANT = 'devops.tenant_id'
const K_TENANT_CODE = 'devops.tenant_code'
const K_VER = 'devops.session_ver'

/**
 * 登录态的响应式镜像。
 *
 * localStorage 本身不是响应式的：`computed(() => Boolean(localStorage.getItem(...)))`
 * 在首次求值时不会追踪到任何依赖，于是永远返回首次的结果。启动时是无 token 状态，
 * 求值一次缓存 false 之后，即便登录成功写入了 token，computed 也不会重新计算 ——
 * 路由守卫因此永远认为未登录，把用户弹回登录页（表现为「登录成功却跳不过去」）。
 * 所以写入时同步更新这个 ref，读取走 ref，让 computed 能正常失效。
 */
const loggedInRef = ref(Boolean(localStorage.getItem(K_ACCESS)))

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

let ver = Number(localStorage.getItem(K_VER) || 0)

/** 会话版本：登录 / 登出递增，旧请求据此判定自己是否已过期。 */
export function sessionVersion(): number {
  return ver
}

function bump(): number {
  ver += 1
  localStorage.setItem(K_VER, String(ver))
  return ver
}

export const session = {
  get accessToken(): string | null {
    return localStorage.getItem(K_ACCESS)
  },
  get refreshToken(): string | null {
    return localStorage.getItem(K_REFRESH)
  },
  get tenantId(): string | null {
    return localStorage.getItem(K_TENANT)
  },
  get tenantCode(): string | null {
    return localStorage.getItem(K_TENANT_CODE)
  },

  get loggedIn(): boolean {
    return loggedInRef.value
  },

  save(pair: TokenPair, tenant?: TenantBrief): number {
    localStorage.setItem(K_ACCESS, pair.access_token)
    localStorage.setItem(K_REFRESH, pair.refresh_token)
    if (tenant) {
      localStorage.setItem(K_TENANT, tenant.id)
      localStorage.setItem(K_TENANT_CODE, tenant.code)
    }
    loggedInRef.value = true
    return bump()
  },

  saveTokens(pair: TokenPair): number {
    localStorage.setItem(K_ACCESS, pair.access_token)
    localStorage.setItem(K_REFRESH, pair.refresh_token)
    loggedInRef.value = true
    return bump()
  },

  /** 登出 / 会话失效统一清理；返回新版本号供在途请求比对。 */
  clear(): number {
    localStorage.removeItem(K_ACCESS)
    localStorage.removeItem(K_REFRESH)
    localStorage.removeItem(K_TENANT)
    localStorage.removeItem(K_TENANT_CODE)
    loggedInRef.value = false
    return bump()
  },
}
