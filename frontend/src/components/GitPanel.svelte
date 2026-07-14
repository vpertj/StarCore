<script>
  import { fade, slide } from 'svelte/transition'
  import { gitStatus, gitBranch, gitLog, gitLoading, commitMessage, stageFile, unstageFile, commitChanges, refreshGitStatus, refreshGitLog, stageAll, createBranch, pullChanges, pushChanges } from '../stores/git.js'
  import { currentProject, activeFile } from '../stores/app.js'
  import { t } from '../stores/i18n.js'

  let showLog = $state(false)
  let committing = $state(false)
  let selectedFile = $state(null)
  let fileDiff = $state(null)
  let loadingDiff = $state(false)
  let showBranchInput = $state(false)
  let newBranchName = $state('')
  let showBranches = $state(false)

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
      case 'modified': return 'var(--warning)'
      case 'added': return 'var(--success)'
      case 'deleted': return 'var(--error)'
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

  async function handleCreateBranch() {
    if (!newBranchName.trim()) return
    await createBranch(newBranchName)
    newBranchName = ''
    showBranchInput = false
  }

  async function viewFileDiff(filePath) {
    if (selectedFile === filePath) { selectedFile = null; fileDiff = null; return }
    selectedFile = filePath
    loadingDiff = true
    try { fileDiff = await window.backend.Diff(filePath) } catch (e) { fileDiff = `Error: ${e.message || e}` }
    loadingDiff = false
  }

  function openFile(filePath) { activeFile.set(filePath) }

  let stagedCount = $derived($gitStatus.filter(e => e.staged).length)
  let unstagedCount = $derived($gitStatus.filter(e => !e.staged).length)
</script>

