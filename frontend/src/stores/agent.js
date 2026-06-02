import { writable } from 'svelte/store'

/** @typedef {{ id: string, name: string, icon: string, description?: string }} Agent */

export const agents = writable(/** @type {Agent[]} */ ([]))
export const activeAgentId = writable('universal-assistant')

export async function loadAgents() {
  if (!window.backend?.GetAgents) return
  try {
    /** @type {Agent[]} */
    const list = await window.backend.GetAgents()
    agents.set(list || [])
  } catch (/** @type {any} */ e) {
    console.error('Failed to load agents:', e)
  }
}

/**
 * @returns {Agent|null}
 */
export function getActiveAgent() {
  const $agents = get(agents)
  const $activeAgentId = get(activeAgentId)
  return $agents.find(a => a.id === $activeAgentId) || $agents[0] || null
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
