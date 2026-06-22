import { writable, get } from 'svelte/store'

/** @typedef {{ id: string, projectPath: string, agentId: string, model: string, providerId: string, title: string, summary: string, createdAt: string, updatedAt: string, messageCount: number }} Conversation */
/** @typedef {{ id: string, conversationId: string, seq: number, role: string, content: string, thinking: string, tokensIn: number, tokensOut: number, createdAt: string }} ConversationMessage */
/** @typedef {{ id: string, projectPath: string, category: string, key: string, value: string, source: string, updatedAt: string }} Knowledge */

export const conversations = writable(/** @type {Conversation[]} */ ([]))
export const activeConversationId = writable(/** @type {string|null} */ (null))
export const activeMessages = writable(/** @type {ConversationMessage[]} */ ([]))
export const knowledge = writable(/** @type {Knowledge[]} */ ([]))
export const isLoadingHistory = writable(false)

/**
 * @param {string} projectPath
 */
const CONVERSATIONS_KEY = 'starcore-conversations'

export async function loadConversations(projectPath) {
  isLoadingHistory.set(true)
  let list = []
  try {
    if (window.backend?.GetConversations) {
      list = await window.backend.GetConversations(projectPath) || []
    }
  } catch (/** @type {any} */ e) {
    console.error('Failed to load conversations from backend:', e)
  }
  // Fallback to localStorage
  if (list.length === 0) {
    try {
      const saved = localStorage.getItem(CONVERSATIONS_KEY)
      if (saved) list = JSON.parse(saved)
    } catch {}
  }
  conversations.set(list)
  isLoadingHistory.set(false)
}

/**
 * @param {string} conversationId
 */
const MESSAGES_KEY = 'starcore-messages-'

export async function loadMessages(conversationId) {
  let list = []
  try {
    if (window.backend?.GetMessages) {
      list = await window.backend.GetMessages(conversationId) || []
    }
  } catch (/** @type {any} */ e) {
    console.error('Failed to load messages from backend:', e)
  }
  // Fallback to localStorage
  if (list.length === 0) {
    try {
      const saved = localStorage.getItem(MESSAGES_KEY + conversationId)
      if (saved) list = JSON.parse(saved)
    } catch {}
  }
  activeMessages.set(list)
  activeConversationId.set(conversationId)
}

/**
 * @param {Conversation} conv
 */
export async function saveConversation(conv) {
  // Always save to localStorage as fallback
  try {
    const saved = JSON.parse(localStorage.getItem(CONVERSATIONS_KEY) || '[]')
    const idx = saved.findIndex((/** @type {{ id: string }} */ c) => c.id === conv.id)
    if (idx >= 0) saved[idx] = conv
    else saved.push(conv)
    // Keep last 50 conversations
    if (saved.length > 50) saved.splice(0, saved.length - 50)
    localStorage.setItem(CONVERSATIONS_KEY, JSON.stringify(saved))
  } catch {}
  // Also try backend
  if (window.backend?.SaveConversation) {
    try { await window.backend.SaveConversation(conv) } catch {}
  }
}

/**
 * @param {string} id
 */
export async function deleteConversation(id) {
  try {
    if (window.backend?.DeleteConversation) {
      await window.backend.DeleteConversation(id)
    }
  } catch (/** @type {any} */ e) {
    console.error('Failed to delete conversation from backend:', e)
  }
  // Always clean localStorage even if backend failed
  try {
    localStorage.removeItem(`starcore-messages-${id}`)
    const saved = JSON.parse(localStorage.getItem(CONVERSATIONS_KEY) || '[]')
    const filtered = saved.filter((/** @type {{id: string}} */ c) => c.id !== id)
    localStorage.setItem(CONVERSATIONS_KEY, JSON.stringify(filtered))
  } catch (/** @type {any} */ e) {
    console.error('Failed to clean localStorage:', e)
  }
  conversations.update(list => list.filter(c => c.id !== id))
  const $activeConversationId = get(activeConversationId)
  if ($activeConversationId === id) {
    activeConversationId.set(null)
    activeMessages.set([])
  }
}

/**
 * @param {ConversationMessage} msg
 */
export async function saveMessage(msg) {
  // Always save to localStorage as fallback
  try {
    const key = MESSAGES_KEY + msg.conversationId
    const saved = JSON.parse(localStorage.getItem(key) || '[]')
    const idx = saved.findIndex((/** @type {{ id: string }} */ m) => m.id === msg.id)
    if (idx >= 0) saved[idx] = msg
    else saved.push(msg)
    localStorage.setItem(key, JSON.stringify(saved))
  } catch {}
  // Also try backend
  if (window.backend?.SaveMessage) {
    try { await window.backend.SaveMessage(msg) } catch {}
  }
}

/**
 * @param {string} projectPath
 */
export async function loadKnowledge(projectPath) {
  if (!window.backend?.GetKnowledge) return
  try {
    /** @type {Knowledge[]} */
    const list = await window.backend.GetKnowledge(projectPath)
    knowledge.set(list || [])
  } catch (/** @type {any} */ e) {
    console.error('Failed to load knowledge:', e)
  }
}

/**
 * @param {Knowledge} entry
 */
export async function saveKnowledge(entry) {
  if (!window.backend?.SaveKnowledge) return
  try {
    await window.backend.SaveKnowledge(entry)
  } catch (/** @type {any} */ e) {
    console.error('Failed to save knowledge:', e)
  }
}

/**
 * @param {string} id
 */
export async function deleteKnowledge(id) {
  if (!window.backend?.DeleteKnowledge) return
  try {
    await window.backend.DeleteKnowledge(id)
    knowledge.update(list => list.filter(k => k.id !== id))
  } catch (/** @type {any} */ e) {
    console.error('Failed to delete knowledge:', e)
  }
}

export function clearActiveConversation() {
  activeConversationId.set(null)
  activeMessages.set([])
}

