<script>
import { onMount } from 'svelte'
import { mcpServers, mcpServerStatuses, loadMCPServers, addMCPServer, removeMCPServer, startMCPServer, stopMCPServer, enableMCPServer } from '../stores/mcp.js'
import { t } from '../stores/i18n.js'

let showForm = false
let editMode = false
let editId = ''
let newId = ''
let newName = ''
let newCommand = ''
let newArgs = ''
let newTransport = 'stdio'
let newEnv = ''
let newEndpoint = ''

onMount(() => { loadMCPServers() })

function resetForm() {
  editMode = false; editId = ''
  newId = ''; newName = ''; newCommand = ''; newArgs = ''
  newTransport = 'stdio'; newEnv = ''; newEndpoint = ''
  showForm = false
}

function editServer(server) {
  editMode = true; editId = server.id
  newId = server.id; newName = server.name
  newCommand = server.command || ''
  newArgs = (server.args || []).join(' ')
  newTransport = server.transport || 'stdio'
  newEndpoint = server.endpoint || ''
  const env = server.env || {}
  newEnv = Object.entries(env).map(([k,v]) => `${k}=${v}`).join('\n')
  showForm = true
}

async function handleAdd() {
  if (!newId || !newName) return
  const env = {}
  if (newEnv) {
    newEnv.split('\n').forEach(line => {
      const [k, ...v] = line.split('=')
      if (k && v.length) env[k.trim()] = v.join('=').trim()
    })
  }
  // Delete old server if editing
  if (editMode && editId !== newId) {
    try { await removeMCPServer(editId) } catch(e) {}
  }
  await addMCPServer({
    id: newId,
    name: newName,
    command: newCommand,
    args: newArgs ? newArgs.split(/\s+/) : [],
    endpoint: newEndpoint,
    transport: newTransport,
    enabled: true,
    env,
  })
  resetForm()
  loadMCPServers()
}
</script>

<div class="space-y-3">
  <div>
    <p class="text-xs" style="color: var(--text-secondary);">{$t('settings.mcpDescription')}</p>
  </div>
  <div class="flex items-center justify-between">
    <h3 class="text-sm font-medium" style="color: var(--text-primary);">服务器列表</h3>
    <button class="btn btn-primary btn-sm" onclick={() => { resetForm(); showForm = true }}>
      {showForm ? $t('settings.mcpClose') : '+ ' + $t('settings.mcpAdd')}
    </button>
  </div>

  {#if showForm}
    <div class="p-3 rounded space-y-2" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
      <div class="text-xs font-medium mb-1" style="color: var(--accent);">{editMode ? '编辑 ' + editId : '添加新服务器'}</div>
      <input bind:value={newId} placeholder={$t('settings.mcpId')} class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" disabled={editMode}>
      <input bind:value={newName} placeholder={$t('settings.mcpName')} class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);">
      <input bind:value={newCommand} placeholder={$t('settings.mcpCommand')} class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);">
      <input bind:value={newArgs} placeholder={$t('settings.mcpArgs')} class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);">
      <select bind:value={newTransport} class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);">
        <option value="stdio">stdio</option>
        <option value="sse">SSE</option>
      </select>
      {#if newTransport === 'sse'}
        <input bind:value={newEndpoint} placeholder={$t('settings.mcpSSE')} class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);">
      {/if}
      <textarea bind:value={newEnv} placeholder={$t('settings.mcpEnv')} class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border); min-height: 60px;" rows="3"></textarea>
      <div class="flex gap-2">
        <button class="flex-1 btn btn-secondary" onclick={resetForm}>{$t('settings.mcpCancel')}</button>
        <button class="flex-1 btn btn-primary" onclick={handleAdd}>{editMode ? $t('settings.mcpSave') : $t('settings.mcpAdd')}</button>
      </div>
    </div>
  {/if}

  {#each $mcpServers as server}
    {@const status = $mcpServerStatuses[server.id] || (server.enabled ? 'stopped' : 'disabled')}
    {@const errMsg = $mcpServerStatuses[server.id + '_err'] || ''}
    <div class="p-2 rounded space-y-1" style="background-color: var(--bg-primary); border: 1px solid var(--border);">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="inline-block w-2 h-2 rounded-full shrink-0" style="background-color: {status === 'running' ? '#2ea043' : status === 'error' ? '#d73a49' : '#666'};"></span>
          <div>
            <div class="text-sm" style="color: var(--text-primary);">{server.name}</div>
            <div class="text-xs" style="color: var(--text-muted);">{server.id} · {server.transport} · <span style="color: {status === 'running' ? '#2ea043' : 'var(--text-muted)'};">{status === 'running' ? '运行中' : status === 'disabled' ? '已禁用' : '已停止'}</span></div>
          </div>
        </div>
        <div class="flex items-center gap-1">
          {#if status === 'running'}
            <button class="btn btn-sm" style="color: #e5c07b;" onclick={() => stopMCPServer(server.id)}>停止</button>
          {:else if status === 'disabled' || !server.enabled}
            <button class="btn btn-secondary btn-sm" onclick={() => enableMCPServer(server.id)}>启用</button>
          {:else}
            <button class="btn btn-secondary btn-sm" onclick={() => startMCPServer(server.id)}>启动</button>
          {/if}
          <button class="btn btn-accent-ghost btn-sm" onclick={() => editServer(server)}>{$t('settings.mcpEdit')}</button>
          <button class="btn btn-danger-ghost btn-sm" onclick={() => { if (window.confirm($t('settings.mcpConfirmDelete'))) removeMCPServer(server.id) }}>{$t('settings.mcpDelete')}</button>
        </div>
      </div>
      {#if errMsg}
        <div class="text-xs px-2 py-1 rounded" style="color: #d73a49; background-color: #d73a4910;">错误: {errMsg}</div>
      {/if}
    </div>
  {/each}
</div>