<div class="h-full flex flex-col" style="background-color: var(--bg-secondary);">
  <!-- Branch Header -->
  <div class="flex items-center gap-2 px-3 py-2 border-b" style="border-color: var(--border);">
    <svg viewBox="0 0 16 16" class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" style="color: var(--warning);">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M5 1v8M5 9a3 3 0 100 6M5 9c0-2 2-4 4-4h2M11 5l2-2-2-2"/>
    </svg>
    <button
      class="text-sm font-mono truncate cursor-pointer hover:underline"
      style="color: var(--warning);"
      onclick={() => showBranches = !showBranches}
    >{$gitBranch || 'no branch'}</button>
    <span class="text-xs ml-auto shrink-0" style="color: var(--text-muted);">{$gitStatus.length} files</span>
    <div class="flex items-center gap-1 ml-1 shrink-0">
      <button class="git-toolbar-btn" onclick={pullChanges} title={$t('git.pull')}>
        <svg viewBox="0 0 16 16" class="w-3.5 h-3.5" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 2v8m0 0l-3-3m3 3l3-3M3 13h10"/></svg>
      </button>
      <button class="git-toolbar-btn" onclick={pushChanges} title={$t('git.push')}>
        <svg viewBox="0 0 16 16" class="w-3.5 h-3.5" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 14V6m0 0l-3 3m3-3l3 3M3 3h10"/></svg>
      </button>
    </div>
  </div>

  <!-- Branch panel -->
  {#if showBranches}
    <div class="border-b px-3 py-2 space-y-2" style="border-color: var(--border); background-color: var(--bg-primary);" transition:slide>
      {#if showBranchInput}
        <div class="flex gap-1">
          <input type="text" bind:value={newBranchName} placeholder={$t('git.branchPlaceholder')} class="flex-1 px-2 py-1 rounded text-xs" style="background-color: var(--bg-secondary); color: var(--text-primary); border: 1px solid var(--border);" onkeydown={(e) => { if (e.key === 'Enter') handleCreateBranch(); if (e.key === 'Escape') showBranchInput = false }} autofocus />
          <button class="btn btn-primary btn-sm" onclick={handleCreateBranch}>{$t('git.create')}</button>
          <button class="btn btn-ghost btn-sm" onclick={() => showBranchInput = false}>{$t('git.cancel')}</button>
        </div>
      {:else}
        <div class="flex gap-1">
          <button class="git-sm-btn flex-1" onclick={() => showBranchInput = true}>+ {$t('git.newBranch')}</button>
        </div>
      {/if}
        <div class="text-[11px]" style="color: var(--text-muted);">
          {$t('git.current')}: <span style="color: var(--warning);">{$gitBranch}</span>
        </div>
    </div>
  {/if}

  <!-- Staged / Unstaged toggle -->
  {#if $gitStatus.length > 0}
    <div class="flex border-b text-xs" style="border-color: var(--border);">
      <button class="flex-1 py-1.5 text-center transition-colors" style="color: {stagedCount > 0 ? 'var(--success)' : 'var(--text-muted)'}; border-bottom: 2px solid {stagedCount > 0 ? 'var(--success)' : 'transparent'};" onclick={stageAll}>
        {$t('git.stagedCount').replace('{0}', stagedCount)}
      </button>
      <button class="flex-1 py-1.5 text-center transition-colors" style="color: {unstagedCount > 0 ? 'var(--warning)' : 'var(--text-muted)'}; border-bottom: 2px solid {unstagedCount > 0 ? 'var(--warning)' : 'transparent'};">
        {$t('git.changesCount').replace('{0}', unstagedCount)}
      </button>
    </div>
  {/if}

  <!-- Changes list -->
  <div class="flex-1 overflow-y-auto">
    {#if $gitStatus.length === 0}
      <div class="flex flex-col items-center justify-center h-full gap-2 p-4">
        <svg viewBox="0 0 24 24" class="w-10 h-10" fill="none" stroke="currentColor" style="color: var(--border);">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/>
        </svg>
        <p class="text-sm" style="color: var(--text-muted);">{$t('git.noChanges')}</p>
      </div>
    {:else}
      {#each $gitStatus as entry}
        <div class="git-file-item" class:selected={selectedFile === entry.path}>
          <span class="git-status-badge" style="color: {getStatusColor(entry.status)}; border-color: {getStatusColor(entry.status)};">{getStatusIcon(entry.status)}</span>
          <span class="git-file-path" onclick={() => viewFileDiff(entry.path)} role="button" tabindex="0" onkeydown={(e) => { if (e.key === 'Enter') viewFileDiff(entry.path) }}>{entry.path}</span>
          <button class="git-action" style="color: var(--text-secondary); opacity: 0.5;" onclick={() => openFile(entry.path)} title={$t('git.open')}>
            <svg viewBox="0 0 16 16" class="w-3 h-3" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-width="1.5" d="M10 2H4a1 1 0 00-1 1v10a1 1 0 001 1h8a1 1 0 001-1V5l-3-3z"/><path stroke-linecap="round" stroke-width="1.5" d="M9 1v3h3"/></svg>
          </button>
          {#if entry.staged}
            <button class="git-action" style="color: var(--text-secondary);" onclick={() => unstageFile(entry.path)} title={$t('git.unstage')}>
              <svg viewBox="0 0 16 16" class="w-3 h-3" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-width="1.5" d="M2 8h12M8 2v12"/></svg>
            </button>
          {:else}
            <button class="git-action" style="color: var(--text-secondary);" onclick={() => stageFile(entry.path)} title={$t('git.stage')}>
              <svg viewBox="0 0 16 16" class="w-3 h-3" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-width="2" d="M13 5l-7 7-3-3"/></svg>
            </button>
          {/if}
        </div>
        {#if selectedFile === entry.path}
          <div class="git-diff-panel" transition:slide>
            {#if loadingDiff}
              <div class="px-3 py-2 text-xs" style="color: var(--text-muted);">{$t('git.loadingDiff')}</div>
            {:else if fileDiff}
              <pre class="git-diff-content">{fileDiff}</pre>
            {/if}
          </div>
        {/if}
      {/each}
    {/if}
  </div>

  <!-- Commit area -->
  <div class="border-t px-3 py-2 space-y-2" style="border-color: var(--border);">
    <textarea
      bind:value={$commitMessage}
      placeholder={$t('git.commitPlaceholder')}
      class="w-full px-2 py-1.5 text-sm rounded resize-none"
      style="background-color: var(--bg-primary); color: var(--text-primary); border: 1px solid var(--border); min-height: 36px; max-height: 80px;"
      rows="2"
    ></textarea>
    <div class="flex gap-2">
      <button
        class="flex-1 btn btn-success"
        style="background-color: {$commitMessage.trim() ? 'var(--success)' : 'var(--border)'};"
        disabled={!$commitMessage.trim() || committing || stagedCount === 0}
        onclick={handleCommit}
      >
        {committing ? $t('git.committing') : $t('git.commitWithCount').replace('{0}', stagedCount)}
      </button>
    </div>
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
    <span>{$t('git.history')}</span>
  </button>

  {#if showLog}
    <div class="border-t overflow-y-auto" style="border-color: var(--border); max-height: 200px;" transition:slide>
      {#if $gitLog.length === 0}
        <div class="px-3 py-2 text-xs" style="color: var(--text-muted);">{$t('git.noCommits')}</div>
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
.git-file-item { display: flex; align-items: center; gap: 6px; padding: 4px 12px; font-size: 12px; font-family: 'Consolas', monospace; border-bottom: 1px solid var(--bg-tertiary); transition: background-color 0.1s; }
.git-file-item:hover { background-color: var(--bg-hover); }
.git-file-item.selected { background-color: var(--bg-hover); }
.git-status-badge { display: inline-flex; align-items: center; justify-content: center; width: 18px; height: 18px; font-size: 10px; font-weight: 700; border: 1px solid; border-radius: 3px; flex-shrink: 0; }
.git-file-path { flex: 1; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: var(--text-primary); cursor: pointer; }
.git-file-path:hover { text-decoration: underline; }
.git-action { display: flex; align-items: center; justify-content: center; width: 20px; height: 20px; border-radius: 3px; background: transparent; border: none; cursor: pointer; opacity: 0; transition: opacity 0.15s, background-color 0.15s; flex-shrink: 0; padding: 0; }
.git-file-item:hover .git-action { opacity: 0.7; }
.git-action:hover { opacity: 1 !important; background-color: var(--bg-hover); }
.git-diff-panel { border-bottom: 1px solid var(--border); background-color: var(--bg-primary); max-height: 300px; overflow-y: auto; }
.git-diff-content { margin: 0; padding: 8px 12px; font-size: 11px; font-family: 'Consolas', monospace; line-height: 1.5; white-space: pre-wrap; word-break: break-all; color: var(--text-primary); }
.git-log-entry { display: flex; align-items: center; gap: 8px; padding: 3px 12px; font-size: 11px; font-family: 'Consolas', monospace; border-bottom: 1px solid var(--bg-primary); }
.git-log-hash { color: var(--warning); flex-shrink: 0; }
.git-log-msg { color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.git-toolbar-btn { display: flex; align-items: center; justify-content: center; width: 24px; height: 24px; border-radius: 4px; background: transparent; border: none; cursor: pointer; color: var(--text-secondary); transition: background-color 0.15s; }
.git-toolbar-btn:hover { background-color: var(--bg-hover); color: var(--text-primary); }
.git-sm-btn { padding: 3px 8px; border-radius: 4px; font-size: 11px; border: 1px solid var(--border); background: transparent; cursor: pointer; color: var(--text-secondary); }
.git-sm-btn:hover { background-color: var(--bg-hover); }
</style>
