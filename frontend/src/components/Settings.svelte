﻿<script>
 import { fade, scale } from 'svelte/transition'
 import { writable } from 'svelte/store'
 import { settingsVisible } from '../stores/app.js'
 import { aiConfig, saveAIConfig } from '../stores/ai.js'
 import { providers, activeProviderId, loadModels, setProviderConfig, builtinProviders, providerModels, customModels, saveCustomModels, modelEnabledMap, setModelEnabled } from '../stores/provider.js'
 import { currentTheme, setTheme, themes } from '../stores/theme.js'
 import { currentLang, setLang, t } from '../stores/i18n.js'
 import { editorSettings, updateEditorSetting } from '../stores/editorSettings.js'
 import MCPManager from './MCPManager.svelte'
 import TokenUsagePanel from './TokenUsagePanel.svelte'
 
 let activeTab = 'general'
 let aboutChecking = false
 let aboutUpdateMsg = ''
 let showAddModelDialog = false
  let addModelProvider = 'openai'
  let addModelProviderName = ''
  let addModelApiKey = ''
  let addModelApiKeyVisible = false
  let addModelEndpoint = ''
  let addModelId = ''
  let addModelName = ''
 let showEditProviderDialog = false
 let editProviderId = ''
 let editApiKey = ''
 let editEndpoint = ''
 let showEditCustomModelDialog = false
 let editCustomModelOrigId = ''
  let editCustomModelId = ''
  let editCustomModelName = ''
  let editCustomModelProviderName = ''
  let editCustomModelApiKey = ''
  let editCustomModelApiKeyVisible = false
  let editCustomModelEndpoint = ''
  let editCustomModelProvider = ''
 let editFetchingModels = false

 // Load skills when tab is selected
 $: if (activeTab === 'skills') loadSkills()

 // Skills tab state
 let skills = writable([])
 let showCreateSkillDialog = false
 let newSkillId = ''
 let newSkillName = ''
 let newSkillIcon = '🔧'
 let newSkillDesc = ''
 let newSkillPrompt = ''

 async function loadSkills() {
   try {
     if (window.backend?.GetSkills) {
       const list = await window.backend.GetSkills()
       skills.set(list || [])
     }
   } catch (e) { console.error('Load skills failed:', e) }
 }

 async function createSkill() {
   if (!newSkillId || !newSkillName) return
   try {
     await window.backend.SaveSkill({
       id: newSkillId.trim().toLowerCase().replace(/\s+/g, '-'),
       name: newSkillName,
       icon: newSkillIcon || '🔧',
       description: newSkillDesc,
       promptTemplate: newSkillPrompt,
       trigger: 'manual',
       resultType: 'text',
       category: 'external',
       associatedAgents: ['universal-assistant'],
     })
     showCreateSkillDialog = false
     newSkillId = ''; newSkillName = ''; newSkillDesc = ''; newSkillPrompt = ''
     loadSkills()
   } catch (e) { console.error('Create skill failed:', e) }
 }

 async function deleteSkill(skillId) {
   if (!confirm('Delete this skill?')) return
   try {
     await window.backend.DeleteSkill(skillId)
     loadSkills()
   } catch (e) { console.error('Delete skill failed:', e) }
 }

 // URL import state
 let showImportSkillDialog = false
 let importSkillUrl = ''
 let importSkillLoading = false
 let importSkillMsg = ''
 let importSkillOk = false

 async function installSkillFromUrl() {
   if (!importSkillUrl) return
   importSkillLoading = true
   importSkillMsg = ''
   try {
     await window.backend.InstallSkillFromURL(importSkillUrl)
     importSkillOk = true
     importSkillMsg = '安装成功！技能已添加到列表。'
     importSkillUrl = ''
     loadSkills()
   } catch (e) {
     importSkillOk = false
     importSkillMsg = '安装失败: ' + (e.message || String(e))
   }
   importSkillLoading = false
 }

  $: editExistingModels = $customModels.filter(m => m.providerId === editProviderId)
  $: editExistingModelIds = new Set(editExistingModels.map(m => m.modelId || m.id.split(':').pop()))
  $: editNewModels = editFetchedModelList.filter(m => !editExistingModelIds.has(m))
 let editFetchedModelList = /** @type {string[]} */ ([])
 let editFetchError = ''
 let editSelectedModelsToAdd = /** @type {string[]} */ ([])
 let testingModelId = /** @type {string|null} */ (null)
 let testModelResults = /** @type {Record<string, {ok: boolean, msg: string}>} */ ({})
 
 const tabs = [
   { id: 'general', labelKey: 'settings.general', icon: 'M4 6h16M4 12h16M4 18h16' },
   { id: 'appearance', labelKey: 'settings.appearance', icon: 'M7 21a4 4 0 01-4-4V5a2 2 0 012-2h4a2 2 0 012 2v12a4 4 0 01-4 4zm0 0h12a2 2 0 002-2v-4a2 2 0 00-2-2h-2.343M11 7.343l1.657-1.657a2 2 0 012.828 0l2.829 2.829a2 2 0 010 2.828l-8.486 8.485M7 17h.01' },
   { id: 'editor', labelKey: 'settings.editor', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z' },
   { id: 'terminal', labelKey: 'settings.terminal', icon: 'M4 6h16M4 12h16M4 18h16' },
   { id: 'ai', labelKey: 'settings.ai', icon: 'M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z' },
   { id: 'skills', label: 'Skills', icon: 'M13 10V3L4 14h7v7l9-11h-7z' },
 { id: 'mcp', labelKey: 'settings.mcp', icon: 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z' },
   { id: 'tokenUsage', labelKey: 'settings.tokenUsage', icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10a2 2 0 01-2 2h-2a2 2 0 01-2-2zm0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z' },
   { id: 'about', labelKey: 'settings.about', icon: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z' },
 ]

let settings = {
  theme: 'dark',
  fontSize: 14,
  fontFamily: 'Cascadia Code',
  wordWrap: true,
  lineNumbers: true,
  minimap: false,
  autoSave: false,
  terminalFontSize: 14,
  terminalFontFamily: 'Cascadia Code',
  provider: 'openai',
  apiKey: '',
  model: 'gpt-4',
  endpoint: 'https://api.openai.com/v1/chat/completions',
  temperature: 0.7,
}

/** @param {string} tabId */
function setTab(tabId) {
  activeTab = tabId
}

function syncAiConfig() {
  aiConfig.set({
    provider: settings.provider,
    apiKey: settings.apiKey,
    model: settings.model,
    endpoint: settings.endpoint,
  })
}

function saveSettings() {
  localStorage.setItem('starcore-settings', JSON.stringify(settings))
  syncAiConfig()
}

function loadSettings() {
  const saved = localStorage.getItem('starcore-settings')
  if (saved) {
    settings = { ...settings, ...JSON.parse(saved) }
  }
  syncAiConfig()
  applyUIFont()
}

// Apply UI font family on load
applyUIFont()

function applyUIFont() {
  const family = settings.fontFamily === 'system'
    ? "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    : "'" + settings.fontFamily + "', monospace"
  document.body.style.fontFamily = family
}

async function checkUpdate() {
  aboutChecking = true
  aboutUpdateMsg = ''
  try {
    await new Promise(r => setTimeout(r, 1500))
    aboutUpdateMsg = 'upToDate'
  } catch {
    aboutUpdateMsg = 'error'
  }
  aboutChecking = false
}

/** @param {string} pid */
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
    editFetchingModels = true
    editFetchedModelList = []
    editSelectedModelsToAdd = []
    editFetchError = ''
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
      const headers = { 'Content-Type': 'application/json' }
      if (editApiKey) { headers['Authorization'] = `Bearer ${editApiKey}` }
      const resp = await fetch(modelsUrl, { headers, signal: controller.signal })
      clearTimeout(timeoutId)
      if (!resp.ok) { editFetchError = `Failed (HTTP ${resp.status})`; return }
      const data = await resp.json()
      const items = data.data || data.models || []
      editFetchedModelList = items.map(m => typeof m === 'string' ? m : (m.id || m.name)).filter(Boolean)
      if (editFetchedModelList.length === 0) { editFetchError = 'No models found' }
    } catch (e) {
      editFetchError = e.message || String(e)
      editFetchedModelList = []
    } finally {
      editFetchingModels = false
    }
  }

  function editAddSelectedModels() {
    if (editSelectedModelsToAdd.length === 0) return
    const $customModels = get(customModels)
    const providerName = builtinProviders.find(p => p.id === editProviderId)?.name || editProviderId
    for (const modelId of editSelectedModelsToAdd) {
      const compositeId = `${editProviderId}:${modelId}`
      if (!$customModels.find(m => m.id === compositeId)) {
        $customModels.push({
          id: compositeId, modelId, name: modelId,
          providerId: editProviderId, providerName,
          apiKey: editApiKey, endpoint: editEndpoint,
          enabled: true, isCustom: true,
        })
      }
    }
    saveCustomModels($customModels)
    editSelectedModelsToAdd = []
    editFetchedModelList = []
  }

function openEditProvider(pid) {
  editProviderId = pid
  // Load existing config: use saved API key/endpoint if available
  const existing = $customModels.find(/** @param {any} m */ m => m.providerId === pid)
  editApiKey = existing?.apiKey || ''
  editEndpoint = existing?.endpoint || builtinProviders.find(p => p.id === pid)?.defaultEndpoint || ''
  editFetchedModelList = []
  editSelectedModelsToAdd = []
  editFetchError = ''
  showEditProviderDialog = true
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

function addCustomModel() {
  if (!addModelId.trim() || !addModelProvider) return
  const $customModels = get(customModels)
  const providerName = addModelProviderName.trim() || builtinProviders.find(p => p.id === addModelProvider)?.name || addModelProvider
  const compositeId = `${addModelProvider}:${addModelId.trim()}`
  $customModels.push({
    id: compositeId,
    modelId: addModelId.trim(),
    name: addModelName.trim() || addModelId.trim(),
    providerId: addModelProvider,
    providerName,
    apiKey: addModelApiKey,
    endpoint: addModelEndpoint,
    enabled: true,
    isCustom: true,
  })
  saveCustomModels($customModels)
  if (addModelApiKey || addModelEndpoint) {
    setProviderConfig(addModelProvider, { apiKey: addModelApiKey, endpoint: addModelEndpoint })
  }
  showAddModelDialog = false
}

/** @param {string} modelId */
function deleteCustomModel(modelId) {
  const $customModels = get(customModels).filter(/** @param {any} m */ m => m.id !== modelId)
  saveCustomModels($customModels)
}

/** @param {string} modelId */
function toggleCustomModel(modelId) {
  const $customModels = get(customModels)
  const model = $customModels.find(/** @param {any} m */ m => m.id === modelId)
  if (model) {
    model.enabled = !model.enabled
    saveCustomModels($customModels)
  }
}

 function get(/** @type {any} */ store) {
   /** @type {any} */ let value = undefined
   store.subscribe(/** @param {any} v */ v => value = v)()
   return value
 }

/** @param {string} providerId @returns {Array<{ id: string, name: string, supportsThinking?: boolean }>} */
function getProviderModelList(providerId) {
   return providerModels[providerId] || []
 }

/** @param {{ id: string, name: string, providerId: string, apiKey?: string, endpoint?: string, providerName?: string }} cm */
function openEditCustomModel(cm) {
  editCustomModelOrigId = cm.id
  editCustomModelId = cm.modelId || cm.id
  editCustomModelName = cm.name
  editCustomModelProviderName = cm.providerName || ''
  editCustomModelApiKey = cm.apiKey || ''
  editCustomModelApiKeyVisible = false
  editCustomModelEndpoint = cm.endpoint || ''
  editCustomModelProvider = cm.providerId
  showEditCustomModelDialog = true
}

function saveEditCustomModel() {
  const $customModels = get(customModels)
  const idx = $customModels.findIndex(/** @param {any} m */ m => m.id === editCustomModelOrigId)
  if (idx === -1) return
  const newId = editCustomModelId.trim() || $customModels[idx].modelId || $customModels[idx].id
  const compositeId = `${editCustomModelProvider}:${newId}`
  $customModels[idx] = {
    ...$customModels[idx],
    id: compositeId,
    modelId: newId,
    name: editCustomModelName.trim() || newId,
    providerName: editCustomModelProviderName.trim() || $customModels[idx].providerName,
    providerId: editCustomModelProvider,
    apiKey: editCustomModelApiKey,
    endpoint: editCustomModelEndpoint,
  }
  saveCustomModels($customModels)
  showEditCustomModelDialog = false
}

let fetchingModels = false
let fetchedModelList = /** @type {string[]} */ ([])
let fetchModelError = ''
 let selectedModelsToAdd = /** @type {string[]} */ ([])

async function fetchModelsFromAPI() {
  if (!addModelApiKey && !addModelEndpoint) return
  fetchingModels = true
  fetchedModelList = []
  fetchModelError = ''
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

    /** @type {{ [key: string]: string }} */
    const headers = { 'Content-Type': 'application/json' }
    if (addModelApiKey) {
      headers['Authorization'] = `Bearer ${addModelApiKey}`
    }

    const resp = await fetch(modelsUrl, { headers, signal: controller.signal })
    clearTimeout(timeoutId)

    if (!resp.ok) {
      if (resp.status === 401 || resp.status === 403) {
        fetchModelError = `认证失败 (HTTP ${resp.status})：API 密钥无效，请检查后重试`
      } else if (resp.status >= 500) {
        fetchModelError = `服务端错误 (HTTP ${resp.status})，请稍后重试`
      } else {
        fetchModelError = `获取模型列表失败 (HTTP ${resp.status})，请手动输入模型 ID`
      }
      return
    }

    const data = await resp.json()
    const items = data.data || data.models || []
    fetchedModelList = items.map(function(m) { return typeof m === "string" ? m : (m.id || m.name) }).filter(Boolean)

    if (fetchedModelList.length === 0) {
      fetchModelError = '未获取到可用模型，请手动输入模型 ID'
    }
  } catch (/** @type {any} */ e) {
    let errMsg = e.message || String(e)
    errMsg = errMsg.replace(/sk-[a-zA-Z0-9]{8,}/g, 'sk-****')

    if (e.name === 'AbortError') {
      fetchModelError = '请求超时（30秒），请检查接口地址是否可达'
    } else if (errMsg.includes('Failed to fetch') || errMsg.includes('NetworkError')) {
      fetchModelError = '无法连接到服务器，请检查接口地址是否正确'
    } else {
      fetchModelError = `获取模型列表失败：${errMsg}，请手动输入模型 ID`
    }
    fetchedModelList = []
  } finally {
    fetchingModels = false
  }
}

async function testModelConnection(modelId, providerId, endpoint, apiKey) {
  testingModelId = modelId
  try {
    // Ensure provider is configured with this model's credentials before testing
    if (apiKey || endpoint) {
      await window.backend.SetProviderConfig(providerId, {
        id: providerId, name: providerId,
        apiKey: apiKey || '', endpoint: endpoint || '',
        enabled: true, isDefault: false,
      })
    }
    await window.backend.TestProvider(providerId)
    testModelResults[modelId] = { ok: true, msg: '连接正常' }
  } catch (e) {
    testModelResults[modelId] = { ok: false, msg: e.message || String(e) }
  }
  testModelResults = testModelResults
  testingModelId = null
}

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
  const $customModels = get(customModels)
  const providerName = addModelProviderName.trim() || builtinProviders.find(p => p.id === addModelProvider)?.name || addModelProvider
  for (const modelId of selectedModelsToAdd) {
    const compositeId = `${addModelProvider}:${modelId}`
    if (!$customModels.find(m => m.id === compositeId)) {
      $customModels.push({
        id: compositeId, modelId, name: modelId,
        providerId: addModelProvider, providerName,
        apiKey: addModelApiKey, endpoint: addModelEndpoint,
        enabled: true, isCustom: true,
      })
    }
  }
  saveCustomModels($customModels)
  if (addModelApiKey || addModelEndpoint) {
    setProviderConfig(addModelProvider, { apiKey: addModelApiKey, endpoint: addModelEndpoint })
  }
  selectedModelsToAdd = []
  showAddModelDialog = false
}

loadSettings()
</script>

{#if $settingsVisible}
  <div class="dialog-backdrop" transition:fade={{ duration: 150 }} onclick={(e) => { if (e.target === e.currentTarget) settingsVisible.set(false); }}>
    <div class="dialog-content flex" transition:scale={{ duration: 200, start: 0.95 }} style="width: min(800px, 90vw); height: min(600px, 85vh);">
      <div class="w-48 border-r flex flex-col" style="background-color: var(--bg-secondary); border-color: var(--border);">
        <div class="p-4 border-b" style="border-color: var(--border);">
          <h2 class="text-lg font-semibold" style="color: var(--text-primary);">{$t('settings.title')}</h2>
        </div>
        <div class="flex-1 py-2 overflow-y-auto">
          {#each tabs as tab}
            <button
              class="w-full flex items-center gap-3 px-4 py-2.5 text-sm transition-colors"
              style="background-color: {activeTab === tab.id ? 'var(--bg-primary)' : 'transparent'}; color: {activeTab === tab.id ? 'var(--text-primary)' : 'var(--text-secondary)'}; border-radius: 0;"
              onclick={() => setTab(tab.id)}
            >
              <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d={tab.icon} />
              </svg>
              {tab.label || $t(tab.labelKey || '')}
            </button>
          {/each}
        </div>
      </div>
      
      <div class="flex-1 flex flex-col">
        <div class="flex items-center justify-between px-6 py-4 border-b" style="border-color: var(--border);">
          <h3 class="text-lg font-medium" style="color: var(--text-primary);">
            {tabs.find(tb => tb.id === activeTab)?.label || $t(tabs.find(tb => tb.id === activeTab)?.labelKey || '')}
          </h3>
          <button 
            class="btn btn-ghost btn-icon"
            onclick={() => settingsVisible.set(false)}
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        
        <div class="flex-1 overflow-y-auto p-6">
          {#if activeTab === 'general'}
            <div class="space-y-6">
              <div>
                <label for="settings-autoSave" class="block text-sm font-medium mb-2" style="color: var(--text-primary);">{$t('settings.general.autoSave')}</label>
                <div class="flex items-center gap-2">
                  <input 
                    id="settings-autoSave"
                    type="checkbox" 
                    bind:checked={settings.autoSave}
                    class="rounded"
                    onchange={saveSettings}
                  />
                  <span class="text-sm" style="color: var(--text-secondary);">{$t('settings.general.autoSaveDesc')}</span>
                </div>
              </div>
            </div>
          {:else if activeTab === 'appearance'}
            <div class="space-y-6">
              <div class="space-y-3">
                <h3 class="text-sm font-medium" style="color: var(--text-primary);">{$t('settings.appearance.theme')}</h3>
                <div class="grid gap-2" style="grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));">
                  {#each themes as theme}
                    <button
                      class="p-3 rounded text-sm transition-all text-left"
                      style="background-color: {theme.colors.bg}; color: {theme.colors.text}; border: 2px solid {$currentTheme === theme.id ? theme.colors.accent : theme.colors.border};"
                      onclick={() => setTheme(theme.id)}
                    >
                      <div class="flex gap-2 mb-1">
                        <span class="w-3 h-3 rounded-full flex-shrink-0" style="background-color: {theme.colors.accent};"></span>
                        <span class="w-3 h-3 rounded-full flex-shrink-0" style="background-color: {theme.colors.text}; opacity: 0.5;"></span>
                        <span class="w-3 h-3 rounded-full flex-shrink-0" style="background-color: {theme.colors.text2}; opacity: 0.5;"></span>
                      </div>
                      <span class="font-medium">{theme.name}</span>
                      {#if $currentTheme === theme.id}
                        <span class="ml-1" style="color: {theme.colors.accent};">✓</span>
                      {/if}
                    </button>
                  {/each}
                </div>
              </div>

              <div class="space-y-3">
                <h3 class="text-sm font-medium" style="color: var(--text-primary);">{$t('settings.appearance.language')}</h3>
                <div class="flex gap-2">
                  {#each [{id: 'zh', key: 'lang.zh'}, {id: 'en', key: 'lang.en'}] as lang}
                    <button
                      class="flex-1 p-3 rounded text-sm transition-colors"
                      style="background-color: {$currentLang === lang.id ? '#094771' : 'var(--bg-secondary)'}; color: var(--text-primary); border: 2px solid {$currentLang === lang.id ? 'var(--accent)' : 'var(--border)'};"
                      onclick={() => setLang(lang.id)}
                    >
                      {$t(lang.key)}
                    </button>
                  {/each}
                </div>
              </div>
              
              <div>
                <label class="block text-sm font-medium mb-2" style="color: var(--text-primary);">{$t('settings.appearance.fontFamily')}</label>
                <select
                  bind:value={settings.fontFamily}
                  class="w-full px-3 py-2 rounded border text-sm"
                  style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);"
                  onchange={() => { saveSettings(); applyUIFont() }}
                >
                  <option value="system">System Default</option>
                  <option value="Cascadia Code">Cascadia Code</option>
                  <option value="JetBrains Mono">JetBrains Mono</option>
                  <option value="Fira Code">Fira Code</option>
                  <option value="Consolas">Consolas</option>
                </select>
              </div>
            </div>
          {:else if activeTab === 'editor'}
            <div class="space-y-6">
              <div>
                <label class="block text-sm font-medium mb-2" style="color: var(--text-primary);">{$t('settings.editor.fontSize')}</label>
                <div class="flex items-center gap-3">
                  <input type="range" value={$editorSettings.fontSize} min="11" max="28" class="flex-1" oninput={(e) => updateEditorSetting('fontSize', parseInt(e.target.value))} />
                  <span class="text-sm font-mono" style="color: var(--text-primary); min-width: 28px;">{$editorSettings.fontSize}px</span>
                </div>
              </div>
              <div>
                <label class="block text-sm font-medium mb-2" style="color: var(--text-primary);">{$t('settings.editor.fontFamily')}</label>
                <select value={$editorSettings.fontFamily} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);" onchange={(e) => updateEditorSetting('fontFamily', e.target.value)}>
                  <option value="'Cascadia Code', 'JetBrains Mono', 'Fira Code', 'Consolas', 'monospace'">Cascadia Code</option>
                  <option value="'JetBrains Mono', 'Cascadia Code', 'Fira Code', 'Consolas', 'monospace'">JetBrains Mono</option>
                  <option value="'Fira Code', 'Cascadia Code', 'JetBrains Mono', 'Consolas', 'monospace'">Fira Code</option>
                  <option value="'Consolas', 'Cascadia Code', 'JetBrains Mono', 'monospace'">Consolas</option>
                  <option value="'Source Code Pro', 'Cascadia Code', 'Consolas', 'monospace'">Source Code Pro</option>
                  <option value="'Monaco', 'Consolas', 'monospace'">Monaco</option>
                </select>
              </div>
              <div>
                <label class="block text-sm font-medium mb-2" style="color: var(--text-primary);">{$t('editor.syntaxHighlight')}</label>
                <select value={$editorSettings.highlightTheme} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);" onchange={(e) => updateEditorSetting('highlightTheme', e.target.value)}>
                  <option value="one-dark">One Dark</option>
                  <option value="dracula">Dracula</option>
                  <option value="monokai">Monokai</option>
                  <option value="nord">Nord</option>
                  <option value="github">GitHub</option>
                </select>
              </div>
              <div>
                <label class="block text-sm font-medium mb-2" style="color: var(--text-primary);">{$t('settings.editor.lineHeight')}</label>
                <div class="flex items-center gap-3">
                  <input type="range" value={$editorSettings.lineHeight} min="1.2" max="2.4" step="0.1" class="flex-1" oninput={(e) => updateEditorSetting('lineHeight', parseFloat(e.target.value))} />
                  <span class="text-sm font-mono" style="color: var(--text-primary); min-width: 28px;">{$editorSettings.lineHeight}</span>
                </div>
              </div>
              <div>
                <label class="block text-sm font-medium mb-2" style="color: var(--text-primary);">{$t('settings.editor.wordWrap')}</label>
                <div class="flex items-center gap-2">
                  <input type="checkbox" checked={$editorSettings.wordWrap} class="rounded" onchange={(e) => updateEditorSetting('wordWrap', e.target.checked)} />
                  <span class="text-sm" style="color: var(--text-secondary);">{$t('settings.editor.wordWrapText')}</span>
                </div>
              </div>

              <div class="pt-4 border-t" style="border-color: var(--border);">
                <h3 class="text-sm font-medium mb-4" style="color: var(--text-primary);">{$t('settings.editor.cursor')}</h3>
                <div class="space-y-4">
                  <div class="flex gap-4">
                    <div class="flex-1">
                      <label class="block text-xs mb-1.5" style="color: var(--text-secondary);">{$t('settings.editor.cursorWidth')}</label>
                      <div class="flex items-center gap-2">
                        <input type="range" value={$editorSettings.cursorWidth} min="1" max="6" class="flex-1" oninput={(e) => updateEditorSetting('cursorWidth', parseInt(e.target.value))} />
                        <span class="text-xs font-mono" style="color: var(--text-primary); min-width: 16px;">{$editorSettings.cursorWidth}px</span>
                      </div>
                    </div>
                    <div class="flex-1">
                      <label class="block text-xs mb-1.5" style="color: var(--text-secondary);">{$t('settings.editor.cursorColor')}</label>
                      <div class="flex items-center gap-2">
                        <input type="color" value={$editorSettings.cursorColor} class="w-8 h-8 rounded border cursor-pointer" style="border-color: var(--border);" oninput={(e) => updateEditorSetting('cursorColor', e.target.value)} />
                        <span class="text-xs font-mono" style="color: var(--text-primary);">{$editorSettings.cursorColor}</span>
                      </div>
                    </div>
                  </div>
                  <div class="flex gap-4">
                    <div class="flex-1">
                      <label class="block text-xs mb-1.5" style="color: var(--text-secondary);">{$t('editor.cursorHeight')}</label>
                      <div class="flex items-center gap-2">
                        <input type="range" value={$editorSettings.cursorHeight} min="20" max="100" step="5" class="flex-1" oninput={(e) => updateEditorSetting('cursorHeight', parseInt(e.target.value))} />
                        <span class="text-xs font-mono" style="color: var(--text-primary); min-width: 28px;">{$editorSettings.cursorHeight}%</span>
                      </div>
                    </div>
                    <div class="flex-1">
                      <label class="block text-xs mb-1.5" style="color: var(--text-secondary);">{$t('settings.editor.cursorStyle')}</label>
                      <select value={$editorSettings.cursorStyle} class="w-full px-2 py-1.5 rounded border text-xs" style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);" onchange={(e) => updateEditorSetting('cursorStyle', e.target.value)}>
                        <option value="block">{$t('settings.editor.cursorStyleBlock')}</option>
                        <option value="line">{$t('settings.editor.cursorStyleLine')}</option>
                        <option value="underline">{$t('settings.editor.cursorStyleUnderline')}</option>
                      </select>
                    </div>
                    <div class="flex-1">
                      <label class="block text-xs mb-1.5" style="color: var(--text-secondary);">{$t('settings.editor.cursorBlinkStyle')}</label>
                      <select value={$editorSettings.cursorBlinkStyle} class="w-full px-2 py-1.5 rounded border text-xs" style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);" onchange={(e) => updateEditorSetting('cursorBlinkStyle', e.target.value)}>
                        <option value="blink">{$t('settings.editor.cursorBlinkBlink')}</option>
                        <option value="smooth">{$t('settings.editor.cursorBlinkSmooth')}</option>
                        <option value="phase">{$t('settings.editor.cursorBlinkPhase')}</option>
                        <option value="expand">{$t('settings.editor.cursorBlinkExpand')}</option>
                        <option value="solid">{$t('settings.editor.cursorBlinkSolid')}</option>
                      </select>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          {:else if activeTab === 'terminal'}
            <!-- svelte-ignore a11y_label_has_associated_control -->
            <div class="space-y-6">
              <div>
                <label class="block text-sm font-medium mb-2" style="color: var(--text-primary);">{$t('settings.terminal.fontSize')}</label>
                <input type="range" bind:value={settings.terminalFontSize} min="10" max="24" class="w-full" onchange={saveSettings} />
                <span class="text-sm" style="color: var(--text-secondary);">{settings.terminalFontSize}px</span>
              </div>
              <div>
                <label class="block text-sm font-medium mb-2" style="color: var(--text-primary);">{$t('settings.terminal.fontFamily')}</label>
                <select bind:value={settings.terminalFontFamily} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);" onchange={saveSettings}>
                  <option value="Cascadia Code">Cascadia Code</option>
                  <option value="JetBrains Mono">JetBrains Mono</option>
                  <option value="Fira Code">Fira Code</option>
                  <option value="Consolas">Consolas</option>
                </select>
              </div>
            </div>
          {:else if activeTab === 'ai'}
            <div class="space-y-4">
              <p class="text-xs" style="color: var(--text-secondary);">{$t('settings.ai.configHint') || 'é…ç½® API Key æ·»åŠ æ›´å¤šå¯ç”¨æ¨¡åž‹'}</p>

              <div class="rounded-lg overflow-hidden border" style="border-color: var(--border);">
                <table class="w-full text-sm" style="border-collapse: collapse;">
                  <thead>
                    <tr style="background-color: var(--bg-secondary);">
                      <th class="text-left px-4 py-2 font-medium text-xs" style="color: var(--text-secondary);">{$t('settings.ai.model') || 'æ¨¡åž‹'}</th>
                      <th class="text-left px-4 py-2 font-medium text-xs" style="color: var(--text-secondary);">{$t('settings.ai.provider') || 'æœåŠ¡å•†'}</th>
                      <th class="text-center px-2 py-2 font-medium text-xs" style="color: var(--text-secondary);">Context</th>
                      <th class="text-center px-4 py-2 font-medium text-xs" style="color: var(--text-secondary);">{$t('settings.ai.enabled') || 'å¯ç”¨'}</th>
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
                          <td class="px-4 py-2" style="color: var(--text-primary);">
                            {model.name}
                            {#if model.supportsThinking}
                              <span class="ml-1 text-[10px] px-1 rounded" style="background-color: var(--border); color: var(--text-secondary);">thinking</span>
                            {/if}
                          </td>
                          <td class="px-4 py-2" style="color: var(--text-secondary);">{provider.name}</td>
                          <td class="text-center px-2 py-2 text-[10px]" style="color: var(--text-muted);">
                            {#if model.maxTokens >= 1000000}{Math.round(model.maxTokens / 10000) / 100}M{:else if model.maxTokens >= 1000}{Math.round(model.maxTokens / 1000)}K{:else}{model.maxTokens}{/if}
                          </td>
                          <td class="text-center px-4 py-2">
                            <div class="flex justify-center">
                              <button
                                class="relative w-8 h-4 rounded-full transition-colors"
                                style="background-color: {modelEnabled ? 'var(--accent)' : 'var(--border)'};"
                                onclick={() => setModelEnabled(modelKey, !modelEnabled)}
                                aria-label={modelEnabled ? 'ç¦ç”¨' : 'å¯ç”¨'}
                              >
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
                      <tr style="background-color: var(--bg-secondary);">
                        <td colspan="4" class="px-4 py-1.5 text-xs font-semibold" style="color: var(--text-muted);">{$t('settings.ai.custom') || 'è‡ªå®šä¹‰'}</td>
                      </tr>
                      {#each $customModels as cm}
                        {@const cmCtx = cm.maxTokens || cm.contextWindow || 0}
                        <tr class="border-t" style="border-color: var(--border);">
                          <td class="px-4 py-2" style="color: var(--text-primary);">{cm.name}</td>
                          <td class="px-4 py-2" style="color: var(--text-secondary);">{cm.providerName || cm.providerId}</td>
                          <td class="text-center px-2 py-2 text-[10px]" style="color: var(--text-muted);">
                            {#if cmCtx >= 1000000}{Math.round(cmCtx / 10000) / 100}M{:else if cmCtx >= 1000}{Math.round(cmCtx / 1000)}K{:else if cmCtx > 0}{cmCtx}{:else}—{/if}
                          </td>
                          <td class="text-center px-4 py-2">
                            <div class="flex justify-center">
                              <button
                                class="relative w-8 h-4 rounded-full transition-colors"
                                style="background-color: {cm.enabled !== false ? 'var(--accent)' : 'var(--border)'};"
                                onclick={() => toggleCustomModel(cm.id)}
                                aria-label={cm.enabled !== false ? 'ç¦ç”¨' : 'å¯ç”¨'}
                              >
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
                <button
                  class="px-4 py-2 rounded text-sm font-medium transition-colors"
                  style="background-color: var(--accent); color: #ffffff;"
                  onclick={openAddModelDialog}
                >
                  + {$t('settings.ai.addModel') || 'æ·»åŠ æ¨¡åž‹'}
                </button>
              </div>

              <!-- temperature & maxTokens auto-managed -->
            </div>

            {#if showAddModelDialog}
              <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions a11y-no-noninteractive-element-interactions -->
              <div class="fixed inset-0 z-[60] flex items-center justify-center" style="background-color: rgba(0, 0, 0, 0.5);" onclick={(e) => { if (e.target === e.currentTarget) showAddModelDialog = false; }}>
                <div class="rounded-lg shadow-xl p-6" style="width: 480px; background-color: var(--bg-primary); border: 1px solid var(--border);">
                  <div class="flex items-center justify-between mb-4">
                    <h3 class="text-base font-medium" style="color: var(--text-primary);">{$t('settings.ai.addModel') || 'æ·»åŠ æ¨¡åž‹'}</h3>
                    <button class="p-1 rounded hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => showAddModelDialog = false} aria-label="å…³é—­">
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
                        <button class="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-colors hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => addModelApiKeyVisible = !addModelApiKeyVisible} aria-label={addModelApiKeyVisible ? 'éšè— API Key' : 'æ˜¾ç¤º API Key'}>
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
                      <button
                        class="flex-1 px-3 py-1.5 rounded text-xs font-medium transition-colors"
                        style="background-color: #094771; color: #ffffff;"
                        onclick={fetchModelsFromAPI}
                        disabled={fetchingModels || (!addModelApiKey && !addModelEndpoint)}
                      >
                        {#if fetchingModels}
                          {$t('settings.ai.fetching') || 'èŽ·å–ä¸­...'}
                        {:else}
                          {$t('settings.ai.fetchModels') || 'èŽ·å–æ¨¡åž‹åˆ—è¡¨'}
                        {/if}
                      </button>
                    </div>

                    {#if fetchModelError}
                      <p class="text-xs mt-1" style="color: #f14c4c;">{fetchModelError}</p>
                    {/if}

                    {#if fetchedModelList.length > 0}
                      <div>
                        <label class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">Available models ({fetchedModelList.length}) - click to select multiple</label>
                        <div class="rounded border overflow-y-auto" style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 180px;">
                          {#each fetchedModelList as fm}
                            <!-- svelte-ignore a11y_click_events_have_key_events -->
                            <div
                              class="w-full flex items-center gap-2 px-3 py-1.5 text-xs transition-colors cursor-pointer"
                              style="background-color: {selectedModelsToAdd.includes(fm) ? '#094771' : 'transparent'}; color: {selectedModelsToAdd.includes(fm) ? '#ffffff' : 'var(--text-primary)'}; border-bottom: 1px solid var(--border);"
                              onclick={() => toggleModelSelection(fm)}
                              role="checkbox"
                              aria-checked={selectedModelsToAdd.includes(fm)}
                              tabindex="0"
                              onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleModelSelection(fm); } }}
                            >
                              <span class="flex-shrink-0" style="width: 16px; text-align: center;">{selectedModelsToAdd.includes(fm) ? '✓' : ''}</span>
                              <span class="truncate">{fm}</span>
                            </div>
                          {/each}
                        </div>
                        {#if selectedModelsToAdd.length > 0}
                          <div class="mt-2 p-2 rounded text-xs" style="background-color: #09477120; border: 1px solid var(--accent); color: var(--text-primary);">
                            Selected: {selectedModelsToAdd.join(', ')}
                          </div>
                        {/if}
                      </div>
                    {:else if getProviderModelList(addModelProvider).length > 0}
                      <div>
                        <label for="add-model-select" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.selectModel') || 'é€‰æ‹©æ¨¡åž‹'}</label>
                        <select id="add-model-select" bind:value={addModelId} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" onchange={() => { const m = getProviderModelList(addModelProvider).find(/** @param {{ id: string }} pm */ pm => pm.id === addModelId); if (m) addModelName = m.name; }}>
                          <option value="">-- {$t('settings.ai.selectModel') || 'é€‰æ‹©æ¨¡åž‹'} --</option>
                          {#each getProviderModelList(addModelProvider) as pm}
                            <option value={pm.id}>{pm.name}</option>
                          {/each}
                        </select>
                      </div>
                    {:else}
                      <div>
                        <label for="add-model-id" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.modelId') || 'æ¨¡åž‹ ID'}</label>
                        <input id="add-model-id" type="text" bind:value={addModelId} placeholder="model-name" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
                      </div>
                      <div>
                        <label for="add-model-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.modelName') || 'æ¨¡åž‹åç§°'}</label>
                        <input id="add-model-name" type="text" bind:value={addModelName} placeholder="My Model" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
                      </div>
                    {/if}

                    <div class="flex justify-end gap-3 pt-2">
                      <button class="px-4 py-2 rounded text-sm" style="background-color: var(--border); color: var(--text-primary);" onclick={() => showAddModelDialog = false}>
                        {$t('settings.cancel')}
                      </button>
                      <button class="px-4 py-2 rounded text-sm font-medium" style="background-color: var(--accent); color: #ffffff;" onclick={saveAllSelectedModels} disabled={selectedModelsToAdd.length === 0}>
                        {$t('settings.ai.addModel') || 'æ·»åŠ æ¨¡åž‹'}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            {/if}

            {#if showEditProviderDialog}
              <div class="fixed inset-0 z-[60] flex items-center justify-center" style="background-color: rgba(0, 0, 0, 0.5);" onclick={(e) => { if (e.target === e.currentTarget) showEditProviderDialog = false; }}>
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
                      <!-- existing models computed inline below -->
                      <div>
                        <label class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">Added models ({editExistingModels.length})</label>
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

                    {#if editFetchError}
                      <p class="text-xs" style="color: #f14c4c;">{editFetchError}</p>
                    {/if}

                    {#if editFetchedModelList.length > 0}
                      {#if editNewModels.length > 0}
                        <div>
                          <label class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">Available to add ({editNewModels.length})</label>
                          <div class="rounded border overflow-y-auto" style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 150px;">
                            {#each editNewModels as fm}
                              <div class="flex items-center gap-2 px-3 py-1.5 text-xs transition-colors cursor-pointer border-b" style="background-color: {editSelectedModelsToAdd.includes(fm) ? '#094771' : 'transparent'}; color: {editSelectedModelsToAdd.includes(fm) ? '#ffffff' : 'var(--text-primary)'}; border-color: var(--border);" onclick={() => editToggleModelSelection(fm)} role="checkbox" aria-checked={editSelectedModelsToAdd.includes(fm)} tabindex="0" onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); editToggleModelSelection(fm); } }}>
                                <span style="width: 16px; text-align: center;">{editSelectedModelsToAdd.includes(fm) ? '✓' : ''}</span>
                                <span class="truncate">{fm}</span>
                              </div>
                            {/each}
                          </div>
                          {#if editSelectedModelsToAdd.length > 0}
                            <button class="mt-2 w-full px-3 py-1.5 rounded text-xs font-medium" style="background-color: #2ea043; color: #ffffff;" onclick={editAddSelectedModels}>
                              Add {editSelectedModelsToAdd.length} selected model(s)
                            </button>
                          {/if}
                        </div>
                      {:else}
                        <p class="text-xs" style="color: var(--text-muted);">All available models are already added.</p>
                      {/if}
                    {/if}

                    <div class="flex justify-end gap-3 pt-2 border-t" style="border-color: var(--border);">
                      <button class="px-4 py-2 rounded text-sm" style="background-color: var(--border); color: var(--text-primary);" onclick={() => showEditProviderDialog = false}>
                        {$t('settings.cancel')}
                      </button>
                      <button class="px-4 py-2 rounded text-sm font-medium" style="background-color: var(--accent); color: #ffffff;" onclick={() => { setProviderConfig(editProviderId, { apiKey: editApiKey, endpoint: editEndpoint }); showEditProviderDialog = false; }}>
                        {$t('settings.save')}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            {/if}

            {#if showEditCustomModelDialog}
              <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions a11y-no-noninteractive-element-interactions -->
              <div class="fixed inset-0 z-[60] flex items-center justify-center" style="background-color: rgba(0, 0, 0, 0.5);" onclick={(e) => { if (e.target === e.currentTarget) showEditCustomModelDialog = false; }}>
                <div class="rounded-lg shadow-xl p-6" style="width: 420px; background-color: var(--bg-primary); border: 1px solid var(--border);">
                  <div class="flex items-center justify-between mb-4">
                    <h3 class="text-base font-medium" style="color: var(--text-primary);">{$t('settings.ai.editModel') || 'ç¼–è¾‘æ¨¡åž‹'}</h3>
                    <button class="p-1 rounded hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => showEditCustomModelDialog = false} aria-label="å…³é—­">
                      <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                    </button>
                  </div>
                  <div class="space-y-4">
                    <div>
                      <label for="edit-cm-provider" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.provider') || 'æœåŠ¡å•†'}</label>
                      <select id="edit-cm-provider" bind:value={editCustomModelProvider} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);">
                        {#each builtinProviders as bp}
                          <option value={bp.id}>{bp.name}</option>
                        {/each}
                      </select>
                    </div>
                    <div>
                      <label for="edit-cm-provider-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">供应商名称</label>
                      <input id="edit-cm-provider-name" type="text" bind:value={editCustomModelProviderName} placeholder="OpenAI" class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
                    </div>
                    <div>
                      <label for="edit-cm-id" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.modelId') || 'æ¨¡åž‹ ID'}</label>
                      <input id="edit-cm-id" type="text" bind:value={editCustomModelId} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
                    </div>
                    <div>
                      <label for="edit-cm-name" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.modelName') || 'æ¨¡åž‹åç§°'}</label>
                      <input id="edit-cm-name" type="text" bind:value={editCustomModelName} class="w-full px-3 py-2 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
                    </div>
                    <div>
                      <label for="edit-cm-apikey" class="block text-xs mb-1.5 font-medium" style="color: var(--text-secondary);">{$t('settings.ai.apiKey')}</label>
                      <div class="relative">
                        <input id="edit-cm-apikey" type={editCustomModelApiKeyVisible ? 'text' : 'password'} bind:value={editCustomModelApiKey} placeholder="sk-..." class="w-full px-3 py-2 pr-9 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" />
                        <button class="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded transition-colors hover:bg-white/10" style="color: var(--text-secondary);" onclick={() => editCustomModelApiKeyVisible = !editCustomModelApiKeyVisible} aria-label={editCustomModelApiKeyVisible ? 'éšè— API Key' : 'æ˜¾ç¤º API Key'}>
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
                        editFetchingModels = true
                        editFetchedModelList = []
                        editFetchError = ''
                        try {
                          let url = (editCustomModelEndpoint || builtinProviders.find(p => p.id === editCustomModelProvider)?.defaultEndpoint || '')
                          if (editCustomModelProvider === 'ollama') {
                            url = url.replace(/\/+$/, '') + '/api/tags'
                          } else {
                            url = url.replace(/\/chat\/completions\/?$/, '').replace(/\/completions\/?$/, '').replace(/\/+$/, '') + '/models'
                          }
                          const headers = { 'Content-Type': 'application/json' }
                          if (editCustomModelApiKey) headers['Authorization'] = `Bearer ${editCustomModelApiKey}`
                          const resp = await fetch(url, { headers, signal: AbortSignal.timeout(15000) })
                          if (!resp.ok) { editFetchError = `Failed (HTTP ${resp.status})`; return }
                          const data = await resp.json()
                          const items = data.data || data.models || []
                          editFetchedModelList = items.map(m => typeof m === 'string' ? m : (m.id || m.name)).filter(Boolean)
                          if (editFetchedModelList.length === 0) editFetchError = 'No models found'
                        } catch (e) {
                          editFetchError = e.message || String(e)
                        } finally {
                          editFetchingModels = false
                        }
                      }} disabled={editFetchingModels || (!editCustomModelApiKey && !editCustomModelEndpoint)}>
                        {editFetchingModels ? ($t('settings.ai.fetching') || '获取中...') : ($t('settings.ai.fetchModels') || '获取模型列表')}
                      </button>
                    </div>
                    {#if editFetchedModelList.length > 0}
                      <div class="rounded border overflow-y-auto" style="background-color: var(--bg-secondary); border-color: var(--border); max-height: 120px;">
                        {#each editFetchedModelList as fm}
                          <div class="flex items-center gap-2 px-3 py-1.5 text-xs cursor-pointer border-b" style="color: var(--text-primary); border-color: var(--border); background-color: {editCustomModelId === fm ? '#094771' : 'transparent'};"
                            onclick={() => { editCustomModelId = fm; editCustomModelName = fm }}
                          >
                            <span style="width: 16px; text-align: center; color: #2ea043;">{editCustomModelId === fm ? '✓' : ''}</span>
                            <span class="truncate">{fm}</span>
                          </div>
                        {/each}
                      </div>
                    {/if}
                    {#if editFetchError}
                      <p class="text-xs" style="color: #f14c4c;">{editFetchError}</p>
                    {/if}
                    <div class="flex justify-end gap-3 pt-2">
                      <button class="px-4 py-2 rounded text-sm" style="background-color: var(--border); color: var(--text-primary);" onclick={() => showEditCustomModelDialog = false}>
                        {$t('settings.cancel')}
                      </button>
                      <button class="px-4 py-2 rounded text-sm font-medium" style="background-color: var(--accent); color: #ffffff;" onclick={saveEditCustomModel}>
                        {$t('settings.save')}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            {/if}
          {:else if activeTab === 'skills'}
            <div class="space-y-6">
              <div>
                <h3 class="text-sm font-medium mb-1" style="color: var(--text-primary);">已安装的扩展技能</h3>
                <p class="text-xs" style="color: var(--text-secondary);">系统内置 {$skills.filter(s => s.category !== 'external').length} 个技能，在对话中用 /技能名 触发。以下是你自己安装的扩展技能：</p>
              </div>
              <div class="flex gap-2">
                <button class="px-4 py-2 rounded text-sm font-medium" style="background-color: var(--accent); color: #fff;" onclick={showCreateSkillDialog = true}>+ 创建新技能</button>
                <button class="px-4 py-2 rounded text-sm" style="background-color: var(--border); color: var(--text-primary);" onclick={showImportSkillDialog = true}>从 URL 安装</button>
              </div>
              <div class="rounded-lg overflow-hidden border" style="border-color: var(--border);">
                {#if $skills.filter(s => s.category === 'external').length > 0}
                  <table class="w-full text-sm" style="border-collapse: collapse;">
                    <thead>
                      <tr style="background-color: var(--bg-secondary);">
                        <th class="text-left px-4 py-2 font-medium text-xs" style="color: var(--text-secondary);">技能</th>
                        <th class="text-left px-4 py-2 font-medium text-xs" style="color: var(--text-secondary);">说明</th>
                        <th class="text-center px-4 py-2 font-medium text-xs" style="color: var(--text-secondary);">命令</th>
                        <th class="text-center px-4 py-2 font-medium text-xs" style="color: var(--text-secondary); width: 40px;"></th>
                      </tr>
                    </thead>
                    <tbody>
                      {#each $skills.filter(s => s.category === 'external') as skill}
                        <tr class="border-t" style="border-color: var(--border);">
                          <td class="px-4 py-2.5" style="color: var(--text-primary);">
                            <span class="mr-2">{skill.icon || '📋'}</span>{skill.name}
                          </td>
                          <td class="px-4 py-2.5 text-xs" style="color: var(--text-muted);">{skill.description}</td>
                          <td class="text-center px-4 py-2.5">
                            <code class="text-[11px] px-1.5 py-0.5 rounded" style="background-color: var(--bg-tertiary); color: var(--accent); font-family: monospace;">/{skill.id}</code>
                          </td>
                          <td class="text-center px-2 py-2.5">
                            <button class="p-1 rounded hover:bg-red-500/10" style="color: var(--text-muted);" onclick={() => deleteSkill(skill.id)} title="删除">
                              <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                            </button>
                          </td>
                        </tr>
                      {/each}
                    </tbody>
                  </table>
                {:else}
                  <div class="text-center py-12 text-xs" style="color: var(--text-muted);">
                    <div class="text-3xl mb-3">🧩</div>
                    <div>还没有安装扩展技能</div>
                    <div class="mt-1">点击上方按钮创建或从 URL 安装</div>
                  </div>
                {/if}
              </div>
            </div>

            {#if showCreateSkillDialog}
              <div class="fixed inset-0 z-[60] flex items-center justify-center" style="background-color: rgba(0,0,0,0.5);" onclick={(e) => { if (e.target === e.currentTarget) showCreateSkillDialog = false }}>
                <div class="rounded-lg shadow-xl p-6 overflow-y-auto" style="width: 520px; max-height: 85vh; background-color: var(--bg-primary); border: 1px solid var(--border);">
                  <h3 class="text-sm font-medium mb-4" style="color: var(--text-primary);">创建新 Skill</h3>
                  <div class="space-y-3">
                    <div>
                      <label class="block text-xs mb-1" style="color: var(--text-secondary);">ID (英文，如 my-code-review)</label>
                      <input class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" bind:value={newSkillId} placeholder="my-skill">
                    </div>
                    <div>
                      <label class="block text-xs mb-1" style="color: var(--text-secondary);">名称</label>
                      <input class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" bind:value={newSkillName} placeholder="我的技能">
                    </div>
                    <div>
                      <label class="block text-xs mb-1" style="color: var(--text-secondary);">图标 (emoji)</label>
                      <input class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" bind:value={newSkillIcon} placeholder="🔍">
                    </div>
                    <div>
                      <label class="block text-xs mb-1" style="color: var(--text-secondary);">描述</label>
                      <input class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" bind:value={newSkillDesc} placeholder="这个技能用来做什么...">
                    </div>
                    <div>
                      <label class="block text-xs mb-1" style="color: var(--text-secondary);">提示词模板 (支持 {code}, {file}, {input} 等变量)</label>
                      <textarea class="w-full px-3 py-1.5 rounded border text-sm" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border); min-height: 120px;" bind:value={newSkillPrompt} placeholder="你是一个代码审查专家，请对以下代码进行审查..."></textarea>
                    </div>
                  </div>
                  <div class="flex justify-end gap-2 mt-4">
                    <button class="px-4 py-1.5 rounded text-sm" style="background-color: var(--border); color: var(--text-primary);" onclick={() => showCreateSkillDialog = false}>取消</button>
                    <button class="px-4 py-1.5 rounded text-sm font-medium" style="background-color: var(--accent); color: #fff;" onclick={createSkill}>创建</button>
                  </div>
                </div>
              </div>
            {/if}

            {#if showImportSkillDialog}
              <div class="fixed inset-0 z-[60] flex items-center justify-center" style="background-color: rgba(0,0,0,0.5);" onclick={(e) => { if (e.target === e.currentTarget) showImportSkillDialog = false }}>
                <div class="rounded-lg shadow-xl p-6" style="width: 440px; background-color: var(--bg-primary); border: 1px solid var(--border);">
                  <h3 class="text-sm font-medium mb-2" style="color: var(--text-primary);">从 URL 安装 Skill</h3>
                  <p class="text-xs mb-3" style="color: var(--text-muted);">粘贴一个 SKILL.md 文件的原始链接，自动下载并安装。</p>
                  <input class="w-full px-3 py-1.5 rounded border text-sm mb-3" style="background-color: var(--bg-secondary); color: var(--text-primary); border-color: var(--border);" bind:value={importSkillUrl} placeholder="https://example.com/skills/my-skill/SKILL.md">
                  {#if importSkillMsg}
                    <div class="text-xs mb-3 px-3 py-2 rounded" style="background-color: {importSkillOk ? '#2ea04315' : '#e8112315'}; color: {importSkillOk ? '#2ea043' : '#e81123'}; border: 1px solid {importSkillOk ? '#2ea04333' : '#e8112333'};">{importSkillMsg}</div>
                  {/if}
                  <div class="flex justify-end gap-2">
                    <button class="px-4 py-1.5 rounded text-sm" style="background-color: var(--border); color: var(--text-primary);" onclick={() => { showImportSkillDialog = false; importSkillMsg = ''; importSkillUrl = '' }}>取消</button>
                    <button class="px-4 py-1.5 rounded text-sm font-medium" style="background-color: var(--accent); color: #fff;" onclick={installSkillFromUrl} disabled={importSkillLoading}>
                      {importSkillLoading ? '安装中...' : '安装'}
                    </button>
                  </div>
                </div>
              </div>
            {/if}

          {:else if activeTab === 'mcp'}
            <MCPManager />
          {:else if activeTab === 'tokenUsage'}
            <TokenUsagePanel />
          {:else if activeTab === 'about'}
            <div class="space-y-8">
              <div class="flex flex-col items-center pt-6">
                <svg xmlns="http://www.w3.org/2000/svg" class="w-16 h-16 mb-4" style="color: var(--accent)" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
                </svg>
                <h2 class="text-2xl font-bold" style="color: var(--accent);">StarCore</h2>
                <p class="text-sm mt-2" style="color: var(--text-secondary);">{$t('app.description')}</p>
              </div>

              <div class="rounded-lg p-5 space-y-4" style="background-color: var(--bg-secondary); border: 1px solid var(--border);">
                <div class="flex justify-between items-center">
                  <span class="text-sm" style="color: var(--text-secondary);">{$t('settings.about.version')}</span>
                  <span class="text-sm font-mono" style="color: var(--text-primary);">{$t('app.version')}</span>
                </div>
                <div class="flex justify-between items-center">
                  <span class="text-sm" style="color: var(--text-secondary);">{$t('settings.about.license')}</span>
                  <span class="text-sm" style="color: var(--text-primary);">{$t('app.license')}</span>
                </div>
                <div class="flex justify-between items-center">
                  <span class="text-sm" style="color: var(--text-secondary);">{$t('settings.about.builtWith')}</span>
                  <span class="text-sm font-mono" style="color: var(--text-primary);">Go + Svelte 5 + Wails</span>
                </div>
              </div>

              <div class="flex justify-center">
                <button
                  class="px-6 py-2.5 rounded-lg text-sm font-medium transition-colors"
                  style="background-color: var(--accent); color: #ffffff;"
                  onclick={checkUpdate}
                  disabled={aboutChecking}
                >
                  {#if aboutChecking}
                    {$t('settings.about.checking')}
                  {:else if aboutUpdateMsg === 'upToDate'}
                    {$t('settings.about.upToDate')} âœ“
                  {:else}
                    {$t('settings.about.checkUpdate')}
                  {/if}
                </button>
              </div>
            </div>
          {/if}
        </div>
        
        <div class="flex items-center justify-end gap-3 px-6 py-4 border-t" style="border-color: var(--border);">
          <button 
            class="px-4 py-2 rounded text-sm transition-colors"
            style="background-color: var(--border); color: var(--text-primary);"
            onclick={() => settingsVisible.set(false)}
          >
            {$t('settings.cancel')}
          </button>
          <button 
            class="px-4 py-2 rounded text-sm transition-colors"
            style="background-color: var(--accent); color: #ffffff;"
            onclick={() => { saveSettings(); settingsVisible.set(false); }}
          >
            {$t('settings.save')}
          </button>
        </div>
    </div>
    </div>
  </div>
{/if}
