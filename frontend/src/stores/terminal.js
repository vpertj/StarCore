import { writable } from 'svelte/store'
import { addLog } from './output.js'
import { currentProject } from './app.js'

/**
 * @typedef {'running' | 'exited'} TerminalStatus
 */

/**
 * @typedef {{ id: string, title: string, status: TerminalStatus, exitCode: number|null }} TerminalTab
 */

export const terminalTabs = writable(/** @type {TerminalTab[]} */ ([]))
export const activeTerminalId = writable(/** @type {string|null} */ (null))

export async function createTerminalTab() {
  const cwd = get(currentProject) || ''
  let id
  try {
    id = await window.backend.NewTerminal(cwd)
  } catch (/** @type {any} */ err) {
    addLog('IDE', 'error', 'Failed to create terminal: ' + (err.message || String(err)))
    return null
  }
  const tab = { id, title: `Terminal ${terminalCounter++}`, status: 'running', exitCode: null }
  terminalTabs.update(tabs => [...tabs, tab])
  activeTerminalId.set(id)
  return id
}

let terminalCounter = 1

export async function closeTerminalTab(id) {
  try { await window.backend.KillTerminal(id) } catch { /* ignore */ }
  terminalTabs.update(tabs => {
    const newTabs = tabs.filter(t => t.id !== id)
    return newTabs
  })
  activeTerminalId.update(curr => {
    if (curr === id) {
      const tabs = get(terminalTabs)
      return tabs.length > 0 ? tabs[tabs.length - 1].id : null
    }
    return curr
  })
}

export async function ensureDefaultTerminal() {
  const tabs = get(terminalTabs)
  if (tabs.length > 0) return tabs[0].id

  const id = await createTerminalTab()
  return id
}

export function setTerminalExited(id, exitCode = null) {
  terminalTabs.update(tabs =>
    tabs.map(t => t.id === id ? { ...t, status: 'exited', exitCode } : t)
  )
}

export async function restartTerminal(id) {
  const cwd = get(currentProject) || ''
  try { await window.backend.KillTerminal(id) } catch { /* ignore */ }

  let newId
  try {
    newId = await window.backend.NewTerminal(cwd)
  } catch (/** @type {any} */ err) {
    addLog('IDE', 'error', 'Failed to restart terminal: ' + (err.message || String(err)))
    return
  }

  terminalTabs.update(tabs =>
    tabs.map(t => t.id === id ? { ...t, id: newId, status: 'running', exitCode: null } : t)
  )
  activeTerminalId.update(curr => curr === id ? newId : curr)

  window.dispatchEvent(new CustomEvent('terminal:restarted:' + id, { detail: { newId } }))
}

/**
 * @template T
 * @param {import('svelte/store').Readable<T>} store
 * @returns {T}
 */
function get(store) {
  /** @type {any} */ let value = undefined
  store.subscribe(v => value = v)()
  return value
}
