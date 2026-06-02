import { writable } from 'svelte/store'
import { KEYS } from './constants.js'

const KEY = KEYS.EDITOR_SETTINGS

const defaults = {
  fontSize: 16,
  fontFamily: "'Cascadia Code', 'JetBrains Mono', 'Fira Code', 'Consolas', 'monospace'",
  lineHeight: 1.7,
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
  try {
    const saved = localStorage.getItem(KEY)
    if (saved) {
      const parsed = JSON.parse(saved)
      // Migrate old cursorBlink boolean to cursorBlinkStyle string
      if (typeof parsed.cursorBlink === 'boolean') {
        parsed.cursorBlinkStyle = parsed.cursorBlink ? 'blink' : 'solid'
        delete parsed.cursorBlink
      }
      return { ...defaults, ...parsed }
    }
  } catch {}
  return { ...defaults }
}

export const editorSettings = writable(load())

editorSettings.subscribe(v => {
  try { localStorage.setItem(KEY, JSON.stringify(v)) } catch {}
})

export function updateEditorSetting(key, value) {
  editorSettings.update(s => ({ ...s, [key]: value }))
}
