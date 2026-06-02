import { writable, get } from 'svelte/store'
import { KEYS } from './constants.js'
import { EventsOn, EventsOff, EventsEmit } from '../../wailsjs/runtime/runtime.js'
import { activeProviderId, activeModelId, customModels, resolveModelProvider } from './provider.js'
import { activeAgentId } from './agent.js'
 import { saveMessage, activeConversationId, saveConversation } from './memory.js'
 import { currentProject, activeFile } from './app.js'
 import { addLog } from './output.js'
import { masterMode } from './masterMode.js'

/** @typedef {{ provider: string, apiKey: string, model: string, endpoint: string, temperature: number, maxTokens: number }} AIConfig */
/** @typedef {{ role: string, content: string, timestamp: number }} ChatMessage */

const AI_CONFIG_KEY = KEYS.AI_CONFIG

const defaultConfig = {
  provider: 'openai',
  apiKey: '',
  model: '',  // auto: first available model
  endpoint: 'https://api.openai.com/v1/chat/completions',
  temperature: 0.7,
  maxTokens: 0,  // 0 = use model's full context window (auto-managed)
}

/** @returns {AIConfig} */
function loadConfigFromStorage() {
  try {
    const stored = localStorage.getItem(AI_CONFIG_KEY)
    if (stored) {
      return { ...defaultConfig, ...JSON.parse(stored) }
    }
  } catch (/** @type {any} */ e) {
    console.error('Failed to parse AI config from localStorage:', e)
  }
  return { ...defaultConfig }
}

export const aiConfig = writable(loadConfigFromStorage())
export const messages = writable(/** @type {ChatMessage[]} */ ([]))
export const isGenerating = writable(false)
export const thinkingContent = writable('')
export const contextFiles = writable(/** @type {string[]} */ ([]))
export const contextCode = writable('')
export const activeFileContent = writable('')
export const selectedCode = writable('')
export const aiMode = writable('chat')
export const detectedMode = writable('chat')
export const connectionStatus = writable(/** @type {'ok'|'warning'|'error'} */ ('ok'))
export const toolCalls = writable(/** @type {{id: string, name: string, args: any, status: string, result?: string, error?: string}[]} */ ([]))

/**
 * @param {string|Error} err
 * @returns {{ type: string, title: string, message: string, action: string, actionLabel: string }}
 */
export function classifyError(err) {
  const msg = (typeof err === 'string' ? err : (err && err.message) || String(err)).toLowerCase()

  if (msg.includes('401') || msg.includes('403') || msg.includes('unauthorized') || msg.includes('api key') || msg.includes('api密钥')) {
    return { type: 'auth', title: 'API密钥无效', message: 'AI提供商认证失败，请检查API密钥配置。', action: 'settings', actionLabel: '前往设置' }
  }
  if (msg.includes('429') || msg.includes('rate limit') || msg.includes('too many requests')) {
    return { type: 'rate_limit', title: '请求频率限制', message: 'AI服务请求过于频繁，请稍后重试。', action: 'retry', actionLabel: '稍后重试' }
  }
  if (msg.includes('context_length') || msg.includes('token limit') || msg.includes('上下文超限')) {
    return { type: 'context_limit', title: '对话过长', message: '对话上下文超出模型限制，建议开始新对话。', action: 'new_chat', actionLabel: '开始新对话' }
  }
  if (msg.includes('500') || msg.includes('502') || msg.includes('503') || msg.includes('504') || msg.includes('server error') || msg.includes('服务不可用')) {
    return { type: 'service', title: 'AI服务暂时不可用', message: 'AI提供商服务异常，请稍后重试或切换提供商。', action: 'retry', actionLabel: '重试' }
  }
  if (msg.includes('network') || msg.includes('fetch') || msg.includes('timeout') || msg.includes('connection') || msg.includes('dns') || msg.includes('网络连接失败') || msg.includes('未返回任何响应')) {
    return { type: 'network', title: '网络连接失败', message: '无法连接到AI服务，请检查网络设置。', action: 'retry', actionLabel: '重试' }
  }
  return { type: 'unknown', title: 'AI请求失败', message: msg.slice(0, 200), action: 'retry', actionLabel: '重试' }
}

