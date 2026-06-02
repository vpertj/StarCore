<script>
/** @type {string[]} */ export let contextFiles = []
/** @type {string} */ export let contextCode = ''
/** @type {string[]} */ export let diagnostics = []

import { createEventDispatcher } from 'svelte'
const dispatch = createEventDispatcher()

/** @param {number} index */
function removeFile(index) {
  dispatch('removefile', { index })
}

function removeCode() {
  dispatch('removecode')
}
</script>

{#if contextFiles.length > 0 || contextCode || diagnostics.length > 0}
  <div class="flex flex-wrap gap-1 px-3 py-1.5 border-b" style="border-color: var(--border); background-color: var(--bg-secondary);">
    {#each contextFiles as file, i}
      <div class="flex items-center gap-1 px-2 py-0.5 rounded text-xs" style="background-color: rgba(9,71,113,0.2); color: #4fc1ff; border: 1px solid rgba(9,71,113,0.33);">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        <span class="truncate" style="max-width: 120px;">{file.split(/[\\/]/).pop()}</span>
        <button on:click={() => removeFile(i)} style="color: var(--text-secondary);" aria-label="移除" class="hover:text-white transition-colors">&times;</button>
      </div>
    {/each}
    {#if contextCode}
      <div class="flex items-center gap-1 px-2 py-0.5 rounded text-xs" style="background-color: rgba(78,201,176,0.2); color: #4ec9b0; border: 1px solid rgba(78,201,176,0.33);">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
        </svg>
        <span>代码片段</span>
        <button on:click={removeCode} style="color: var(--text-secondary);" aria-label="移除" class="hover:text-white transition-colors">&times;</button>
      </div>
    {/if}
    {#if diagnostics.length > 0}
      <div class="flex items-center gap-1 px-2 py-0.5 rounded text-xs" style="background-color: rgba(215,58,73,0.2); color: #d73a49; border: 1px solid rgba(215,58,73,0.33);">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <span>{diagnostics.length} 错误</span>
      </div>
    {/if}
  </div>
{/if}
