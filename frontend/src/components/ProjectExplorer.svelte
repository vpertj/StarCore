﻿<script>
 import { fade } from 'svelte/transition'
 import { currentProject, fileTree, openFile, createNewFile, createNewFolder, deleteFileOrFolder, renameFileOrFolder } from '../stores/app.js'
 import { t } from '../stores/i18n.js'
 import { addLog } from '../stores/output.js'
 import TreeNode from './TreeNode.svelte'

  /** @typedef {{ name: string, path: string, isDir: boolean, children: any[], loaded?: boolean }} FileItem */

  let expandedDirs = $state(new Set())
  let contextMenuVisible = $state(false)
  let contextMenuPosition = $state({ x: 0, y: 0 })
  /** @type {FileItem|null} */ let selectedItem = $state(null)
  let renameMode = $state(false)
  let renameValue = $state('')
  let newFileMode = $state(false)
  let newFileName = $state('')
 let newFolderMode = $state(false)
 let newFolderName = $state('')
 let analyzing = $state(false)
 let analysisResult = $state('')

 async function analyzeProject() {
   if (!window.backend?.AnalyzeProject || !$currentProject) return
   analyzing = true
   addLog('IDE', 'info', 'Analyzing project structure...')
   try {
     const result = await window.backend.AnalyzeProject($currentProject)
     analysisResult = result
     addLog('IDE', 'info', 'Project analysis complete')
     window.dispatchEvent(new CustomEvent('insert-code', { detail: { code: result } }))
   } catch (/** @type {any} */ err) {
     addLog('IDE', 'error', 'Analysis failed: ' + (err.message || String(err)))
   } finally {
     analyzing = false
   }
 }

 function triggerUpdate() {
    expandedDirs = new Set(expandedDirs)
  }

  async function openProject() {
    const result = await window.backend.OpenFolder()
    if (result) {
      currentProject.set(result)
    }
  }

  /**
   * @param {MouseEvent} e
   * @param {FileItem} item
   */
  function showContextMenu(e, item) {
    e.preventDefault()
    e.stopPropagation()
    selectedItem = item
    contextMenuPosition = { x: e.clientX, y: e.clientY }
    contextMenuVisible = true
    renameMode = false
    newFileMode = false
    newFolderMode = false
  }

  function hideContextMenu() {
    contextMenuVisible = false
    renameMode = false
    newFileMode = false
    newFolderMode = false
  }

  async function handleRename() {
    if (renameValue.trim() && selectedItem) {
      await renameFileOrFolder(selectedItem.path, renameValue.trim())
      hideContextMenu()
    }
  }

  function getParentPath(item) {
    const lastSep = Math.max(item.path.lastIndexOf('/'), item.path.lastIndexOf('\\'))
    return item.isDir ? item.path : (lastSep >= 0 ? item.path.substring(0, lastSep) : item.path)
  }

  async function handleNewFile() {
    if (newFileName.trim() && selectedItem) {
      await createNewFile(getParentPath(selectedItem), newFileName.trim())
      hideContextMenu()
    }
  }

  async function handleNewFolder() {
    if (newFolderName.trim() && selectedItem) {
      await createNewFolder(getParentPath(selectedItem), newFolderName.trim())
      hideContextMenu()
    }
  }

  async function handleDelete() {
    if (selectedItem) {
      if (confirm(`Are you sure you want to delete "${selectedItem.name}"?`)) {
        await deleteFileOrFolder(selectedItem.path)
        hideContextMenu()
      }
    }
  }

  function startRename() {
    if (!selectedItem) return
    renameValue = selectedItem.name
    renameMode = true
  }

  function startNewFile() {
    newFileName = ''
    newFileMode = true
  }

  function startNewFolder() {
    newFolderName = ''
    newFolderMode = true
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions a11y_no_noninteractive_element_interactions -->
<div class="h-full flex flex-col" onclick={hideContextMenu} role="region" aria-label="é¡¹ç›®èµ„æºç®¡ç†å™¨">
  <div class="flex items-center justify-between px-3 py-2 border-b" style="border-color: var(--border);">
    <h2 class="text-sm font-medium" style="color: var(--text-secondary);">{$t('sidebar.explorer').toUpperCase()}</h2>
    <button
      class="p-1 rounded transition-colors"
      title="Open Folder"
      style="color: var(--text-secondary);"
      onclick={openProject}
      aria-label="æ‰“å¼€æ–‡ä»¶å¤¹"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
         <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v3m0 0v3m0-3h3m-3 0H7" />
       </svg>
     </button>
     {#if $currentProject}
       <button
         class="p-1 rounded transition-colors"
         title="Analyze Project"
         style="color: var(--text-secondary);"
         onclick={analyzeProject}
         disabled={analyzing}
       >
         {#if analyzing}
           <svg class="w-4 h-4 animate-spin" viewBox="0 0 16 16" fill="none">
             <circle cx="8" cy="8" r="6" stroke="currentColor" stroke-width="2" stroke-dasharray="8 6" />
           </svg>
         {:else}
           <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
             <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z" />
           </svg>
         {/if}
       </button>
     {/if}
   </div>

  <div class="flex-1 overflow-y-auto p-2">
    {#if !$currentProject}
      <div class="text-center py-8">
        <p class="text-sm mb-4" style="color: var(--text-secondary);">{$t('noProject')}</p>
        <button
          class="px-4 py-2 rounded transition-colors text-sm"
          style="background-color: var(--accent); color: #ffffff;"
          onclick={openProject}
        >
          {$t('openFolder')}
        </button>
      </div>
    {:else}
      <div class="font-mono text-sm">
        <div class="px-2 py-1 truncate-text" style="color: var(--text-secondary);" title={$currentProject}>
          {$currentProject}
        </div>
        {#each $fileTree as file (file.path)}
          <TreeNode
            item={file}
            depth={0}
            expandedDirs={expandedDirs}
            onToggle={triggerUpdate}
            onFileClick={openFile}
            onContextMenu={showContextMenu}
          />
        {/each}
      </div>
    {/if}
  </div>
</div>

{#if contextMenuVisible}
  <div
    class="dropdown-menu fixed z-50"
    style="left: {contextMenuPosition.x}px; top: {contextMenuPosition.y}px; min-width: 150px;"
    transition:fade={{ duration: 100 }}
  >
    {#if renameMode}
      <div class="px-3 py-2">
        <input
          type="text"
          bind:value={renameValue}
          class="input-field input-field-sm"
          placeholder="New name"
          onkeydown={(e) => e.key === 'Enter' && handleRename()}
        />
        <div class="flex gap-2 mt-2">
          <button class="btn btn-primary btn-sm flex-1" onclick={handleRename}>Rename</button>
          <button class="btn btn-secondary btn-sm flex-1" onclick={hideContextMenu}>Cancel</button>
        </div>
      </div>
    {:else if newFileMode}
      <div class="px-3 py-2">
        <input
          type="text"
          bind:value={newFileName}
          class="input-field input-field-sm"
          placeholder="File name"
          onkeydown={(e) => e.key === 'Enter' && handleNewFile()}
        />
        <div class="flex gap-2 mt-2">
          <button class="btn btn-primary btn-sm flex-1" onclick={handleNewFile}>Create</button>
          <button class="btn btn-secondary btn-sm flex-1" onclick={hideContextMenu}>Cancel</button>
        </div>
      </div>
    {:else if newFolderMode}
      <div class="px-3 py-2">
        <input
          type="text"
          bind:value={newFolderName}
          class="input-field input-field-sm"
          placeholder="Folder name"
          onkeydown={(e) => e.key === 'Enter' && handleNewFolder()}
        />
        <div class="flex gap-2 mt-2">
          <button class="btn btn-primary btn-sm flex-1" onclick={handleNewFolder}>Create</button>
          <button class="btn btn-secondary btn-sm flex-1" onclick={hideContextMenu}>Cancel</button>
        </div>
      </div>
    {:else}
      <button class="dropdown-item" onclick={startNewFile}>New File</button>
      <button class="dropdown-item" onclick={startNewFolder}>New Folder</button>
      <div class="border-t my-1" style="border-color: var(--border);"></div>
      <button class="dropdown-item" onclick={startRename}>Rename</button>
      <button class="dropdown-item" style="color: var(--error);" onclick={handleDelete}>Delete</button>
    {/if}
  </div>
{/if}

<style>
.truncate-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
