import { writable } from 'svelte/store'
import { KEYS } from './constants.js'

const THEME_KEY = KEYS.THEME

export const themes = [
  { id: 'dark', name: 'Dark', colors: { bg: '#1e1e1e', bg2: '#252526', bg3: '#2d2d2d', border: '#3c3c3c', accent: '#0078d4', text: '#cccccc', text2: '#858585' } },
  { id: 'light', name: 'Light', colors: { bg: '#ffffff', bg2: '#f3f3f3', bg3: '#e5e5e5', border: '#d0d0d0', accent: '#0078d4', text: '#333333', text2: '#666666' } },
  { id: 'hc', name: 'High Contrast', colors: { bg: '#000000', bg2: '#0a0a0a', bg3: '#1a1a1a', border: '#6fc3df', accent: '#4fc3f7', text: '#ffffff', text2: '#c0c0c0' } },
  { id: 'one-dark', name: 'One Dark', colors: { bg: '#282c34', bg2: '#21252b', bg3: '#333842', border: '#3e4451', accent: '#61afef', text: '#abb2bf', text2: '#5c6370' } },
  { id: 'dracula', name: 'Dracula', colors: { bg: '#282a36', bg2: '#21222c', bg3: '#343746', border: '#44475a', accent: '#bd93f9', text: '#f8f8f2', text2: '#6272a4' } },
  { id: 'nord', name: 'Nord', colors: { bg: '#2e3440', bg2: '#3b4252', bg3: '#434c5e', border: '#4c566a', accent: '#88c0d0', text: '#eceff4', text2: '#d8dee9' } },
  { id: 'monokai', name: 'Monokai', colors: { bg: '#272822', bg2: '#1e1f1c', bg3: '#3e3d32', border: '#49483e', accent: '#a6e22e', text: '#f8f8f2', text2: '#75715e' } },
  { id: 'github-dark', name: 'GitHub Dark', colors: { bg: '#0d1117', bg2: '#161b22', bg3: '#21262d', border: '#30363d', accent: '#58a6ff', text: '#c9d1d9', text2: '#8b949e' } },
]

export const currentTheme = writable(loadTheme())

function loadTheme() {
    try { return localStorage.getItem(THEME_KEY) || 'dark' } catch (e) { return 'dark' }
}

export function setTheme(themeId) {
    currentTheme.set(themeId)
    try { localStorage.setItem(THEME_KEY, themeId) } catch (e) {}
    applyTheme(themeId)
}

function applyTheme(themeId) {
    const theme = themes.find(t => t.id === themeId) || themes[0]
    const root = document.documentElement
    root.setAttribute('data-theme', themeId)
    root.classList.remove(...themes.map(t => 'theme-' + t.id))
    root.classList.add('theme-' + themeId)

    const c = theme.colors
    const isLight = themeId === 'light'

    // Primary colors
    root.style.setProperty('--bg-primary', c.bg)
    root.style.setProperty('--bg-secondary', c.bg2)
    root.style.setProperty('--bg-tertiary', c.bg3)
    root.style.setProperty('--border', c.border)
    root.style.setProperty('--accent', c.accent)
    root.style.setProperty('--text-primary', c.text)
    root.style.setProperty('--text-secondary', c.text2)

    // Derived colors
    root.style.setProperty('--bg-hover', isLight ? '#e8e8e8' : lighten(c.bg, 8))
    root.style.setProperty('--bg-active', isLight ? '#d0d0d0' : lighten(c.bg, 14))
    root.style.setProperty('--text-muted', isLight ? '#999999' : darken(c.text, 30))
    root.style.setProperty('--accent-hover', lighten(c.accent, 8))
    root.style.setProperty('--accent-active', darken(c.accent, 8))
    root.style.setProperty('--border-focus', c.accent)
    root.style.setProperty('--selection', isLight ? 'rgba(0,120,212,0.2)' : 'rgba(0,120,212,0.35)')
    root.style.setProperty('--ai-color', isLight ? '#0078d4' : '#4ec9b0')
    root.style.setProperty('--success', '#2ea043')
    root.style.setProperty('--success-hover', '#238636')
    root.style.setProperty('--warning', isLight ? '#b8950a' : '#e5c07b')
    root.style.setProperty('--error', '#d73a49')
    root.style.setProperty('--error-hover', '#c53030')
    root.style.setProperty('--info', isLight ? '#0078d4' : '#4fc1ff')
    root.style.setProperty('--ring', `0 0 0 2px ${c.accent}`)
    root.style.setProperty('--shadow-sm', isLight ? '0 1px 3px rgba(0,0,0,0.08)' : '0 1px 3px rgba(0,0,0,0.3)')
    root.style.setProperty('--shadow-md', isLight ? '0 4px 12px rgba(0,0,0,0.12)' : '0 4px 12px rgba(0,0,0,0.4)')
    root.style.setProperty('--shadow-lg', isLight ? '0 8px 24px rgba(0,0,0,0.16)' : '0 8px 24px rgba(0,0,0,0.5)')
}

// Simple color helpers
function lighten(hex, percent) {
    const num = parseInt(hex.replace('#',''), 16)
    const r = Math.min(255, (num >> 16) + Math.round(2.55 * percent))
    const g = Math.min(255, ((num >> 8) & 0x00FF) + Math.round(2.55 * percent))
    const b = Math.min(255, (num & 0x0000FF) + Math.round(2.55 * percent))
    return '#' + (0x1000000 + (r << 16) + (g << 8) + b).toString(16).slice(1)
}

function darken(hex, percent) {
    const num = parseInt(hex.replace('#',''), 16)
    const r = Math.max(0, (num >> 16) - Math.round(2.55 * percent))
    const g = Math.max(0, ((num >> 8) & 0x00FF) - Math.round(2.55 * percent))
    const b = Math.max(0, (num & 0x0000FF) - Math.round(2.55 * percent))
    return '#' + (0x1000000 + (r << 16) + (g << 8) + b).toString(16).slice(1)
}

if (typeof document !== 'undefined') {
    applyTheme(loadTheme())
}
