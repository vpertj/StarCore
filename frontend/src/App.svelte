<script>
import { bottomPanelVisible, bottomPanelTab, aiPanelVisible } from './stores/ui.js'
import { settingsVisible, activeFile, editorGroups } from './stores/app.js'
 import { initCustomModels } from './stores/provider.js'
import { EventsOn } from '../wailsjs/runtime/runtime.js'
import { onMount, onDestroy } from 'svelte'
import ActivityBar from './components/ActivityBar.svelte'
import Sidebar from './components/Sidebar.svelte'
import Toolbar from './components/Toolbar.svelte'
import TabBar from './components/TabBar.svelte'
import CodeEditor from './components/CodeEditor.svelte'
import BottomPanel from './components/BottomPanel.svelte'
import AIPanel from './components/AIPanel.svelte'
import StatusBar from './components/StatusBar.svelte'
import Settings from './components/Settings.svelte'
import CommandPalette from './components/CommandPalette.svelte'
import Breadcrumb from './components/Breadcrumb.svelte'

/** @type {boolean} */
let isSplitDragging = false

/** @type {number} */
let splitRatio = 0.5

/** @param {MouseEvent} e */
function startSplitResize(e) {
  isSplitDragging = true
  const startX = e.clientX
  const container = /** @type {HTMLElement} */ (e.currentTarget?.parentElement)
  const totalWidth = container?.offsetWidth || 1
  const startRatio = splitRatio

  /** @param {MouseEvent} ev */
  function onResize(ev) {
    const delta = ev.clientX - startX
    const newRatio = startRatio + delta / totalWidth
    splitRatio = Math.max(0.2, Math.min(0.8, newRatio))
  }

  function stopResize() {
    isSplitDragging = false
    document.removeEventListener('mousemove', onResize)
    document.removeEventListener('mouseup', stopResize)
  }

  document.addEventListener('mousemove', onResize)
  document.addEventListener('mouseup', stopResize)
}

let hasSplit = $derived($editorGroups.length > 1)
initCustomModels()

let showWelcome = $state(false)
let welcomeUnsub = null

onMount(() => {
  welcomeUnsub = EventsOn('app:first-run', (data) => {
    if (data?.needsSetup) showWelcome = true
  })
})

onDestroy(() => {
  if (welcomeUnsub) { welcomeUnsub(); welcomeUnsub = null }
})
</script>

<div class="app-shell">
  <Toolbar />

  <div class="app-body">
    <ActivityBar />
    <Sidebar />

    <main class="app-main">
      <div class="editor-area-wrapper">
        <div class="editor-split-area">
          <div class="editor-pane" style="flex: {hasSplit ? splitRatio : 1};">
            <TabBar />
            <Breadcrumb filePath={$activeFile || ''} />
            <div class="editor-content">
              <CodeEditor />
            </div>
          </div>

          {#if hasSplit}
            <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions a11y-no-noninteractive-element-interactions -->
            <div
              class="split-divider"
              class:active={isSplitDragging}
              onmousedown={startSplitResize}
              role="separator"
              aria-orientation="vertical"
            ></div>
            <div class="editor-pane" style="flex: {1 - splitRatio};">
              <TabBar />
              <Breadcrumb filePath={$activeFile || ''} />
              <div class="editor-content">
                <CodeEditor />
              </div>
            </div>
          {/if}
        </div>

        <BottomPanel />
      </div>
    </main>

    <AIPanel />
  </div>

  <StatusBar />
</div>

<Settings />
<CommandPalette />

{#if showWelcome}
  <div class="fixed inset-0 z-[200] flex items-center justify-center" style="background: rgba(0,0,0,0.7); backdrop-filter: blur(4px);" role="dialog" aria-label="欢迎使用 StarCore">
    <div class="rounded-xl shadow-2xl p-8 text-center" style="width: 460px; background: var(--bg-primary); border: 1px solid var(--border);">
      <div class="text-5xl mb-4">🚀</div>
      <h2 class="text-xl font-bold mb-2" style="color: var(--text-primary);">欢迎使用 StarCore</h2>
      <p class="text-sm mb-6" style="color: var(--text-secondary);">AI 驱动的下一代 IDE，让你的编码效率提升 10 倍。</p>
      <div class="text-left space-y-2 mb-6 text-xs" style="color: var(--text-muted);">
        <div class="flex items-start gap-2"><span style="color: #4ade80;">✓</span> 多模型支持：OpenAI · Anthropic · DeepSeek · Ollama</div>
        <div class="flex items-start gap-2"><span style="color: #4ade80;">✓</span> 24 个内置 AI 技能：代码审查、测试生成、重构、安全检查...</div>
        <div class="flex items-start gap-2"><span style="color: #4ade80;">✓</span> Agent 自动编程：让 AI 读代码、写代码、运行命令</div>
        <div class="flex items-start gap-2"><span style="color: #4ade80;">✓</span> 完整 IDE：编辑器、终端、Git、文件管理</div>
      </div>
      <p class="text-xs mb-4 px-3 py-2 rounded" style="background: rgba(255,204,0,0.1); color: #ffcc00; border: 1px solid rgba(255,204,0,0.2);">
        ⚡ 请先配置 AI 提供商以启用智能功能。可免费使用 Ollama 本地模型或 DeepSeek API。
      </p>
      <div class="flex gap-3 justify-center">
        <button class="px-5 py-2 rounded-lg text-sm font-medium transition-all hover:opacity-90" style="background: var(--border); color: var(--text-primary);" onclick={() => showWelcome = false}>
          稍后配置
        </button>
        <button class="px-5 py-2 rounded-lg text-sm font-medium transition-all hover:opacity-90" style="background: var(--accent); color: #fff;" onclick={() => { showWelcome = false; settingsVisible.set(true); }}>
          前往配置 →
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
.app-shell {
  height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background-color: var(--bg-primary);
  position: relative;
}

.app-body {
  flex: 1;
  display: flex;
  overflow: hidden;
  min-height: 0;
  position: relative;
  z-index: 0;
}

.app-main {
  flex: 1;
  display: flex;
  overflow: hidden;
  min-width: 0;
}

.editor-area-wrapper {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-width: 0;
}

.editor-split-area {
  display: flex;
  overflow: hidden;
  flex: 1;
}

.editor-pane {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-width: 0;
}

.editor-content {
  flex: 1;
  overflow: hidden;
  background-color: var(--bg-primary);
}

.split-divider {
  width: 3px;
  cursor: col-resize;
  background-color: var(--border);
  flex-shrink: 0;
}

.split-divider.active {
  background-color: var(--accent);
}

/* Responsive: collapse AIPanel on narrow windows */
@media (max-width: 900px) {
  .app-body > :global(.h-full.flex.flex-col.border-l) {
    width: 320px !important;
  }
}

@media (max-width: 680px) {
  .app-body > :global(.h-full.flex.flex-col.border-l) {
    display: none;
  }
}
</style>
