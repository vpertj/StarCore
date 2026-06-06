import { writable } from 'svelte/store'

/** @typedef {{ totalTokensIn: number, totalTokensOut: number, totalCost: number, byProvider: Record<string, { tokensIn: number, tokensOut: number, cost: number }> }} TokenStats */

/** @type {import('svelte/store').Writable<TokenStats|null>} */
export const tokenStats = writable(null)
export const isLoadingStats = writable(false)
export const statsError = writable(/** @type {string|null} */ (null))

/**
 * @param {string} [period='month']
 */
export async function loadTokenUsage(period = 'month') {
    statsError.set(null)
    isLoadingStats.set(true)
    try {
        if (window.backend?.GetTokenUsage) {
            const stats = await window.backend.GetTokenUsage(period)
            if (stats && (stats.totalTokensIn > 0 || stats.totalTokensOut > 0)) {
                tokenStats.set(stats)
                return
            }
        }
        // Fallback: localStorage accumulated token usage
        const key = 'starcore-token-usage'
        const saved = JSON.parse(localStorage.getItem(key) || '{"tokensIn":0,"tokensOut":0,"count":0}')
        if (saved.tokensIn > 0 || saved.tokensOut > 0) {
            tokenStats.set({
                totalTokensIn: saved.tokensIn,
                totalTokensOut: saved.tokensOut,
                totalCost: 0,
                byProvider: { local: { tokensIn: saved.tokensIn, tokensOut: saved.tokensOut, cost: 0 } }
            })
        } else {
            tokenStats.set(null)
        }
    } catch (/** @type {any} */ e) {
        console.error('Failed to load token usage:', e)
        statsError.set('加载失败: ' + (e.message || String(e)))
    } finally {
        isLoadingStats.set(false)
    }
}

export async function clearTokenUsage() {
    if (!window.backend?.ClearTokenUsage) return
    try {
        await window.backend.ClearTokenUsage()
        tokenStats.set(null)
    } catch (/** @type {any} */ e) {
        console.error('Failed to clear token usage:', e)
    }
}
