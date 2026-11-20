export function formatDateTime(v: string | number | null | undefined): string {
  if (v === null || v === undefined || v === '') return '-'
  const d = new Date(typeof v === 'number' ? v : v.replace(' ', 'T'))
  if (Number.isNaN(d.getTime())) return String(v)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

export function formatDate(v: string | number | null | undefined): string {
  return formatDateTime(v).slice(0, 10)
}
