<script>
import { bottomPanelVisible, bottomPanelTab, aiPanelVisible } from './stores/ui.js'
import { settingsVisible, activeFile, editorGroups } from './stores/app.js'
 import { initCustomModels } from './stores/provider.js'
  import { EventsOn } from '../wailsjs/runtime/runtime.js'
  import { editorSettings, updateEditorSetting, initEditorSettings } from './stores/editorSettings.js'
  import { get } from 'svelte/store'
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

let isSplitDragging = $state(false)

let splitRatio = $state(0.5)

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

// TODO: Wire up first-run check — show welcome dialog when no AI provider is configured.
// Currently disabled; there is no isProviderConfigured() check yet.
let showWelcome = $state(false)
let welcomeUnsub = null

// Sync editor settings from backend file system (authoritative source)
onMount(() => {
  initEditorSettings()

  // Debug keyboard shortcuts
  window.addEventListener('keydown', handleDebugKeys)

  EventsOn('app:session_restore', async (state) => {
    if (!state?.activeConvId) return
    try {
      const { activeConversationId, loadMessages } = await import('./stores/memory.js')
      activeConversationId.set(state.activeConvId)
      const msgs = await window.backend.GetMessages(state.activeConvId)
      if (msgs && msgs.length > 0) {
        const { messages } = await import('./stores/ai.js')
        messages.set(msgs.map(m => ({
          role: m.role,
          content: m.content,
          timestamp: new Date(m.createdAt).getTime()
        })))
      }
      if (state.providerId) {
        const { activeProviderId } = await import('./stores/provider.js')
        activeProviderId.set(state.providerId)
      }
      if (state.model) {
        const { activeModelId } = await import('./stores/provider.js')
        activeModelId.set(state.model)
      }
      if (state.agentId) {
        const { activeAgentId } = await import('./stores/agent.js')
        activeAgentId.set(state.agentId)
      }
    } catch (e) {
      console.error('Failed to restore session:', e)
    }
  })
})

async function handleDebugKeys(e) {
  if (e.target?.tagName === 'INPUT' || e.target?.tagName === 'TEXTAREA') return
  const { debugSession, continueDebug, stepOver, stepIn, stepOut, stopDebug } = await import('./stores/debug.js')
  let session = null
  debugSession.subscribe(v => session = v)()
  if (!session && e.key !== 'F5') return

  if (e.key === 'F5' && !e.shiftKey) {
    e.preventDefault()
    if (session) await continueDebug()
    else {
      const { startDebug } = await import('./stores/debug.js')
      const { currentProject } = await import('./stores/app.js')
      let proj = null
      currentProject.subscribe(v => proj = v)()
      if (proj) await startDebug(proj)
    }
  } else if (e.key === 'F5' && e.shiftKey) {
    e.preventDefault()
    await stopDebug()
  } else if (e.key === 'F10') {
    e.preventDefault()
    await stepOver()
  } else if (e.key === 'F11' && !e.shiftKey) {
    e.preventDefault()
    await stepIn()
  } else if (e.key === 'F11' && e.shiftKey) {
    e.preventDefault()
    await stepOut()
  }
}

/** @param {WheelEvent} e */
function handleWheel(e) {
  if (!e.ctrlKey && !e.metaKey) return
  e.preventDefault()
  const el = document.activeElement
  const editorArea = el?.closest('.editor-pane, .cm-editor, .cm-content, .editor-content')
  if (editorArea) {
    const settings = get(editorSettings)
    const delta = e.deltaY > 0 ? -1 : 1
    const next = Math.max(8, Math.min(32, settings.fontSize + delta))
    updateEditorSetting('fontSize', next)
  }
}

onDestroy(() => {
  if (welcomeUnsub) { welcomeUnsub(); welcomeUnsub = null }
  document.removeEventListener('wheel', handleWheel)
  window.removeEventListener('keydown', handleDebugKeys)
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
            <TabBar groupId="group-1" />
            <Breadcrumb filePath={$activeFile || ''} />
            <div class="editor-content">
              <CodeEditor groupId="group-1" />
            </div>
          </div>

          {#if hasSplit}
            <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions, a11y_no_noninteractive_element_interactions -->
            <div
              class="split-divider"
              class:active={isSplitDragging}
              onmousedown={startSplitResize}
              role="separator"
              aria-orientation="vertical"
            ></div>
            {@const group2 = $editorGroups.find(g => g.id === 'group-2')}
            <div class="editor-pane" style="flex: {1 - splitRatio};">
              <TabBar groupId="group-2" />
              <Breadcrumb filePath={group2?.activeFile || ''} />
              <div class="editor-content">
                <CodeEditor groupId="group-2" />
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
        <button class="px-5 py-2 rounded-lg text-sm font-medium transition-all hover:opacity-90" style="background: var(--accent); color: var(--text-on-accent);" onclick={() => { showWelcome = false; settingsVisible.set(true); }}>
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
