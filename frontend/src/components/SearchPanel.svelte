<script>
import { openFile } from '../stores/app.js'
import { setActiveView } from '../stores/ui.js'
import { t } from '../stores/i18n.js'

/** @typedef {{ filePath: string, line: number, content: string }} SearchResult */

let searchQuery = $state('')
let replaceQuery = $state('')
/** @type {SearchResult[]} */ let searchResults = $state([])
let isSearching = $state(false)
let caseSensitive = $state(false)
let wholeWord = $state(false)
let useRegex = $state(false)
let includePattern = $state('')
let excludePattern = $state('')
let expandedFiles = $state(new Set())
let replaceConfirm = $state(false)

/** @type {Record<string, SearchResult[]>} */
let groupedResults = $derived.by(() => {
  const groups = {}
  for (const r of searchResults) {
    if (!groups[r.filePath]) groups[r.filePath] = []
    groups[r.filePath].push(r)
  }
  return groups
})

let totalFiles = $derived(Object.keys(groupedResults).length)

async function performSearch() {
  if (!searchQuery.trim()) return
  
  isSearching = true
  searchResults = []
  
  try {
    const results = await window.backend.SearchFiles(searchQuery, {
      caseSensitive,
      wholeWord,
      useRegex,
      includePattern,
      excludePattern,
    })
    searchResults = results
    expandedFiles = new Set(results.length > 0 ? [results[0].filePath] : [])
  } catch (err) {
    console.error('Search failed:', err)
  } finally {
    isSearching = false
  }
}

function toggleFile(filePath) {
  if (expandedFiles.has(filePath)) {
    expandedFiles.delete(filePath)
  } else {
    expandedFiles.add(filePath)
  }
  expandedFiles = new Set(expandedFiles)
}

async function performReplace() {
  if (!searchQuery.trim() || !replaceQuery.trim()) return
  if (!replaceConfirm) {
    replaceConfirm = true
    return
  }
  
  try {
    await window.backend.ReplaceInFiles(searchQuery, replaceQuery, {
      caseSensitive,
      wholeWord,
      useRegex,
      includePattern,
      excludePattern,
    })
    replaceConfirm = false
    performSearch()
  } catch (err) {
    console.error('Replace failed:', err)
    replaceConfirm = false
  }
}

function cancelReplace() {
  replaceConfirm = false
}

/** @param {SearchResult} result */
function handleResultClick(result) {
  openFile(result.filePath)
  window.dispatchEvent(new CustomEvent('search:goto-line', { detail: { line: result.line } }))
}

/** @param {KeyboardEvent} e */
function handleKeyDown(e) {
  if (e.key === 'Enter') {
    performSearch()
  }
}
</script>

