<script>
    import { get } from 'svelte/store'
    import { clearMessages } from '../stores/ai.js'
    import { loadConversations, conversations, loadMessages, activeMessages } from '../stores/memory.js'
  import { messages } from '../stores/ai.js'
    import { currentProject } from '../stores/app.js'
  import { aiPanelVisible } from '../stores/ui.js'
 import { t } from '../stores/i18n.js'

    let showHistoryPanel = false

  function closeDropdowns(e) {
    if (!/** @type {HTMLElement} */ (e.target).closest('.dropdown-trigger')) {
      showHistoryPanel = false
    }
  }

  async function toggleHistory() {
    showHistoryPanel = !showHistoryPanel
    if (showHistoryPanel && $currentProject) {
      await loadConversations($currentProject)
    }
  }
</script>

<svelte:window onclick={closeDropdowns} />

<div class="ai-header" style="border-color: var(--border); background-color: var(--bg-secondary);">
  <div class="ai-header-left">
    <span class="text-xs font-medium" style="color: var(--text-secondary);">AI Chat</span>
  </div>
  <div class="ai-header-right">
    <div class="relative">
      <button
        class="dropdown-trigger p-1 rounded transition-colors flex-shrink-0"
        style="color: var(--text-secondary);"
        onclick={(e) => { e.stopPropagation(); toggleHistory() }}
        title={$t('history.title')}
        aria-label={$t('history.title')}
      >
        <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      </button>
      {#if showHistoryPanel}
        <div class="absolute top-full right-0 mt-1 z-50 rounded shadow-lg overflow-hidden" style="background-color: var(--bg-secondary); border: 1px solid var(--border); width: 280px; max-height: 360px;">
        <div class="flex items-center justify-between px-3 py-2 border-b" style="border-color: var(--border);">
          <span class="text-xs font-medium" style="color: var(--text-primary);">{$t('history.title')}</span>
          <button class="p-0.5 rounded hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => showHistoryPanel = false}>
            <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>
        </div>
        <div class="overflow-y-auto" style="max-height: 320px;">
          {#if $conversations.length === 0}
            <div class="px-3 py-4 text-xs text-center" style="color: var(--text-muted);">{$t('history.empty')}</div>
          {:else}
            {#each $conversations as conv}
              <button
                class="w-full flex flex-col gap-0.5 px-3 py-2 text-xs transition-colors text-left border-b" style="border-color: var(--border); color: var(--text-primary);"
                onclick={async () => {
                  await loadMessages(conv.id)
                  // Display loaded messages in chat
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
            {/each}
          {/if}
        </div>
      </div>
    {/if}
    </div>

    
    <button
      class="p-1 rounded transition-colors flex-shrink-0"
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
      class="p-1 rounded transition-colors flex-shrink-0"
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
