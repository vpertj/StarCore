import { writable, derived, get } from 'svelte/store'
import { KEYS } from './constants.js'
import { LoadCustomModels, SaveCustomModels } from '../../wailsjs/go/main/App.js'

/** @typedef {{ id: string, name: string, endpoint?: string, enabled?: boolean, isDefault?: boolean }} Provider */
/** @typedef {{ id: string, name: string, providerId: string, providerName?: string, groupId?: string, supportsThinking?: boolean, enabled?: boolean, isCustom?: boolean, apiKey?: string, modelId?: string, endpoint?: string, maxTokens?: number, contextWindow?: number }} Model */

export const providers = writable(/** @type {Provider[]} */ ([]))

const ACTIVE_PROVIDER_KEY = KEYS.ACTIVE_PROVIDER
const ACTIVE_MODEL_KEY = KEYS.ACTIVE_MODEL

export const activeProviderId = writable(localStorage.getItem(ACTIVE_PROVIDER_KEY) || 'openai')
export const models = writable(/** @type {Model[]} */ ([]))
export const activeModelId = writable(localStorage.getItem(ACTIVE_MODEL_KEY) || '')
export const customModels = writable(/** @type {Model[]} */ ([]))

// Load custom models on startup - try Go backend first, fallback to localStorage
export async function initCustomModels() {
  try {
    const list = await LoadCustomModels()
    if (list && list.length > 0) {
      customModels.set(list)
      return
    }
  } catch {}
  // Fallback to localStorage
  customModels.set(getCustomModels())
}

activeProviderId.subscribe(v => { try { localStorage.setItem(ACTIVE_PROVIDER_KEY, v) } catch {} })
activeModelId.subscribe(v => { try { localStorage.setItem(ACTIVE_MODEL_KEY, v) } catch {} })

const CUSTOM_MODELS_KEY = KEYS.CUSTOM_MODELS
const MODEL_ENABLED_KEY = KEYS.MODEL_ENABLED

function loadModelEnabledMap() {
  try {
    return JSON.parse(localStorage.getItem(MODEL_ENABLED_KEY) || '{}')
  } catch {
    return {}
  }
}

export const modelEnabledMap = writable(/** @type {Record<string, boolean>} */ (loadModelEnabledMap()))

export function isModelEnabled(modelId) {
  const map = get(modelEnabledMap)
  if (modelId in map) return map[modelId]
  return true
}

export function setModelEnabled(modelId, enabled) {
  modelEnabledMap.update(map => {
    map[modelId] = enabled
    localStorage.setItem(MODEL_ENABLED_KEY, JSON.stringify(map))
    return map
  })
}

export const builtinProviders = [
  { id: 'openai', name: 'OpenAI', defaultEndpoint: 'https://api.openai.com/v1/chat/completions' },
  { id: 'anthropic', name: 'Anthropic', defaultEndpoint: 'https://api.anthropic.com/v1/messages' },
  { id: 'deepseek', name: 'DeepSeek', defaultEndpoint: 'https://api.deepseek.com/v1/chat/completions' },
  { id: 'google', name: 'Google Gemini', defaultEndpoint: 'https://generativelanguage.googleapis.com/v1beta' },
  { id: 'xai', name: 'xAI', defaultEndpoint: 'https://api.x.ai/v1/chat/completions' },
  { id: 'kimi', name: 'Kimi (月之暗面)', defaultEndpoint: 'https://api.moonshot.cn/v1/chat/completions' },
  { id: 'volcengine', name: '火山引擎', defaultEndpoint: 'https://ark.cn-beijing.volces.com/api/v3/chat/completions' },
  { id: 'siliconflow', name: '硅基流动', defaultEndpoint: 'https://api.siliconflow.cn/v1/chat/completions' },
  { id: 'ollama', name: 'Ollama (本地)', defaultEndpoint: 'http://localhost:11434' },
  { id: 'custom', name: '自定义 (OpenAI 兼容)', defaultEndpoint: '' },
]

/** @type {Record<string, Array<{ id: string, name: string, supportsThinking?: boolean, maxTokens?: number }>>} */
export const providerModels = /** @type {Record<string, Array<{ id: string, name: string, supportsThinking?: boolean, maxTokens?: number }>>} */ ({
  openai: [],
  anthropic: [],
  deepseek: [],
  google: [],
  xai: [],
  kimi: [],
  volcengine: [],
  siliconflow: [],
  ollama: [],
  custom: [],
})

export function getCustomModels() {
  try {
    return JSON.parse(localStorage.getItem(CUSTOM_MODELS_KEY) || '[]')
  } catch {
    return []
  }
}

