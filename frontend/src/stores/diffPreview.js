import { writable, get } from 'svelte/store'
import { activeFile } from './app.js'

/**
 * @typedef {{ filePath: string, hunks: any[], oldContent?: string, newContent?: string }} DiffPreview
 */

export const pendingDiff = writable(/** @type {DiffPreview|null} */ (null))
export const diffVisible = writable(false)

export async function showDiffForFile(filePath, newContent) {
  try {
    const oldContent = await window.backend.ReadFile(filePath)
    const hunks = await window.backend.ComputeDiff(filePath, newContent)
    pendingDiff.set({ filePath, hunks, oldContent, newContent })
    diffVisible.set(true)
  } catch (/** @type {any} */ e) {
    console.error('Failed to compute diff:', e)
  }
}

export async function applyDiff(filePath, hunks) {
  try {
    await window.backend.ApplyDiff({ filePath, hunks })
    pendingDiff.set(null)
    diffVisible.set(false)
    // Refresh editor content
    const content = await window.backend.ReadFile(filePath)
    window.dispatchEvent(new CustomEvent('file-changed', { detail: { path: filePath, content } }))
  } catch (/** @type {any} */ e) {
    console.error('Failed to apply diff:', e)
  }
}

export function dismissDiff() {
  pendingDiff.set(null)
  diffVisible.set(false)
}