<div class="h-full flex flex-col" style="background-color: var(--bg-primary);">
  <!-- 搜索头部 -->
  <div class="flex items-center justify-between px-3 py-2 border-b" style="border-color: var(--border);">
    <h2 class="text-sm font-medium" style="color: var(--text-secondary);">{$t('sidebar.search').toUpperCase()}</h2>
    <button 
      class="p-1 rounded transition-colors"
      style="color: var(--text-secondary);"
      onclick={() => setActiveView('explorer')}
      aria-label="关闭搜索"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </button>
  </div>
  
  <!-- 搜索输入区 -->
  <div class="p-3 space-y-3">
    <!-- 搜索框 -->
    <div class="relative">
      <input 
        type="text" 
        bind:value={searchQuery}
        placeholder={$t('search.placeholder')}
        class="w-full px-3 py-2 rounded border text-sm"
        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
        onkeydown={handleKeyDown}
      />
      <button 
        class="absolute right-2 top-1/2 transform -translate-y-1/2 p-1 rounded"
        style="color: var(--text-secondary);"
        onclick={performSearch}
        aria-label="搜索"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
      </button>
    </div>
    
    <!-- 替换框 -->
    <div class="relative">
      <input 
        type="text" 
        bind:value={replaceQuery}
        placeholder={$t('search.replacePlaceholder')}
        class="w-full px-3 py-2 rounded border text-sm"
        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
      />
      <div class="absolute right-2 top-1/2 transform -translate-y-1/2 flex gap-1">
        {#if replaceConfirm}
          <button 
            class="px-2 py-1 rounded text-xs"
            style="background-color: var(--error); color: var(--text-on-accent);"
            onclick={performReplace}
          >
            {$t('search.confirm')} ({searchResults.length})
          </button>
          <button 
            class="px-2 py-1 rounded text-xs"
            style="background-color: var(--bg-tertiary); color: var(--text-secondary);"
            onclick={cancelReplace}
          >
            {$t('settings.cancel')}
          </button>
        {:else}
          <button 
            class="px-2 py-1 rounded text-xs"
            style="background-color: var(--accent); color: var(--text-on-accent);"
            onclick={performReplace}
            disabled={!searchQuery.trim() || !replaceQuery.trim()}
          >
            {$t('search.replaceAll')}
          </button>
        {/if}
      </div>
    </div>
    
    <!-- 搜索选项 -->
    <div class="flex items-center gap-3">
      <label class="flex items-center gap-1 text-xs" style="color: var(--text-secondary);">
        <input type="checkbox" bind:checked={caseSensitive} class="rounded" />
        Aa
      </label>
      <label class="flex items-center gap-1 text-xs" style="color: var(--text-secondary);">
        <input type="checkbox" bind:checked={wholeWord} class="rounded" />
        W
      </label>
      <label class="flex items-center gap-1 text-xs" style="color: var(--text-secondary);">
        <input type="checkbox" bind:checked={useRegex} class="rounded" />
        .*
      </label>
    </div>
    
    <!-- 包含/排除模式 -->
    <div class="space-y-2">
      <input 
        type="text" 
        bind:value={includePattern}
        placeholder={$t('search.includePlaceholder')}
        class="w-full px-3 py-1.5 rounded border text-xs"
        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
      />
      <input 
        type="text" 
        bind:value={excludePattern}
        placeholder={$t('search.excludePlaceholder')}
        class="w-full px-3 py-1.5 rounded border text-xs"
        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
      />
    </div>
  </div>
  
  <!-- 搜索结果 -->
  <div class="flex-1 overflow-y-auto">
    {#if isSearching}
      <div class="flex items-center justify-center py-8">
        <div class="animate-spin rounded-full h-6 w-6 border-b-2" style="border-color: var(--accent);"></div>
      </div>
    {:else if searchResults.length > 0}
      <div class="px-3 py-2 text-xs" style="color: var(--text-secondary);">
        {$t('search.resultsCount').replace('{0}', searchResults.length).replace('{1}', totalFiles)}
      </div>
      {#each Object.entries(groupedResults) as [filePath, results]}
        <!-- svelte-ignore a11y_click_events_have_key_events -->
        <div 
          class="file-group-header"
          onclick={() => toggleFile(filePath)}
          role="button"
          tabindex="0"
          onkeydown={(e) => { if (e.key === 'Enter') toggleFile(filePath) }}
        >
          <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="transform: rotate({expandedFiles.has(filePath) ? 90 : 0}deg); transition: transform 0.15s;">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--text-secondary);">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          <span class="text-sm truncate" style="color: var(--text-primary);">{filePath}</span>
          <span class="text-xs ml-auto" style="color: var(--text-muted);">{results.length}</span>
        </div>
        {#if expandedFiles.has(filePath)}
          {#each results as result}
            <!-- svelte-ignore a11y_click_events_have_key_events -->
            <div 
              class="search-result-item"
              role="button"
              tabindex="0"
              onclick={() => handleResultClick(result)}
            >
              <span class="text-xs shrink-0 w-8 text-right" style="color: var(--text-muted);">{result.line}</span>
              <span class="text-xs flex-1 truncate" style="color: var(--text-secondary);">{result.content}</span>
            </div>
          {/each}
        {/if}
      {/each}
    {:else if searchQuery}
      <div class="flex items-center justify-center py-8">
        <span class="text-sm" style="color: var(--text-secondary);">{$t('search.noResults')}</span>
      </div>
    {/if}
  </div>
</div>

<style>
.file-group-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  font-size: 12px;
  font-family: 'Consolas', monospace;
  cursor: pointer;
  transition: background-color 0.1s;
  border-bottom: 1px solid var(--border);
}
.file-group-header:hover {
  background-color: var(--bg-hover);
}
.search-result-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 12px 3px 28px;
  font-size: 12px;
  font-family: 'Consolas', monospace;
  cursor: pointer;
  transition: background-color 0.1s;
}
.search-result-item:hover {
  background-color: var(--bg-hover);
}
</style>
