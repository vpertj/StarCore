<script>
 import { openedFiles, activeFile, closeFile, togglePinTab, reorderTab } from '../stores/app.js'
 import { t } from '../stores/i18n.js'

 let dragIndex = $state(-1)

 function handleClose(e, filePath) {
   e.stopPropagation()
   closeFile(filePath)
 }

 function getFileIcon(fileName) {
   const ext = fileName.split('.').pop()?.toLowerCase() || ''
   switch (ext) {
     case 'go':
     case 'js':
     case 'ts':
       return 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z'
     case 'json':
       return 'M4 6h16M4 10h16M4 14h16M4 18h16'
     case 'md':
     case 'txt':
       return 'M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V4a2 2 0 00-2-2zM4 6h16v4H4V6zm0 6h16v4H4v-4z'
     default:
       return 'M14 2H6a2 2 0 00-2 2v16a2 2 0 002 2h12a2 2 0 002-2V4a2 2 0 00-2-2zM4 6h16M4 10h16M4 14h16M4 18h16'
   }
 }

 function handleDragStart(e, i) {
   e.dataTransfer?.setData('text/plain', String(i))
   dragIndex = i
 }

 function handleDragOver(e) {
   e.preventDefault()
 }

 function handleDrop(e, i) {
   e.preventDefault()
   const from = parseInt(e.dataTransfer?.getData('text/plain') || '-1')
   reorderTab(from, i)
   dragIndex = -1
 }

 function handleDblClick(e, filePath) {
   togglePinTab(filePath)
 }

 let sortedFiles = $derived(
   $openedFiles.slice().sort((a, b) => {
     if (a.pinned && !b.pinned) return -1
     if (!a.pinned && b.pinned) return 1
     return 0
   })
 )
</script>

<div class="flex items-center gap-1 px-2 py-1 border-b overflow-x-auto" style="background-color: var(--bg-tertiary); border-color: var(--border);">
  {#if $openedFiles.length === 0}
    <div class="px-4 py-2 text-sm" style="color: var(--text-secondary)">{$t('filesOpen')}</div>
  {/if}

  {#each sortedFiles as file, i}
    <button
      class="tab-btn"
      style="background-color: {$activeFile === file.path ? 'var(--bg-primary)' : 'var(--bg-tertiary)'}; color: {$activeFile === file.path ? 'var(--text-primary)' : 'var(--text-secondary)'}"
      onclick={() => activeFile.set(file.path)}
      ondblclick={(e) => handleDblClick(e, file.path)}
      draggable="true"
      ondragstart={(e) => handleDragStart(e, i)}
      ondragover={handleDragOver}
      ondrop={(e) => handleDrop(e, i)}
      role="tab"
      tabindex="0"
    >
      {#if file.pinned}
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 flex-shrink-0" fill="currentColor" viewBox="0 0 16 16" style="color: var(--text-primary);">
          <path d="M9.828.722a.5.5 0 0 1 .354.146l4.95 4.95a.5.5 0 0 1 0 .707c-.48.48-1.072.588-1.503.588-.776 0-1.722-.447-2.322-.934l-2.172 2.172 3.086 3.086a.5.5 0 0 1-.707.707l-3.086-3.086-2.172 2.172c.487.6.934 1.546.934 2.322 0 .43-.108 1.023-.588 1.503a.5.5 0 0 1-.707 0l-4.95-4.95a.5.5 0 0 1 0-.707c.48-.48 1.072-.588 1.503-.588.776 0 1.722.447 2.322.934l2.172-2.172-3.086-3.086a.5.5 0 0 1 .707-.707l3.086 3.086 2.172-2.172c-.487-.6-.934-1.546-.934-2.322 0-.43.108-1.023.588-1.503a.5.5 0 0 1 .354-.146z"/>
        </svg>
      {:else}
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={getFileIcon(file.name)} />
        </svg>
      {/if}
      <span class="tab-label" title={file.name}>{file.name}</span>
      <span
        class="tab-close"
        onclick={(e) => handleClose(e, file.path)}
        onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') handleClose(e, file.path); }}
        role="button"
        tabindex="0"
        aria-label="Close tab"
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </span>
    </button>
  {/each}
</div>


<style>
.tab-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 4px 4px 0 0;
  transition: background-color 0.15s;
  min-width: max-content;
  cursor: pointer;
  border: none;
  font-size: 13px;
}

.tab-label {
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.tab-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 3px;
  color: var(--text-secondary);
  opacity: 0;
  transition: opacity 0.1s, background-color 0.1s;
  cursor: pointer;
  margin-left: 2px;
}

.tab-btn:hover .tab-close {
  opacity: 0.7;
}

.tab-close:hover {
  opacity: 1 !important;
  background-color: rgba(255, 255, 255, 0.1);
  color: var(--text-primary);
}
</style>