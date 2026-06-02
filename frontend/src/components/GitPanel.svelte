<script>
  import { fade, slide } from 'svelte/transition'
  import { gitStatus, gitBranch, gitLog, gitLoading, commitMessage, stageFile, unstageFile, commitChanges, refreshGitStatus, refreshGitLog } from '../stores/git.js'
  import { currentProject } from '../stores/app.js'
  import { t } from '../stores/i18n.js'

  let showLog = $state(false)
  let committing = $state(false)

  function getStatusIcon(status) {
    switch (status) {
      case 'modified': return 'M'
      case 'added': return 'A'
      case 'deleted': return 'D'
      case 'renamed': return 'R'
      case 'untracked': return '?'
      default: return ' '
    }
  }

  function getStatusColor(status) {
    switch (status) {
      case 'modified': return '#e5c07b'
      case 'added': return '#2ea043'
      case 'deleted': return '#e74856'
      case 'renamed': return '#4fc1ff'
      case 'untracked': return 'var(--text-secondary)'
      default: return 'var(--text-primary)'
    }
  }

  async function handleCommit() {
    if (committing) return
    committing = true
    await commitChanges($commitMessage)
    committing = false
  }
</script>

<div class="h-full flex flex-col" style="background-color: var(--bg-secondary);">
  <!-- Branch Header -->
  <div class="flex items-center gap-2 px-3 py-2 border-b" style="border-color: var(--border);">
    <svg viewBox="0 0 16 16" class="w-4 h-4" fill="none" stroke="currentColor" style="color: #e5c07b;">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M5 1v8M5 9a3 3 0 100 6M5 9c0-2 2-4 4-4h2M11 5l2-2-2-2"/>
    </svg>
    <span class="text-sm font-mono" style="color: #e5c07b;">{$gitBranch || 'no branch'}</span>
    <span class="text-xs ml-auto" style="color: var(--text-muted);">{$gitStatus.length} changes</span>
  </div>

  <!-- Changes list -->
  <div class="flex-1 overflow-y-auto">
    {#if $gitStatus.length === 0}
      <div class="flex flex-col items-center justify-center h-full gap-2 p-4">
        <svg viewBox="0 0 24 24" class="w-10 h-10" fill="none" stroke="currentColor" style="color: var(--border);">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
        <p class="text-sm" style="color: var(--text-muted);">No changes</p>
      </div>
    {:else}
      {#each $gitStatus as entry}
        <div class="git-file-item">
          <span class="git-status-badge" style="color: {getStatusColor(entry.status)}; border-color: {getStatusColor(entry.status)};">
            {getStatusIcon(entry.status)}
          </span>
          <span class="git-file-path">{entry.path}</span>
          {#if entry.staged}
            <button class="git-action" style="color: var(--text-secondary);" onclick={() => unstageFile(entry.path)} title="Unstage">
              <svg viewBox="0 0 16 16" class="w-3 h-3" fill="none" stroke="currentColor">
                <path stroke-linecap="round" stroke-width="1.5" d="M2 8h12M8 2v12"/>
              </svg>
            </button>
          {:else}
            <button class="git-action" style="color: var(--text-secondary);" onclick={() => stageFile(entry.path)} title="Stage">
              <svg viewBox="0 0 16 16" class="w-3 h-3" fill="none" stroke="currentColor">
                <path stroke-linecap="round" stroke-width="2" d="M13 5l-7 7-3-3"/>
              </svg>
            </button>
          {/if}
        </div>
      {/each}
    {/if}
  </div>

  <!-- Commit area -->
  <div class="border-t px-3 py-2 space-y-2" style="border-color: var(--border);">
    <textarea
      bind:value={$commitMessage}
      placeholder="Commit message..."
      class="w-full px-2 py-1.5 text-sm rounded resize-none"
      style="background-color: var(--bg-primary); color: var(--text-primary); border: 1px solid var(--border); min-height: 36px; max-height: 80px;"
      rows="2"
    ></textarea>
    <button
      class="w-full px-3 py-1.5 text-sm rounded font-medium transition-colors"
      style="background-color: {$commitMessage.trim() ? '#2ea043' : 'var(--border)'}; color: #ffffff;"
      disabled={!$commitMessage.trim() || committing}
      onclick={handleCommit}
    >
      {committing ? 'Committing...' : 'Commit'}
    </button>
  </div>

  <!-- Toggle log -->
  <button
    class="flex items-center gap-1 px-3 py-1.5 text-xs border-t transition-colors"
    style="border-color: var(--border); color: var(--text-secondary);"
    onclick={() => { showLog = !showLog; if (showLog) refreshGitLog(); }}
  >
    <svg viewBox="0 0 16 16" class="w-3 h-3" fill="none" stroke="currentColor" style="transform: rotate({showLog ? 90 : 0}deg); transition: transform 0.15s;">
      <path stroke-linecap="round" stroke-width="1.5" d="M6 3l5 5-5 5"/>
    </svg>
    <span>History</span>
  </button>

  {#if showLog}
    <div class="border-t overflow-y-auto" style="border-color: var(--border); max-height: 200px;" transition:slide>
      {#if $gitLog.length === 0}
        <div class="px-3 py-2 text-xs" style="color: var(--text-muted);">No commits yet</div>
      {:else}
        {#each $gitLog as entry}
          <div class="git-log-entry">
            <span class="git-log-hash">{entry.hash}</span>
            <span class="git-log-msg">{entry.message}</span>
          </div>
        {/each}
      {/if}
    </div>
  {/if}
</div>

<style>
.git-file-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  font-size: 12px;
  font-family: 'Consolas', monospace;
  border-bottom: 1px solid var(--bg-tertiary);
  transition: background-color 0.1s;
}
.git-file-item:hover {
  background-color: rgba(255,255,255,0.03);
}
.git-status-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  font-size: 10px;
  font-weight: 700;
  border: 1px solid;
  border-radius: 3px;
  flex-shrink: 0;
}
.git-file-path {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--text-primary);
}
.git-action {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  border-radius: 3px;
  background: transparent;
  border: none;
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.15s, background-color 0.15s;
  flex-shrink: 0;
  padding: 0;
}
.git-file-item:hover .git-action {
  opacity: 0.7;
}
.git-action:hover {
  opacity: 1 !important;
  background-color: rgba(255,255,255,0.08);
}
.git-log-entry {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 12px;
  font-size: 11px;
  font-family: 'Consolas', monospace;
  border-bottom: 1px solid var(--bg-primary);
}
.git-log-hash {
  color: #e5c07b;
  flex-shrink: 0;
}
.git-log-msg {
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
