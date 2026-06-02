import { writable, get } from 'svelte/store'

/** @typedef {{ id: string, name: string, command: string, args: string[], endpoint: string, transport: string, env: Record<string, string>, enabled: boolean }} MCPServerConfig */

/** @type {import('svelte/store').Writable<MCPServerConfig[]>} */
export const mcpServers = writable([])
/** @type {import('svelte/store').Writable<Record<string, string>>} */
export const mcpServerStatuses = writable({})
/** @type {import('svelte/store').Writable<any[]>} */
export const mcpTools = writable([])

export async function loadMCPServers() {
  if (!window.backend?.GetMCPServers) return
  try {
    const list = await window.backend.GetMCPServers()
    mcpServers.set(list || [])
  } catch (/** @type {any} */ e) {
    console.error('Failed to load MCP servers:', e)
  }
}

/** @param {MCPServerConfig} config */
export async function addMCPServer(config) {
  if (!window.backend?.AddMCPServer) return
  try {
    await window.backend.AddMCPServer(config)
    await loadMCPServers()
  } catch (/** @type {any} */ e) {
    console.error('Failed to add MCP server:', e)
  }
}

/** @param {string} id */
export async function removeMCPServer(id) {
  if (!window.backend?.RemoveMCPServer) return
  try {
    await window.backend.RemoveMCPServer(id)
    await loadMCPServers()
  } catch (/** @type {any} */ e) {
    console.error('Failed to remove MCP server:', e)
  }
}

/** @param {string} id */
export async function startMCPServer(id) {
  if (!window.backend?.StartMCPServer) return
  try {
    await window.backend.StartMCPServer(id)
    mcpServerStatuses.update(s => ({ ...s, [id]: 'running' }))
  } catch (/** @type {any} */ e) {
    let msg = e?.message || String(e)
    // Friendly hints
    if (msg.includes('uvx') && msg.includes('not found')) {
      msg = '需要安装 uvx: pip install uv  或编辑此服务器改用其他命令'
    } else if (msg.includes('npx') && msg.includes('not found')) {
      msg = '需要安装 Node.js (npx 随 Node.js 附带): https://nodejs.org'
    } else if (msg.includes('executable file not found')) {
      msg = msg + ' — 请编辑此服务器，将命令改为本机已安装的程序路径'
    }
    mcpServerStatuses.update(s => ({ ...s, [id]: 'error', [`${id}_err`]: msg }))
    console.error('Failed to start MCP server:', e)
  }
}

export async function enableMCPServer(id) {
  const servers = get(mcpServers)
  const srv = servers.find(s => s.id === id)
  if (!srv || !window.backend?.AddMCPServer) return
  await window.backend.AddMCPServer({ ...srv, enabled: true })
  await loadMCPServers()
}

/** @param {string} id */
export async function stopMCPServer(id) {
  if (!window.backend?.StopMCPServer) return
  try {
    await window.backend.StopMCPServer(id)
    mcpServerStatuses.update(s => ({ ...s, [id]: 'stopped' }))
  } catch (/** @type {any} */ e) {
    console.error('Failed to stop MCP server:', e)
  }
}
