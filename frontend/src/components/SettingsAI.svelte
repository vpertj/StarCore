<script>
  import { providers, setProviderConfig, builtinProviders, providerModels, customModels, saveCustomModels, modelEnabledMap, setModelEnabled } from '../stores/provider.js'
  import { t } from '../stores/i18n.js'
  import { SetProviderConfig, TestProvider } from '../../wailsjs/go/main/App.js'

  let showAddModelDialog = $state(false)
  let addModelProvider = $state('openai')
  let addModelProviderName = $state('')
  let addModelApiKey = $state('')
  let addModelApiKeyVisible = $state(false)
  let addModelEndpoint = $state('')
  let addModelId = $state('')
  let addModelName = $state('')
  let showEditProviderDialog = $state(false)
  let editProviderId = $state('')
  let editApiKey = $state('')
  let editEndpoint = $state('')
  let showEditCustomModelDialog = $state(false)
  let editCustomModelOrigId = $state('')
  let editCustomModelId = $state('')
  let editCustomModelName = $state('')
  let editCustomModelProviderName = $state('')
  let editCustomModelApiKey = $state('')
  let editCustomModelApiKeyVisible = $state(false)
  let editCustomModelEndpoint = $state('')
  let editCustomModelProvider = $state('')
  let editFetchingModels = $state(false)

  let fetchModelError = $state('')
  let fetchingModels = $state(false)
  let fetchedModelList = $state(/** @type {string[]} */ ([]))
  let selectedModelsToAdd = $state(/** @type {string[]} */ ([]))

  let testingModelId = $state(/** @type {string|null} */ (null))
  let testModelResults = $state(/** @type {Record<string, {ok: boolean, msg: string}>} */ ({}))

  let editFetchedModelList = $state(/** @type {string[]} */ ([]))
  let editFetchError = $state('')
  let editSelectedModelsToAdd = $state(/** @type {string[]} */ ([]))

  let editExistingModels = $derived($customModels.filter(m => m.providerId === editProviderId))
  let editExistingModelIds = $derived(new Set(editExistingModels.map(m => m.modelId || m.id.split(':').pop())))
  let editNewModels = $derived(editFetchedModelList.filter(m => !editExistingModelIds.has(m)))

  function get(/** @type {any} */ store) {
    let value
    store.subscribe(/** @type {(v: any) => void} */ (v => value = v))()
    return value
  }

  /** @param {string} providerId */
  function getProviderModelList(providerId) {
    return providerModels[providerId] || []
  }

  function openAddModelDialog() {
    addModelProvider = 'openai'
    addModelProviderName = builtinProviders.find(p => p.id === 'openai')?.name || 'OpenAI'
    addModelApiKey = ''
    addModelApiKeyVisible = false
    addModelEndpoint = builtinProviders.find(p => p.id === 'openai')?.defaultEndpoint || ''
    addModelId = ''
    addModelName = ''
    fetchedModelList = []
    showAddModelDialog = true
  }

  function onAddProviderChange() {
    const p = builtinProviders.find(bp => bp.id === addModelProvider)
    addModelEndpoint = p?.defaultEndpoint || ''
    addModelProviderName = p?.name || addModelProvider
    addModelId = ''
    addModelName = ''
  }

  /** @param {string} modelId */
  function toggleModelSelection(modelId) {
    const idx = selectedModelsToAdd.indexOf(modelId)
    if (idx >= 0) {
      selectedModelsToAdd = selectedModelsToAdd.filter(m => m !== modelId)
    } else {
      selectedModelsToAdd = [...selectedModelsToAdd, modelId]
    }
  }

  function saveAllSelectedModels() {
    if (selectedModelsToAdd.length === 0) { showAddModelDialog = false; return }
    const models = /** @type {Array<any>} */ (get(customModels) || [])
    const providerName = addModelProviderName.trim() || builtinProviders.find(p => p.id === addModelProvider)?.name || addModelProvider
    for (const modelId of selectedModelsToAdd) {
      const compositeId = `${addModelProvider}:${modelId}`
      if (!models.find(/** @param {any} m */ m => m.id === compositeId)) {
        models.push({ id: compositeId, modelId, name: modelId, providerId: addModelProvider, providerName, apiKey: addModelApiKey, endpoint: addModelEndpoint, enabled: true, isCustom: true })
      }
    }
    saveCustomModels(models)
    if (addModelApiKey || addModelEndpoint) {
      setProviderConfig(addModelProvider, { apiKey: addModelApiKey, endpoint: addModelEndpoint })
    }
    selectedModelsToAdd = []
    showAddModelDialog = false
  }

  function addCustomModel() {
    if (!addModelId.trim() || !addModelProvider) return
    const models = /** @type {Array<any>} */ (get(customModels) || [])
    const providerName = addModelProviderName.trim() || builtinProviders.find(p => p.id === addModelProvider)?.name || addModelProvider
    const compositeId = `${addModelProvider}:${addModelId.trim()}`
    models.push({ id: compositeId, modelId: addModelId.trim(), name: addModelName.trim() || addModelId.trim(), providerId: addModelProvider, providerName, apiKey: addModelApiKey, endpoint: addModelEndpoint, enabled: true, isCustom: true })
    saveCustomModels(models)
    if (addModelApiKey || addModelEndpoint) {
      setProviderConfig(addModelProvider, { apiKey: addModelApiKey, endpoint: addModelEndpoint })
    }
    showAddModelDialog = false
  }

  /** @param {string} modelId */
  function deleteCustomModel(modelId) {
    const models = /** @type {Array<any>} */ ((get(customModels) || []).filter(/** @param {any} m */ m => m.id !== modelId))
    saveCustomModels(models)
  }

  /** @param {string} modelId */
  function toggleCustomModel(modelId) {
    const models = /** @type {Array<any>} */ (get(customModels) || [])
    const model = models.find(/** @param {any} m */ m => m.id === modelId)
    if (model) { model.enabled = !model.enabled; saveCustomModels(models) }
  }

  /** @param {string} pid */
  function openEditProvider(pid) {
    editProviderId = pid
    const existing = /** @type {Array<any>} */ (get(customModels) || []).find(m => m.providerId === pid)
    editApiKey = existing?.apiKey || ''
    editEndpoint = existing?.endpoint || builtinProviders.find(p => p.id === pid)?.defaultEndpoint || ''
    editFetchedModelList = []
    editSelectedModelsToAdd = []
    editFetchError = ''
    showEditProviderDialog = true
  }

  /** @param {string} modelId */
  function editToggleModelSelection(modelId) {
    const idx = editSelectedModelsToAdd.indexOf(modelId)
    if (idx >= 0) {
      editSelectedModelsToAdd = editSelectedModelsToAdd.filter(m => m !== modelId)
    } else {
      editSelectedModelsToAdd = [...editSelectedModelsToAdd, modelId]
    }
  }

  async function editFetchModelsFromAPI() {
    if (!editApiKey && !editEndpoint) return
    editFetchingModels = true; editFetchedModelList = []; editSelectedModelsToAdd = []; editFetchError = ''
    try {
      let modelsUrl
      if (editProviderId === 'ollama') {
        modelsUrl = editEndpoint.replace(/\/+$/, '') + '/api/tags'
      } else {
        modelsUrl = editEndpoint.replace(/\/chat\/completions\/?$/, '').replace(/\/completions\/?$/, '').replace(/\/+$/, '')
        modelsUrl += '/models'
      }
      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), 30000)
      const headers = /** @type {Record<string, string>} */ ({ 'Content-Type': 'application/json' })
      if (editApiKey) headers['Authorization'] = `Bearer ${editApiKey}`
      const resp = await fetch(modelsUrl, { headers, signal: controller.signal })
      clearTimeout(timeoutId)
      if (!resp.ok) { editFetchError = `Failed (HTTP ${resp.status})`; return }
      const data = await resp.json()
      const items = data.data || data.models || []
      editFetchedModelList = items.map(/** @type {(m: any) => string} */ (m => typeof m === 'string' ? m : (m.id || m.name))).filter(Boolean)
      if (editFetchedModelList.length === 0) editFetchError = 'No models found'
    } catch (/** @type {any} */ e) {
      editFetchError = e.message || String(e); editFetchedModelList = []
    } finally { editFetchingModels = false }
  }

  function editAddSelectedModels() {
    if (editSelectedModelsToAdd.length === 0) return
    const currentCustomModels = /** @type {Array<any>} */ (get(customModels) || [])
    const providerName = builtinProviders.find(p => p.id === editProviderId)?.name || editProviderId
    for (const modelId of editSelectedModelsToAdd) {
      const compositeId = `${editProviderId}:${modelId}`
      if (!currentCustomModels.find(m => m.id === compositeId)) {
        currentCustomModels.push({ id: compositeId, modelId, name: modelId, providerId: editProviderId, providerName, apiKey: editApiKey, endpoint: editEndpoint, enabled: true, isCustom: true })
      }
    }
    saveCustomModels(currentCustomModels)
    editSelectedModelsToAdd = []; editFetchedModelList = []
  }

  /** @param {any} cm */
  function openEditCustomModel(cm) {
    editCustomModelOrigId = cm.id; editCustomModelId = cm.modelId || cm.id; editCustomModelName = cm.name
    editCustomModelProviderName = cm.providerName || ''; editCustomModelApiKey = cm.apiKey || ''
    editCustomModelApiKeyVisible = false; editCustomModelEndpoint = cm.endpoint || ''
    editCustomModelProvider = cm.providerId; showEditCustomModelDialog = true
  }

  function saveEditCustomModel() {
    const currentCustomModels = /** @type {Array<any>} */ (get(customModels) || [])
    const idx = currentCustomModels.findIndex(m => m.id === editCustomModelOrigId)
    if (idx === -1) return
    const newId = editCustomModelId.trim() || currentCustomModels[idx].modelId || currentCustomModels[idx].id
    const compositeId = `${editCustomModelProvider}:${newId}`
    currentCustomModels[idx] = { ...currentCustomModels[idx], id: compositeId, modelId: newId, name: editCustomModelName.trim() || newId, providerName: editCustomModelProviderName.trim() || currentCustomModels[idx].providerName, providerId: editCustomModelProvider, apiKey: editCustomModelApiKey, endpoint: editCustomModelEndpoint }
    saveCustomModels(currentCustomModels)
    showEditCustomModelDialog = false
  }

  async function fetchModelsFromAPI() {
    if (!addModelApiKey && !addModelEndpoint) return
    fetchingModels = true; fetchedModelList = []; fetchModelError = ''
    try {
      let modelsUrl
      if (addModelProvider === 'ollama') {
        modelsUrl = addModelEndpoint.replace(/\/+$/, '') + '/api/tags'
      } else {
        modelsUrl = addModelEndpoint.replace(/\/chat\/completions\/?$/, '').replace(/\/completions\/?$/, '').replace(/\/+$/, '')
        modelsUrl += '/models'
      }
      const controller = new AbortController()
      const timeoutId = setTimeout(() => controller.abort(), 30000)
      const headers = /** @type {Record<string, string>} */ ({ 'Content-Type': 'application/json' })
      if (addModelApiKey) headers['Authorization'] = `Bearer ${addModelApiKey}`
      const resp = await fetch(modelsUrl, { headers, signal: controller.signal })
      clearTimeout(timeoutId)
      if (!resp.ok) {
        if (resp.status === 401 || resp.status === 403) fetchModelError = `认证失败 (HTTP ${resp.status})：API 密钥无效`
        else if (resp.status >= 500) fetchModelError = `服务端错误 (HTTP ${resp.status})`
        else fetchModelError = `获取模型列表失败 (HTTP ${resp.status})`
        return
      }
      const data = await resp.json()
      const items = data.data || data.models || []
      fetchedModelList = items.map(function(/** @type {any} */ m) { return typeof m === 'string' ? m : (m.id || m.name) }).filter(Boolean)
      if (fetchedModelList.length === 0) fetchModelError = '未获取到可用模型'
    } catch (/** @type {any} */ e) {
      let errMsg = (e.message || String(e)).replace(/sk-[a-zA-Z0-9]{8,}/g, 'sk-****')
      if (e.name === 'AbortError') fetchModelError = '请求超时（30秒）'
      else if (errMsg.includes('Failed to fetch') || errMsg.includes('NetworkError')) fetchModelError = '无法连接到服务器'
      else fetchModelError = `获取模型列表失败：${errMsg}`
      fetchedModelList = []
    } finally { fetchingModels = false }
  }

  /** @param {string} modelId @param {string} providerId @param {string} endpoint @param {string} apiKey */
  async function testModelConnection(modelId, providerId, endpoint, apiKey) {
    testingModelId = modelId
    try {
      if (apiKey || endpoint) {
        await SetProviderConfig(providerId, { id: providerId, name: providerId, apiKey: apiKey || '', endpoint: endpoint || '', enabled: true, isDefault: false })
      }
      await TestProvider(providerId)
      testModelResults[modelId] = { ok: true, msg: '连接正常' }
    } catch (/** @type {any} */ e) {
      testModelResults[modelId] = { ok: false, msg: e.message || String(e) }
    }
    testModelResults = testModelResults
    testingModelId = null
  }