/** @type {string|null} */ let lastUserMessage = null

export function retryLastMessage() {
  if (lastUserMessage) {
    const msg = lastUserMessage
    lastUserMessage = null
    clearMessages().then(() => sendMessage(msg))
  }
}

/** Heuristically detect intent mode from message content */
function detectMode(content) {
  const lower = content.toLowerCase()
  const planWords = ['plan', 'analyze', 'review', 'design', 'architect', 'outline', 'steps', '分析', '规划', '审查', '检查', '看看', '有什么问题', '设计']
  const buildWords = ['write', 'create', 'implement', 'fix', 'build', 'make', 'add', 'update', 'change', 'refactor', 'modify', 'delete', 'remove', '写', '修复', '改', '加', '创建', '生成', '实现', '构建', '删', '重构', '优化', '帮我', '帮忙', '处理', '修改']
  let scorePlan = 0, scoreBuild = 0
  for (const w of planWords) { if (lower.includes(w)) scorePlan++ }
  for (const w of buildWords) { if (lower.includes(w)) scoreBuild++ }
  // Explicit mode override
  if (lower.startsWith('/plan')) return 'plan'
  if (lower.startsWith('/build')) return 'build'
  if (lower.startsWith('/chat')) return 'chat'
  if (scoreBuild > scorePlan && scoreBuild >= 1) return 'build'
  if (scorePlan > scoreBuild && scorePlan >= 1) return 'plan'
  // In Master mode, default to build for any non-trivial message
  if (lower.length > 10 && scoreBuild === 0 && scorePlan === 0) return 'build'
  return 'chat'
}

/**
 * @param {string} content - User message
 * @param {string} currentFile - Currently active file path
 * @returns {{ agentId: string, skillId: string | null }}
 */
