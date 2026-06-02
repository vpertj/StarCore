import { writable } from 'svelte/store'

/**
 * @typedef {{ from: number, to: number, severity: 'error'|'warning'|'info', message: string, filePath?: string }} Diagnostic
 */

export const diagnostics = writable(/** @type {Diagnostic[]} */ ([]))
export const activeFileDiagnostics = writable(/** @type {Diagnostic[]} */ ([]))