</script>

<div class="space-y-4">
  <p class="text-xs" style="color: var(--text-secondary);">{$t('settings.ai.configHint') || '配置 API Key 添加更多可用模型'}</p>

  <div class="rounded-lg overflow-hidden border" style="border-color: var(--border);">
    <table class="w-full text-sm" style="border-collapse: collapse;">
      <thead>
        <tr style="background-color: var(--bg-secondary);">
          <th class="text-left px-4 py-2 font-medium text-xs" style="color: var(--text-secondary);">{$t('settings.ai.model') || '模型'}</th>
          <th class="text-left px-4 py-2 font-medium text-xs" style="color: var(--text-secondary);">{$t('settings.ai.provider') || '服务商'}</th>
          <th class="text-center px-2 py-2 font-medium text-xs" style="color: var(--text-secondary);">Context</th>
          <th class="text-center px-4 py-2 font-medium text-xs" style="color: var(--text-secondary);">{$t('settings.ai.enabled') || '启用'}</th>
          <th class="text-center px-4 py-2 font-medium text-xs" style="color: var(--text-secondary);"></th>
        </tr>
      </thead>
      <tbody>
        {#each $providers as provider}
          {@const providerModelList = getProviderModelList(provider.id)}
          {#each providerModelList as model}
            {@const modelKey = `${provider.id}:${model.id}`}
            {@const modelEnabled = $modelEnabledMap[modelKey] !== false}
            <tr class="border-t" style="border-color: var(--border);">
              <td class="px-4 py-2" style="color: var(--text-primary);">{model.name}{#if model.supportsThinking}<span class="ml-1 text-[10px] px-1 rounded" style="background-color: var(--border); color: var(--text-secondary);">thinking</span>{/if}</td>
              <td class="px-4 py-2" style="color: var(--text-secondary);">{provider.name}</td>
              <td class="text-center px-2 py-2 text-[10px]" style="color: var(--text-muted);">{#if (model.maxTokens || 0) >= 1000000}{Math.round((model.maxTokens || 0) / 10000) / 100}M{:else if (model.maxTokens || 0) >= 1000}{Math.round((model.maxTokens || 0) / 1000)}K{:else}{model.maxTokens || 0}{/if}</td>
              <td class="text-center px-4 py-2">
                <div class="flex justify-center">
                  <button class="relative w-8 h-4 rounded-full transition-colors" style="background-color: {modelEnabled ? 'var(--accent)' : 'var(--border)'};" onclick={() => setModelEnabled(modelKey, !modelEnabled)} title="{$t('settings.ai.enabled') || '启用'}">
                    <span class="absolute top-0.5 w-3 h-3 rounded-full transition-transform" style="background-color: #ffffff; left: {modelEnabled ? 'calc(100% - 14px)' : '2px'};"></span>
                  </button>
                </div>
              </td>
              <td class="text-center px-4 py-2">
                <div class="flex items-center justify-center gap-1">
                  <button class="p-1 rounded transition-colors hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => testModelConnection(modelKey, provider.id, '', '')} title="Test Connection">
                    {#if testingModelId === modelKey}
                      <span class="inline-block w-3.5 h-3.5 border-2 border-t-transparent rounded-full animate-spin" style="border-color: var(--text-muted); border-top-color: transparent;"></span>
                    {:else}
                      <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
                    {/if}
                  </button>
                  {#if testModelResults[modelKey]}
                    {@const r = testModelResults[modelKey]}
                    <span style="color: {r.ok ? '#2ea043' : '#f14c4c'}; font-size: 10px; max-width: 80px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title={r.msg}>{r.ok ? '✓' : '✗'} {r.msg}</span>
                  {/if}
                  <button class="p-1 rounded transition-colors hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => openEditProvider(provider.id)} title="Edit">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" /></svg>
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        {/each}
        {#if $customModels.length > 0}
          <tr style="background-color: var(--bg-secondary);"><td colspan="4" class="px-4 py-1.5 text-xs font-semibold" style="color: var(--text-muted);">{$t('settings.ai.custom') || '自定义'}</td></tr>
          {#each $customModels as cm}
            {@const cmCtx = cm.maxTokens || cm.contextWindow || 0}
            <tr class="border-t" style="border-color: var(--border);">
              <td class="px-4 py-2" style="color: var(--text-primary);">{cm.name}</td>
              <td class="px-4 py-2" style="color: var(--text-secondary);">{cm.providerName || cm.providerId}</td>
              <td class="text-center px-2 py-2 text-[10px]" style="color: var(--text-muted);">{#if cmCtx >= 1000000}{Math.round(cmCtx / 10000) / 100}M{:else if cmCtx >= 1000}{Math.round(cmCtx / 1000)}K{:else if cmCtx > 0}{cmCtx}{:else}—{/if}</td>
              <td class="text-center px-4 py-2">
                <div class="flex justify-center">
                  <button class="relative w-8 h-4 rounded-full transition-colors" style="background-color: {cm.enabled !== false ? 'var(--accent)' : 'var(--border)'};" onclick={() => toggleCustomModel(cm.id)} title="{$t('settings.ai.enabled') || '启用'}">
                    <span class="absolute top-0.5 w-3 h-3 rounded-full transition-transform" style="background-color: #ffffff; left: {cm.enabled !== false ? 'calc(100% - 14px)' : '2px'};"></span>
                  </button>
                </div>
              </td>
              <td class="text-center px-4 py-2">
                <div class="flex items-center justify-center gap-1">
                  <button class="p-1 rounded transition-colors hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => testModelConnection(cm.id, cm.providerId, cm.endpoint || '', cm.apiKey || '')} title="Test Connection">
                    {#if testingModelId === cm.id}
                      <span class="inline-block w-3.5 h-3.5 border-2 border-t-transparent rounded-full animate-spin" style="border-color: var(--text-muted); border-top-color: transparent;"></span>
                    {:else}
                      <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" /></svg>
                    {/if}
                  </button>
                  {#if testModelResults[cm.id]}
                    {@const r = testModelResults[cm.id]}
                    <span style="color: {r.ok ? '#2ea043' : '#f14c4c'}; font-size: 10px; max-width: 80px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title={r.msg}>{r.ok ? '✓' : '✗'} {r.msg}</span>
                  {/if}
                  <button class="p-1 rounded transition-colors hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => openEditCustomModel(cm)} title="Edit">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" /></svg>
                  </button>
                  <button class="p-1 rounded transition-colors hover:bg-white/10" style="color: #f14c4c;" onclick={() => deleteCustomModel(cm.id)} title="Delete">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        {/if}
      </tbody>
    </table>
  </div>

  <div class="flex justify-end">
    <button class="px-4 py-2 rounded text-sm font-medium transition-colors" style="background-color: var(--accent); color: #ffffff;" onclick={openAddModelDialog}>+ {$t('settings.ai.addModel') || '添加模型'}</button>
  </div>
</div>

{#if showAddModelDialog}
  <div class="fixed inset-0 z-[60] flex items-center justify-center" style="background-color: rgba(0, 0, 0, 0.5);" onclick={(e) => { if (e.target === e.currentTarget) showAddModelDialog = false; }} role="button" tabindex="-1" onkeydown={(e) => { if (e.key === 'Escape') showAddModelDialog = false }}>
    <div class="rounded-lg shadow-xl p-6" style="width: 480px; background-color: var(--bg-primary); border: 1px solid var(--border);">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-base font-medium" style="color: var(--text-primary);">{$t('settings.ai.addModel') || '添加模型'}</h3>
        <button class="p-1 rounded hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => showAddModelDialog = false} aria-label="关闭">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
        </button>
      </div>
      <div class="space-y-4">
        <div>
          <label for="add-model-provider" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.provider') || '服务商'}</label>
          <select id="add-model-provider" bind:value={addModelProvider} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" onchange={onAddProviderChange}>
            {#each builtinProviders as bp}
              <option value={bp.id}>{bp.name}</option>
            {/each}
          </select>
        </div>
        <div>
          <label for="add-model-provider-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">供应商名称</label>
          <input id="add-model-provider-name" type="text" bind:value={addModelProviderName} placeholder="OpenAI" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
        </div>
        <div>
          <label for="add-model-apikey" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.apiKey')}</label>
          <div class="relative">
            <input id="add-model-apikey" type={addModelApiKeyVisible ? 'text' : 'password'} bind:value={addModelApiKey} placeholder="sk-..." class="w-full px-3 py-2 pr-9 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
            <button class="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-colors hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => addModelApiKeyVisible = !addModelApiKeyVisible}>
              {#if addModelApiKeyVisible}
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" /></svg>
              {:else}
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" /></svg>
              {/if}
            </button>
          </div>
        </div>
        <div>
          <label for="add-model-endpoint" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.endpoint')}</label>
          <input id="add-model-endpoint" type="text" bind:value={addModelEndpoint} placeholder="https://api.example.com/v1/chat/completions" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
        </div>
        <div class="flex gap-2">
          <button class="flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors" style="background-color: #094771; color: #ffffff;" onclick={fetchModelsFromAPI} disabled={fetchingModels || (!addModelApiKey && !addModelEndpoint)}>
            {#if fetchingModels}{$t('settings.ai.fetching') || '获取中...'}{:else}{$t('settings.ai.fetchModels') || '获取模型列表'}{/if}
          </button>
        </div>
        {#if fetchModelError}<p class="text-xs mt-1" style="color: #f14c4c;">{fetchModelError}</p>{/if}
        {#if fetchedModelList.length > 0}
          <div>
            <!-- svelte-ignore a11y_label_has_associated_control -->
            <label class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">可用模型 ({fetchedModelList.length})</label>
            <div class="rounded border overflow-y-auto" style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 180px;">
              {#each fetchedModelList as fm}
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <div class="w-full flex items-center gap-2 px-3 py-1.5 text-xs transition-colors cursor-pointer" style="background-color: {selectedModelsToAdd.includes(fm) ? '#094771' : 'transparent'}; color: {selectedModelsToAdd.includes(fm) ? '#ffffff' : 'var(--text-primary)'}; border-bottom: 1px solid var(--border);" onclick={() => toggleModelSelection(fm)} role="checkbox" aria-checked={selectedModelsToAdd.includes(fm)} tabindex="0" onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleModelSelection(fm); } }}>
                  <span class="flex-shrink-0" style="width: 16px; text-align: center;">{selectedModelsToAdd.includes(fm) ? '✓' : ''}</span>
                  <span class="truncate">{fm}</span>
                </div>
              {/each}
            </div>
            {#if selectedModelsToAdd.length > 0}
              <div class="mt-2 p-2 rounded text-xs" style="background-color: #09477120; border: 1px solid var(--accent); color: var(--text-primary);">已选: {selectedModelsToAdd.join(', ')}</div>
            {/if}
          </div>
        {:else if getProviderModelList(addModelProvider).length > 0}
          <div>
            <label for="add-model-select" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.selectModel') || '选择模型'}</label>
            <select id="add-model-select" bind:value={addModelId} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" onchange={() => { const m = getProviderModelList(addModelProvider).find(pm => pm.id === addModelId); if (m) addModelName = m.name; }}>
              <option value="">-- {$t('settings.ai.selectModel') || '选择模型'} --</option>
              {#each getProviderModelList(addModelProvider) as pm}
                <option value={pm.id}>{pm.name}</option>
              {/each}
            </select>
          </div>
        {:else}
          <div>
            <label for="add-model-id" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.modelId') || '模型 ID'}</label>
            <input id="add-model-id" type="text" bind:value={addModelId} placeholder="model-name" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
          </div>
          <div>
            <label for="add-model-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.modelName') || '模型名称'}</label>
            <input id="add-model-name" type="text" bind:value={addModelName} placeholder="My Model" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
          </div>
        {/if}
        <div class="flex justify-end gap-3 pt-2">
          <button class="px-4 py-2 rounded text-sm" style="background-color: var(--border); color: var(--text-primary);" onclick={() => showAddModelDialog = false}>{$t('settings.cancel')}</button>
          <button class="px-4 py-2 rounded text-sm font-medium" style="background-color: var(--accent); color: #ffffff;" onclick={saveAllSelectedModels} disabled={selectedModelsToAdd.length === 0}>{$t('settings.ai.addModel') || '添加模型'}</button>
        </div>
      </div>
    </div>
  </div>
{/if}

{#if showEditProviderDialog}
  <div class="fixed inset-0 z-[60] flex items-center justify-center" style="background-color: rgba(0, 0, 0, 0.5);" onclick={(e) => { if (e.target === e.currentTarget) showEditProviderDialog = false; }} role="button" tabindex="-1" onkeydown={(e) => { if (e.key === 'Escape') showEditProviderDialog = false }}>
    <div class="rounded-lg shadow-xl p-6 overflow-y-auto" style="width: 480px; max-height: 85vh; background-color: var(--bg-primary); border: 1px solid var(--border);">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-base font-medium" style="color: var(--text-primary);">{builtinProviders.find(p => p.id === editProviderId)?.name || editProviderId}</h3>
        <button class="p-1 rounded hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => showEditProviderDialog = false} aria-label="关闭">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
        </button>
      </div>
      <div class="space-y-4">
        <div>
          <label for="edit-apikey" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.apiKey')}</label>
          <input id="edit-apikey" type="password" bind:value={editApiKey} placeholder="sk-..." class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
        </div>
        <div>
          <label for="edit-endpoint" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.endpoint')}</label>
          <input id="edit-endpoint" type="text" bind:value={editEndpoint} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
        </div>
        {#if editExistingModels.length > 0}
          <div>
            <!-- svelte-ignore a11y_label_has_associated_control -->
            <label class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">已添加 ({editExistingModels.length})</label>
            <div class="rounded border overflow-y-auto" style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 120px;">
              {#each editExistingModels as em}
                <div class="flex items-center justify-between px-3 py-1.5 text-xs border-b" style="color: var(--text-primary); border-color: var(--border);">
                  <span class="truncate">{em.name}</span>
                  <button class="p-1 rounded hover:bg-white/10 flex-shrink-0" style="color: #f14c4c;" onclick={() => deleteCustomModel(em.id)} title="Remove">
                    <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                  </button>
                </div>
              {/each}
            </div>
          </div>
        {/if}
        <div class="flex gap-2">
          <button class="flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors" style="background-color: #094771; color: #ffffff;" onclick={editFetchModelsFromAPI} disabled={editFetchingModels || (!editApiKey && !editEndpoint)}>
            {#if editFetchingModels}{$t('settings.ai.fetching') || '获取中...'}{:else}{$t('settings.ai.fetchModels') || '获取模型列表'}{/if}
          </button>
        </div>
        {#if editFetchError}<p class="text-xs" style="color: #f14c4c;">{editFetchError}</p>{/if}
        {#if editFetchedModelList.length > 0 && editNewModels.length > 0}
          <div>
            <!-- svelte-ignore a11y_label_has_associated_control -->
            <label class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">可供添加 ({editNewModels.length})</label>
            <div class="rounded border overflow-y-auto" style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 150px;">
              {#each editNewModels as fm}
                <div class="flex items-center gap-2 px-3 py-1.5 text-xs transition-colors cursor-pointer border-b" style="background-color: {editSelectedModelsToAdd.includes(fm) ? '#094771' : 'transparent'}; color: {editSelectedModelsToAdd.includes(fm) ? '#ffffff' : 'var(--text-primary)'}; border-color: var(--border);" onclick={() => editToggleModelSelection(fm)} role="checkbox" aria-checked={editSelectedModelsToAdd.includes(fm)} tabindex="0" onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); editToggleModelSelection(fm); } }}>
                  <span style="width: 16px; text-align: center;">{editSelectedModelsToAdd.includes(fm) ? '✓' : ''}</span>
                  <span class="truncate">{fm}</span>
                </div>
              {/each}
            </div>
            {#if editSelectedModelsToAdd.length > 0}
              <button class="mt-2 w-full px-3 py-1.5 rounded text-xs font-medium" style="background-color: #2ea043; color: #ffffff;" onclick={editAddSelectedModels}>添加 {editSelectedModelsToAdd.length} 个模型</button>
            {/if}
          </div>
        {:else if editFetchedModelList.length > 0}
          <p class="text-xs" style="color: var(--text-muted);">所有可用模型已添加。</p>
        {/if}
        <div class="flex justify-end gap-3 pt-2 border-t" style="border-color: var(--border);">
          <button class="px-4 py-2 rounded text-sm" style="background-color: var(--border); color: var(--text-primary);" onclick={() => showEditProviderDialog = false}>{$t('settings.cancel')}</button>
          <button class="px-4 py-2 rounded text-sm font-medium" style="background-color: var(--accent); color: #ffffff;" onclick={() => { setProviderConfig(editProviderId, { apiKey: editApiKey, endpoint: editEndpoint }); showEditProviderDialog = false; }}>{$t('settings.save')}</button>
        </div>
      </div>
    </div>
  </div>
{/if}

{#if showEditCustomModelDialog}
  <div class="fixed inset-0 z-[60] flex items-center justify-center" style="background-color: rgba(0, 0, 0, 0.5);" onclick={(e) => { if (e.target === e.currentTarget) showEditCustomModelDialog = false; }} role="button" tabindex="-1" onkeydown={(e) => { if (e.key === 'Escape') showEditCustomModelDialog = false }}>
    <div class="rounded-lg shadow-xl p-6" style="width: 420px; background-color: var(--bg-primary); border: 1px solid var(--border);">
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-base font-medium" style="color: var(--text-primary);">{$t('settings.ai.editModel') || '编辑模型'}</h3>
        <button class="p-1 rounded hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => showEditCustomModelDialog = false} aria-label="关闭">
          <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
        </button>
      </div>
      <div class="space-y-4">
        <div>
          <label for="edit-cm-provider" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.provider') || '服务商'}</label>
          <select id="edit-cm-provider" bind:value={editCustomModelProvider} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);">
            {#each builtinProviders as bp}<option value={bp.id}>{bp.name}</option>{/each}
          </select>
        </div>
        <div>
          <label for="edit-cm-provider-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">供应商名称</label>
          <input id="edit-cm-provider-name" type="text" bind:value={editCustomModelProviderName} placeholder="OpenAI" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
        </div>
        <div>
          <label for="edit-cm-id" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.modelId') || '模型 ID'}</label>
          <input id="edit-cm-id" type="text" bind:value={editCustomModelId} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
        </div>
        <div>
          <label for="edit-cm-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.modelName') || '模型名称'}</label>
          <input id="edit-cm-name" type="text" bind:value={editCustomModelName} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
        </div>
        <div>
          <label for="edit-cm-apikey" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.apiKey')}</label>
          <div class="relative">
            <input id="edit-cm-apikey" type={editCustomModelApiKeyVisible ? 'text' : 'password'} bind:value={editCustomModelApiKey} placeholder="sk-..." class="w-full px-3 py-2 pr-9 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
            <button class="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-colors hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => editCustomModelApiKeyVisible = !editCustomModelApiKeyVisible}>
              {#if editCustomModelApiKeyVisible}
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" /></svg>
              {:else}
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" /><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" /></svg>
              {/if}
            </button>
          </div>
        </div>
        <div>
          <label for="edit-cm-endpoint" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.endpoint')}</label>
          <input id="edit-cm-endpoint" type="text" bind:value={editCustomModelEndpoint} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
        </div>
        <div class="flex gap-2">
          <button class="flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors" style="background-color: #094771; color: #ffffff;" onclick={async () => {
            if (!editCustomModelApiKey && !editCustomModelEndpoint) return
            editFetchingModels = true; editFetchedModelList = []; editFetchError = ''
            try {
              let url = (editCustomModelEndpoint || builtinProviders.find(p => p.id === editCustomModelProvider)?.defaultEndpoint || '')
              if (editCustomModelProvider === 'ollama') { url = url.replace(/\/+$/, '') + '/api/tags' }
              else { url = url.replace(/\/chat\/completions\/?$/, '').replace(/\/completions\/?$/, '').replace(/\/+$/, '') + '/models' }
              const headers = /** @type {Record<string, string>} */ ({ 'Content-Type': 'application/json' })
              if (editCustomModelApiKey) headers['Authorization'] = `Bearer ${editCustomModelApiKey}`
              const resp = await fetch(url, { headers, signal: AbortSignal.timeout(15000) })
              if (!resp.ok) { editFetchError = `Failed (HTTP ${resp.status})`; return }
              const data = await resp.json()
              const items = data.data || data.models || []
      editFetchedModelList = items.map(/** @type {(m: any) => string} */ (m => typeof m === 'string' ? m : (m.id || m.name))).filter(Boolean)
              if (editFetchedModelList.length === 0) editFetchError = 'No models found'
            } catch (/** @type {any} */ e) { editFetchError = e.message || String(e) }
            finally { editFetchingModels = false }
          }} disabled={editFetchingModels || (!editCustomModelApiKey && !editCustomModelEndpoint)}>
            {editFetchingModels ? ($t('settings.ai.fetching') || '获取中...') : ($t('settings.ai.fetchModels') || '获取模型列表')}
          </button>
        </div>
        {#if editFetchedModelList.length > 0}
          <div class="rounded border overflow-y-auto" style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 120px;">
            {#each editFetchedModelList as fm}
              <div class="flex items-center gap-2 px-3 py-1.5 text-xs cursor-pointer border-b" style="color: var(--text-primary); border-color: var(--border); background-color: {editCustomModelId === fm ? '#094771' : 'transparent'};" onclick={() => { editCustomModelId = fm; editCustomModelName = fm }} role="button" tabindex="0" onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); editCustomModelId = fm; editCustomModelName = fm } }}>
                <span style="width: 16px; text-align: center; color: #2ea043;">{editCustomModelId === fm ? '✓' : ''}</span>
                <span class="truncate">{fm}</span>
              </div>
            {/each}
          </div>
{/if}
        {#if editFetchError}<p class="text-xs" style="color: #f14c4c;">{editFetchError}</p>{/if}
        <div class="flex justify-end gap-3 pt-2">
          <button class="px-4 py-2 rounded text-sm" style="background-color: var(--border); color: var(--text-primary);" onclick={() => showEditCustomModelDialog = false}>{$t('settings.cancel')}</button>
          <button class="px-4 py-2 rounded text-sm font-medium" style="background-color: var(--accent); color: #ffffff;" onclick={saveEditCustomModel}>{$t('settings.save')}</button>
        </div>
      </div>
    </div>
  </div>
{/if}
