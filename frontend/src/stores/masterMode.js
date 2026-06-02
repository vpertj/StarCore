import { writable } from 'svelte/store'
import { KEYS } from './constants.js'

const KEY = KEYS.MASTER_MODE

export const masterMode = writable(load())

function load() {
  try { return localStorage.getItem(KEY) === 'true' } catch { return false }
}

masterMode.subscribe(v => {
  try { localStorage.setItem(KEY, String(v)) } catch {}
})

export function toggleMasterMode() {
  masterMode.update(v => !v)
}