export function saveCustomModels(newCustomModels) {
  customModels.set(newCustomModels)
  // Persist to Go backend (file-based, unlimited storage)
  SaveCustomModels(newCustomModels).catch(() => {})
  // Also save to localStorage as fallback (may fail if data > 5MB)
  try {
    localStorage.setItem(CUSTOM_MODELS_KEY, JSON.stringify(newCustomModels))
  } catch (e) {
    // localStorage quota exceeded — Go backend is the primary store, so this is fine
  }
}

export function refreshCustomModels() {
  // Try Go backend first (authoritative source), fallback to localStorage
  LoadCustomModels().then(list => {
    if (list && list.length > 0) {
      customModels.set(list)
    } else {
      customModels.set(getCustomModels())
    }
  }).catch(() => {
    customModels.set(getCustomModels())
  })
}

export const allAvailableModels = derived(
  [models, customModels, modelEnabledMap],
  ([$models, $customModels, $modelEnabledMap]) => {
    const builtin = Object.entries(providerModels).flatMap(([providerId, modelList]) =>
      modelList.map(m => {
        const key = `${providerId}:${m.id}`
        const enabled = key in $modelEnabledMap ? $modelEnabledMap[key] : true
        return { ...m, providerId, isCustom: false, enabled }
      })
    )
    const custom = $customModels
      .map(/** @param {any} m */ m => ({ ...m, isCustom: true }))
    const customIds = new Set(custom.map(/** @param {any} m */ m => m.id))
    const mergedBuiltin = builtin.filter(/** @param {any} m */ m => !customIds.has(m.id))
    return [...mergedBuiltin, ...custom]
  }
)

export async function loadProviders() {
  if (window.backend?.GetProviders) {
    try {
      const list = await window.backend.GetProviders()
      providers.set(list || [])
      const $activeProviderId = get(activeProviderId)
      const found = (list || []).find(p => p.id === $activeProviderId)
      if (!found && list?.length > 0) {
        activeProviderId.set(list[0].id)
      }
    } catch (/** @type {any} */ e) {
      console.error('Failed to load providers:', e)
    }
  }
  refreshCustomModels()
}

export async function loadModels() {
  if (window.backend?.GetModels) {
    const pid = get(activeProviderId)
    if (!pid) return
    try {
      const list = await window.backend.GetModels(pid)
      models.set(list || [])
    } catch (/** @type {any} */ e) {
      console.error('Failed to load models:', e)
      models.set([])
    }
  }
  refreshCustomModels()
}

/**
 * @param {string} providerId
 * @returns {Promise<boolean>}
 */
export async function testProvider(providerId) {
  if (!window.backend?.TestProvider) return false
  try {
    await window.backend.TestProvider(providerId)
    return true
  } catch {
    return false
  }
}

/**
 * @param {string} providerId
 * @param {{ apiKey?: string, endpoint?: string, enabled?: boolean }} config
 */
export async function setProviderConfig(providerId, config) {
  if (window.backend?.SetProviderConfig) {
    await window.backend.SetProviderConfig(providerId, config)
    // Fire-and-forget: reload provider list in background
    // Don't await loadProviders() as GetProviders() calls ListModels()
    // for ALL providers, which can block for a long time
    loadProviders().catch(() => {})
  }
}
/**
 * Resolve a model ID (possibly a composite "provider:model" custom model ID)
 * and return the effective provider ID, model name, and credentials to configure.
 *
 * @param {string} modelId - The model ID or composite custom model ID
 * @param {string} providerId - The currently active provider ID
 * @param {import('svelte/store').Readable<any>} customModelsStore - The customModels store
 * @returns {{ providerId: string, model: string, apiKey?: string, endpoint?: string }}
 */
export function resolveModelProvider(modelId, providerId, customModelsStore) {
  const $customModels = get(customModelsStore)
  const customModel = $customModels.find(/** @param {any} m */ m => m.id === modelId)

  if (customModel) {
    const resolvedProviderId = customModel.groupId || customModel.providerId || providerId
    const resolvedModel = customModel.modelId || modelId
    const apiKey = customModel.apiKey || ''
    let endpoint = customModel.endpoint || ''

    if (resolvedProviderId === 'ollama' && endpoint) {
      endpoint = endpoint.replace(/\/api\/chat\/?$/, '').replace(/\/v1\/chat\/completions\/?$/, '').replace(/\/+$/, '')
    }

    return {
      providerId: resolvedProviderId,
      model: resolvedModel,
      apiKey,
      endpoint,
    }
  }

  return { providerId, model: modelId }
}
