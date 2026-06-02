<script>
  import { onMount } from 'svelte'
  import { currentProject } from '../stores/app.js'
  import { t } from '../stores/i18n.js'
  import { mcpServers, mcpServerStatuses, loadMCPServers, startMCPServer, stopMCPServer } from '../stores/mcp.js'

  let loading = $state(true)

  onMount(async () => {
    await loadMCPServers()
    loading = false
  })

  async function toggleServer(id) {
    const servers = $mcpServers
    const server = servers.find(s => s.id === id)
    if (!server) return
    const status = $mcpServerStatuses[id]
    if (status === 'running') {
      await stopMCPServer(id)
    } else {
      await startMCPServer(id)
    }
  }
</script>

<div class="h-full flex flex-col" style="background-color: var(--bg-secondary);">
  <div class="px-4 py-2 text-xs font-semibold tracking-wider border-b" style="color: var(--text-secondary); border-color: var(--border);">
    EXTENSIONS
  </div>

  <div class="flex-1 overflow-y-auto p-3">
    {#if loading}
      <div class="text-center py-8 text-xs" style="color: var(--text-muted);">Loading...</div>
    {:else}
      <!-- MCP Servers -->
      <div class="mb-4">
        <h3 class="text-xs font-medium mb-2" style="color: var(--text-secondary);">MCP Servers</h3>
        {#if mcpServers.length === 0}
          <p class="text-xs" style="color: var(--text-muted);">No MCP servers configured.</p>
        {:else}
          {#each $mcpServers as server}
            <div class="flex items-center gap-3 px-3 py-2 rounded mb-1" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
              <div class="w-2 h-2 rounded-full" style="background-color: {server.enabled ? '#2ea043' : 'var(--text-muted)'};"></div>
              <div class="flex-1 min-w-0">
                <div class="text-sm truncate" style="color: var(--text-primary);">{server.name || server.id}</div>
                <div class="text-xs truncate" style="color: var(--text-muted);">{server.command || 'MCP'}</div>
              </div>
              <button
                class="px-2 py-0.5 rounded text-xs transition-colors"
                style="background-color: {server.enabled ? '#d73a4933' : '#2ea04333'}; color: {server.enabled ? '#d73a49' : '#2ea043'};"
                on:click={() => toggleServer(server.id)}
              >
                {server.enabled ? 'Stop' : 'Start'}
              </button>
            </div>
          {/each}
        {/if}
      </div>

      <!-- Built-in Plugins -->
      <div>
        <h3 class="text-xs font-medium mb-2" style="color: var(--text-secondary);">Built-in</h3>
        <div class="flex items-center gap-3 px-3 py-2 rounded mb-1" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
          <svg viewBox="0 0 16 16" class="w-4 h-4 flex-shrink-0" fill="none" stroke="#4ec9b0">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M2 2h4v4H2zM10 2h4v4h-4zM2 10h4v4H2zM10 10h4v4h-4z"/>
          </svg>
          <div class="flex-1 min-w-0">
            <div class="text-sm" style="color: var(--text-primary);">CodeMirror 6</div>
            <div class="text-xs" style="color: var(--text-muted);">Code editor with multi-language support</div>
          </div>
          <span class="text-xs" style="color: #2ea043;">active</span>
        </div>
        <div class="flex items-center gap-3 px-3 py-2 rounded mb-1" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
          <svg viewBox="0 0 16 16" class="w-4 h-4 flex-shrink-0" fill="none" stroke="#e5c07b">
            <path stroke-linecap="round" stroke-width="1.5" d="M2 3h12M2 8h8M2 13h12"/>
          </svg>
          <div class="flex-1 min-w-0">
            <div class="text-sm" style="color: var(--text-primary);">Xterm Terminal</div>
            <div class="text-xs" style="color: var(--text-muted);">Integrated terminal with multi-tab support</div>
          </div>
          <span class="text-xs" style="color: #2ea043;">active</span>
        </div>
        <div class="flex items-center gap-3 px-3 py-2 rounded mb-1" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
          <svg viewBox="0 0 16 16" class="w-4 h-4 flex-shrink-0" fill="none" stroke="#4fc1ff">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/>
          </svg>
          <div class="flex-1 min-w-0">
            <div class="text-sm" style="color: var(--text-primary);">AI Chat</div>
            <div class="text-xs" style="color: var(--text-muted);">Multi-provider AI assistant with tool calling</div>
          </div>
          <span class="text-xs" style="color: #2ea043;">active</span>
        </div>
      </div>
    {/if}
  </div>
</div>
