export function formatTime(value) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Date(date.getTime() + 8 * 3600000).toISOString().slice(0, 19).replace('T', ' ')
}
