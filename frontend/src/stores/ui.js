import { writable, get } from 'svelte/store'
import { KEYS } from './constants.js'

function loadPersisted(key, fallback) {
  try {
    const v = localStorage.getItem(key)
    return v ? JSON.parse(v) : fallback
  } catch { return fallback }
}

function persist(key, store) {
  store.subscribe(v => {
    try { localStorage.setItem(key, JSON.stringify(v)) } catch {}
  })
}

export const activeView = writable('explorer')

export const sidebarVisible = writable(true)
export const sidebarWidth = writable(loadPersisted(KEYS.SIDEBAR_WIDTH, 240))
persist(KEYS.SIDEBAR_WIDTH, sidebarWidth)

export const aiPanelVisible = writable(true)
export const aiPanelWidth = writable(loadPersisted(KEYS.AI_PANEL_WIDTH, 440))
persist(KEYS.AI_PANEL_WIDTH, aiPanelWidth)

export const bottomPanelVisible = writable(true)
export const bottomPanelHeight = writable(loadPersisted(KEYS.BOTTOM_HEIGHT, 200))
persist(KEYS.BOTTOM_HEIGHT, bottomPanelHeight)
export const bottomPanelTab = writable('terminal')

export const MIN_PANEL_HEIGHT = 60
export const DEFAULT_PANEL_HEIGHT = 200
export const bottomPanelMaximized = writable(false)
export const preMaximizeHeight = writable(DEFAULT_PANEL_HEIGHT)

export const commandPaletteOpen = writable(false)

export function toggleSidebar() {
  sidebarVisible.update(v => !v)
}

export function toggleAIPanel() {
  aiPanelVisible.update(v => !v)
}

export function toggleBottomPanel() {
  bottomPanelVisible.update(v => !v)
}

export function toggleMaximizePanel() {
  const maximized = get(bottomPanelMaximized)
  if (maximized) {
    const h = get(preMaximizeHeight)
    bottomPanelHeight.set(h)
    bottomPanelMaximized.set(false)
  } else {
    preMaximizeHeight.set(get(bottomPanelHeight))
    const maxH = window.innerHeight - 35 - 100
    bottomPanelHeight.set(Math.max(maxH, MIN_PANEL_HEIGHT))
    bottomPanelMaximized.set(true)
  }
}

/**
 * @param {string} viewId
 */
export function setActiveView(viewId) {
  activeView.set(viewId)
  if (viewId === 'ai') {
    aiPanelVisible.set(true)
  } else if (!get(sidebarVisible)) {
    sidebarVisible.set(true)
  }
}
