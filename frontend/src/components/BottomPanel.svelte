<script>
  import { bottomPanelVisible, bottomPanelHeight, bottomPanelTab, bottomPanelMaximized, preMaximizeHeight, MIN_PANEL_HEIGHT, toggleMaximizePanel } from '../stores/ui.js'
  import Terminal from './Terminal.svelte'
  import { activeFileDiagnostics } from '../stores/diagnostics.js'
  import { logEntries, clearLogs } from '../stores/output.js'
  import { onMount } from 'svelte'
  import { terminalTabs, activeTerminalId, createTerminalTab, closeTerminalTab, ensureDefaultTerminal } from '../stores/terminal.js'
  import { t } from '../stores/i18n.js'

  onMount(async () => {
    await ensureDefaultTerminal()
  })

  import { get } from 'svelte/store'

  let isResizing = $state(false)

  const tabs = [
    { id: 'problems', labelKey: 'terminal.problems', icon: 'problems' },
    { id: 'output', labelKey: 'terminal.output', icon: 'output' },
    { id: 'terminal', labelKey: 'terminal.title', icon: 'terminal' },
  ]

  function onPanelHeaderMouseDown(e) {
    // Only start resize when clicking near the top edge of the header (within 12px)
    const rect = e.currentTarget.getBoundingClientRect()
    const offsetY = e.clientY - rect.top
    if (offsetY > 12 || e.button !== 0) return

    e.preventDefault()
    isResizing = true
    const startY = e.clientY
    const startHeight = get(bottomPanelHeight)

    function onMouseMove(ev) {
      ev.preventDefault()
      const delta = startY - ev.clientY
      const newHeight = Math.max(MIN_PANEL_HEIGHT, Math.min(window.innerHeight * 0.85, startHeight + delta))
      bottomPanelHeight.set(newHeight)
    }

    function onMouseUp() {
      isResizing = false
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    }

    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
  }

  function handleMaximize() {
    toggleMaximizePanel()
  }

  function handleKillTerminal(id) {
    closeTerminalTab(id)
  }
</script>

