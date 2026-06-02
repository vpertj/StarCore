<script>
import { aiPanelVisible, bottomPanelVisible, bottomPanelTab } from '../stores/ui.js'
import { settingsVisible, currentProject } from '../stores/app.js'
import { masterMode, toggleMasterMode } from '../stores/masterMode.js'
import { t } from '../stores/i18n.js'
import { onDestroy } from 'svelte'
import { WindowGetPosition, WindowSetPosition, WindowToggleMaximise } from '../../wailsjs/runtime/runtime.js'

function minimizeWindow() {
  window.backend.MinimizeWindow()
}

function maximizeWindow() {
  window.backend.MaximizeWindow()
}

function closeWindow() {
  window.backend.CloseWindow()
}

/**
 * @param {string|null} path
 * @returns {string}
 */
function getProjectName(path) {
  if (!path) return ''
  return path.split(/[\\/]/).pop() || ''
}

function toggleAIPanel() {
  aiPanelVisible.update(v => !v)
}

function toggleTerminal() {
  if ($bottomPanelVisible && $bottomPanelTab === 'terminal') {
    bottomPanelVisible.set(false)
  } else {
    bottomPanelVisible.set(true)
    bottomPanelTab.set('terminal')
  }
}

let isDragging = false
let dragOffsetX = 0
let dragOffsetY = 0
let rafId = null

/**
 * @param {MouseEvent} event
 */
let dragStarted = false
let dragStartX = 0
let dragStartY = 0

function onWindowMouseMove(event) {
  if (!isDragging) return
  // Only start moving after 3px threshold (distinguish click from drag)
  if (!dragStarted) {
    if (Math.abs(event.screenX - dragStartX) < 3 && Math.abs(event.screenY - dragStartY) < 3) return
    dragStarted = true
  }
  if (rafId !== null) return
  rafId = requestAnimationFrame(() => {
    const newX = event.screenX - dragOffsetX
    const newY = event.screenY - dragOffsetY
    WindowSetPosition(newX, newY)
    rafId = null
  })
}

function stopDragging() {
  isDragging = false
  dragStarted = false
  if (rafId !== null) {
    cancelAnimationFrame(rafId)
    rafId = null
  }
  document.removeEventListener('mousemove', onWindowMouseMove)
  document.removeEventListener('mouseup', onWindowMouseUp)
  document.removeEventListener('mouseleave', stopDragging)
}

function onWindowMouseUp() {
  stopDragging()
}

/**
 * @param {MouseEvent} event
 */
async function onToolbarMouseDown(event) {
  if (event.button !== 0) return
  // Double-click handled by ondblclick
  if (event.detail > 1) return

  const target = /** @type {HTMLElement} */ (event.target)
  if (target.closest('button, .toolbar-btn, .win-btn, .project-badge')) return

  event.preventDefault()

  try {
    const pos = await WindowGetPosition()
    dragOffsetX = event.screenX - pos.x
    dragOffsetY = event.screenY - pos.y
  } catch {
    dragOffsetX = 0
    dragOffsetY = 0
  }

  isDragging = true
  dragStarted = false
  dragStartX = event.screenX
  dragStartY = event.screenY
  document.addEventListener('mousemove', onWindowMouseMove)
  document.addEventListener('mouseup', onWindowMouseUp)
  // Safety: reset drag if mouse leaves the window (e.g. after native resize)
  document.addEventListener('mouseleave', onWindowMouseUp, { once: true })
}

onDestroy(() => {
  if (isDragging) {
    isDragging = false
    if (rafId !== null) {
      cancelAnimationFrame(rafId)
      rafId = null
    }
    document.removeEventListener('mousemove', onWindowMouseMove)
    document.removeEventListener('mouseup', onWindowMouseUp)
  }
})
</script>

<!-- svelte-ignore a11y-no-static-element-interactions -->
<!-- -webkit-app-region 为 Electron 专有属性，在 Wails/WebView2 下无效，已改用前端鼠标事件实现窗口拖动 -->
<div class="toolbar-root" onmousedown={onToolbarMouseDown} ondblclick={(e) => { if (!e.target.closest('button')) { isDragging = false; WindowToggleMaximise() } }}>
  <div class="left-section">
    <span class="logo-dot"></span>
    <span class="font-semibold text-xs" style="color: var(--text-primary)">StarCore</span>

    {#if $currentProject}
      <div class="project-badge">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
        </svg>
        <span title={$currentProject}>{getProjectName($currentProject)}</span>
      </div>
    {/if}
  </div>

  <div class="center-section">
    <button
      class="toolbar-btn"
      style="background-color: {$masterMode ? '#ffcc0015' : 'transparent'}; color: {$masterMode ? '#ffcc00' : 'var(--text-secondary)'};"
      onclick={toggleMasterMode}
      title="AI Auto-Programming Mode"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
      </svg>
      <span>{$t('toolbar.master')}</span>
    </button>

    <button
      class="toolbar-btn"
      style="background-color: {$bottomPanelVisible && $bottomPanelTab === 'terminal' ? '#ffffff10' : 'transparent'}; color: {$bottomPanelVisible && $bottomPanelTab === 'terminal' ? 'var(--text-primary)' : 'var(--text-secondary)'}"
      onclick={toggleTerminal}
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16" />
      </svg>
      <span>{$t('toolbar.terminal')}</span>
    </button>

    <button
      class="toolbar-btn"
      style="background-color: {$aiPanelVisible ? '#ffffff10' : 'transparent'}; color: {$aiPanelVisible ? 'var(--text-primary)' : 'var(--text-secondary)'}"
      onclick={toggleAIPanel}
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
      </svg>
      <span>{$t('toolbar.ai')}</span>
    </button>
  </div>

  <div class="window-controls">
    <button class="win-btn" onclick={minimizeWindow} title="Minimize" aria-label="最小化">
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 12H4" />
      </svg>
    </button>
    <button class="win-btn" onclick={maximizeWindow} title="Maximize" aria-label="最大化">
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
      </svg>
    </button>
    <button class="win-btn win-btn-close" onclick={closeWindow} title="Close" aria-label="关闭">
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </button>
  </div>
</div>

<style>
.toolbar-root {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 28px;
  padding: 0 6px 0 0;
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--bg-tertiary);
  user-select: none;
  flex-shrink: 0;
  z-index: 100;
  position: relative;
}

.left-section {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  height: 100%;
  padding: 0 10px;
  background: linear-gradient(135deg, #1a5c2a 0%, #1e6930 50%, #175023 100%);
  border-right: 1px solid #2a8a3e;
}

.logo-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: #4ade80;
  box-shadow: 0 0 6px #4ade80;
  flex-shrink: 0;
}

@media (max-width: 700px) {
  .toolbar-root .project-badge {
    display: none;
  }
  .toolbar-root .left-section span {
    display: none;
  }
}

.project-badge {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  color: var(--text-secondary);
  background-color: #ffffff08;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.center-section {
  display: flex;
  align-items: center;
  gap: 2px;
}

.toolbar-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 3px 10px;
  border-radius: 4px;
  font-size: 11px;
  transition: background-color 0.15s;
  border: none;
  cursor: pointer;
}

.toolbar-btn:hover {
  background-color: #ffffff10 !important;
}

.window-controls {
  display: flex;
  align-items: center;
}

.win-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 28px;
  color: var(--text-secondary);
  transition: background-color 0.15s;
  border: none;
  cursor: pointer;
  background: transparent;
}

.win-btn:hover {
  background-color: #ffffff10;
  color: var(--text-primary);
}

.win-btn-close:hover {
  background-color: #e81123;
  color: #ffffff;
}
</style>
