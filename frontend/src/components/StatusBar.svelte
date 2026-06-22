<script>
import { activeFile } from '../stores/app.js'
import { activeProviderId, activeModelId } from '../stores/provider.js'
import { activeAgentId, agents } from '../stores/agent.js'
import { bottomPanelVisible } from '../stores/ui.js'
import { completionLoading, completionVisible } from '../stores/completion.js'
import { t } from '../stores/i18n.js'

/** @param {string|null} filePath */
function getLanguage(filePath) {
  if (!filePath) return ''
  const ext = filePath.split('.').pop()?.toLowerCase() || ''
  switch (ext) {
    case 'go': return 'Go'
    case 'js': case 'mjs': case 'cjs': return 'JavaScript'
    case 'ts': case 'tsx': return 'TypeScript'
    case 'json': return 'JSON'
    case 'md': return 'Markdown'
    case 'html': return 'HTML'
    case 'css': return 'CSS'
    case 'svelte': return 'Svelte'
    case 'py': return 'Python'
    case 'rs': return 'Rust'
    default: return 'Plain Text'
  }
}

let activeAgentName = $derived($agents.find(a => a.id === $activeAgentId)?.name || 'AI')
</script>

  <div class="flex items-center justify-between px-4 py-1 border-t text-xs" style="background-color: var(--accent); color: var(--text-on-accent); border-color: var(--border);">
  <div class="flex items-center gap-4">
    <span class="truncate-text max-w-[300px]" title={$activeFile}>
      {#if $activeFile}
        {$activeFile}
      {:else}
        {$t('statusbar.noFile')}
      {/if}
    </span>
    <span>{getLanguage($activeFile)}</span>
  </div>

  <div class="flex items-center gap-4">
    <button class="flex items-center gap-1 cursor-pointer bg-transparent border-none text-inherit text-xs p-0 truncate max-w-[300px]" title="AI Provider" onclick={() => bottomPanelVisible.set(true)}>
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
      </svg>
      <span class="truncate">{activeAgentName} · {$activeProviderId} / {$activeModelId}</span>
    </button>
    {#if $completionLoading}
      <span style="color: var(--warning);">⟳ {$t('completion.loading')}</span>
    {:else if $completionVisible}
      <span style="color: var(--text-muted);">{$t('completion.ready')} ({$t('completion.tabAccept')})</span>
    {/if}
    <span>UTF-8</span>
    <span>LF</span>
    <span class="flex items-center gap-1">
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      {$t('statusbar.version')}
    </span>
  </div>
</div>


