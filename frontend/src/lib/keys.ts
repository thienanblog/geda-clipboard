/** Platform-aware keyboard shortcut labels and matching. */

export type Platform = 'darwin' | 'windows' | string

let platform: Platform = 'darwin'

export function setPlatform(value: Platform): void {
  platform = value
  document.documentElement.dataset.platform = value
}

export function isMac(): boolean {
  return platform === 'darwin'
}

/** Symbols used to render shortcut hints in the UI. */
export const sym = {
  get cmd() {
    return isMac() ? '⌘' : 'Ctrl'
  },
  get alt() {
    return isMac() ? '⌥' : 'Alt'
  },
  get shift() {
    return isMac() ? '⇧' : 'Shift'
  },
  get del() {
    return isMac() ? '⌫' : 'Backspace'
  },
  get enter() {
    return isMac() ? '↩' : 'Enter'
  },
}

/** Joins shortcut parts the way each platform writes them: macOS packs symbols
 *  together ("⌥⇧⌘⌫"), Windows separates words ("Ctrl+Shift+Alt+Backspace"). */
export function combo(...parts: string[]): string {
  return isMac() ? parts.join('') : parts.join('+')
}

/** True when the event carries the platform's primary modifier. */
export function hasPrimary(event: KeyboardEvent): boolean {
  return isMac() ? event.metaKey : event.ctrlKey
}

/** Renders a stored hotkey spec such as "cmd+shift+v" for display. */
export function formatHotkey(spec: string): string {
  if (!spec) return 'None'

  const order = ['ctrl', 'alt', 'option', 'opt', 'shift', 'cmd', 'command', 'meta', 'super', 'win']
  const parts = spec.toLowerCase().split('+').map((p) => p.trim()).filter(Boolean)

  const mods = parts.filter((p) => order.includes(p))
  const keys = parts.filter((p) => !order.includes(p))

  const rendered = mods
    .sort((a, b) => order.indexOf(a) - order.indexOf(b))
    .map((mod) => {
      switch (mod) {
        case 'ctrl':
          return isMac() ? '⌃' : 'Ctrl'
        case 'alt':
        case 'option':
        case 'opt':
          return sym.alt
        case 'shift':
          return sym.shift
        default:
          return sym.cmd
      }
    })

  const key = keys.map(keyLabel).join('')
  return isMac() ? [...rendered, key].join('') : [...rendered, key].join('+')
}

function keyLabel(key: string): string {
  switch (key) {
    case 'space':
      return 'Space'
    case 'return':
    case 'enter':
      return sym.enter
    case 'escape':
    case 'esc':
      return 'Esc'
    case 'delete':
      return sym.del
    case 'up':
      return '↑'
    case 'down':
      return '↓'
    case 'left':
      return '←'
    case 'right':
      return '→'
    default:
      return key.length === 1 ? key.toUpperCase() : key.toUpperCase()
  }
}

/** Converts a KeyboardEvent into a hotkey spec the Go side can parse, or "" if
 *  the combination is not a usable global shortcut. */
export function eventToSpec(event: KeyboardEvent): string {
  const mods: string[] = []
  if (event.ctrlKey) mods.push('ctrl')
  if (event.altKey) mods.push('alt')
  if (event.shiftKey) mods.push('shift')
  if (event.metaKey) mods.push('cmd')

  if (mods.length === 0) return ''

  const key = normaliseKey(event)
  if (!key) return ''

  return [...mods, key].join('+')
}

function normaliseKey(event: KeyboardEvent): string {
  const code = event.code

  if (/^Key[A-Z]$/.test(code)) return code.slice(3).toLowerCase()
  if (/^Digit[0-9]$/.test(code)) return code.slice(5)
  if (/^F([1-9]|1[0-2])$/.test(code)) return code.toLowerCase()

  switch (code) {
    case 'Space':
      return 'space'
    case 'Enter':
      return 'return'
    case 'Tab':
      return 'tab'
    case 'ArrowUp':
      return 'up'
    case 'ArrowDown':
      return 'down'
    case 'ArrowLeft':
      return 'left'
    case 'ArrowRight':
      return 'right'
    default:
      return ''
  }
}
