/** Formatting helpers shared by the popup and the detail column. */

/** Formats a Go time.Time (RFC3339 string) the way the reference app does:
 *  "July 30, 2026 10:40". Returns "" for missing/unparseable values. */
export function formatCopyTime(value: unknown): string {
  const date = toDate(value)
  if (!date) return ''
  const day = date.toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })
  const time = date.toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
  })
  return `${day} ${time}`
}

/** Formats a timestamp as a short relative age, e.g. "just now", "4m", "3h". */
export function formatAge(value: unknown): string {
  const date = toDate(value)
  if (!date) return ''

  const seconds = Math.max(0, (Date.now() - date.getTime()) / 1000)
  if (seconds < 45) return 'just now'
  if (seconds < 3600) return `${Math.round(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h ago`
  const days = Math.round(seconds / 86400)
  if (days < 30) return `${days}d ago`
  return formatCopyTime(value)
}

function toDate(value: unknown): Date | null {
  if (!value) return null
  const date = value instanceof Date ? value : new Date(String(value))
  return Number.isNaN(date.getTime()) ? null : date
}

/** Formats a byte count for the detail column. */
export function formatBytes(bytes: number | undefined): string {
  if (!bytes || bytes < 0) return ''
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

/** Collapses whitespace so a multi-line payload fits on one row. */
export function oneLine(text: string | undefined): string {
  if (!text) return ''
  return text.replace(/\s+/g, ' ').trim()
}

/** Human label for an entry, mirroring the Go side's Preview(). */
export function itemLabel(item: { kind: string; text?: string }): string {
  if (item.kind === 'image') return 'Image'
  const collapsed = oneLine(item.text)
  return collapsed === '' ? '(whitespace)' : collapsed
}

/** Shortens a label to `budget` characters by cutting out the middle rather
 *  than the tail, so both ends stay readable -- the end of a path or URL is
 *  usually what tells two entries apart. Counting code points keeps accented
 *  text and emoji from being split in half. */
export function middleEllipsis(text: string, budget: number): string {
  if (budget < 8) return text
  const chars = [...text]
  if (chars.length <= budget) return text

  // Leading context is worth more than trailing, hence the 60/40 split.
  const head = Math.ceil((budget - 1) * 0.6)
  const tail = budget - 1 - head
  return chars.slice(0, head).join('') + '…' + (tail > 0 ? chars.slice(-tail).join('') : '')
}

/** Label for a list row: one line, middle-truncated to the row's budget. */
export function rowLabel(item: { kind: string; text?: string }, budget: number): string {
  return middleEllipsis(itemLabel(item), budget)
}

/** Counts lines and characters, for the detail column. */
export function textStats(text: string | undefined): string {
  if (!text) return ''
  const chars = [...text].length
  const lines = text.split('\n').length
  const charLabel = `${chars.toLocaleString()} character${chars === 1 ? '' : 's'}`
  return lines > 1 ? `${charLabel} · ${lines} lines` : charLabel
}
