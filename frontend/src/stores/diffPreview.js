import { writable, get } from 'svelte/store'
import { activeFile } from './app.js'

/**
 * @typedef {{ filePath: string, hunks: any[], oldContent?: string, newContent?: string, accepted?: boolean }} DiffPreview
 */

export const pendingDiff = writable(/** @type {DiffPreview|null} */ (null))
export const diffVisible = writable(false)

/** @type {import('svelte/store').Writable<DiffPreview[]>} */
export const pendingDiffs = writable([])
/** @type {import('svelte/store').Writable<boolean>} */
export const multiDiffVisible = writable(false)
/** @type {import('svelte/store').Writable<number>} */
export const activeDiffIndex = writable(0)

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

export async function addPendingDiff(filePath, newContent) {
  try {
    const oldContent = await window.backend.ReadFile(filePath)
    const hunks = await window.backend.ComputeDiff(filePath, newContent)
    if (!hunks || hunks.length === 0) return
    const diffs = get(pendingDiffs)
    const existing = diffs.findIndex(d => d.filePath === filePath)
    if (existing >= 0) {
      diffs[existing] = { filePath, hunks, oldContent, newContent, accepted: true }
      pendingDiffs.set([...diffs])
    } else {
      pendingDiffs.set([...diffs, { filePath, hunks, oldContent, newContent, accepted: true }])
    }
    multiDiffVisible.set(true)
  } catch (/** @type {any} */ e) {
    console.error('Failed to compute diff:', e)
  }
}

export function toggleDiffAccepted(index) {
  const diffs = get(pendingDiffs)
  if (diffs[index]) {
    diffs[index].accepted = !diffs[index].accepted
    pendingDiffs.set([...diffs])
  }
}

export function acceptAllDiffs() {
  const diffs = get(pendingDiffs)
  diffs.forEach(d => d.accepted = true)
  pendingDiffs.set([...diffs])
}

export function rejectAllDiffs() {
  const diffs = get(pendingDiffs)
  diffs.forEach(d => d.accepted = false)
  pendingDiffs.set([...diffs])
}

export async function applyAcceptedDiffs() {
  const diffs = get(pendingDiffs)
  const accepted = diffs.filter(d => d.accepted)
  for (const diff of accepted) {
    try {
      await window.backend.ApplyDiff({ filePath: diff.filePath, hunks: diff.hunks })
      const content = await window.backend.ReadFile(diff.filePath)
      window.dispatchEvent(new CustomEvent('file-changed', { detail: { path: diff.filePath, content } }))
    } catch (/** @type {any} */ e) {
      console.error(`Failed to apply diff for ${diff.filePath}:`, e)
    }
  }
  pendingDiffs.set([])
  multiDiffVisible.set(false)
  activeDiffIndex.set(0)
}

export async function applyDiff(filePath, hunks) {
  try {
    await window.backend.ApplyDiff({ filePath, hunks })
    pendingDiff.set(null)
    diffVisible.set(false)
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

export function dismissMultiDiff() {
  pendingDiffs.set([])
  multiDiffVisible.set(false)
  activeDiffIndex.set(0)
}