<div class="bottom-panel" style="height: {$bottomPanelHeight}px;" class:hidden={!$bottomPanelVisible}>
    <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions a11y_no_noninteractive_element_interactions -->
    <div
      class="panel-header"
      class:resizing={isResizing}
      onmousedown={onPanelHeaderMouseDown}
    >
      <div class="tab-bar">
        {#each tabs as tab}
          <button
            class="tab-btn"
            class:active={$bottomPanelTab === tab.id}
            onclick={() => bottomPanelTab.set(tab.id)}
          >
            {#if tab.icon === 'terminal'}
              <svg viewBox="0 0 16 16" class="tab-icon" fill="none" stroke="currentColor" stroke-width="1.2">
                <rect x="1" y="1.5" width="14" height="13" rx="1.5"/>
                <path d="M4 6l3 2.5L4 11" stroke-linecap="round" stroke-linejoin="round"/>
                <path d="M9 11h3"/>
              </svg>
            {:else if tab.icon === 'output'}
              <svg viewBox="0 0 16 16" class="tab-icon" fill="none" stroke="currentColor" stroke-width="1.2">
                <rect x="1.5" y="1.5" width="13" height="13" rx="1.5"/>
                <path d="M5 5h6M5 8h4M5 11h5"/>
              </svg>
            {:else}
              <svg viewBox="0 0 16 16" class="tab-icon" fill="none" stroke="currentColor" stroke-width="1.2">
                <circle cx="8" cy="8" r="5.5"/>
                <path d="M8 5v3.5l2.5 1" stroke-linecap="round"/>
              </svg>
            {/if}
            <span>{$t(tab.labelKey)}</span>
          </button>
        {/each}
      </div>

      <div class="header-actions">
        {#if $bottomPanelTab === 'terminal'}
          <div class="terminal-tabs">
            {#each $terminalTabs as termTab (termTab.id)}
              <button
                class="term-tab-btn"
                class:active={$activeTerminalId === termTab.id}
                onclick={() => activeTerminalId.set(termTab.id)}
              >
                <svg viewBox="0 0 16 16" class="term-tab-icon" fill="none" stroke="currentColor" stroke-width="1.2">
                  <rect x="1" y="1.5" width="14" height="13" rx="1.5"/>
                  <path d="M4 6l3 2L4 10"/>
                  <path d="M9 10h3"/>
                </svg>
                <span class="term-tab-name">{termTab.title}</span>
                {#if $terminalTabs.length > 1}
                  <!-- svelte-ignore a11y_click_events_have_key_events -->
                  <span
                    class="term-close"
                    onclick={(e) => { e.stopPropagation(); handleKillTerminal(termTab.id); }}
                    role="button"
                    tabindex="0"
                  >
                    <svg viewBox="0 0 12 12" class="term-close-icon" fill="none" stroke="currentColor" stroke-width="1.5">
                      <path d="M3 3l6 6M9 3l-6 6"/>
                    </svg>
                  </span>
                {/if}
              </button>
            {/each}
          </div>
          <div class="action-separator"></div>
          <button class="action-btn" title={$t('terminal.newTerminal')} onclick={createTerminalTab}>
            <svg viewBox="0 0 16 16" class="action-icon" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M8 3v10M3 8h10"/>
            </svg>
          </button>
          <button class="action-btn" title={$t('terminal.clear')} onclick={() => { if ($activeTerminalId && window.backend) window.backend.TerminalWrite($activeTerminalId, '\x1b[2J\x1b[H') }}>
            <svg viewBox="0 0 16 16" class="action-icon" fill="none" stroke="currentColor" stroke-width="1.5">
              <path d="M2 4l4-2 4 2M14 4l-4-2-4 2M2 12l4 2 4-2M14 12l-4 2-4-2"/>
            </svg>
          </button>
        {/if}
        <button class="action-btn" title={$t('terminal.maximize')} onclick={handleMaximize}>
          {#if $bottomPanelMaximized}
            <svg viewBox="0 0 16 16" class="action-icon" fill="none" stroke="currentColor" stroke-width="1.2">
              <path d="M3 5h10M3 11h10M5 3v10M11 3v10"/>
            </svg>
          {:else}
            <svg viewBox="0 0 16 16" class="action-icon" fill="none" stroke="currentColor" stroke-width="1.2">
              <path d="M3 3h10v10H3z"/>
            </svg>
          {/if}
        </button>
        <button
          class="action-btn close-btn"
          onclick={() => bottomPanelVisible.set(false)}
          title={$t('terminal.close')}
        >
          <svg viewBox="0 0 16 16" class="action-icon" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M4 4l8 8M12 4l-8 8"/>
          </svg>
        </button>
      </div>
    </div>

    <div class="panel-content">
      {#if $bottomPanelTab === 'terminal'}
        {#if $terminalTabs.length > 0}
          {#each $terminalTabs as termTab (termTab.id)}
            <div class="terminal-pane" class:hidden={$activeTerminalId !== termTab.id}>
              <Terminal terminalId={termTab.id} />
            </div>
          {/each}
        {:else}
          <div class="empty-panel">
            <svg viewBox="0 0 24 24" class="empty-icon" fill="none" stroke="currentColor" stroke-width="1.2">
              <rect x="2" y="3" width="20" height="18" rx="2"/>
              <path d="M6 9l4 3-4 3M12 15h5"/>
            </svg>
            <span>{$t('terminal.clickNew')}</span>
          </div>
        {/if}
      {:else if $bottomPanelTab === 'output'}
        <div class="panel-content-inner">
          {#if $logEntries.length === 0}
            <div class="empty-panel">
              <span>{$t('terminal.noOutput')}</span>
            </div>
          {:else}
            <div class="output-actions">
              <button class="action-btn" title={$t('terminal.clearOutput')} onclick={clearLogs}>
                <svg viewBox="0 0 16 16" class="action-icon" fill="none" stroke="currentColor" stroke-width="1.5">
                  <path d="M2 2l12 12M14 2L2 14"/>
                </svg>
              </button>
            </div>
            {#each $logEntries as entry}
              <div class="output-entry output-{entry.level}">
                <span class="output-time">{new Date(entry.timestamp).toLocaleTimeString()}</span>
                <span class="output-source">{entry.source}</span>
                <span class="output-message">{entry.message}</span>
              </div>
            {/each}
          {/if}
        </div>
      {:else if $bottomPanelTab === 'problems'}
        <div class="panel-content-inner">
          {#if $activeFileDiagnostics.length === 0}
            <div class="empty-panel">
              <span>{$t('terminal.noProblems')}</span>
            </div>
          {:else}
            {#each $activeFileDiagnostics as diag}
              <div class="diagnostic-item diagnostic-{diag.severity}">
                <span class="diag-severity">{diag.severity === 'error' ? '\u2718' : diag.severity === 'warning' ? '\u26A0' : '\u2139'}</span>
                <span class="diag-message">{diag.message}</span>
                {#if diag.filePath}
                  <span class="diag-file">{diag.filePath.split(/[\\/]/).pop()}</span>
                {/if}
              </div>
            {/each}
          {/if}
        </div>
      {/if}
    </div>
  </div>

<style>
.bottom-panel {
  display: flex;
  flex-direction: column;
  background-color: var(--bg-primary);
  min-height: 0;
  flex-shrink: 0;
}

.bottom-panel.hidden {
  display: none;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 35px;
  padding: 0;
  border-bottom: 1px solid var(--bg-tertiary);
  border-top: 3px solid var(--accent);
  flex-shrink: 0;
  background-color: var(--bg-primary);
  cursor: ns-resize;
  user-select: none;
}

.panel-header.resizing {
  border-top-color: var(--accent-hover);
}

.tab-bar {
  display: flex;
  align-items: center;
  height: 100%;
  gap: 0;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 0 12px;
  height: 100%;
  font-size: 11px;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-bottom: 1px solid transparent;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  white-space: nowrap;
}

.tab-btn:hover {
  color: var(--text-secondary);
}

.tab-btn.active {
  color: var(--text-primary);
  border-bottom-color: var(--accent);
}

.tab-icon {
  width: 14px;
  height: 14px;
  flex-shrink: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 0;
  padding-right: 2px;
  height: 100%;
}

.terminal-tabs {
  display: flex;
  align-items: center;
  height: 100%;
  gap: 0;
  border-left: 1px solid var(--bg-tertiary);
  margin-left: 4px;
}

.term-tab-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 0 10px;
  height: 100%;
  font-size: 11px;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-right: 1px solid var(--bg-tertiary);
  cursor: pointer;
  transition: background-color 0.1s, color 0.1s;
  white-space: nowrap;
}

.term-tab-btn:hover {
  background-color: var(--bg-hover);
  color: var(--text-secondary);
}

.term-tab-btn.active {
  background-color: var(--bg-primary);
  color: var(--text-primary);
}

.term-tab-icon {
  width: 12px;
  height: 12px;
  flex-shrink: 0;
}

.term-tab-name {
  max-width: 100px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.term-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 3px;
  opacity: 0;
  transition: opacity 0.1s;
  flex-shrink: 0;
  margin-left: 2px;
}

.term-tab-btn:hover .term-close {
  opacity: 0.5;
}

.term-close:hover {
  opacity: 1 !important;
  background-color: var(--bg-active);
}

.term-close-icon {
  width: 10px;
  height: 10px;
}

.action-separator {
  width: 1px;
  height: 16px;
  background-color: var(--bg-tertiary);
  margin: 0 2px;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-radius: 3px;
  cursor: pointer;
  transition: background-color 0.1s, color 0.1s;
  padding: 0;
}

.action-btn:hover {
  background-color: var(--bg-active);
  color: var(--text-primary);
}

.action-icon {
  width: 14px;
  height: 14px;
}

.close-btn:hover {
  background-color: rgba(241, 76, 76, 0.15);
  color: #f14c4c;
}

.panel-content {
  flex: 1;
  overflow: hidden;
  min-height: 0;
  background-color: var(--bg-primary);
}

.panel-content-inner {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.diagnostic-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px 12px;
  font-size: 12px;
  border-bottom: 1px solid var(--border);
  font-family: 'Consolas', 'Cascadia Code', monospace;
}

.diagnostic-error { color: #f14c4c; }
.diagnostic-warning { color: #cca700; }

.output-actions {
  display: flex;
  justify-content: flex-end;
  padding: 2px 4px;
  border-bottom: 1px solid var(--bg-tertiary);
}

.output-entry {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 3px 12px;
  font-size: 12px;
  font-family: 'Consolas', 'Cascadia Code', monospace;
  border-bottom: 1px solid var(--border);
}

.output-info { color: var(--text-primary); }
.output-warn { color: #cca700; background-color: rgba(204, 167, 0, 0.05); }
.output-error { color: #f14c4c; background-color: rgba(241, 76, 76, 0.05); }

.output-time {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 11px;
  width: 70px;
}

.output-source {
  flex-shrink: 0;
  color: var(--accent);
  font-size: 11px;
  font-weight: 600;
  width: 40px;
}

.output-message {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.diag-severity {
  flex-shrink: 0;
  width: 16px;
  text-align: center;
}

.diag-message {
  flex: 1;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.diag-file {
  flex-shrink: 0;
  color: var(--text-muted);
  font-size: 11px;
  margin-left: auto;
}

.terminal-pane {
  width: 100%;
  height: 100%;
}

.terminal-pane.hidden {
  display: none;
}

.empty-panel {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--text-muted);
  font-size: 12px;
  gap: 8px;
}

.empty-icon {
  width: 32px;
  height: 32px;
  opacity: 0.4;
}
</style>