export function detectTaskType(content, currentFile) {
  const lower = content.toLowerCase()
  const ext = (currentFile || '').split('.').pop()?.toLowerCase() || ''
  const fileName = (currentFile || '').toLowerCase()

  // File extension hints
  const frontendExts = new Set(['jsx', 'tsx', 'vue', 'svelte', 'css', 'scss', 'less', 'html', 'htm'])
  const backendExts = new Set(['go', 'rs', 'java', 'py', 'rb', 'php', 'cs', 'c', 'cpp', 'h', 'hpp'])
  const testExts = new Set(['test.js', 'test.ts', 'spec.js', 'spec.ts', '_test.go', 'test.py', 'test.rs'])
  const configExts = new Set(['yaml', 'yml', 'toml', 'dockerfile', 'tf', 'json', 'env', 'ini', 'cfg'])
  const isTestFile = testExts.has(ext) || fileName.includes('.test.') || fileName.includes('_test.') || fileName.includes('.spec.') || fileName.includes('_test.go')

  // Scoring for each agent
  const scores = {
    'frontend-architect': 0,
    'backend-architect': 0,
    'ui-designer': 0,
    'api-test-engineer': 0,
    'performance-expert': 0,
    'devops-engineer': 0,
    'compliance-checker': 0,
    'product-manager': 0,
    'ai-integration-engineer': 0,
  }

  // Frontend keywords
  const fw = ['前端', 'frontend', 'react', 'vue', 'svelte', 'angular', '组件', 'component', '页面', 'page', '路由', 'router', '状态管理', 'state', 'props', 'hook', 'effect', 'jsx', 'tsx', 'dom', 'browser', '浏览器', '响应式', 'responsive', 'css', '样式', 'style', 'ui', '界面', '渲染', 'render', 'html', 'svelte组件', '布局', '事件', 'event', '点击', 'click']
  for (const w of fw) { if (lower.includes(w)) scores['frontend-architect']++ }

  // Backend keywords
  const bw = ['后端', 'backend', 'api', '接口', '服务', 'server', '数据库', 'database', 'sql', 'orm', '中间件', 'middleware', '微服务', 'microservice', 'grpc', 'rest', 'graphql', '并发', 'concurrent', '线程', 'goroutine', 'spring', 'express', 'gin', 'echo', 'fiber', 'go', 'golang', 'rust', 'python', 'java', 'php', 'node', 'app.go', 'main.go', 'handler', '路由', '数据', '存储', '文件', '读取', '写入']
  for (const w of bw) { if (lower.includes(w)) scores['backend-architect']++ }

  const hasContentKeywords = Object.values(scores).some(s => s > 0)

  // File extension hints — only apply when message has content-level technical keywords
  if (hasContentKeywords) {
    if (frontendExts.has(ext)) scores['frontend-architect'] += 2
    if (backendExts.has(ext)) scores['backend-architect'] += 2
  }

  // UI/Design keywords
  const dw = ['设计', 'design', '配色', 'color', '布局', 'layout', '组件库', 'design system', 'figma', 'svg', '图标', 'icon', '动画', 'animation', '间距', '字体', 'font', 'tailwind', '暗色模式', 'dark mode', '主题', 'theme']
  for (const w of dw) { if (lower.includes(w)) scores['ui-designer']++ }

  // Testing keywords
  const tw = ['测试', 'test', '单元测试', 'unit test', '集成测试', 'integration test', 'e2e', 'mock', '覆盖率', 'coverage', '断言', 'assert', 'jest', 'pytest', 'vitest', 'playwright', 'cypress']
  for (const w of tw) { if (lower.includes(w)) scores['api-test-engineer']++ }
  if (isTestFile) scores['api-test-engineer'] += 3

  // Performance keywords
  const pw = ['性能', 'performance', '优化', 'optimize', '慢', 'slow', '瓶颈', 'bottleneck', '内存', 'memory', 'cpu', '缓存', 'cache', '延迟', 'latency', '加载', 'loading', 'profile', 'profiling', '加速', 'speed']
  for (const w of pw) { if (lower.includes(w)) scores['performance-expert']++ }

  // DevOps keywords
  const ow = ['部署', 'deploy', 'docker', 'kubernetes', 'k8s', 'ci/cd', 'ci', 'cd', 'pipeline', '监控', 'monitor', '日志', 'log', '报警', 'alert', '环境', 'environment', '构建', 'build', '发布', 'release', '域名', 'domain', '证书', 'cert', 'nginx']
  for (const w of ow) { if (lower.includes(w)) scores['devops-engineer']++ }
  if (configExts.has(ext)) scores['devops-engineer'] += 1

  // Compliance/security keywords
  const cw = ['审查', 'review', '安全', 'security', '漏洞', 'vulnerability', '审计', 'audit', '合规', 'compliance', '规范', 'lint', 'eslint', '注入', 'injection', 'xss', 'csrf', 'auth', '认证', '授权', 'permission', '加密', 'encrypt']
  for (const w of cw) { if (lower.includes(w)) scores['compliance-checker']++ }

  // Product keywords
  const pmw = ['需求', 'requirement', 'prd', '用户故事', 'user story', '产品', 'product', '原型', 'prototype', '优先级', 'priority', '路线图', 'roadmap', '竞品', 'competitor']
  for (const w of pmw) { if (lower.includes(w)) scores['product-manager']++ }

  // AI keywords
  const aw = ['ai', 'llm', 'prompt', '模型', 'model', '嵌入', 'embedding', 'rag', 'token', 'gpt', 'claude', 'openai', 'anthropic', 'agent', '智能', '流式', 'stream', 'chatgpt']
  for (const w of aw) { if (lower.includes(w)) scores['ai-integration-engineer']++ }

  // Find best agent (max score). Require score >= 2 to switch.
  let bestAgent = 'universal-assistant'
  let bestScore = 0
  for (const [agent, score] of Object.entries(scores)) {
    if (score > bestScore) {
      bestScore = score
      bestAgent = agent
    }
  }
  if (bestScore < 2) {
    bestAgent = 'universal-assistant'
  }

  // Don't switch agents for short non-technical messages (greetings, thanks, small talk)
  if (!hasContentKeywords && lower.length < 15) {
    bestAgent = 'universal-assistant'
  }

  // Default: no auto-skill, let agent decide via skill tool
  return { agentId: bestAgent, skillId: null }
}

