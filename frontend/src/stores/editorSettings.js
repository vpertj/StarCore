import { writable } from 'svelte/store'
import { KEYS } from './constants.js'

const KEY = KEYS.EDITOR_SETTINGS
const VERSION_KEY = KEY + '-v'
const VERSION = 6

const defaults = {
  fontSize: 16,
  fontFamily: "'Lilex', 'Cascadia Code', 'JetBrains Mono', 'Consolas', 'monospace'",
  lineHeight: 1.6,
  wordWrap: true,
  showLineNumbers: true,
  showMinimap: false,
  cursorWidth: 4,
  cursorColor: '#ffcc00',
  cursorStyle: 'block',
  cursorBlinkStyle: 'blink',
  cursorHeight: 100,
  highlightTheme: 'one-dark',
}

function load() {
  const savedVersion = parseInt(localStorage.getItem(VERSION_KEY) || '0', 10)
  const saved = localStorage.getItem(KEY)

  if (savedVersion < VERSION || !saved) {
    localStorage.removeItem(KEY)
    localStorage.setItem(VERSION_KEY, String(VERSION))
    return { ...defaults }
  }

  try {
    const parsed = JSON.parse(saved)
    if (typeof parsed.cursorBlink === 'boolean') {
      parsed.cursorBlinkStyle = parsed.cursorBlink ? 'blink' : 'solid'
      delete parsed.cursorBlink
    }
    return { ...defaults, ...parsed }
  } catch {
    localStorage.removeItem(KEY)
    localStorage.setItem(VERSION_KEY, String(VERSION))
    return { ...defaults }
  }
}

export const editorSettings = writable(load())

editorSettings.subscribe(v => {
  try { localStorage.setItem(KEY, JSON.stringify(v)) } catch {}
})

export function updateEditorSetting(key, value) {
  editorSettings.update(s => ({ ...s, [key]: value }))
}
