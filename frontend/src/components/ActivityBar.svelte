<script>
import { activeView, setActiveView, aiPanelVisible } from '../stores/ui.js'
import { settingsVisible, currentProject } from '../stores/app.js'
import { t } from '../stores/i18n.js'

const views = [
  { id: 'explorer', labelKey: 'activitybar.explorer', shortcut: 'Ctrl+Shift+E', path: 'M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z' },
  { id: 'search', labelKey: 'activitybar.search', shortcut: 'Ctrl+Shift+F', path: 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z' },
  { id: 'git', labelKey: 'activitybar.git', shortcut: 'Ctrl+Shift+G', path: 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1' },
  { id: 'ai', labelKey: 'activitybar.ai', shortcut: 'Ctrl+Shift+A', path: 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z' },
  { id: 'extensions', labelKey: 'activitybar.extensions', shortcut: '', path: 'M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01' }
]

/** @param {string} viewId */
function handleClick(viewId) {
  if (viewId === 'ai') {
    aiPanelVisible.update(v => !v)
  } else {
    setActiveView(viewId)
  }
}

/** @param {string} viewId */
function isActive(viewId) {
  if (viewId === 'ai') return $aiPanelVisible
  return $activeView === viewId
}

async function openFolder() {
  try {
    const folder = await window.backend.OpenFolder()
    if (folder) {
      currentProject.set(folder)
    }
  } catch (e) {
    console.error('Failed to open folder:', e)
  }
}
</script>

<div class="activitybar flex flex-col items-center py-2 gap-1" style="width: 48px; background-color: var(--bg-primary);">
  {#each views as view}
    <button
      class="w-10 h-10 flex items-center justify-center rounded transition-colors relative"
      style="color: {isActive(view.id) ? '#ffffff' : 'var(--text-secondary)'}; background-color: {isActive(view.id) ? 'var(--bg-secondary)' : 'transparent'};"
      onclick={() => handleClick(view.id)}
      title="{$t(view.labelKey)}{view.shortcut ? ` (${view.shortcut})` : ''}"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d={view.path} />
      </svg>
      {#if isActive(view.id)}
        <div class="absolute left-0 top-1/2 -translate-y-1/2 w-[2px] h-5 rounded-r" style="background-color: var(--accent);"></div>
      {/if}
    </button>
  {/each}

  <div class="flex-1"></div>

  {#if !$currentProject}
    <button
      class="w-10 h-10 flex items-center justify-center rounded transition-colors"
      style="color: #4ec9b0"
      title={$t('openFolder')}
      onclick={openFolder}
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
      </svg>
    </button>
  {/if}

  <button
    class="w-10 h-10 flex items-center justify-center rounded transition-colors"
    style="color: var(--text-secondary)"
    title={$t('activitybar.settings')}
     onclick={() => settingsVisible.set(!$settingsVisible)}
  >
    <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
      <path stroke-linecap="round" stroke-linejoin="round" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path stroke-linecap="round" stroke-linejoin="round" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  </button>
</div>

<style>
.activitybar button {
  display: flex;
  align-items: center;
  justify-content: center;
}

.activitybar button svg {
  flex-shrink: 0;
}
</style>
