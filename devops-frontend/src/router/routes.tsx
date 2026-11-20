import { createElement, type ComponentType, type ReactElement } from 'react'
import type { RouterNode } from '@/lib/types'
import Dashboard from '@/views/Dashboard'
import Users from '@/views/system/Users'
import Roles from '@/views/system/Roles'
import Menus from '@/views/system/Menus'
import Depts from '@/views/system/Depts'
import Posts from '@/views/system/Posts'
import Tenants from '@/views/system/Tenants'
import Dict from '@/views/system/Dict'
import Configs from '@/views/system/Configs'
import LoginLogs from '@/views/monitor/LoginLogs'
import OperationLogs from '@/views/monitor/OperationLogs'

// component 键 → 页面组件（spec §4.4 的 11 键）
export const COMPONENT_MAP: Record<string, ComponentType> = {
  'dashboard/index': Dashboard,
  'system/user/index': Users,
  'system/role/index': Roles,
  'system/menu/index': Menus,
  'system/dept/index': Depts,
  'system/post/index': Posts,
  'system/dict/index': Dict,
  'system/config/index': Configs,
  'system/loginlog/index': LoginLogs,
  'system/operationlog/index': OperationLogs,
  'platform/tenant/index': Tenants,
}

export interface DynamicRouteDef {
  path: string
  element: ReactElement
}

export function buildDynamicRoutes(nodes: RouterNode[]): DynamicRouteDef[] {
  const defs: DynamicRouteDef[] = []
  const walk = (list: RouterNode[]) => {
    for (const n of list) {
      const rawPath = typeof n.path === 'string' ? n.path : ''
      const path = rawPath.replace(/^\/+/, '')
      if (!rawPath) {
        if (n.component) {
          console.warn(`[router] missing path for component key: ${n.component}`)
        }
      } else if (n.component && COMPONENT_MAP[n.component]) {
        const Comp = COMPONENT_MAP[n.component]!
        defs.push({ path: rawPath, element: createElement(Comp) })
      } else if (n.component) {
        // 未注册的 component 键：跳过并告警，不虚构菜单
        console.warn(`[router] unregistered component key: ${n.component} (path=${n.path})`)
      }
      if (n.children?.length) walk(n.children)
    }
  }
  walk(nodes)
  return defs
}

export function flattenMenus(nodes: RouterNode[]): RouterNode[] {
  const out: RouterNode[] = []
  const walk = (list: RouterNode[]) => {
    for (const n of list) {
      // 隐藏父节点的整棵子树都不应出现在菜单中
      if (!n.visible) continue
      out.push(n)
      if (n.children?.length) walk(n.children)
    }
  }
  walk(nodes)
  return out
}