/**
 * @param {string} role
 * @param {string} content
 */
export function addMessage(role, content) {
  messages.update(msgs => [...msgs, { role, content, timestamp: Date.now() }])
}

/** @param {string} content */
export function updateLastMessage(content) {
  messages.update(msgs => {
    const last = msgs[msgs.length - 1]
    if (last && last.role === 'assistant') {
      last.content = content
    }
    return msgs
  })
}

export async function clearMessages() {
  // Save current conversation to history before clearing
  await persistMessages()
  messages.set([])
  toolCalls.set([])
  thinkingContent.set('')
  contextFiles.set([])
  contextCode.set('')
  // Generate a fresh conversation ID for the new chat
  activeConversationId.set('conv_' + Date.now() + '_' + Math.random().toString(36).slice(2, 8))
}

/** @param {AIConfig} config */
export function saveAIConfig(config) {
  try {
    localStorage.setItem(AI_CONFIG_KEY, JSON.stringify(config))
  } catch (/** @type {any} */ e) {
    console.error('Failed to save AI config to localStorage:', e)
  }
  aiConfig.set(config)
}

/**
 * @param {string} callId
 */
export async function approveToolCall(callId) {
  toolCalls.update(cs => cs.map(c => c.id === callId ? { ...c, status: 'executing' } : c))
  EventsEmit('tool:approve:' + callId, true)
}

export function rejectToolCall(callId) {
  toolCalls.update(cs => cs.map(c => c.id === callId ? { ...c, status: 'rejected' } : c))
  EventsEmit('tool:approve:' + callId, false)
}

/** @type {(() => void)|null} */ let currentCleanup = null

export function stopGenerating() {
  if (currentCleanup) {
    currentCleanup()
    currentCleanup = null
  }
  isGenerating.set(false)
}

/**
 * @param {string} content
 * @param {string[]} [attachedFiles]
 */
