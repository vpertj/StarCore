import { writable } from 'svelte/store'
import { KEYS } from './constants.js'

const KEY = KEYS.EDITOR_SETTINGS
const VERSION_KEY = KEY + '-v'
const VERSION = 7

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
}

/**
 * Load settings from localStorage (fast cache).
 * Returns defaults on first run or if the cache is empty.
 */
function loadFromCache() {
  const savedVersion = parseInt(localStorage.getItem(VERSION_KEY) || '0', 10)
  const saved = localStorage.getItem(KEY)

  if (!saved) {
    localStorage.setItem(VERSION_KEY, String(VERSION))
    return { ...defaults }
  }

  try {
    const parsed = JSON.parse(saved)

    // Version migration: preserve existing settings, fill in new defaults
    if (savedVersion < VERSION) {
      // V5→V6: cursorBlink (boolean) → cursorBlinkStyle (string)
      if (typeof parsed.cursorBlink === 'boolean') {
        parsed.cursorBlinkStyle = parsed.cursorBlink ? 'blink' : 'solid'
        delete parsed.cursorBlink
      }
      const migrated = { ...defaults, ...parsed }
      saveToCache(migrated)
      localStorage.setItem(VERSION_KEY, String(VERSION))
      return migrated
    }

    return { ...defaults, ...parsed }
  } catch (e) {
    // Don't delete stored data — preserve it for debugging.
    // Return defaults for this session only; next load will re-attempt parse.
    console.error('[editorSettings] Failed to parse cached settings, using defaults:', e)
    localStorage.setItem(VERSION_KEY, String(VERSION))
    return { ...defaults }
  }
}

function saveToCache(settings) {
  try { localStorage.setItem(KEY, JSON.stringify(settings)) } catch {}
}

// --- Store (fast init from localStorage, then backend syncs in the background) ---

export const editorSettings = writable(loadFromCache())

// Mirror to localStorage on every change (fast cache for next startup)
editorSettings.subscribe(v => {
  saveToCache(v)
})

let _backendReady = false

/**
 * Initialize editor settings from the Go backend file system.
 * Backend is the authoritative source; localStorage is only a fast cache.
 * Call this once after the Wails runtime is ready (e.g. App.svelte onMount).
 */
export async function initEditorSettings() {
  if (typeof window === 'undefined') return

  try {
    const raw = await window.backend?.LoadEditorSettings()
    if (raw) {
      const backendSettings = JSON.parse(raw)
      if (backendSettings && typeof backendSettings === 'object') {
        // Merge: current cache as base, backend overrides (backend is authoritative)
        const current = loadFromCache()
        const merged = { ...defaults, ...current, ...backendSettings }
        editorSettings.set(merged)
        saveToCache(merged)
      }
    }
  } catch (e) {
    // Backend not available or file doesn't exist — keep using localStorage value.
    // This is normal on first run or if the backend isn't ready yet.
  }

  _backendReady = true
}

// Persist to backend file system on every change (authoritative store)
editorSettings.subscribe(v => {
  if (!_backendReady) return
  try {
    window.backend?.SaveEditorSettings(JSON.stringify(v))
  } catch (e) {
    // Silently ignore — localStorage is the fallback
  }
})

export function updateEditorSetting(key, value) {
  editorSettings.update(s => ({ ...s, [key]: value }))
}
