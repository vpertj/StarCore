<script>
  import { fade, slide } from 'svelte/transition'
  import {
    debugSession, debugState, debugRunning,
    startDebug, stopDebug, continueDebug, stepOver, stepIn, stepOut,
    watchExpressions, addWatch, removeWatch, refreshWatches,
    consoleHistory, executeConsole,
  } from '../stores/debug.js'
  import { currentProject, activeFile } from '../stores/app.js'
  import { t } from '../stores/i18n.js'

  let activeTab = $state('variables')
  let consoleInput = $state('')
  let watchInput = $state('')

  let isRunning = $derived($debugRunning)
  let isStopped = $derived($debugState?.status === 'stopped')
  let hasSession = $derived(!!$debugSession)
  let stackFrames = $derived($debugState?.stack || [])
  let variables = $derived($debugState?.variables || [])
  let currentFile = $derived($debugState?.file || '')
  let currentLine = $derived($debugState?.line || 0)

  function goToFrame(frame) {
    if (frame?.file) {
      activeFile.set(frame.file)
      setTimeout(() => {
        window.dispatchEvent(new CustomEvent('search:goto-line', { detail: { line: frame.line } }))
      }, 100)
    }
  }

  function goToCurrentLocation() {
    if (currentFile) {
      activeFile.set(currentFile)
      setTimeout(() => {
        window.dispatchEvent(new CustomEvent('search:goto-line', { detail: { line: currentLine } }))
      }, 100)
    }
  }

  async function handleStartDebug() {
    if (!$currentProject) return
    await startDebug($currentProject)
  }

  async function handleConsoleSubmit() {
    if (!consoleInput.trim()) return
    await executeConsole(consoleInput)
    consoleInput = ''
  }

  async function handleWatchSubmit() {
    if (!watchInput.trim()) return
    await addWatch(watchInput)
    watchInput = ''
  }

  function formatVarValue(v) {
    if (!v) return ''
    if (v.error) return v.error
    return v.value || v.Value || ''
  }

  function formatVarType(v) {
    if (!v) return ''
    return v.type || v.Type || ''
  }
</script>

