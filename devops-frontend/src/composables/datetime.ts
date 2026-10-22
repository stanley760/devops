/**
 * 时间展示工具。
 *
 * 后端统一输出 "YYYY-MM-DD HH:mm:ss"（无时区），所以这里**不能再用 new Date() 解析**：
 * `new Date('2026-10-22 17:07:24')` 会被当成本地时间解析，在部分浏览器/引擎下还会
 * 抛 Invalid Date（Safari 对非 ISO 格式的 Date 解析尤其宽松而不可靠）。
 *
 * 正确做法是把后端给的字符串当作**已经是本地墙钟读数**的文本，直接处理：
 * 要显示就原样显示，要拆分就按分隔符切。这样无论后端在什么时区落库，
 * 前端拿到的都是同一个"墙上时间"，不会因为解析而偏移几小时。
 */

/** 后端统一时间格式：YYYY-MM-DD HH:mm:ss */
export const DATE_TIME_PATTERN = /^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/

/** 只到日期：YYYY-MM-DD */
export const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/

/**
 * 格式化时间戳为 "YYYY-MM-DD HH:mm:ss"。
 *
 * 后端已经是这个格式，所以正常情况下原样返回即可；这里做的是**兜底**：
 * 遇到空值、null、或历史遗留的 RFC3339 也能给出合理结果，而不是显示 Invalid Date。
 *
 * @param value 后端返回的时间字符串
 * @param fallback 空值时显示的占位符
 */
export function formatDateTime(value?: string | null, fallback = '—'): string {
  const v = (value ?? '').trim()
  if (!v) return fallback
  // 标准格式：原样返回（不要 new Date，理由见文件头注释）
  if (DATE_TIME_PATTERN.test(v)) return v
  // 历史 RFC3339：截到秒并去掉时区尾巴，交给调用方看到一致的墙钟读数
  const m = /^(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2}:\d{2})/.exec(v)
  if (m) return `${m[1]} ${m[2]}`
  return v
}

/**
 * 只取日期部分：YYYY-MM-DD。
 */
export function formatDate(value?: string | null, fallback = '—'): string {
  const v = (value ?? '').trim()
  if (!v) return fallback
  if (DATE_PATTERN.test(v)) return v
  if (DATE_TIME_PATTERN.test(v)) return v.slice(0, 10)
  const m = /^(\d{4}-\d{2}-\d{2})/.exec(v)
  return m ? m[1] : v
}

/**
 * 只取时间部分：HH:mm:ss。
 */
export function formatTimeOfDay(value?: string | null, fallback = '—'): string {
  const v = (value ?? '').trim()
  if (!v) return fallback
  if (DATE_TIME_PATTERN.test(v)) return v.slice(11)
  const m = /[T ](\d{2}:\d{2}:\d{2})/.exec(v)
  if (m) return m[1]
  // 纯日期没有时间部分，如实返回占位符而不是把日期当时间显示
  return fallback
}

/**
 * 相对时间描述，用于"最近登录"这类需要一眼看出多久之前的场景。
 * 超过 7 天回落到具体日期，避免出现"180 天前"这种没用的信息。
 */
export function formatRelative(value?: string | null, fallback = '—'): string {
  const v = (value ?? '').trim()
  if (!v || !DATE_TIME_PATTERN.test(v)) return formatDateTime(v, fallback)
  const [d, t] = v.split(' ')
  const [y, mo, da] = d.split('-').map(Number)
  const [h, mi, s] = t.split(':').map(Number)
  const then = new Date(y, mo - 1, da, h, mi, s).getTime()
  if (Number.isNaN(then)) return v
  const diff = Date.now() - then
  if (diff < 0) return v
  const min = Math.floor(diff / 60000)
  if (min < 1) return '刚刚'
  if (min < 60) return `${min} 分钟前`
  const hour = Math.floor(min / 60)
  if (hour < 24) return `${hour} 小时前`
  const day = Math.floor(hour / 24)
  if (day < 7) return `${day} 天前`
  return d
}
