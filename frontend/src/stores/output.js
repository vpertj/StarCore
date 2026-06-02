import { writable } from 'svelte/store'

/**
 * @typedef {{ timestamp: number, source: string, level: 'info'|'warn'|'error', message: string }} LogEntry
 */

export const logEntries = writable(/** @type {LogEntry[]} */ ([]))

export function addLog(source, level, message) {
  logEntries.update(entries => [...entries, {
    timestamp: Date.now(),
    source,
    level,
    message
  }])
}

export function clearLogs() {
  logEntries.set([])
}