<div class="h-full flex flex-col" style="background-color: var(--bg-secondary);">
  <!-- Debug toolbar -->
  <div class="flex items-center gap-1 px-2 py-1.5 border-b" style="border-color: var(--border);">
    {#if !hasSession}
      <button class="debug-btn" style="color: var(--success);" onclick={handleStartDebug} title={$t('debug.startDebugging')}>
        <svg viewBox="0 0 16 16" class="w-4 h-4" fill="currentColor"><path d="M4 2l10 6-10 6V2z"/></svg>
      </button>
      <span class="text-[10px] ml-1" style="color: var(--text-muted);">{$t('debug.f5ToStart')}</span>
    {:else}
      {#if isRunning}
        <button class="debug-btn" style="color: var(--warning);" onclick={stopDebug} title={$t('debug.stop')}>
          <svg viewBox="0 0 16 16" class="w-4 h-4" fill="currentColor"><rect x="3" y="3" width="10" height="10" rx="1"/></svg>
        </button>
      {:else}
        <button class="debug-btn" style="color: var(--success);" onclick={continueDebug} title={$t('debug.continue')} disabled={!isStopped}>
          <svg viewBox="0 0 16 16" class="w-4 h-4" fill="currentColor"><path d="M4 2l10 6-10 6V2z"/></svg>
        </button>
        <button class="debug-btn" style="color: var(--accent);" onclick={stepOver} title={$t('debug.stepOver')} disabled={!isStopped}>
          <svg viewBox="0 0 16 16" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 4h4l2 2-2 2H3M9 4h4M11 4v8"/></svg>
        </button>
        <button class="debug-btn" style="color: var(--accent);" onclick={stepIn} title={$t('debug.stepIn')} disabled={!isStopped}>
          <svg viewBox="0 0 16 16" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 3v8M5 8l3 3 3-3M3 13h10"/></svg>
        </button>
        <button class="debug-btn" style="color: var(--accent);" onclick={stepOut} title={$t('debug.stepOut')} disabled={!isStopped}>
          <svg viewBox="0 0 16 16" class="w-4 h-4" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M8 13V5M5 8l3-3 3 3M3 3h10"/></svg>
        </button>
        <button class="debug-btn" style="color: var(--error);" onclick={stopDebug} title={$t('debug.stop')}>
          <svg viewBox="0 0 16 16" class="w-4 h-4" fill="currentColor"><rect x="3" y="3" width="10" height="10" rx="1"/></svg>
        </button>
      {/if}
    {/if}

    {#if hasSession}
      <div class="ml-auto flex items-center gap-2">
        {#if currentFile}
          <button class="text-[10px] px-1.5 py-0.5 rounded cursor-pointer hover:opacity-80" style="background-color: color-mix(in srgb, var(--accent) 10%, transparent); color: var(--accent);" onclick={goToCurrentLocation}>
            {currentFile.split(/[\\/]/).pop()}:{currentLine}
          </button>
        {/if}
        <span class="text-[10px] px-1.5 py-0.5 rounded" style="background-color: {isRunning ? 'color-mix(in srgb, var(--success) 15%, transparent)' : 'color-mix(in srgb, var(--warning) 15%, transparent)'}; color: {isRunning ? 'var(--success)' : 'var(--warning)'};">
          {isRunning ? $t('debug.running') : $debugState?.reason || $t('debug.stopped')}
        </span>
      </div>
    {/if}
  </div>

  {#if hasSession}
    <!-- Tab bar -->
    <div class="flex border-b" style="border-color: var(--border);">
      {#each [{ id: 'variables', labelKey: 'debug.variables' }, { id: 'watch', labelKey: 'debug.watch' }, { id: 'stack', labelKey: 'debug.callStack' }, { id: 'console', labelKey: 'debug.console' }] as tab}
        <button
          class="flex-1 py-1.5 text-[10px] font-medium text-center transition-colors"
          style="color: {activeTab === tab.id ? 'var(--accent)' : 'var(--text-muted)'}; border-bottom: 2px solid {activeTab === tab.id ? 'var(--accent)' : 'transparent'};"
          onclick={() => activeTab = tab.id}
        >{$t(tab.labelKey)}</button>
      {/each}
    </div>
  {/if}

  <div class="flex-1 overflow-y-auto">
    {#if !hasSession}
      <div class="flex flex-col items-center justify-center h-full gap-3 p-4">
        <svg viewBox="0 0 24 24" class="w-10 h-10" fill="none" stroke="currentColor" style="color: var(--border);">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/>
        </svg>
        <p class="text-xs text-center" style="color: var(--text-muted);">{$t('debug.noSession')}</p>
        <button class="px-3 py-1.5 text-xs rounded font-medium" style="background-color: var(--success); color: var(--text-on-accent);" onclick={handleStartDebug}>
          {$t('debug.startDebugging')}
        </button>
      </div>

    {:else if activeTab === 'variables'}
      <!-- Arguments + Locals -->
      {#if variables.length > 0}
        <div class="section-header">
          <span>{$t('debug.argumentsLocals')}</span>
          <span class="text-[10px]" style="color: var(--text-muted);">{variables.length}</span>
        </div>
        {#each variables as v}
          <div class="var-row">
            <span class="var-name">{v.name}</span>
            <span class="var-value">{v.value}</span>
            <span class="var-type">{v.type}</span>
          </div>
          {#if v.children?.length > 0}
            {#each v.children as child}
              <div class="var-row var-child">
                <span class="var-name">.{child.name}</span>
                <span class="var-value">{child.value}</span>
                <span class="var-type">{child.type}</span>
              </div>
            {/each}
          {/if}
        {/each}
      {:else}
        <div class="px-3 py-4 text-xs text-center" style="color: var(--text-muted);">{$t('debug.noVariables')}</div>
      {/if}

    {:else if activeTab === 'watch'}
      <div class="px-2 py-2 border-b" style="border-color: var(--border);">
        <div class="flex gap-1">
          <input
            type="text"
            bind:value={watchInput}
            placeholder={$t('debug.watchPlaceholder')}
            class="flex-1 px-2 py-1 rounded text-xs"
            style="background-color: var(--bg-primary); color: var(--text-primary); border: 1px solid var(--border);"
            onkeydown={(e) => { if (e.key === 'Enter') handleWatchSubmit() }}
          />
          <button class="px-2 py-1 rounded text-[10px]" style="background-color: var(--accent); color: var(--text-on-accent);" onclick={handleWatchSubmit}>{$t('debug.add')}</button>
          <button class="px-2 py-1 rounded text-[10px]" style="background-color: var(--bg-tertiary); color: var(--text-secondary);" onclick={refreshWatches}>{$t('debug.refresh')}</button>
        </div>
      </div>
      {#each $watchExpressions as watch, i}
        <div class="var-row group">
          <span class="var-name">{watch.expr}</span>
          <span class="var-value">{formatVarValue(watch.result)}</span>
          <span class="var-type">{formatVarType(watch.result)}</span>
          <button class="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-red-500/20 ml-auto shrink-0" style="color: var(--error);" onclick={() => removeWatch(i)}>
            <svg viewBox="0 0 12 12" class="w-3 h-3" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-width="1.5" d="M3 3l6 6M9 3l-6 6"/></svg>
          </button>
        </div>
      {/each}
      {#if $watchExpressions.length === 0}
        <div class="px-3 py-4 text-xs text-center" style="color: var(--text-muted);">{$t('debug.noWatch')}</div>
      {/if}

    {:else if activeTab === 'stack'}
      {#if stackFrames.length > 0}
        {#each stackFrames as frame, i}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <div class="stack-row" role="button" tabindex="0" onclick={() => goToFrame(frame)} style="{i === 0 ? 'background-color: color-mix(in srgb, var(--accent) 8%, transparent);' : ''}">
            <span class="stack-fn" style="color: {i === 0 ? 'var(--accent)' : 'var(--text-primary)'};">{frame.function}</span>
            <span class="stack-loc">{frame.file?.split(/[\\/]/).pop()}:{frame.line}</span>
          </div>
        {/each}
      {:else}
        <div class="px-3 py-4 text-xs text-center" style="color: var(--text-muted);">{$t('debug.noStackFrames')}</div>
      {/if}

    {:else if activeTab === 'console'}
      <div class="flex-1 overflow-y-auto px-2 py-1" style="max-height: 200px;">
        {#each $consoleHistory as entry}
          <div class="console-entry">
            <span class="console-prompt">&gt;</span>
            <span class="console-input">{entry.input}</span>
          </div>
          {#if entry.output}
            <div class="console-output">{entry.output}</div>
          {/if}
          {#if entry.error}
            <div class="console-error">{entry.error}</div>
          {/if}
        {/each}
      </div>
      <div class="flex gap-1 px-2 py-2 border-t" style="border-color: var(--border);">
        <span class="text-xs self-center" style="color: var(--text-muted);">&gt;</span>
        <input
          type="text"
          bind:value={consoleInput}
          placeholder={$t('debug.consolePlaceholder')}
          class="flex-1 px-2 py-1 rounded text-xs font-mono"
          style="background-color: var(--bg-primary); color: var(--text-primary); border: 1px solid var(--border);"
          onkeydown={(e) => { if (e.key === 'Enter') handleConsoleSubmit() }}
        />
      </div>
    {/if}
  </div>
</div>

<style>
.debug-btn {
  display: flex; align-items: center; justify-content: center;
  width: 28px; height: 28px; border-radius: 4px;
  background: transparent; border: none; cursor: pointer;
  transition: background-color 0.15s;
}
.debug-btn:hover { background-color: var(--bg-hover); }
.debug-btn:disabled { opacity: 0.3; cursor: default; }
.debug-btn:disabled:hover { background: transparent; }

.section-header {
  display: flex; align-items: center; gap: 6px;
  padding: 5px 12px; font-size: 11px; font-weight: 600;
  color: var(--text-secondary);
  border-bottom: 1px solid var(--border);
}

.var-row {
  display: grid;
  grid-template-columns: 1fr 1.5fr auto;
  gap: 8px;
  padding: 3px 12px;
  font-size: 11px;
  font-family: 'Consolas', monospace;
  transition: background-color 0.1s;
  align-items: center;
}
.var-row:hover { background-color: var(--bg-hover); }
.var-child { padding-left: 28px; grid-template-columns: 1fr 1.5fr auto; }
.var-name { color: var(--error); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.var-value { color: var(--text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.var-type { color: var(--text-muted); font-size: 10px; text-align: right; }

.stack-row {
  display: flex; flex-direction: column; gap: 1px;
  padding: 5px 12px; font-size: 11px;
  cursor: pointer; transition: background-color 0.1s;
}
.stack-row:hover { background-color: var(--bg-hover); }
.stack-fn { font-family: 'Consolas', monospace; }
.stack-loc { font-size: 10px; color: var(--text-muted); }

.console-entry { display: flex; gap: 6px; padding: 2px 0; font-size: 11px; font-family: 'Consolas', monospace; }
.console-prompt { color: var(--success); font-weight: bold; }
.console-input { color: var(--text-primary); }
.console-output { padding: 2px 0 2px 18px; font-size: 11px; font-family: 'Consolas', monospace; color: var(--text-secondary); white-space: pre-wrap; }
.console-error { padding: 2px 0 2px 18px; font-size: 11px; font-family: 'Consolas', monospace; color: var(--error); }
</style>
