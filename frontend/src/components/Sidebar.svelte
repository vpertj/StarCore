<script>
import { activeView, sidebarVisible, sidebarWidth } from '../stores/ui.js'
import { currentProject, openProjectFolder, recentProjects, openProjectPath, openProjects, switchProject, closeProject } from '../stores/app.js'
import ProjectExplorer from './ProjectExplorer.svelte'
import SearchPanel from './SearchPanel.svelte'
import GitPanel from './GitPanel.svelte'
import ExtensionsPanel from './ExtensionsPanel.svelte'
import { t } from '../stores/i18n.js'

let isDragging = $state(false)
let showProjectSwitcher = $state(false)

/** @param {MouseEvent} e */
function startResize(e) {
  isDragging = true
  const startX = e.clientX
  const startWidth = $sidebarWidth
  document.addEventListener('mousemove', onResize)
  document.addEventListener('mouseup', stopResize)

  /** @param {MouseEvent} e */
  function onResize(e) {
    const delta = e.clientX - startX
    const newWidth = Math.max(180, Math.min(500, startWidth + delta))
    sidebarWidth.set(newWidth)
  }

  function stopResize() {
    isDragging = false
    document.removeEventListener('mousemove', onResize)
    document.removeEventListener('mouseup', stopResize)
  }
}

function getViewTitle() {
  switch ($activeView) {
    case 'explorer': return $t('sidebar.explorer').toUpperCase()
    case 'search': return $t('sidebar.search').toUpperCase()
    case 'git': return $t('sidebar.git').toUpperCase()
    case 'extensions': return $t('sidebar.extensions').toUpperCase()
    default: return ''
  }
}

async function openFolder() {
  await openProjectFolder()
}
</script>

<div class="sidebar-container" class:hidden={!$sidebarVisible} style="width: {$sidebarVisible ? $sidebarWidth + 'px' : '0px'};">
  <div class="h-full flex flex-col border-r" style="width: {$sidebarWidth}px; background-color: var(--bg-secondary); border-color: var(--border); min-width: {$sidebarWidth}px;">
    <!-- Project switcher -->
    {#if $openProjects.length > 0}
      <div class="relative border-b" style="border-color: var(--border);">
        <button
          class="w-full flex items-center gap-2 px-3 py-2 text-xs transition-colors hover:bg-white/5"
          style="color: var(--text-primary);"
          onclick={() => showProjectSwitcher = !showProjectSwitcher}
        >
          <svg viewBox="0 0 16 16" class="w-3.5 h-3.5 shrink-0" fill="none" stroke="currentColor" style="color: var(--accent);">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/>
          </svg>
          <span class="truncate font-medium">{$currentProject?.split(/[\\/]/).pop() || $t('sidebar.noProject')}</span>
          <svg viewBox="0 0 16 16" class="w-3 h-3 shrink-0 ml-auto" fill="none" stroke="currentColor" style="color: var(--text-muted); transform: rotate({showProjectSwitcher ? 180 : 0}deg); transition: transform 0.15s;">
            <path stroke-linecap="round" stroke-width="1.5" d="M4 6l4 4 4-4"/>
          </svg>
        </button>
        {#if showProjectSwitcher}
          <div class="absolute left-0 right-0 z-50 border-b shadow-lg" style="background-color: var(--bg-secondary); border-color: var(--border);">
            {#each $openProjects as proj}
              {@const isActive = proj === $currentProject}
              <div class="flex items-center gap-2 px-3 py-1.5 text-xs transition-colors hover:bg-white/5" style="color: {isActive ? 'var(--accent)' : 'var(--text-primary)'};">
                <button class="flex-1 text-left truncate" onclick={() => { switchProject(proj); showProjectSwitcher = false }}>
                  {#if isActive}<span class="mr-1">●</span>{/if}
                  {proj.split(/[\\/]/).pop() || proj}
                </button>
                <button class="p-0.5 rounded opacity-0 hover:opacity-100 hover:bg-red-500/20" style="color: #f14c4c;" onclick={(e) => { e.stopPropagation(); closeProject(proj) }} title={$t('sidebar.closeProject')}>
                  <svg viewBox="0 0 16 16" class="w-3 h-3" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-width="2" d="M4 4l8 8M12 4l-8 8"/></svg>
                </button>
              </div>
            {/each}
            <button class="w-full flex items-center gap-2 px-3 py-1.5 text-xs transition-colors hover:bg-white/5" style="color: var(--text-secondary);" onclick={() => { openProjectFolder(); showProjectSwitcher = false }}>
              <span>+</span>
              <span>{$t('sidebar.addProjectFolder')}</span>
            </button>
          </div>
        {/if}
      </div>
    {/if}

    <div class="flex items-center justify-between px-4 py-2 text-xs font-semibold tracking-wider" style="color: var(--text-secondary);">
      <span>{getViewTitle()}</span>
    </div>

    <div class="flex-1 overflow-hidden">
      {#if $activeView === 'explorer'}
        {#if !$currentProject}
          <div class="flex flex-col items-center justify-center h-full gap-4 px-6">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-12 h-12" style="color: var(--border);" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
            </svg>
            <p class="text-sm text-center" style="color: var(--text-secondary);">{$t('noProject')}</p>
            <button
              class="px-4 py-2 rounded text-sm font-medium transition-colors"
              style="background-color: var(--accent); color: var(--text-on-accent);"
              onclick={openFolder}
            >
              {$t('openFolder')}
            </button>
            {#if $recentProjects.length > 0}
              <div class="w-full mt-2">
                <p class="text-xs font-medium mb-2" style="color: var(--text-secondary);">{$t('recentProjects')}</p>
                {#each $recentProjects as path}
                  <button
                    class="w-full text-left px-3 py-1.5 rounded text-xs truncate transition-colors hover:opacity-80"
                    style="color: var(--text-primary); background-color: var(--bg-primary);"
                    title={path}
                    onclick={() => openProjectPath(path)}
                  >
                    {path.split(/[\\/]/).pop() || path}
                  </button>
                {/each}
              </div>
            {/if}
          </div>
        {:else}
          <ProjectExplorer />
        {/if}
      {:else if $activeView === 'search'}
        <SearchPanel />
      {:else if $activeView === 'git'}
        <GitPanel />
      {:else if $activeView === 'extensions'}
        <ExtensionsPanel />
      {/if}
    </div>
  </div>

  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions a11y-no-noninteractive-element-interactions -->
  <div
    class="cursor-col-resize hover:bg-blue-500/20 transition-colors"
    style="width: 3px; background-color: {isDragging ? 'var(--accent)' : 'transparent'};"
    role="separator"
    aria-orientation="vertical"
    onmousedown={startResize}
  ></div>
</div>

<style>
.sidebar-container {
  display: flex;
  flex-shrink: 0;
  height: 100%;
  transition: width 0.2s ease;
  overflow: hidden;
}
.sidebar-container.hidden {
  width: 0 !important;
  overflow: hidden;
}
</style>
