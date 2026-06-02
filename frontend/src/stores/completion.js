import { writable } from 'svelte/store'

export const completionText = writable('')
export const completionVisible = writable(false)
export const completionLoading = writable(false)
export const completionConfig = writable({
  triggerMode: 'auto',
  debounceMs: 300,
  enabled: true
})

/** @type {ReturnType<typeof setTimeout>|null} */ let debounceTimer = null
/** @type {string|null} */ let currentRequestId = null

/**
 * @param {string} filePath
 * @param {string} content
 * @param {number} cursorPos
 * @param {string} language
 * @param {string} providerId
 */
export async function requestCompletion(filePath, content, cursorPos, language, providerId) {
  if (!window.backend?.AICompletion) return
  completionLoading.set(true)
  currentRequestId = String(Date.now())
  const requestId = currentRequestId

  try {
    const result = await window.backend.AICompletion(providerId, {
      file: filePath,
      content: content,
      cursorPos: cursorPos,
      language: language
    })
    if (requestId !== currentRequestId) return
    const text = typeof result === 'string' ? result : ''
    if (text) {
      completionText.set(text)
      completionVisible.set(true)
    } else {
      dismissCompletion()
    }
  } catch (e) {
    console.error('Completion failed:', e)
    dismissCompletion()
  } finally {
    completionLoading.set(false)
  }
}

/**
 * @param {string} filePath
 * @param {string} content
 * @param {number} cursorPos
 * @param {string} language
 * @param {string} providerId
 * @param {number} [debounceMs=300]
 */
export function debouncedRequestCompletion(filePath, content, cursorPos, language, providerId, debounceMs = 300) {
  cancelPendingCompletion()
  debounceTimer = setTimeout(() => {
    requestCompletion(filePath, content, cursorPos, language, providerId)
  }, debounceMs)
}

export function cancelPendingCompletion() {
  if (debounceTimer) {
    clearTimeout(debounceTimer)
    debounceTimer = null
  }
  currentRequestId = null
}

export function dismissCompletion() {
  completionText.set('')
  completionVisible.set(false)
  completionLoading.set(false)
}

/** @returns {string} */
export function acceptNextWord() {
  const text = get(completionText)
  if (!text) return ''
  const match = text.match(/^(\S+\s*)/)
  if (!match) return ''
  const word = match[1]
  const remaining = text.slice(word.length)
  completionText.set(remaining)
  if (!remaining) {
    completionVisible.set(false)
  }
  return word
}

export function acceptFullCompletion() {
  const text = get(completionText)
  dismissCompletion()
  return text
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
