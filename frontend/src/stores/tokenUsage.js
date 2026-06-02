import { writable } from 'svelte/store'

/** @typedef {{ totalTokensIn: number, totalTokensOut: number, totalCost: number, byProvider: Record<string, { tokensIn: number, tokensOut: number, cost: number }> }} TokenStats */

/** @type {import('svelte/store').Writable<TokenStats|null>} */
export const tokenStats = writable(null)
export const isLoadingStats = writable(false)

/**
 * @param {string} [period='month']
 */
export async function loadTokenUsage(period = 'month') {
    if (!window.backend?.GetTokenUsage) return
    isLoadingStats.set(true)
    try {
        const stats = await window.backend.GetTokenUsage(period)
        tokenStats.set(stats)
    } catch (/** @type {any} */ e) {
        console.error('Failed to load token usage:', e)
    } finally {
        isLoadingStats.set(false)
    }
}
