import { writable } from 'svelte/store'
import { KEYS } from './constants.js'
import { THEMES } from '../themes/data.js'

const THEME_KEY = KEYS.THEME
const ICON_THEME_KEY = 'starcore-icon-theme'

export const themes = THEMES

function loadTheme() {
  try { return localStorage.getItem(THEME_KEY) || 'nord' } catch { return 'nord' }
}

function loadIconTheme() {
  try { return localStorage.getItem(ICON_THEME_KEY) || 'colorful' } catch { return 'colorful' }
}

export const currentTheme = writable(loadTheme())
export const iconTheme = writable(loadIconTheme())

export function setTheme(themeId) {
  currentTheme.set(themeId)
  try { localStorage.setItem(THEME_KEY, themeId) } catch {}
  applyTheme(themeId)
}

export function setIconTheme(themeId) {
  iconTheme.set(themeId)
  try { localStorage.setItem(ICON_THEME_KEY, themeId) } catch {}
}

export function getThemeById(id) {
  return themes.find(t => t.id === id) || themes[0]
}

export function getThemeColors(id) {
  return getThemeById(id).colors
}

export function getThemeSyntax(id) {
  return getThemeById(id).syntax
}

export function getThemeType(id) {
  return getThemeById(id).type
}

export function isLightTheme(id) {
  return getThemeType(id) === 'light'
}

function lighten(hex, percent) {
  const num = parseInt(hex.replace('#', ''), 16)
  const r = Math.min(255, (num >> 16) + Math.round(2.55 * percent))
  const g = Math.min(255, ((num >> 8) & 0x00FF) + Math.round(2.55 * percent))
  const b = Math.min(255, (num & 0x0000FF) + Math.round(2.55 * percent))
  return '#' + (0x1000000 + (r << 16) + (g << 8) + b).toString(16).slice(1)
}

function darken(hex, percent) {
  const num = parseInt(hex.replace('#', ''), 16)
  const r = Math.max(0, (num >> 16) - Math.round(2.55 * percent))
  const g = Math.max(0, ((num >> 8) & 0x00FF) - Math.round(2.55 * percent))
  const b = Math.max(0, (num & 0x0000FF) - Math.round(2.55 * percent))
  return '#' + (0x1000000 + (r << 16) + (g << 8) + b).toString(16).slice(1)
}

function applyTheme(themeId) {
  if (typeof document === 'undefined') return
  const theme = getThemeById(themeId)
  const c = theme.colors
  const isLight = theme.type === 'light'
  const root = document.documentElement

  root.setAttribute('data-theme', themeId)
  root.classList.remove(...themes.map(t => 'theme-' + t.id))
  root.classList.add('theme-' + themeId)

  root.style.setProperty('--bg-primary', c.bg)
  root.style.setProperty('--bg-secondary', c.bg2)
  root.style.setProperty('--bg-tertiary', c.bg3)
  root.style.setProperty('--border', c.border)
  root.style.setProperty('--accent', c.accent)
  root.style.setProperty('--text-primary', c.text)
  root.style.setProperty('--text-secondary', c.text2)
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

if (typeof document !== 'undefined') {
  applyTheme(loadTheme())
}
