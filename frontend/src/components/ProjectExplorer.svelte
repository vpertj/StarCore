<script>
  import { fade } from 'svelte/transition'
  import { currentProject, fileTree, openFile, createNewFile, createNewFolder, deleteFileOrFolder, renameFileOrFolder, openProjectFolder, recentProjects, openProjectPath, removeRecentProject } from '../stores/app.js'
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
  let showRecentDropdown = $state(false)

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
    await openProjectFolder()
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

  function toggleRecentDropdown() {
    showRecentDropdown = !showRecentDropdown
  }

  function closeRecentDropdown() {
    showRecentDropdown = false
  }
</script>

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions, a11y_no_noninteractive_element_interactions -->
<div class="h-full flex flex-col" onclick={hideContextMenu} role="region" aria-label="项目管理器">
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
        {#if $recentProjects.length > 0}
          <div class="mt-4 text-left">
            <p class="text-xs font-medium mb-2 px-2" style="color: var(--text-secondary);">{$t('recentProjects')}</p>
            {#each $recentProjects as path}
              <div class="flex items-center gap-1">
                <button
                  class="flex-1 text-left px-3 py-1.5 rounded text-xs truncate transition-colors"
                  style="color: var(--text-primary);"
                  onmouseenter={(e) => e.currentTarget.style.backgroundColor = 'var(--selection)'}
                  onmouseleave={(e) => e.currentTarget.style.backgroundColor = 'transparent'}
                  title={path}
                  onclick={() => openProjectPath(path)}
                >
                  {path.split(/[\\/]/).pop() || path}
                </button>
                <button
                  class="px-1 text-xs opacity-0 transition-opacity"
                  style="color: var(--text-muted);"
                  onmouseenter={(e) => { e.currentTarget.style.opacity = '1'; e.currentTarget.parentElement.querySelector('button').style.paddingRight = '0' }}
                  onmouseleave={(e) => { e.currentTarget.style.opacity = '0' }}
                  onclick={() => removeRecentProject(path)}
                  title={$t('recentProjects.remove')}
                >&times;</button>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {:else}
      <div class="text-sm">
        <div class="flex items-center gap-1 px-2 py-1 group" style="color: var(--text-secondary);">
          <span class="truncate-text flex-1" title={$currentProject}>{$currentProject.split(/[\\/]/).pop() || $currentProject}</span>
          <button
            class="px-1.5 py-0.5 rounded text-xs transition-colors shrink-0"
            style="color: var(--accent);"
            onmouseenter={(e) => e.currentTarget.style.backgroundColor = 'var(--selection)'}
            onmouseleave={(e) => e.currentTarget.style.backgroundColor = 'transparent'}
            onclick={openProject}
            title={$t('openFolder')}
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
            </svg>
          </button>
          {#if $recentProjects.length > 0}
            <div class="relative">
              <button
                class="px-1.5 py-0.5 rounded text-xs transition-colors shrink-0"
                style="color: var(--text-muted);"
                onmouseenter={(e) => e.currentTarget.style.backgroundColor = 'var(--selection)'}
                onmouseleave={(e) => e.currentTarget.style.backgroundColor = 'transparent'}
                onclick={(e) => { e.stopPropagation(); toggleRecentDropdown() }}
                title={$t('recentProjects')}
              >
                <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </button>
              {#if showRecentDropdown}
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div
                  class="absolute right-0 top-full mt-1 z-50 rounded shadow-lg py-1 max-h-64 overflow-y-auto"
                  style="background-color: var(--bg-primary); border: 1px solid var(--border); min-width: 180px; max-width: calc(var(--sidebar-width, 250px) - 16px);"
                  onclick={(e) => e.stopPropagation()}
                >
                  <div class="px-3 py-1 text-xs font-medium" style="color: var(--text-secondary);">{$t('recentProjects')}</div>
                  {#each $recentProjects as path}
                    <button
                      class="w-full text-left px-3 py-1.5 text-xs truncate transition-colors"
                      style="color: {path === $currentProject ? 'var(--accent)' : 'var(--text-primary)'}; background-color: {path === $currentProject ? 'var(--selection)' : 'transparent'};"
                      onmouseenter={(e) => { if (path !== $currentProject) e.currentTarget.style.backgroundColor = 'var(--selection)' }}
                      onmouseleave={(e) => { if (path !== $currentProject) e.currentTarget.style.backgroundColor = 'transparent' }}
                      title={path}
                      onclick={() => { openProjectPath(path); closeRecentDropdown() }}
                    >
                      {path.split(/[\\/]/).pop() || path}
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
        </div>
        <div class="px-2 pb-1 text-xs truncate-text" style="color: var(--text-muted);" title={$currentProject}>
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

{#if showRecentDropdown}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div class="fixed inset-0 z-40" onclick={closeRecentDropdown}></div>
{/if}

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
