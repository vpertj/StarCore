import { writable } from 'svelte/store'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime.js'
import { customModels, resolveModelProvider } from './provider.js'

/** @typedef {{ id: string, name: string, icon: string, description: string, trigger: string, resultType: string, associatedAgents: string[], category: string }} SkillDef */
/** @typedef {{ selectedCode: string, filePath: string, fileContent: string, diagnostics: string[], language: string, projectPath: string }} SkillContext */

export const skills = writable(/** @type {SkillDef[]} */ ([]))
export const executingSkillId = writable(/** @type {string|null} */ (null))
export const skillResult = writable(/** @type {string} */ (''))
export const isSkillExecuting = writable(false)

export async function loadSkills() {
  if (!window.backend?.GetSkills) return
  try {
    /** @type {SkillDef[]} */
    const list = await window.backend.GetSkills()
    skills.set(list || [])
  } catch (/** @type {any} */ e) {
    console.error('Failed to load skills:', e)
  }
}

/**
 * @param {string} skillId
 * @param {SkillContext} context
 * @param {string} providerId
 * @param {string} model
 */
export async function executeSkill(skillId, context, providerId, model) {
  if (!window.backend?.ExecuteSkill) return
  executingSkillId.set(skillId)
  isSkillExecuting.set(true)
  skillResult.set('')

  // Resolve custom model and auto-configure provider before executing skill
  const resolved = resolveModelProvider(model, providerId, customModels)
  if (resolved.apiKey || resolved.endpoint) {
    try {
      await window.backend.SetProviderConfig(resolved.providerId, {
        id: resolved.providerId,
        name: resolved.providerId,
        apiKey: resolved.apiKey || '',
        endpoint: resolved.endpoint || '',
        enabled: true,
        isDefault: false,
      })
    } catch (e) {
      console.error('Failed to set provider config for skill:', e)
    }
  }

  const startEvent = 'skill:stream:start'
  const dataEvent = 'skill:stream:data'
  const doneEvent = 'skill:stream:done'
  const errorEvent = 'skill:stream:error'

  let result = ''

  const offStart = EventsOn(startEvent, (/** @type {string} */ id) => {
    executingSkillId.set(id)
  })

  const offData = EventsOn(dataEvent, (/** @type {string} */ chunk) => {
    result += chunk
    skillResult.set(result)
  })

  const offDone = EventsOn(doneEvent, () => {
    cleanup()
  })

  const offError = EventsOn(errorEvent, (/** @type {string} */ err) => {
    if (result === '') {
      skillResult.set(`Error: ${err}`)
    }
    cleanup()
  })

  function cleanup() {
    offStart()
    offData()
    offDone()
    offError()
    EventsOff(startEvent)
    EventsOff(dataEvent)
    EventsOff(doneEvent)
    EventsOff(errorEvent)
    isSkillExecuting.set(false)
  }

  try {
    await window.backend.ExecuteSkill(skillId, context, resolved.providerId, resolved.model)
  } catch (/** @type {any} */ e) {
    console.error('Skill execution failed:', e)
    skillResult.set(`Error: ${e.message || String(e)}`)
    cleanup()
  }
}

export function clearSkillResult() {
  skillResult.set('')
  executingSkillId.set(null)
}

/**
 * @template T
 * @param {import('svelte/store').Readable<T>} store
 * @returns {T}
 */
function get(store) {
  /** @type {any} */ let value = undefined
  store.subscribe(v => value = v)()
  return value
}
