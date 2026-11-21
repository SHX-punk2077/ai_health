// BIGINT 在 JSON 里是字符串，展示层统一转成可读文本
export function fmtDateTime(v?: string | null): string {
  if (!v) return '-'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  const pad = (n: number) => String(n).padStart(2, '0')
  return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate()) +
    ' ' + pad(d.getHours()) + ':' + pad(d.getMinutes())
}

export function fmtNumber(v?: number | null, unit = ''): string {
  if (v === null || v === undefined) return '-'
  return String(v) + (unit ? ' ' + unit : '')
}
