<script>
import { openFile } from '../stores/app.js'
import { setActiveView } from '../stores/ui.js'
import { t } from '../stores/i18n.js'

/** @typedef {{ filePath: string, line: number, content: string }} SearchResult */

let searchQuery = ''
let replaceQuery = ''
/** @type {SearchResult[]} */ let searchResults = []
let isSearching = false
let caseSensitive = false
let wholeWord = false
let useRegex = false
let includePattern = ''
let excludePattern = ''

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
  } catch (err) {
    console.error('Search failed:', err)
  } finally {
    isSearching = false
  }
}

async function performReplace() {
  if (!searchQuery.trim() || !replaceQuery.trim()) return
  
  try {
    await window.backend.ReplaceInFiles(searchQuery, replaceQuery, {
      caseSensitive,
      wholeWord,
      useRegex,
      includePattern,
      excludePattern,
    })
    // 重新搜索以更新结果
    performSearch()
  } catch (err) {
    console.error('Replace failed:', err)
  }
}

/** @param {SearchResult} result */
function handleResultClick(result) {
  openFile(result.filePath)
  // TODO: 跳转到具体行号
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
      on:click={() => setActiveView('explorer')}
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
        placeholder="Search"
        class="w-full px-3 py-2 rounded border text-sm"
        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
        on:keydown={handleKeyDown}
      />
      <button 
        class="absolute right-2 top-1/2 transform -translate-y-1/2 p-1 rounded"
        style="color: var(--text-secondary);"
        on:click={performSearch}
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
        placeholder="Replace"
        class="w-full px-3 py-2 rounded border text-sm"
        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
      />
      <button 
        class="absolute right-2 top-1/2 transform -translate-y-1/2 px-2 py-1 rounded text-xs"
        style="background-color: var(--accent); color: #ffffff;"
        on:click={performReplace}
      >
        Replace All
      </button>
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
        placeholder="files to include"
        class="w-full px-3 py-1.5 rounded border text-xs"
        style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);"
      />
      <input 
        type="text" 
        bind:value={excludePattern}
        placeholder="files to exclude"
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
        {searchResults.length} results
      </div>
      {#each searchResults as result}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <div 
          class="px-3 py-2 cursor-pointer transition-colors hover:bg-dark"
          style="border-bottom: 1px solid var(--border);"
          role="button"
          tabindex="0"
          on:click={() => handleResultClick(result)}
        >
          <div class="flex items-center gap-2 mb-1">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--text-secondary);">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
            </svg>
            <span class="text-sm truncate-text" style="color: var(--text-primary);">{result.filePath}</span>
          </div>
          <div class="pl-6 text-xs" style="color: var(--text-secondary);">
            Line {result.line}: {result.content}
          </div>
        </div>
      {/each}
    {:else if searchQuery}
      <div class="flex items-center justify-center py-8">
        <span class="text-sm" style="color: var(--text-secondary);">No results found</span>
      </div>
    {/if}
  </div>
</div>

<style>
.truncate-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>