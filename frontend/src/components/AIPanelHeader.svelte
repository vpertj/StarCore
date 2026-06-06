<script>
    import { get } from 'svelte/store'
    import { clearMessages } from '../stores/ai.js'
    import { loadConversations, conversations, loadMessages, activeMessages, deleteConversation } from '../stores/memory.js'
  import { messages } from '../stores/ai.js'
    import { currentProject } from '../stores/app.js'
  import { aiPanelVisible } from '../stores/ui.js'
  import { activeModelId, allAvailableModels } from '../stores/provider.js'
 import { t } from '../stores/i18n.js'

    let showHistoryPanel = $state(false)
    let dropdownPos = $state({ top: 0, right: 0 })
    let expandedConvId = $state(/** @type {string|null} */ (null))
    let showContextDetail = $state(false)

  /** Estimate token count from message content (CJK ~1.5, ASCII words ~1.3, punct ~0.4) */
  function estimateTokens(text) {
    if (!text) return 0
    let cjk = 0, asciiWords = 0, otherChars = 0, inWord = false
    for (const ch of text) {
      const code = ch.charCodeAt(0)
      if ((code >= 0x4E00 && code <= 0x9FFF) || (code >= 0x3400 && code <= 0x4DBF) ||
          (code >= 0x3000 && code <= 0x303F) || (code >= 0xFF00 && code <= 0xFFEF) ||
          (code >= 0x3040 && code <= 0x309F) || (code >= 0x30A0 && code <= 0x30FF) ||
          (code >= 0xAC00 && code <= 0xD7AF)) {
        cjk++
        inWord = false
      } else if ((code >= 0x61 && code <= 0x7A) || (code >= 0x41 && code <= 0x5A) || (code >= 0x30 && code <= 0x39) || code === 0x5F) {
        if (!inWord) { asciiWords++; inWord = true }
      } else {
        if (inWord) inWord = false
        otherChars++
      }
    }
    return Math.round(cjk * 1.5 + asciiWords * 1.3 + otherChars * 0.4) + 1
  }

  /** Estimate context window from model name (mirrors Go's EstimateContextWindow in openai.go) */
  function estimateContextWindow(/** @type {string} */ modelId) {
    if (!modelId) return 128000
    const m = modelId.toLowerCase()
    // GPT-4o / o1 / o3
    if (m.includes('gpt-4o') || m.includes('o1-') || m.includes('o3-') || m.includes('o1-mini') || m.includes('o3-mini')) return 200000
    if (m.includes('gpt-4')) return 128000
    // Claude
    if (m.includes('claude-3-opus') || m.includes('claude-opus-4')) return 200000
    if (m.includes('claude-3.5') || m.includes('claude-sonnet-4') || m.includes('claude-3-sonnet') || m.includes('claude-3-haiku') || m.includes('claude-haiku-4')) return 200000
    // DeepSeek
    if (m.includes('deepseek-v4') || m.includes('deepseek-r1')) return 1048576
    if (m.includes('deepseek')) return 65536
    // Gemini
    if (m.includes('gemini-2') || m.includes('gemma')) return 1048576
    if (m.includes('gemini')) return 32768
    // Open source models
    if (m.includes('llama') || m.includes('mistral') || m.includes('mixtral') || m.includes('qwen')) return 32768
    // GPT-3.5
    if (m.includes('gpt-3.5')) return 16385
    return 128000
  }

  function formatK(n) {
    if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
    if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
    return String(n)
  }

  let contextUsage = $derived.by(() => {
    const msgs = $messages
    let tokens = 0
    for (const m of msgs) {
      if (typeof m.content === 'string') tokens += estimateTokens(m.content)
    }
    const model = $allAvailableModels.find(m => m.id === $activeModelId)
    const window = model?.contextWindow || estimateContextWindow(model?.id || '')
    const pct = window > 0 ? Math.round(tokens / window * 1000) / 10 : 0
    return { tokens, window, pct }
  })

  function closeDropdowns(e) {
    const t = /** @type {HTMLElement} */ (e.target)
    if (!t.closest('.dropdown-trigger') && !t.closest('.history-dropdown') && !t.closest('.context-detail')) {
      showHistoryPanel = false
      showContextDetail = false
    }
  }

  async function toggleHistory(e) {
    showHistoryPanel = !showHistoryPanel
    if (showHistoryPanel) {
      // Position dropdown relative to viewport (fixed) to escape overflow:hidden ancestors
      const btn = /** @type {HTMLElement} */ (e.currentTarget)
      const rect = btn.getBoundingClientRect()
      dropdownPos = { top: rect.bottom + 4, right: window.innerWidth - rect.right }
      if ($currentProject) {
        await loadConversations($currentProject)
      }
    }
  }
