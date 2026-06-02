<script>
import { onMount, onDestroy } from 'svelte'
import { completionVisible, dismissCompletion, acceptNextWord, acceptFullCompletion } from '../stores/completion.js'

/** @type {any} */ export let editorView = null

onMount(() => {
  window.addEventListener('keydown', handleKeydown)
})

onDestroy(() => {
  window.removeEventListener('keydown', handleKeydown)
})

/** @param {KeyboardEvent} e */
function handleKeydown(e) {
  const visible = get(completionVisible)
  if (!visible || !editorView) return

  if (e.key === 'Tab') {
    e.preventDefault()
    const text = acceptFullCompletion()
    if (text) {
      const pos = editorView.state.selection.main.head
      editorView.dispatch({
        changes: { from: pos, insert: text }
      })
    }
    return
  }

  if (e.key === 'Escape') {
    e.preventDefault()
    dismissCompletion()
    return
  }

  if (e.key === 'ArrowRight' && e.ctrlKey) {
    e.preventDefault()
    const word = acceptNextWord()
    if (word) {
      const pos = editorView.state.selection.main.head
      editorView.dispatch({
        changes: { from: pos, insert: word }
      })
    }
    return
  }

  if (e.key.length === 1 || e.key === 'Backspace' || e.key === 'Delete') {
    dismissCompletion()
  }
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
</script>
