/**
 * UI 组件公共契约（props / slot 约定）——批次实现与页面引用的唯一类型来源。
 */

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'link'
export type ControlSize = 'sm' | 'md'

export interface SelectOption {
  value: string | number
  label: string
  disabled?: boolean
}

export interface Column {
  key: string
  title: string
  width?: string | number
  align?: 'left' | 'center' | 'right'
  /** 吸附右侧的操作列传 'col-actions' */
  className?: string
  /** 表头排序（本地） */
  sortable?: boolean
}

export type TagKind = 'default' | 'ok' | 'warn' | 'crit' | 'blue' | 'violet'
export type HealthKind = 'ok' | 'warn' | 'crit' | 'off'

export interface SpecRow {
  key: string
  val: string | number
}

export interface MetricDelta {
  value: number
  /** up=上升好 / down=下降好 / auto=按符号 / fresh=新增 */
  kind?: 'up' | 'down' | 'auto' | 'fresh'
  suffix?: string
}
