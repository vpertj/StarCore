<script>
  import { onMount } from 'svelte'
  import { t } from '../stores/i18n.js'
  import { mcpServers, mcpServerStatuses, loadMCPServers, startMCPServer, stopMCPServer } from '../stores/mcp.js'
  import { registerCustomLanguage, unregisterCustomLanguage } from '../stores/languages.js'

  let loading = $state(true)
  let lspServers = $state([])
  let languagePackages = $state([])
  let installedChecks = $state({})
  let installing = $state({})
  let showAddForm = $state(false)
  let showMarketplace = $state(true)
  let newLangId = $state('')
  let newCommand = $state('')
  let newArgs = $state('')
  let newExtensions = $state('')
  let searchQuery = $state('')

  onMount(async () => {
    await loadMCPServers()
    await loadLSPServers()
    await loadLanguagePackages()
    loading = false
  })

  async function loadLSPServers() {
    try {
      if (window.backend?.GetLSPServers) {
        lspServers = await window.backend.GetLSPServers()
        for (const server of lspServers) {
          if (server.custom && server.extensions?.length) {
            registerCustomLanguage(server.extensions, server.languageId)
          }
        }
      }
    } catch (e) {
      console.error('Failed to load LSP servers:', e)
    }
  }

  async function loadLanguagePackages() {
    try {
      if (window.backend?.GetLanguagePackages) {
        languagePackages = await window.backend.GetLanguagePackages()
        for (const pkg of languagePackages) {
          checkInstalled(pkg)
        }
      }
    } catch (e) {
      console.error('Failed to load language packages:', e)
    }
  }

  async function checkInstalled(pkg) {
    try {
      if (window.backend?.CheckCommandExists) {
        const exists = await window.backend.CheckCommandExists(pkg.command)
        installedChecks[pkg.id] = exists
        installedChecks = { ...installedChecks }
      }
    } catch {
      installedChecks[pkg.id] = false
      installedChecks = { ...installedChecks }
    }
  }

  function isLSPRegistered(languageId) {
    return lspServers.some(s => s.languageId === languageId)
  }

  async function installAndEnable(pkg) {
    installing[pkg.id] = 'installing'
    installing = { ...installing }
    try {
      if (pkg.installCmd) {
        const output = await window.backend.InstallLanguagePackage(pkg.id)
        console.log('Install output:', output)
      }
      installedChecks[pkg.id] = true
      installedChecks = { ...installedChecks }
      await enablePackage(pkg)
      installing[pkg.id] = 'done'
    } catch (e) {
      console.error('Install failed:', e)
      installing[pkg.id] = 'error'
    }
    installing = { ...installing }
  }

  async function enablePackage(pkg) {
    try {
      await window.backend.AddLSPServer(pkg.languageId, pkg.command, pkg.args, pkg.extensions)
      if (pkg.extensions?.length) {
        registerCustomLanguage(pkg.extensions, pkg.languageId)
      }
      await loadLSPServers()
    } catch (e) {
      console.error('Failed to enable language package:', e)
    }
  }

  async function disablePackage(pkg) {
    try {
      await window.backend.RemoveLSPServer(pkg.languageId)
      if (pkg.extensions?.length) {
        unregisterCustomLanguage(pkg.extensions)
      }
      await loadLSPServers()
    } catch (e) {
      console.error('Failed to disable language package:', e)
    }
  }

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

  async function addLSPServer() {
    if (!newLangId.trim() || !newCommand.trim()) return
    const args = newArgs.trim() ? newArgs.trim().split(/\s+/) : []
    const exts = newExtensions.trim() ? newExtensions.trim().split(/[,;\s]+/).map(e => e.startsWith('.') ? e : '.' + e) : []
    try {
      await window.backend.AddLSPServer(newLangId.trim(), newCommand.trim(), args, exts)
      if (exts.length) {
        registerCustomLanguage(exts, newLangId.trim())
      }
      showAddForm = false
      newLangId = ''
      newCommand = ''
      newArgs = ''
      newExtensions = ''
      await loadLSPServers()
    } catch (e) {
      console.error('Failed to add LSP server:', e)
    }
  }

  async function removeLSPServer(langId) {
    const server = lspServers.find(s => s.languageId === langId)
    try {
      await window.backend.RemoveLSPServer(langId)
      if (server?.extensions?.length) {
        unregisterCustomLanguage(server.extensions)
      }
      await loadLSPServers()
    } catch (e) {
      console.error('Failed to remove LSP server:', e)
    }
  }

  let filteredPackages = $derived(
    searchQuery.trim()
      ? languagePackages.filter(p =>
          p.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          p.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
          p.languageId.toLowerCase().includes(searchQuery.toLowerCase())
        )
      : languagePackages
  )

  let languageGroup = $derived(filteredPackages.filter(p => p.category === 'language'))
  let frameworkGroup = $derived(filteredPackages.filter(p => p.category === 'framework'))
  let toolGroup = $derived(filteredPackages.filter(p => p.category === 'tool'))
