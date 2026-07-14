<script>
    import { get } from 'svelte/store'
    import { clearMessages, openTraceViewer } from '../stores/ai.js'
    import { loadConversations, conversations, loadMessages, activeMessages, deleteConversation, activeConversationId, saveConversation } from '../stores/memory.js'
  import { messages } from '../stores/ai.js'
    import { currentProject } from '../stores/app.js'
  import { aiPanelVisible } from '../stores/ui.js'
  import { activeModelId, allAvailableModels } from '../stores/provider.js'
  import { t } from '../stores/i18n.js'

    let showHistoryPanel = $state(false)
    let dropdownPos = $state({ top: 0, right: 0 })
    let expandedConvId = $state(/** @type {string|null} */ (null))
    let showContextDetail = $state(false)
    let searchQuery = $state('')
    let renamingId = $state(null)
    let renameValue = $state('')

  function estimateTokens(text) {
    if (!text) return 0
    let cjk = 0, asciiWords = 0, otherChars = 0, inWord = false
    for (const ch of text) {
      const code = ch.charCodeAt(0)
      if ((code >= 0x4E00 && code <= 0x9FFF) || (code >= 0x3400 && code <= 0x4DBF) ||
          (code >= 0x3000 && code <= 0x303F) || (code >= 0xFF00 && code <= 0xFFEF) ||
          (code >= 0x3040 && code <= 0x309F) || (code >= 0x30A0 && code <= 0x30FF) ||
          (code >= 0xAC00 && code <= 0xD7AF)) {
        cjk++; inWord = false
      } else if ((code >= 0x61 && code <= 0x7A) || (code >= 0x41 && code <= 0x5A) || (code >= 0x30 && code <= 0x39) || code === 0x5F) {
        if (!inWord) { asciiWords++; inWord = true }
      } else { if (inWord) inWord = false; otherChars++ }
    }
    return Math.round(cjk * 1.5 + asciiWords * 1.3 + otherChars * 0.4) + 1
  }

  function estimateContextWindow(/** @type {string} */ modelId) {
    if (!modelId) return 128000
    const m = modelId.toLowerCase()
    if (m.includes('gpt-4o') || m.includes('o1') || m.includes('o3')) return 200000
    if (m.includes('gpt-4')) return 128000
    if (m.includes('gpt-3.5')) return 16385
    if (m.includes('claude-3-opus') || m.includes('claude-opus-4')) return 200000
    if (m.includes('claude-3.5') || m.includes('claude-sonnet-4') ||
        m.includes('claude-3-sonnet') || m.includes('claude-3-haiku') ||
        m.includes('claude-haiku-4')) return 200000
    if (m.includes('deepseek-v4') || m.includes('deepseek-r1')) return 1048576
    if (m.includes('deepseek')) return 65536
    if (m.includes('gemini-2') || m.includes('gemma')) return 1048576
    if (m.includes('gemini')) return 32768
    if (m.includes('llama') || m.includes('mistral') || m.includes('mixtral') || m.includes('qwen')) return 32768
    return 128000
  }

  function formatK(n) {
    if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M'
    if (n >= 1000) return (n / 1000).toFixed(1) + 'K'
    return String(n)
  }

  let tokenStats = $state({ tokensIn: 0, tokensOut: 0, cachedTokens: 0, count: 0 })

  // Load token stats from localStorage
  function loadTokenStats() {
    try {
      const saved = JSON.parse(localStorage.getItem('starcore-token-usage') || '{}')
      tokenStats = {
        tokensIn: saved.tokensIn || 0,
        tokensOut: saved.tokensOut || 0,
        cachedTokens: saved.cachedTokens || 0,
        count: saved.count || 0,
      }
    } catch {}
  }

  let contextUsage = $derived.by(() => {
    const msgs = $messages
    let tokens = 0

    // Estimate system prompt overhead (~2000 tokens for agent + mode prompts)
    tokens += 2000

    // Estimate tool definitions (~150 tokens per tool, ~15 tools)
    tokens += 2250

    // Estimate context files, project structure, rules (~500-3000 tokens)
    tokens += 1500

    // Count message text content
    for (const m of msgs) {
      if (typeof m.content === 'string') tokens += estimateTokens(m.content)
      // Count tool call arguments if present
      if (m.toolCalls) {
        for (const tc of m.toolCalls) {
          if (tc.args) tokens += estimateTokens(JSON.stringify(tc.args))
        }
      }
    }

    const model = $allAvailableModels.find(m => m.id === $activeModelId)
    const window = model?.contextWindow || estimateContextWindow(model?.id || '')
    // Backend compresses at 80% — show against effective limit
    const effectiveWindow = Math.floor(window * 0.8)
    const pct = effectiveWindow > 0 ? Math.round(tokens / effectiveWindow * 1000) / 10 : 0

    // Load cumulative stats
    loadTokenStats()

    return { tokens, window, effectiveWindow, pct, ...tokenStats }
  })

  let filteredConversations = $derived.by(() => {
    if (!searchQuery.trim()) return $conversations
    const q = searchQuery.toLowerCase()
    return $conversations.filter(c => (c.title || '').toLowerCase().includes(q) || (c.model || '').toLowerCase().includes(q))
  })

  let groupedConversations = $derived.by(() => {
    const groups = {}
    const now = new Date()
    const today = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
    const yesterday = today - 86400000
    const weekAgo = today - 7 * 86400000
    for (const conv of filteredConversations) {
      const ts = new Date(conv.updatedAt || conv.createdAt).getTime()
      let label = $t('history.older')
      if (ts >= today) label = $t('history.today')
      else if (ts >= yesterday) label = $t('history.yesterday')
      else if (ts >= weekAgo) label = $t('history.thisWeek')
      if (!groups[label]) groups[label] = []
      groups[label].push(conv)
    }
    return groups
  })

  function closeDropdowns(e) {
    const el = /** @type {HTMLElement} */ (e.target)
    if (!el.closest('.dropdown-trigger') && !el.closest('.history-dropdown') && !el.closest('.context-detail')) {
      showHistoryPanel = false
      showContextDetail = false
      if (renamingId) cancelRename()
    }
  }

  async function toggleHistory(e) {
    showHistoryPanel = !showHistoryPanel
    if (showHistoryPanel) {
      const btn = /** @type {HTMLElement} */ (e.currentTarget)
      const rect = btn.getBoundingClientRect()
      dropdownPos = { top: rect.bottom + 4, right: window.innerWidth - rect.right }
      if ($currentProject) await loadConversations($currentProject)
    }
  }

  function startRename(conv) {
    renamingId = conv.id
    renameValue = conv.title || ''
  }

  function cancelRename() {
    renamingId = null
    renameValue = ''
  }

  async function confirmRename(conv) {
    if (renameValue.trim() && renameValue !== conv.title) {
      await saveConversation({ ...conv, title: renameValue.trim() })
      if ($currentProject) await loadConversations($currentProject)
    }
    cancelRename()
  }