</script>

<svelte:window onclick={closeDropdowns} />

<div class="ai-header" style="border-color: var(--border); background-color: var(--bg-secondary);">
  <div class="ai-header-left">
    <span class="text-xs font-medium" style="color: var(--text-secondary);">AI Chat</span>
    <div class="relative">
      <button
        class="text-[11px] px-1.5 py-0.5 rounded-full font-semibold cursor-pointer transition-opacity hover:opacity-80 flex items-center gap-1"
        style="background-color: {contextUsage.pct > 80 ? 'rgba(241,76,76,0.15)' : contextUsage.pct > 50 ? 'rgba(255,165,0,0.12)' : 'rgba(100,180,255,0.1)'}; color: {contextUsage.pct > 80 ? '#f14c4c' : contextUsage.pct > 50 ? '#ff8c00' : '#64b5f6'};"
        onclick={(e) => { e.stopPropagation(); const rect = /** @type {HTMLElement} */ (e.currentTarget).getBoundingClientRect(); dropdownPos = { top: rect.bottom + 4, right: window.innerWidth - rect.right }; showContextDetail = !showContextDetail; showHistoryPanel = false }}
      >
        <svg viewBox="0 0 16 16" class="w-3 h-3" fill="none">
          <circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="2" stroke-opacity="0.2"/>
          <circle cx="8" cy="8" r="6.5" stroke="currentColor" stroke-width="2"
            stroke-dasharray="{Math.min(contextUsage.pct, 100) * 0.4084} 40.84"
            stroke-dashoffset="10.21"
            stroke-linecap="round"
            style="transform: rotate(-90deg); transform-origin: center;"/>
        </svg>
        {contextUsage.pct}%
      </button>
      {#if showContextDetail}
        <div class="context-detail fixed z-[9999] mt-1 rounded shadow-lg p-3 text-xs space-y-1.5" style="top: {dropdownPos.top}px; right: {dropdownPos.right}px; background-color: var(--bg-secondary); border: 1px solid var(--border); width: 220px;">
          <div class="flex justify-between">
            <span style="color: var(--text-muted);">已用 tokens</span>
            <span class="font-medium" style="color: var(--text-primary);">{formatK(contextUsage.tokens)}</span>
          </div>
          <div class="flex justify-between">
            <span style="color: var(--text-muted);">上下文窗口</span>
            <span class="font-medium" style="color: var(--text-primary);">{formatK(contextUsage.window)}</span>
          </div>
          <div class="flex justify-between">
            <span style="color: var(--text-muted);">使用比例</span>
            <span class="font-semibold" style="color: {contextUsage.pct > 80 ? '#f14c4c' : contextUsage.pct > 50 ? '#ff8c00' : '#64b5f6'};">{contextUsage.pct}%</span>
          </div>
          <div class="flex justify-between">
            <span style="color: var(--text-muted);">消息数</span>
            <span style="color: var(--text-primary);">{$messages.length}</span>
          </div>
          <div class="border-t pt-1.5 mt-1.5" style="border-color: var(--border);">
            <div class="w-full rounded-full h-1.5" style="background-color: var(--border);">
              <div class="h-full rounded-full transition-all" style="width: {Math.min(contextUsage.pct, 100)}%; background-color: {contextUsage.pct > 80 ? '#f14c4c' : contextUsage.pct > 50 ? '#ff8c00' : '#64b5f6'};"></div>
            </div>
          </div>
        </div>
      {/if}
    </div>
  </div>
  <div class="ai-header-right">
    <div class="relative">
      <button
        class="dropdown-trigger p-1 rounded transition-colors shrink-0"
        style="color: var(--text-secondary);"
        onclick={(e) => { e.stopPropagation(); toggleHistory(e) }}
        title={$t('history.title')}
        aria-label={$t('history.title')}
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      </button>
      {#if showHistoryPanel}
        <!-- svelte-ignore a11y_no_static_element_interactions -->
        <div class="history-dropdown fixed z-[9999] rounded shadow-lg overflow-hidden" style="top: {dropdownPos.top}px; right: {dropdownPos.right}px; background-color: var(--bg-secondary); border: 1px solid var(--border); width: 280px; max-height: 360px;">
        <div class="flex items-center justify-between px-3 py-2 border-b" style="border-color: var(--border);">
          <span class="text-xs font-medium" style="color: var(--text-primary);">{$t('history.title')}</span>
          <button class="p-0.5 rounded hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => showHistoryPanel = false} aria-label="Close history">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>
        </div>
        <div class="overflow-y-auto" style="max-height: 320px;">
          {#if $conversations.length === 0}
            <div class="px-3 py-4 text-xs text-center" style="color: var(--text-muted);">{$t('history.empty')}</div>
          {:else}
            {#each $conversations as conv}
              {@const isExpanded = expandedConvId === conv.id}
              <div class="border-b" style="border-color: var(--border);">
                <div
                  class="w-full flex items-center gap-1 px-3 py-2 text-xs transition-colors text-left group" style="color: var(--text-primary);"
                >
                  <button
                    class="shrink-0 p-1 rounded transition-colors"
                    style="color: {isExpanded ? 'var(--accent)' : 'var(--text-muted)'};"
                    title="上下文信息"
                    onclick={(e) => {
                      e.stopPropagation()
                      expandedConvId = isExpanded ? null : conv.id
                    }}
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                  </button>
                  <button
                    class="flex-1 flex flex-col gap-0.5 text-left min-w-0"
                    onclick={async () => {
                      await loadMessages(conv.id)
                      const msgs = get(activeMessages)
                      if (msgs && msgs.length > 0) {
                        messages.set(msgs.map(m => ({
                          role: m.role, content: m.content, timestamp: new Date(m.createdAt).getTime()
                        })))
                      }
                      showHistoryPanel = false
                    }}
                  >
                    <span class="truncate font-medium">{conv.title || 'Untitled'}</span>
                    <span class="truncate" style="color: var(--text-muted);">{conv.model || ''} · {conv.messageCount || 0} msgs</span>
                  </button>
                  <button
                    class="shrink-0 p-1 rounded opacity-0 group-hover:opacity-100 transition-opacity hover:bg-red-500/20"
                    style="color: #f14c4c;"
                    title="删除对话"
                    onclick={async (e) => {
                      e.stopPropagation()
                      await deleteConversation(conv.id)
                      await loadConversations($currentProject)
                    }}
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                  </button>
                </div>
                {#if isExpanded}
                  <div class="px-3 pb-2 space-y-1 text-[11px]" style="color: var(--text-muted);">
                    <div class="flex justify-between">
                      <span>消息数</span>
                      <span style="color: var(--text-secondary);">{conv.messageCount || 0}</span>
                    </div>
                    <div class="flex justify-between">
                      <span>模型</span>
                      <span style="color: var(--text-secondary);">{conv.model || '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span>Agent</span>
                      <span style="color: var(--text-secondary);">{conv.agentId || '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span>创建时间</span>
                      <span style="color: var(--text-secondary);">{conv.createdAt ? new Date(conv.createdAt).toLocaleString() : '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span>更新时间</span>
                      <span style="color: var(--text-secondary);">{conv.updatedAt ? new Date(conv.updatedAt).toLocaleString() : '-'}</span>
                    </div>
                    <div class="flex justify-between">
                      <span>预估上下文占用</span>
                      <span style="color: var(--text-secondary);">{conv.messageCount ? Math.round(conv.messageCount * 1.5) : 0}K tokens</span>
                    </div>
                  </div>
                {/if}
              </div>
            {/each}
          {/if}
        </div>
      </div>
    {/if}
    </div>

    
    <button
      class="p-1 rounded transition-colors shrink-0"
      style="color: var(--text-secondary);"
      onclick={clearMessages}
      title={$t('history.newChat')}
      aria-label="新建对话"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
      </svg>
    </button>

    <button
      class="p-1 rounded transition-colors shrink-0"
      style="color: var(--text-secondary);"
      onclick={() => aiPanelVisible.set(false)}
      title={$t('common.close')}
      aria-label="关闭 AI 面板"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </button>
  </div>
</div>

<style>
.ai-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  border-bottom: 1px solid;
  min-width: 0;
  overflow: hidden;
  gap: 4px;
}

.ai-header-left {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
  overflow: hidden;
  flex-shrink: 1;
}

.ai-header-right {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}
</style>