</script>

<div class="h-full flex flex-col" style="background-color: var(--bg-secondary);">
  <div class="px-4 py-2 text-xs font-semibold tracking-wider border-b" style="color: var(--text-secondary); border-color: var(--border);">
    EXTENSIONS
  </div>

  <div class="flex-1 overflow-y-auto p-3">
    {#if loading}
      <div class="text-center py-8 text-xs" style="color: var(--text-muted);">Loading...</div>
    {:else}
      <!-- Language Support Marketplace -->
      <div class="mb-4">
        <div class="flex items-center justify-between mb-2">
          <h3 class="text-xs font-medium" style="color: var(--text-secondary);">{$t('extensions.languageSupport')}</h3>
          <div class="flex gap-1">
            <button
              class="px-2 py-0.5 rounded text-xs transition-colors"
              style="background-color: {showMarketplace ? 'var(--accent)' : 'transparent'}; color: {showMarketplace ? '#ffffff' : 'var(--text-muted)'};"
              onclick={() => { showMarketplace = true; showAddForm = false }}
            >
              {$t('extensions.marketplace')}
            </button>
            <button
              class="px-2 py-0.5 rounded text-xs transition-colors"
              style="background-color: {!showMarketplace ? 'var(--accent)' : 'transparent'}; color: {!showMarketplace ? '#ffffff' : 'var(--text-muted)'};"
              onclick={() => { showMarketplace = false; showAddForm = true }}
            >
              {$t('extensions.custom')}
            </button>
          </div>
        </div>

        {#if showMarketplace}
          <!-- Search -->
          <div class="mb-3">
            <input
              type="text"
              bind:value={searchQuery}
              class="input-field input-field-sm w-full"
              placeholder={$t('extensions.searchPlaceholder')}
            />
          </div>

          <!-- Language group -->
          {#if languageGroup.length > 0}
            <div class="mb-3">
              <p class="text-xs font-medium mb-1.5" style="color: var(--text-muted);">{$t('extensions.category.language')}</p>
              {#each languageGroup as pkg}
                {@const isInstalled = installedChecks[pkg.id]}
                {@const isEnabled = isLSPRegistered(pkg.languageId)}
                {@const status = installing[pkg.id]}
                <div class="flex items-center gap-2 px-3 py-2 rounded mb-1" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
                  <div class="w-2 h-2 rounded-full shrink-0" style="background-color: {isEnabled ? '#2ea043' : isInstalled ? '#e5c07b' : 'var(--text-muted)'};"></div>
                  <div class="flex-1 min-w-0">
                    <div class="text-sm" style="color: var(--text-primary);">{pkg.name}</div>
                    <div class="text-xs truncate" style="color: var(--text-muted);">{pkg.description} {pkg.hasHighlight ? '✓ ' + $t('extensions.syntaxHighlight') : ''}</div>
                  </div>
                  {#if status === 'installing'}
                    <span class="text-xs px-2 py-0.5" style="color: var(--accent);">{$t('extensions.installing')}</span>
                  {:else if status === 'error'}
                    <span class="text-xs px-2 py-0.5" style="color: #d73a49;">{$t('extensions.installFailed')}</span>
                  {:else if isEnabled}
                    <button
                      class="px-2 py-0.5 rounded text-xs transition-colors"
                      style="background-color: #d73a4933; color: #d73a49;"
                      onclick={() => disablePackage(pkg)}
                    >
                      {$t('extensions.disable')}
                    </button>
                  {:else if isInstalled}
                    <button
                      class="px-2 py-0.5 rounded text-xs transition-colors"
                      style="background-color: #2ea04333; color: #2ea043;"
                      onclick={() => enablePackage(pkg)}
                    >
                      {$t('extensions.enable')}
                    </button>
                  {:else if pkg.downloadUrl || pkg.installCmd}
                    <button
                      class="px-2 py-0.5 rounded text-xs transition-colors"
                      style="background-color: var(--accent); color: #ffffff;"
                      onclick={() => installAndEnable(pkg)}
                    >
                      {pkg.downloadUrl ? $t('extensions.install') : $t('extensions.installNpm')}
                    </button>
                  {:else}
                    <span class="text-xs px-2 py-0.5" style="color: var(--text-muted);">{$t('extensions.manualInstall')}</span>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}

          <!-- Framework group -->
          {#if frameworkGroup.length > 0}
            <div class="mb-3">
              <p class="text-xs font-medium mb-1.5" style="color: var(--text-muted);">{$t('extensions.category.framework')}</p>
              {#each frameworkGroup as pkg}
                {@const isInstalled = installedChecks[pkg.id]}
                {@const isEnabled = isLSPRegistered(pkg.languageId)}
                {@const status = installing[pkg.id]}
                <div class="flex items-center gap-2 px-3 py-2 rounded mb-1" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
                  <div class="w-2 h-2 rounded-full shrink-0" style="background-color: {isEnabled ? '#2ea043' : isInstalled ? '#e5c07b' : 'var(--text-muted)'};"></div>
                  <div class="flex-1 min-w-0">
                    <div class="text-sm" style="color: var(--text-primary);">{pkg.name}</div>
                    <div class="text-xs truncate" style="color: var(--text-muted);">{pkg.description} {pkg.hasHighlight ? '✓ ' + $t('extensions.syntaxHighlight') : ''}</div>
                  </div>
                  {#if status === 'installing'}
                    <span class="text-xs px-2 py-0.5" style="color: var(--accent);">{$t('extensions.installing')}</span>
                  {:else if isEnabled}
                    <button
                      class="px-2 py-0.5 rounded text-xs transition-colors"
                      style="background-color: #d73a4933; color: #d73a49;"
                      onclick={() => disablePackage(pkg)}
                    >
                      {$t('extensions.disable')}
                    </button>
                  {:else if isInstalled}
                    <button
                      class="px-2 py-0.5 rounded text-xs transition-colors"
                      style="background-color: #2ea04333; color: #2ea043;"
                      onclick={() => enablePackage(pkg)}
                    >
                      {$t('extensions.enable')}
                    </button>
                  {:else if pkg.downloadUrl || pkg.installCmd}
                    <button
                      class="px-2 py-0.5 rounded text-xs transition-colors"
                      style="background-color: var(--accent); color: #ffffff;"
                      onclick={() => installAndEnable(pkg)}
                    >
                      {pkg.downloadUrl ? $t('extensions.install') : $t('extensions.installNpm')}
                    </button>
                  {:else}
                    <span class="text-xs px-2 py-0.5" style="color: var(--text-muted);">{$t('extensions.manualInstall')}</span>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}

          <!-- Tool group -->
          {#if toolGroup.length > 0}
            <div class="mb-3">
              <p class="text-xs font-medium mb-1.5" style="color: var(--text-muted);">{$t('extensions.category.tool')}</p>
              {#each toolGroup as pkg}
                {@const isInstalled = installedChecks[pkg.id]}
                {@const isEnabled = isLSPRegistered(pkg.languageId)}
                {@const status = installing[pkg.id]}
                <div class="flex items-center gap-2 px-3 py-2 rounded mb-1" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
                  <div class="w-2 h-2 rounded-full shrink-0" style="background-color: {isEnabled ? '#2ea043' : isInstalled ? '#e5c07b' : 'var(--text-muted)'};"></div>
                  <div class="flex-1 min-w-0">
                    <div class="text-sm" style="color: var(--text-primary);">{pkg.name}</div>
                    <div class="text-xs truncate" style="color: var(--text-muted);">{pkg.description} {pkg.hasHighlight ? '✓ ' + $t('extensions.syntaxHighlight') : ''}</div>
                  </div>
                  {#if status === 'installing'}
                    <span class="text-xs px-2 py-0.5" style="color: var(--accent);">{$t('extensions.installing')}</span>
                  {:else if isEnabled}
                    <button
                      class="px-2 py-0.5 rounded text-xs transition-colors"
                      style="background-color: #d73a4933; color: #d73a49;"
                      onclick={() => disablePackage(pkg)}
                    >
                      {$t('extensions.disable')}
                    </button>
                  {:else if isInstalled}
                    <button
                      class="px-2 py-0.5 rounded text-xs transition-colors"
                      style="background-color: #2ea04333; color: #2ea043;"
                      onclick={() => enablePackage(pkg)}
                    >
                      {$t('extensions.enable')}
                    </button>
                  {:else if pkg.downloadUrl || pkg.installCmd}
                    <button
                      class="px-2 py-0.5 rounded text-xs transition-colors"
                      style="background-color: var(--accent); color: #ffffff;"
                      onclick={() => installAndEnable(pkg)}
                    >
                      {pkg.downloadUrl ? $t('extensions.install') : $t('extensions.installNpm')}
                    </button>
                  {:else}
                    <span class="text-xs px-2 py-0.5" style="color: var(--text-muted);">{$t('extensions.manualInstall')}</span>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}

          {#if filteredPackages.length === 0}
            <p class="text-xs text-center py-4" style="color: var(--text-muted);">{$t('extensions.noResults')}</p>
          {/if}
        {:else}
          <!-- Custom LSP form -->
          {#if showAddForm}
            <div class="px-3 py-2 rounded mb-2" style="background-color: var(--bg-primary); border: 1px solid var(--accent);">
              <div class="mb-2">
                <label class="text-xs block mb-1" style="color: var(--text-secondary);">{$t('extensions.languageSupport.langId')}</label>
                <input type="text" bind:value={newLangId} class="input-field input-field-sm w-full" placeholder="rust" />
              </div>
              <div class="mb-2">
                <label class="text-xs block mb-1" style="color: var(--text-secondary);">{$t('extensions.languageSupport.command')}</label>
                <input type="text" bind:value={newCommand} class="input-field input-field-sm w-full" placeholder="rust-analyzer" />
              </div>
              <div class="mb-2">
                <label class="text-xs block mb-1" style="color: var(--text-secondary);">{$t('extensions.languageSupport.args')}</label>
                <input type="text" bind:value={newArgs} class="input-field input-field-sm w-full" placeholder="--stdio" />
              </div>
              <div class="mb-2">
                <label class="text-xs block mb-1" style="color: var(--text-secondary);">{$t('extensions.languageSupport.extensions')}</label>
                <input type="text" bind:value={newExtensions} class="input-field input-field-sm w-full" placeholder=".rs" />
              </div>
              <div class="flex gap-2">
                <button class="btn btn-primary btn-sm flex-1" onclick={addLSPServer}>{$t('extensions.languageSupport.add')}</button>
                <button class="btn btn-secondary btn-sm flex-1" onclick={() => showAddForm = false}>{$t('settings.cancel')}</button>
              </div>
            </div>
          {/if}

          <!-- Custom LSP servers list -->
          {#each lspServers.filter(s => s.custom) as server}
            <div class="flex items-center gap-3 px-3 py-2 rounded mb-1" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
              <div class="w-2 h-2 rounded-full" style="background-color: {server.running ? '#2ea043' : 'var(--text-muted)'};"></div>
              <div class="flex-1 min-w-0">
                <div class="text-sm truncate" style="color: var(--text-primary);">{server.languageId}</div>
                <div class="text-xs truncate" style="color: var(--text-muted);">{server.command} {server.args?.join(' ')}</div>
              </div>
              <button
                class="px-1.5 py-0.5 rounded text-xs transition-colors"
                style="color: #d73a49;"
                onclick={() => removeLSPServer(server.languageId)}
              >
                &times;
              </button>
            </div>
          {/each}
        {/if}
      </div>

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
                onclick={() => toggleServer(server.id)}
              >
                {server.enabled ? 'Stop' : 'Start'}
              </button>
            </div>
          {/each}
        {/if}
      </div>

      <!-- Built-in -->
      <div>
        <h3 class="text-xs font-medium mb-2" style="color: var(--text-secondary);">Built-in</h3>
        {#each lspServers.filter(s => !s.custom) as server}
          <div class="flex items-center gap-3 px-3 py-2 rounded mb-1" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
            <div class="w-2 h-2 rounded-full" style="background-color: #2ea043;"></div>
            <div class="flex-1 min-w-0">
              <div class="text-sm truncate" style="color: var(--text-primary);">{server.languageId}</div>
              <div class="text-xs truncate" style="color: var(--text-muted);">{server.command}</div>
            </div>
            <span class="text-xs" style="color: #4ec9b0;">{$t('extensions.languageSupport.builtin')}</span>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</div>