</script>

<svelte:window onclick={closeDropdowns} />

<div class="ai-header" style="border-color: var(--border); background-color: var(--bg-secondary);">
  <div class="ai-header-left">
      <span class="text-xs font-medium" style="color: var(--text-secondary);">{$t('ai.header.title')}</span>
    <div class="relative">
      <button
        class="text-[11px] px-1.5 py-0.5 rounded-full font-semibold cursor-pointer transition-opacity hover:opacity-80 flex items-center gap-1"
        style="background-color: {contextUsage.pct > 80 ? 'color-mix(in srgb, var(--error) 15%, transparent)' : contextUsage.pct > 50 ? 'color-mix(in srgb, var(--warning) 12%, transparent)' : 'color-mix(in srgb, var(--info) 10%, transparent)'}; color: {contextUsage.pct > 80 ? 'var(--error)' : contextUsage.pct > 50 ? 'var(--warning)' : 'var(--info)'};"
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
        <span class="text-[9px] px-1 py-0 rounded" style="background-color: {contextUsage.cachedTokens > 0 ? 'color-mix(in srgb, var(--success) 20%, transparent)' : 'color-mix(in srgb, var(--text-muted) 10%, transparent)'}; color: {contextUsage.cachedTokens > 0 ? 'var(--success)' : 'var(--text-muted)'};">{contextUsage.cachedTokens > 0 ? formatK(contextUsage.cachedTokens) + ' cached' : 'cache'}</span>
      </button>
      {#if showContextDetail}
        <div class="context-detail fixed z-[9999] mt-1 rounded shadow-lg p-3 text-xs space-y-1.5" style="top: {dropdownPos.top}px; right: {dropdownPos.right}px; background-color: var(--bg-secondary); border: 1px solid var(--border); width: 260px;">
          <!-- Current Context -->
          <div class="font-semibold mb-1" style="color: var(--text-primary);">{$t('ai.context.currentRequest')}</div>
          <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.contextTokens')}</span><span class="font-medium" style="color: var(--text-primary);">{formatK(contextUsage.tokens)}</span></div>
          <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.modelLimit')}</span><span class="font-medium" style="color: var(--text-primary);">{formatK(contextUsage.window)}</span></div>
          <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.effectiveLimit')}</span><span class="font-medium" style="color: var(--text-primary);">{formatK(contextUsage.effectiveWindow)}</span></div>
          <div class="w-full rounded-full h-1.5 mt-1" style="background-color: var(--border);">
            <div class="h-full rounded-full transition-all" style="width: {Math.min(contextUsage.pct, 100)}%; background-color: {contextUsage.pct > 80 ? 'var(--error)' : contextUsage.pct > 50 ? 'var(--warning)' : 'var(--info)'};"></div>
          </div>

          <!-- Cache Info -->
          <div class="border-t pt-1.5 mt-1.5" style="border-color: var(--border);">
            <div class="font-semibold mb-1" style="color: {contextUsage.cachedTokens > 0 ? 'var(--success)' : 'var(--text-muted)'};">{$t('ai.context.cacheHit')}</div>
            <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.cachedTokens')}</span><span class="font-medium" style="color: {contextUsage.cachedTokens > 0 ? 'var(--success)' : 'var(--text-secondary)'};">{formatK(contextUsage.cachedTokens)}</span></div>
            <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.cacheRate')}</span><span class="font-medium" style="color: {contextUsage.cachedTokens > 0 ? 'var(--success)' : 'var(--text-secondary)'};">{contextUsage.tokens > 0 ? Math.round(contextUsage.cachedTokens / contextUsage.tokens * 100) : 0}%</span></div>
            {#if contextUsage.cachedTokens === 0}
              <div class="text-[10px] mt-1" style="color: var(--text-muted);">OpenAI: 自动缓存 &gt;1K tokens 前缀 (50% 折扣)</div>
            {/if}
          </div>

          <!-- Session Stats -->
          {#if contextUsage.count > 0}
            <div class="border-t pt-1.5 mt-1.5" style="border-color: var(--border);">
              <div class="font-semibold mb-1" style="color: var(--text-primary);">{$t('ai.context.sessionStats')}</div>
              <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.requests')}</span><span style="color: var(--text-secondary);">{contextUsage.count}</span></div>
              <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.totalInput')}</span><span style="color: var(--text-secondary);">{formatK(contextUsage.tokensIn)} tokens</span></div>
              <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.totalOutput')}</span><span style="color: var(--text-secondary);">{formatK(contextUsage.tokensOut)} tokens</span></div>
              {#if contextUsage.cachedTokens > 0}
                <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.totalCached')}</span><span style="color: var(--success);">{formatK(contextUsage.cachedTokens)} tokens</span></div>
              {/if}
            </div>
          {/if}
          <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.usageRatio')}</span><span class="font-semibold" style="color: {contextUsage.pct > 80 ? 'var(--error)' : contextUsage.pct > 50 ? 'var(--warning)' : 'var(--info)'};">{contextUsage.pct}%</span></div>
          <div class="flex justify-between"><span style="color: var(--text-muted);">{$t('ai.context.messageCount')}</span><span style="color: var(--text-primary);">{$messages.length}</span></div>
          <div class="border-t pt-1.5 mt-1.5" style="border-color: var(--border);">
            <div class="w-full rounded-full h-1.5" style="background-color: var(--border);"><div class="h-full rounded-full transition-all" style="width: {Math.min(contextUsage.pct, 100)}%; background-color: {contextUsage.pct > 80 ? 'var(--error)' : contextUsage.pct > 50 ? 'var(--warning)' : 'var(--info)'};"></div></div>
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
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      </button>
      {#if showHistoryPanel}
        <div class="history-dropdown fixed z-[9999] rounded shadow-lg overflow-hidden" style="top: {dropdownPos.top}px; right: {dropdownPos.right}px; background-color: var(--bg-secondary); border: 1px solid var(--border); width: 320px; max-height: 480px;">
          <div class="flex items-center justify-between px-3 py-2 border-b" style="border-color: var(--border);">
            <span class="text-xs font-medium" style="color: var(--text-primary);">{$t('history.conversations')}</span>
            <button class="p-0.5 rounded hover:bg-[var(--bg-hover)]" style="color: var(--text-secondary);" onclick={() => showHistoryPanel = false}>
              <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>
          <!-- Search -->
          <div class="px-3 py-2 border-b" style="border-color: var(--border);">
            <input
              type="text"
              bind:value={searchQuery}
              placeholder={$t('history.searchPlaceholder')}
              class="w-full px-2 py-1.5 rounded text-xs"
              style="background-color: var(--bg-primary); color: var(--text-primary); border: 1px solid var(--border);"
            />
          </div>
          <div class="overflow-y-auto" style="max-height: 400px;">
            {#if filteredConversations.length === 0}
              <div class="px-3 py-4 text-xs text-center" style="color: var(--text-muted);">
                {searchQuery ? $t('history.noMatch') : $t('history.empty')}
              </div>
            {:else}
              {#each Object.entries(groupedConversations) as [label, convs]}
                <div class="px-3 py-1.5 text-[10px] font-semibold uppercase tracking-wider" style="color: var(--text-muted);">{label}</div>
                {#each convs as conv}
                  {@const isActive = conv.id === $activeConversationId}
                  {@const isExpanded = expandedConvId === conv.id}
                  <div class="conv-item border-b" style="border-color: var(--border); {isActive ? 'background-color: var(--bg-hover);' : ''}">
                    <div class="w-full flex items-center gap-1 px-3 py-2 text-xs text-left group">
                      {#if isActive}
                        <div class="w-1.5 h-1.5 rounded-full shrink-0" style="background-color: var(--accent);"></div>
                      {/if}
                      <button
                        class="shrink-0 p-0.5 rounded transition-colors"
                        style="color: {isExpanded ? 'var(--accent)' : 'var(--text-muted)'};"
                        onclick={(e) => { e.stopPropagation(); expandedConvId = isExpanded ? null : conv.id }}
                      >
                        <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>
                      </button>
                      {#if renamingId === conv.id}
                        <input
                          type="text"
                          bind:value={renameValue}
                          class="flex-1 px-1 py-0.5 rounded text-xs min-w-0"
                          style="background-color: var(--bg-primary); color: var(--text-primary); border: 1px solid var(--accent);"
                          onkeydown={(e) => { if (e.key === 'Enter') confirmRename(conv); if (e.key === 'Escape') cancelRename() }}
                          onblur={() => confirmRename(conv)}
                          autofocus
                        />
                      {:else}
                        <button
                          class="flex-1 flex flex-col gap-0.5 text-left min-w-0"
                          onclick={async () => {
                            await loadMessages(conv.id)
                            const msgs = get(activeMessages)
                            if (msgs && msgs.length > 0) {
                              messages.set(msgs.map(m => ({
                                role: m.role, content: m.content, timestamp: new Date(m.createdAt).getTime()
                              })))
                              activeConversationId.set(conv.id)
                            }
                            showHistoryPanel = false
                          }}
                        >
                          <span class="truncate font-medium" style="color: {isActive ? 'var(--accent)' : 'var(--text-primary)'};">{conv.title || $t('history.untitled')}</span>
                          <span class="truncate" style="color: var(--text-muted);">{conv.model || ''} · {conv.messageCount || 0} msgs</span>
                        </button>
                      {/if}
                      <div class="flex items-center gap-0.5 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                        <button
                          class="p-0.5 rounded hover:bg-[var(--bg-hover)]"
                          style="color: var(--text-muted);"
                          title={$t('history.rename')}
                          onclick={(e) => { e.stopPropagation(); startRename(conv) }}
                        >
                          <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" /></svg>
                        </button>
                        <button
                          class="p-0.5 rounded hover:bg-red-500/20"
                          style="color: var(--error);"
                          title={$t('history.delete')}
                          onclick={async (e) => {
                            e.stopPropagation()
                            await deleteConversation(conv.id)
                            await loadConversations($currentProject)
                          }}
                        >
                          <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                        </button>
                      </div>
                    </div>
                    {#if isExpanded}
                      <div class="px-3 pb-2 space-y-1 text-[11px]" style="color: var(--text-muted);">
                        <div class="flex justify-between"><span>{$t('history.messages')}</span><span style="color: var(--text-secondary);">{conv.messageCount || 0}</span></div>
                        <div class="flex justify-between"><span>{$t('history.model')}</span><span style="color: var(--text-secondary);">{conv.model || '-'}</span></div>
                        <div class="flex justify-between"><span>{$t('history.agent')}</span><span style="color: var(--text-secondary);">{conv.agentId || '-'}</span></div>
                        <div class="flex justify-between"><span>{$t('history.created')}</span><span style="color: var(--text-secondary);">{conv.createdAt ? new Date(conv.createdAt).toLocaleString() : '-'}</span></div>
                        <div class="flex justify-between"><span>{$t('history.updated')}</span><span style="color: var(--text-secondary);">{conv.updatedAt ? new Date(conv.updatedAt).toLocaleString() : '-'}</span></div>
                      </div>
                    {/if}
                  </div>
                {/each}
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
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
      </svg>
    </button>

    <button
      class="p-1 rounded transition-colors shrink-0"
      style="color: var(--text-secondary);"
      onclick={() => openTraceViewer()}
      title="Trace Viewer"
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/>
      </svg>
    </button>

    <button
      class="p-1 rounded transition-colors shrink-0"
      style="color: var(--text-secondary);"
      onclick={() => aiPanelVisible.set(false)}
      title={$t('common.close')}
    >
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </button>
  </div>
</div>

<style>
.ai-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 4px 8px; border-bottom: 1px solid; min-width: 0; overflow: hidden; gap: 4px;
}
.ai-header-left { display: flex; align-items: center; gap: 4px; min-width: 0; overflow: hidden; flex-shrink: 1; }
.ai-header-right { display: flex; align-items: center; gap: 2px; flex-shrink: 0; }
.conv-item { transition: background-color 0.1s; }
.conv-item:hover { background-color: var(--bg-hover); }
</style>
