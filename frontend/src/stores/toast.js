import { writable, get } from 'svelte/store'

/**
 * @typedef {'success'|'error'|'warning'|'info'} ToastLevel
 */

/**
 * @typedef {{
 *   id: string,
 *   message: string,
 *   level: ToastLevel,
 *   duration: number,
 *   dismissible: boolean
 * }} ToastItem
 */

let nextId = 0

/** @type {import('svelte/store').Writable<Array<{id: string, message: string, level: ToastLevel, duration: number, dismissible: boolean}>>} */
export const toasts = writable([])

/**
 * Show a toast notification.
 * @param {string} message - The message to display.
 * @param {ToastLevel} [level='info'] - The toast type.
 * @param {number} [duration=4000] - Auto-dismiss duration in ms.
 * @returns {string} The toast ID.
 */
export function showToast(message, level = 'info', duration = 4000) {
	const id = String(++nextId)
	toasts.update(list => [...list, { id, message, level, duration, dismissible: true }])
	if (duration > 0) {
		setTimeout(() => dismissToast(id), duration)
	}
	return id
}

/** Dismiss a specific toast by ID. */
export function dismissToast(id) {
	toasts.update(list => list.filter(t => t.id !== id))
}

/** Convenience functions for each level. */
export function showSuccess(message, duration) { return showToast(message, 'success', duration) }
export function showError(message, duration) { return showToast(message, 'error', duration) }
export function showWarning(message, duration) { return showToast(message, 'warning', duration) }
export function showInfo(message, duration) { return showToast(message, 'info', duration) }