export async function sendMessage(content, attachedFiles) {
  // Auto-generate a conversation ID if none exists (first message of a new chat)
  if (!get(activeConversationId)) {
    activeConversationId.set('conv_' + Date.now() + '_' + Math.random().toString(36).slice(2, 8))
  }

  const config = get(aiConfig)
  const rawProviderId = get(activeProviderId) || config.provider || 'openai'
  const modelCompositeId = get(activeModelId) || config.model || ''
  const agentId = get(activeAgentId) || ''

  const resolved = resolveModelProvider(modelCompositeId, rawProviderId, customModels)
  const providerId = resolved.providerId
  const model = resolved.model

  if (resolved.apiKey || resolved.endpoint) {
    try {
      await window.backend.SetProviderConfig(providerId, {
        id: providerId,
        name: providerId,
        apiKey: resolved.apiKey || '',
        endpoint: resolved.endpoint || '',
        enabled: true,
        isDefault: false,
      })
    } catch (e) {
      console.error('Failed to set provider config:', e)
    }
  }

  thinkingContent.set('')
  addMessage('user', content)
  lastUserMessage = content
  connectionStatus.set('ok')

  // Master mode: always Build (full tool access). Non-Master: always Chat.
  if (get(masterMode)) {
    if (get(aiMode) !== 'build') aiMode.set('build')
  } else {
    if (get(aiMode) !== 'chat') aiMode.set('chat')
  }

  isGenerating.set(true)
  addMessage('assistant', '')

  const chatMessages = get(messages)
    .filter(m => m.role === 'user' || m.role === 'assistant')
    .slice(0, -1)
    .map(m => ({ role: m.role, content: m.content }))

  const $activeFileContent = get(activeFileContent)
  const $selectedCode = get(selectedCode)

  const req = {
    providerId: providerId,
    model: model,
    messages: chatMessages,
    temperature: config.temperature,
    maxTokens: 0,  // 0 = auto: use model's full context window
    stream: true,
    agentId: agentId,
    contextFiles: attachedFiles || [],
    projectPath: get(currentProject) || '',
    activeFile: get(activeFile) || '',
    activeFileContent: $activeFileContent || '',
    selectedCode: $selectedCode || '',
    mode: get(aiMode) || 'chat',
  }

  const hasStreamBackend = typeof window.backend?.AIChatStream === 'function'
  const hasNonStreamBackend = typeof window.backend?.AIChat === 'function'

  try {
    if (hasStreamBackend) {
      const dataEvent = 'ai:stream:data'
      const doneEvent = 'ai:stream:done'
      const errorEvent = 'ai:stream:error'
      const thinkingEvent = 'ai:stream:thinking'
      const toolCallEvent = 'ai:stream:tool_call'
      const toolResultEvent = 'ai:stream:tool_result'
      const summarizedEvent = 'ai:context:summarized'
      let assistantMessage = ''
      let thinkingText = ''
      let pendingChunk = ''
      let rafId = /** @type {number|null} */ (null)
      let firstChunkReceived = false
      const STREAM_TIMEOUT_MS = 120000
      const FIRST_CHUNK_TIMEOUT_MS = 30000
      let /** @type {number|null} */ streamTimeoutId = null
      let /** @type {number|null} */ firstChunkTimeoutId = null

      function resetStreamTimeout() {
        if (streamTimeoutId) clearTimeout(streamTimeoutId)
        streamTimeoutId = setTimeout(() => {
          cleanup()
          updateLastMessage('⚠️ 请求超时：30秒内未收到AI响应。\n\n可能原因：\n1. 网络连接问题\n2. AI服务暂时不可用\n3. API密钥无效\n\n请检查网络或尝试重新发送。')
        }, STREAM_TIMEOUT_MS)
      }

      firstChunkTimeoutId = setTimeout(() => {
        if (!firstChunkReceived) {
          updateLastMessage('⏳ AI响应较慢，正在等待...')
        }
      }, FIRST_CHUNK_TIMEOUT_MS)
      resetStreamTimeout()

      function flushUpdate() {
        if (pendingChunk) {
          assistantMessage += pendingChunk
          pendingChunk = ''
          updateLastMessage(assistantMessage)
        }
        rafId = null
      }

      const offData = EventsOn(dataEvent, (/** @type {string} */ chunk) => {
        if (typeof chunk === 'string') {
          if (!firstChunkReceived) {
            firstChunkReceived = true
            if (firstChunkTimeoutId) { clearTimeout(firstChunkTimeoutId); firstChunkTimeoutId = null }
          }
          resetStreamTimeout()
          pendingChunk += chunk
          if (!rafId) {
            rafId = requestAnimationFrame(flushUpdate)
          }
        }
      })

      const offThinking = EventsOn(thinkingEvent, (/** @type {string} */ chunk) => {
        if (typeof chunk === 'string') {
          thinkingText += chunk
          thinkingContent.set(thinkingText)
          resetStreamTimeout()
        }
      })

      const offDone = EventsOn(doneEvent, () => {
        if (streamTimeoutId) { clearTimeout(streamTimeoutId); streamTimeoutId = null }
        if (firstChunkTimeoutId) { clearTimeout(firstChunkTimeoutId); firstChunkTimeoutId = null }
        if (rafId) {
          cancelAnimationFrame(rafId)
          rafId = null
        }
        flushUpdate()
        connectionStatus.set('ok')
        cleanup()
      })

      const offError = EventsOn(errorEvent, (/** @type {string} */ err) => {
        if (streamTimeoutId) { clearTimeout(streamTimeoutId); streamTimeoutId = null }
        if (firstChunkTimeoutId) { clearTimeout(firstChunkTimeoutId); firstChunkTimeoutId = null }
        if (rafId) {
          cancelAnimationFrame(rafId)
          rafId = null
        }
        const classified = classifyError(err)
        if (classified.type === 'network' || classified.type === 'auth') {
          connectionStatus.set('error')
        } else {
          connectionStatus.set('warning')
        }
        if (assistantMessage === '' && pendingChunk === '') {
          updateLastMessage(`Error: ${err}`)
        } else {
          flushUpdate()
        }
        cleanup()
      })

       const offToolCall = EventsOn(toolCallEvent, (/** @type {any} */ tc) => {
         const id = tc.id || tc.ID || ''
         const name = tc.name || tc.Name || 'tool'
         const args = tc.args || tc.Args || {}
         toolCalls.update(calls => [...calls, {
           id,
           name,
           args,
           status: 'pending_approval',
         }])
         addLog('AI', 'info', `${name}(${JSON.stringify(args).slice(0, 100)})`)
         resetStreamTimeout()
       })

       const offToolResult = EventsOn(toolResultEvent, (/** @type {any} */ tr) => {
         toolCalls.update(calls => calls.map(c => {
           if (c.id === tr.callId || c.id === tr.CallID) {
             const status = tr.error ? 'error' : 'completed'
             if (tr.error) addLog('AI', 'error', `${c.name}: ${tr.error}`)
             return { ...c, status, result: tr.result || tr.Result || '', error: tr.error || tr.Error || '' }
           }
           return c
         }))
         resetStreamTimeout()
       })

      const offSummarized = EventsOn(summarizedEvent, (msg) => {
        addMessage('system', `📝 ${msg}`)
      })

      function cleanup() {
        if (streamTimeoutId) { clearTimeout(streamTimeoutId); streamTimeoutId = null }
        if (firstChunkTimeoutId) { clearTimeout(firstChunkTimeoutId); firstChunkTimeoutId = null }
        offData()
        offThinking()
        offDone()
        offError()
        offToolCall()
        offToolResult()
        offSummarized()
        EventsOff(dataEvent)
        EventsOff(thinkingEvent)
        EventsOff(doneEvent)
        EventsOff(errorEvent)
        EventsOff(toolCallEvent)
        EventsOff(toolResultEvent)
        EventsOff(summarizedEvent)
        isGenerating.set(false)
        currentCleanup = null
        persistMessages()
      }

      currentCleanup = cleanup
      await window.backend.AIChatStream(req)
    } else if (hasNonStreamBackend) {
      const response = await window.backend.AIChat(req)
      updateLastMessage(typeof response === 'string' ? response : JSON.stringify(response))
    } else {
      throw new Error('Backend AI methods not available')
    }
  } catch (/** @type {any} */ err) {
    console.error('AI request failed:', err)
    updateLastMessage(`Error: ${err.message || String(err)}`)
    isGenerating.set(false)
  }
  // isGenerating is set false by cleanup() in stream event handlers above
}

async function persistMessages() {
  try {
    const convId = get(activeConversationId)
    const msgs = get(messages)
    if (!convId || msgs.length === 0) return
    for (let i = 0; i < msgs.length; i++) {
      await saveMessage({
        id: `${convId}-${i}`,
        conversationId: convId,
        seq: i,
        role: msgs[i].role,
        content: msgs[i].content,
        thinking: '',
        tokensIn: 0,
        tokensOut: 0,
        createdAt: new Date(msgs[i].timestamp).toISOString()
      })
    }
    const lastMsg = msgs[msgs.length - 1]
    if (lastMsg) {
      const proj = get(currentProject) || ''
      await saveConversation({
        id: convId,
        projectPath: proj,
        agentId: get(activeAgentId) || '',
        model: get(activeModelId) || '',
        providerId: get(activeProviderId) || '',
        title: msgs[0]?.content?.slice(0, 50) || 'New Chat',
        summary: '',
        createdAt: new Date(msgs[0]?.timestamp || Date.now()).toISOString(),
        updatedAt: new Date(lastMsg.timestamp).toISOString(),
        messageCount: msgs.length
      })
    }
  } catch (/** @type {any} */ e) {
    console.error('Failed to persist messages:', e)
  }
}

