<script>
  import { onMount } from 'svelte'
  import { fade, fly } from 'svelte/transition'
  import { aiPanelVisible, aiPanelWidth } from '../stores/ui.js'
  import AIPanelHeader from './AIPanelHeader.svelte'
  import ContextPreview from './ContextPreview.svelte'
  import DiffViewer from './DiffViewer.svelte'
  import { pendingDiff, diffVisible, applyDiff, dismissDiff, showDiffForFile } from '../stores/diffPreview.js'
   import { messages, isGenerating, sendMessage, addMessage, clearMessages, thinkingContent, contextFiles, contextCode, stopGenerating, toolCalls, approveToolCall, rejectToolCall, selectedCode, activeFileContent, detectTaskType, classifyError, retryLastMessage, pendingAsk, persistMessages, loopExhausted, continueLoop } from '../stores/ai.js'
   import { get } from 'svelte/store'
  import { skills, executeSkill, isSkillExecuting, skillResult, executingSkillId, clearSkillResult, loadSkills } from '../stores/skill.js'
  import { activeProviderId, activeModelId, allAvailableModels, builtinProviders, loadModels } from '../stores/provider.js'
   import { activeAgentId, agents, loadAgents } from '../stores/agent.js'
   import { activeConversationId } from '../stores/memory.js'
  import { aiMode } from '../stores/ai.js'
  import { Marked } from 'marked'
  import hljs from 'highlight.js'
 import { masterMode } from '../stores/masterMode.js'

 import { currentProject, fileTree, activeFile } from '../stores/app.js'
import { activeFileDiagnostics } from '../stores/diagnostics.js'
 import { t } from '../stores/i18n.js'

 let inputValue = ''
 /** @type {HTMLDivElement} */ let messagesContainer
 let isDragging = false
 let showSkillHint = false
 let showFilePicker = false
 let filePickerQuery = ''

 let showAgentDropdown = false
 let showModelDropdown = false
 /** @type {Record<string, boolean>} */ let toolExpanded = {}
 let showModeDropdown = false
 let dropdownLeft = 0
 let dropdownBottom = 0
 let dropdownWidth = 300
 /** @type {HTMLElement|null} */ let inputAreaEl = null
 /** @type {HTMLTextAreaElement|null} */ let textareaEl = null
 let focusedSkillIndex = 0
 let focusedFileIndex = 0
 /** @type {{id: string, name: string, icon: string}|null} */ let currentSkill = null
 let showDone = false
 /** @type {ReturnType<typeof setTimeout>|null} */ let doneTimeout = null

 function updateDropdownPos() {
   const el = inputAreaEl || document.querySelector('.ai-panel-input')
   if (!el) {
     dropdownLeft = 200
     dropdownBottom = 120
     dropdownWidth = 400
     return
   }
   const r = el.getBoundingClientRect()
   dropdownLeft = Math.max(8, r.left)
   dropdownBottom = window.innerHeight - r.top + 4
   dropdownWidth = Math.max(240, r.width)
 }

 /** @type {Map<number, boolean>} */ let thinkingVisibleMap = new Map()

 $: diagnostics = $activeFileDiagnostics?.map(d => d.message) || []

 // Mode can be freely switched by user; auto-detect happens in sendMessage()

 // Update dropdown position whenever shown - delay for DOM render
 $: if (showSkillHint || showFilePicker) {
   setTimeout(() => updateDropdownPos(), 30)
 }
 // Scroll focused dropdown item into view
 $: if (focusedSkillIndex >= 0 || focusedFileIndex >= 0) {
   const items = document.querySelectorAll('.skill-dropdown-menu .dropdown-item, .file-dropdown-menu .dropdown-item')
   const idx = showSkillHint ? focusedSkillIndex : focusedFileIndex
   if (items[idx]) {
     setTimeout(() => items[idx].scrollIntoView({ block: 'nearest' }), 40)
   }
 }

 $: activeAgent = $agents.find(a => a.id === $activeAgentId) || $agents[0] || { name: 'AI', icon: '⚡' }
 $: activeModel = allModels.find(m => m.id === $activeModelId)
 $: displayProviderName = activeModel?.providerName || builtinProviders.find(p => p.id === activeModel?.providerId)?.name || activeModel?.providerId || ''
 $: allModels = $allAvailableModels.filter(m => m.enabled !== false)

 /** @param {string} userInput */
function buildSkillContext(userInput) {
  return {
    selectedCode: $selectedCode || $activeFileContent || '',
    filePath: $activeFile || '',
    fileContent: $activeFileContent || $selectedCode || '',
    diagnostics: [],
    language: $activeFile ? $activeFile.split('.').pop() || '' : '',
    projectPath: $currentProject || '',
    userInput: userInput || '',
  }
}

 /** @param {{id: string}} agent */
function selectAgent(agent) { activeAgentId.set(agent.id); showAgentDropdown = false }
/** @param {{id: string, providerId?: string}} model */
function selectModel(model) { activeModelId.set(model.id); if (model.providerId && model.providerId !== $activeProviderId) { activeProviderId.set(model.providerId); loadModels() } showModelDropdown = false }
/** @param {string} mode */
function setMode(mode) { aiMode.set(mode); showModeDropdown = false }
/** @param {MouseEvent} e */
function closeDropdowns(e) { const target = /** @type {HTMLElement|null} */ (e.target); if (target && !target.closest('.dropdown-trigger')) { showAgentDropdown = false; showModelDropdown = false; showModeDropdown = false } }

 onMount(() => {
    loadAgents()
    loadSkills()
    window.addEventListener('skill-trigger', /** @type {EventListener} */ (handleSkillTrigger))
    window.addEventListener('apply-code', /** @type {EventListener} */ (handleApplyCode))
    return () => {
      window.removeEventListener('skill-trigger', /** @type {EventListener} */ (handleSkillTrigger))
      window.removeEventListener('apply-code', /** @type {EventListener} */ (handleApplyCode))
    }
  })

  /** @param {CustomEvent} e */
  function handleApplyCode(e) {
    const code = e.detail?.code
    const filePath = $activeFile || ''
    if (!code || !filePath) return
    showDiffForFile(filePath, code)
  }

 /** @param {CustomEvent} e */
 function handleSkillTrigger(e) {
   const skill = e.detail
   if (!skill) return
   const ctx = buildSkillContext('')
   const provId = $activeProviderId || ''
   const model = $activeModelId || ''
   executeSkill(skill.id, ctx, provId, model)
 }

 /** @param {MouseEvent} e */
 function startResize(e) {
   isDragging = true
   const startX = e.clientX
   const startWidth = $aiPanelWidth
   document.addEventListener('mousemove', onResize)
   document.addEventListener('mouseup', stopResize)

   /** @param {MouseEvent} e */
   function onResize(e) {
     const delta = startX - e.clientX
     const maxWidth = Math.min(700, window.innerWidth * 0.5)
    const newWidth = Math.max(360, Math.min(maxWidth, startWidth + delta))
     aiPanelWidth.set(newWidth)
   }

   function stopResize() {
     isDragging = false
     document.removeEventListener('mousemove', onResize)
     document.removeEventListener('mouseup', stopResize)
   }
 }

 function handleSend() {
   if ($isGenerating) {
     stopGenerating()
     return
   }

   // Execute selected skill if active — user input goes TO the skill, not direct chat
   if (currentSkill) {
     const userText = inputValue.trim()
     if (!userText) {
       textareaEl?.focus()
       return
     }
     const skillId = currentSkill.id
     const ctx = buildSkillContext(userText)
     addMessage('user', userText)
     executeSkill(skillId, ctx, $activeProviderId || '', $activeModelId || '')
     inputValue = ''
     contextFiles.set([])
     textareaEl?.focus()
     return
   }

   // Also support typing /skill-name directly
   const skillMatch = inputValue.trim().match(/^\/(\S+)(?:\s+(.*))?/)
   if (skillMatch) {
     const skillId = skillMatch[1]
     const skill = $skills.find((/** @type {{ id: string }} */ s) => s.id === skillId)
     if (skill) {
       const userText = (skillMatch[2] || '').trim()
       inputValue = ''
       const ctx = buildSkillContext(userText)
       executeSkill(skill.id, ctx, $activeProviderId || '', $activeModelId || '')
       textareaEl?.focus()
       return
     }
   }

   if (!inputValue.trim() && !currentSkill) return

   // In Master mode, auto-detect best agent for the task
   if ($masterMode) {
     const task = detectTaskType(inputValue.trim(), $activeFile || '')
     if (task.agentId && task.agentId !== $activeAgentId) {
       activeAgentId.set(task.agentId)
       const agent = $agents.find(a => a.id === task.agentId)
       if (agent) {
         addMessage('system', `${agent.icon} 自动调用 ${agent.name}`)
       }
     }
   }

   const files = $contextFiles.slice()
   sendMessage(inputValue.trim(), files)
   inputValue = ''
   contextFiles.set([])
   textareaEl?.focus()
 }

 /** @param {KeyboardEvent} e */
 function handleKeyDown(e) {
   // Check if dropdowns are visible in the DOM (more reliable than state variable)
   const skillDropdown = document.querySelector('.skill-dropdown-menu')

   // Skill dropdown navigation
   if (skillDropdown && showSkillHint) {
     const query = inputValue.slice(1).toLowerCase()
     const entrySkills = $skills.filter(s => s.category !== 'external' || s.id === 'using-superpowers')
     const matched = query === '' ? entrySkills : $skills.filter(s => s.id.includes(query) || s.name.includes(query))
     if (e.key === 'ArrowDown') {
       e.preventDefault()
       focusedSkillIndex = Math.min(focusedSkillIndex + 1, matched.length - 1)
       return
     }
     if (e.key === 'ArrowUp') {
       e.preventDefault()
       focusedSkillIndex = Math.max(focusedSkillIndex - 1, 0)
       return
     }
     if (e.key === 'Enter' && matched[focusedSkillIndex]) {
       e.preventDefault()
       const sk = matched[focusedSkillIndex]
       currentSkill = { id: sk.id, name: sk.name, icon: sk.icon }
       inputValue = ''
       showSkillHint = false
       focusedSkillIndex = 0
       setTimeout(() => { const target = /** @type {HTMLElement} */ (e.target); target?.focus(); }, 50)
       return
     }
     if (e.key === 'Escape') {
       showSkillHint = false
       focusedSkillIndex = 0
       return
     }
   }
   // File picker navigation
   if (showFilePicker) {
     if (e.key === 'ArrowDown') { e.preventDefault(); focusedFileIndex = Math.min(focusedFileIndex + 1, filteredFiles.length - 1); return }
     if (e.key === 'ArrowUp') { e.preventDefault(); focusedFileIndex = Math.max(focusedFileIndex - 1, 0); return }
     if (e.key === 'Enter' && filteredFiles[focusedFileIndex]) {
       e.preventDefault()
       selectFile(filteredFiles[focusedFileIndex])
       focusedFileIndex = 0
       return
     }
     if (e.key === 'Escape') { showFilePicker = false; focusedFileIndex = 0; return }
   }
   // Normal Enter to send
   if (e.key === 'Enter' && !e.shiftKey) {
     e.preventDefault()
     handleSend()
   }
 }

 /** @param {KeyboardEvent} e */
 function handleInputKeyup(e) {
   if (inputValue.startsWith('/')) {
     if (!showSkillHint) focusedSkillIndex = 0  // Only reset index when first opening
     showSkillHint = true
     updateDropdownPos()
   } else {
     showSkillHint = false
   }
   if (e.key === '@' || (inputValue.endsWith('@') && e.key !== 'Backspace')) {
     showFilePicker = true
     focusedFileIndex = 0
     filePickerQuery = ''
     updateDropdownPos()
   }
   if (e.key === 'Escape') {
     showFilePicker = false
     showSkillHint = false
   }
 }

 /** @param {string} code */
 function insertCode(code) {
   window.dispatchEvent(new CustomEvent('insert-code', { detail: { code } }))
 }

 /** @param {MouseEvent} e */
 function handleMessageClick(e) {
   const btn = /** @type {HTMLElement|null} */ (/** @type {HTMLElement} */ (e.target).closest('.insert-btn'))
   if (!btn) return
   const code = btn.dataset.code
   if (code !== undefined) {
     insertCode(code)
   }
   const applyBtn = /** @type {HTMLElement|null} */ (/** @type {HTMLElement} */ (e.target).closest('.apply-btn'))
   if (applyBtn) {
     const applyCode = applyBtn.dataset.code
     if (applyCode !== undefined) {
       window.dispatchEvent(new CustomEvent('apply-code', { detail: { code: applyCode } }))
     }
   }
   const runBtn = /** @type {HTMLElement|null} */ (/** @type {HTMLElement} */ (e.target).closest('.run-btn'))
   if (runBtn) {
     const runCode = runBtn.dataset.code
     if (runCode !== undefined) {
       window.dispatchEvent(new CustomEvent('run-in-terminal', { detail: { code: runCode } }))
     }
   }
 }

 /** @param {string} content */
  const marked = new Marked({
    renderer: {
      code({ text, lang }) {
        const language = lang && hljs.getLanguage(lang) ? lang : 'plaintext'
        const highlighted = hljs.highlight(text, { language }).value
        const dataCode = text.replace(/&/g, '&amp;').replace(/"/g, '&quot;')
        return `<div class="code-block">
          <div class="code-header">
            <span class="code-lang">${language}</span>
            <div class="code-actions">
              <button class="code-action-btn copy-btn" onclick="navigator.clipboard.writeText(this.closest('.code-block').querySelector('code').textContent)">Copy</button>
              <button class="code-action-btn apply-btn" data-code="${dataCode}">Apply</button>
              <button class="code-action-btn run-btn" data-code="${dataCode}">Run</button>
              <button class="code-action-btn insert-btn" data-code="${dataCode}">Insert</button>
            </div>
          </div>
          <pre><code class="hljs language-${language}">${highlighted}</code></pre>
        </div>`
      }
    }
  })

  function formatMessage(content) {
    try {
      return marked.parse(content || '')
    } catch {
      return escapeHtml(content || '')
    }
  }

 /** @param {string} text */
 function escapeHtml(text) {
   const div = document.createElement('div')
   div.textContent = text
   return div.innerHTML
 }

 /** @param {number} index */
 function removeContextFile(index) {
   contextFiles.update(files => files.filter((_, i) => i !== index))
 }

 function removeContextCode() {
   contextCode.set('')
 }

 /** @param {{name: string, path: string, isDir: boolean, children: any[]}[]} tree */
 function flattenFileTree(tree) {
   /** @type {string[]} */ let result = []
   for (const node of tree) {
     if (!node.isDir) {
       result.push(node.path)
     }
     if (node.children && node.children.length > 0) {
       result = result.concat(flattenFileTree(node.children))
     }
   }
   return result
 }

 /** @param {string} filePath */
 function selectFile(filePath) {
   contextFiles.update(files => {
     if (files.includes(filePath)) return files
     return [...files, filePath]
   })
   showFilePicker = false
   filePickerQuery = ''
   inputValue = inputValue.replace(/@$/, '')
 }

 /** @param {number} msgIdx */
 function copyMessage(msgIdx) {
   const msg = $messages[msgIdx]
   if (msg) navigator.clipboard.writeText(msg.content)
 }

  /** @param {number} msgIdx */
  async function deleteMessage(msgIdx) {
    const convId = get(activeConversationId)
    if (convId) {
      const msgId = `${convId}-${msgIdx}`
      try {
        if (window.backend?.DeleteMessage) {
          await window.backend.DeleteMessage(msgId)
        }
      } catch (e) {
        console.error('Failed to delete message from backend:', e)
      }
      // Also clean localStorage
      try {
        const key = `starcore-messages-${convId}`
        const saved = JSON.parse(localStorage.getItem(key) || '[]')
        const updated = saved.filter((/** @type {{id: string}} */ m) => m.id !== msgId)
        localStorage.setItem(key, JSON.stringify(updated))
      } catch {}
    }
    messages.update(msgs => msgs.filter((_, idx) => idx !== msgIdx))
  }

  /** @param {number} msgIdx */
  async function regenerateMessage(msgIdx) {
    const userMsg = $messages.slice(0, msgIdx).reverse().find(m => m.role === 'user')
    if (!userMsg) return
    // Persist current messages before truncating so history is preserved
    const { persistMessages } = await import('../stores/ai.js')
    await persistMessages()
    messages.update(msgs => msgs.slice(0, msgIdx))
    sendMessage(userMsg.content)
  }

 /** @param {number} msgIdx */
 function editMessage(msgIdx) {
   const msg = $messages[msgIdx]
   if (!msg || msg.role !== 'user') return
   inputValue = msg.content
   deleteMessage(msgIdx)
 }

 /** @param {number} msgIdx */
 function toggleThinking(msgIdx) {
   thinkingVisibleMap.set(msgIdx, !thinkingVisibleMap.get(msgIdx))
   thinkingVisibleMap = thinkingVisibleMap
 }

 $: allFiles = flattenFileTree($fileTree)
 $: filteredFiles = filePickerQuery
   ? allFiles.filter(f => f.toLowerCase().includes(filePickerQuery.toLowerCase()))
   : allFiles

 $: {
   if (messagesContainer && ($messages.length > 0 || $skillResult)) {
     requestAnimationFrame(() => {
       requestAnimationFrame(() => {
         const el = messagesContainer
         if (!el) return
         el.scrollTop = el.scrollHeight
       })
     })
   }
 }
 // Also scroll when tool calls update
 $: {
   if (messagesContainer && $toolCalls.length > 0) {
     setTimeout(() => {
       const el = messagesContainer
       if (!el) return
       el.scrollTop = el.scrollHeight
     }, 50)
   }
 }

 // Refocus textarea when generation ends (textarea is disabled during generation)
 $: if (!$isGenerating && !$isSkillExecuting && textareaEl) {
   setTimeout(() => textareaEl?.focus(), 50)
 }

 // Show "done" indicator briefly after generation, hide on new activity
 $: if (!$isGenerating && !$isSkillExecuting && $messages.length > 0) {
   if (!showDone) {
     showDone = true
     if (doneTimeout) clearTimeout(doneTimeout)
     doneTimeout = setTimeout(() => showDone = false, 4000)
   }
 }
 $: if ($isGenerating || $isSkillExecuting) {
   showDone = false
   if (doneTimeout) clearTimeout(doneTimeout)
 }
 $: if (inputValue) {
   showDone = false
   if (doneTimeout) clearTimeout(doneTimeout)
 }
</script>

<svelte:window onclick={closeDropdowns} />

{#if $aiPanelVisible}
  <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions, a11y_no_noninteractive_element_interactions -->
  <div
    role="separator"
    aria-orientation="vertical"
    class="ai-resize-handle"
    class:active={isDragging}
    onmousedown={startResize}
  ></div>

  <div class="h-full flex flex-col border-l shrink-0 mode-{$aiMode}" style="width: {$aiPanelWidth}px; max-width: 50vw; background-color: var(--bg-primary); border-color: var(--border); overflow: clip;">
    <AIPanelHeader />

    <ContextPreview
      contextFiles={$contextFiles}
      contextCode={$contextCode}
      {diagnostics}
      onremovefile={removeContextFile}
      onremovecode={removeContextCode}
    />

    <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
    <div
      bind:this={messagesContainer}
      class="flex-1 p-3"
      style="overflow-y: auto; min-height: 0; overscroll-behavior: contain;"
      role="region"
      onclick={handleMessageClick}
    >
      {#if $messages.length === 0}
        <div class="flex items-center justify-center h-full">
          <div class="text-center">
            <div class="w-14 h-14 mx-auto mb-4 rounded-2xl flex items-center justify-center" style="background: linear-gradient(135deg, rgba(0,120,212,0.15), rgba(78,201,176,0.15));">
              <svg xmlns="http://www.w3.org/2000/svg" class="w-7 h-7" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--ai-color);">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
              </svg>
            </div>
            <p class="text-sm font-medium" style="color: var(--text-primary);">{$t('ai.panel.title')}</p>
            <p class="text-xs mt-1" style="color: var(--text-muted);">{$t('ai.panel.subtitle')}</p>
            <div class="mt-5 space-y-2">
              <button class="btn btn-ghost w-full justify-start text-xs" onclick={() => { inputValue = '@'; handleSend(); }}>
                <span class="chip chip-accent">@</span> {$t('ai.panel.refFile').replace('@ ','')}
              </button>
              <button class="btn btn-ghost w-full justify-start text-xs" onclick={() => { inputValue = '/'; handleSend(); }}>
                <span class="chip chip-ai">/</span> {$t('ai.panel.execSkill').replace('/ ','')}
              </button>
            </div>
          </div>
        </div>
      {#if $toolCalls.length > 0}
          {#each $toolCalls as tc}
            {@const fm = tc.fileMeta}
            {@const opLabel = fm ? (fm.operation === 'read' ? 'Read' : fm.operation === 'write' ? 'Write' : fm.operation === 'edit' ? 'Edit' : fm.operation === 'search' ? 'Search' : fm.operation === 'glob' ? 'Glob' : fm.operation === 'exec' ? 'Run' : tc.name) : tc.name}
            {@const fileName = fm?.filePath ? fm.filePath.split('/').pop().split('\\').pop() : ''}
            {@const lineInfo = fm?.summary || (fm?.startLine ? `L${fm.startLine}${fm.endLine && fm.endLine !== fm.startLine ? '-'+fm.endLine : ''}` : '')}
            <div class="panel-card mt-2 text-xs" in:fly={{ y: 8, duration: 200 }}>
              <div class="flex items-center gap-2">
                {#if fm?.operation === 'read'}
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: #64b5f6;"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/></svg>
                {:else if fm?.operation === 'write' || fm?.operation === 'edit'}
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: #ff8c00;"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>
                {:else if fm?.operation === 'search' || fm?.operation === 'glob'}
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: #ab47bc;"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
                {:else if fm?.operation === 'exec'}
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--text-muted);"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/></svg>
                {:else}
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--warning);"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.57 2.572-1.065z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/></svg>
                {/if}
                <span class="font-medium" style="color: {fm?.operation === 'read' ? '#64b5f6' : fm?.operation === 'write' || fm?.operation === 'edit' ? '#ff8c00' : fm?.operation === 'search' || fm?.operation === 'glob' ? '#ab47bc' : 'var(--warning)'};">{opLabel}</span>
                {#if fileName}
                  <span style="color: var(--text-primary);">{fileName}</span>
                {/if}
                {#if lineInfo}
                  <span style="color: var(--text-muted);">{lineInfo}</span>
                {/if}
                {#if tc.status === 'executing'}
                  <span class="animate-pulse-subtle" style="color: var(--text-muted);">...</span>
                {:else if tc.status === 'completed'}
                  <span style="color: var(--success);">✓</span>
                {:else if tc.status === 'error'}
                  <span style="color: var(--error);">✗</span>
                {:else if tc.status === 'pending_approval'}
                  <button class="btn btn-success btn-sm" onclick={() => approveToolCall(tc.id)}>{$t('tool.approve')}</button>
                  <button class="btn btn-danger btn-sm" onclick={() => rejectToolCall(tc.id)}>{$t('tool.reject')}</button>
                {:else if tc.status === 'rejected'}
                  <span style="color: var(--error);">✗</span>
                {/if}
                {#if tc.result && tc.status === 'completed' && !toolExpanded[tc.id]}
                  <button class="ml-auto px-1 rounded text-[10px]" style="color: var(--text-muted); background: var(--bg-primary);" onclick={() => toolExpanded[tc.id] = true}>▸</button>
                {/if}
              </div>
              {#if tc.result && tc.status === 'completed' && toolExpanded[tc.id]}
                <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
                <div class="mt-1 text-xs" style="color: var(--text-muted);">
                  <pre class="p-2 rounded overflow-x-auto text-xs" style="background-color: var(--bg-primary); color: var(--text-primary); max-height: 200px; overflow-y: auto;">{tc.result.slice(0, 5000)}</pre>
                  <button class="mt-1 px-2 py-0.5 rounded text-[10px]" style="color: var(--text-muted); background: var(--bg-primary);" onclick={() => toolExpanded[tc.id] = false}>收起</button>
                </div>
              {/if}
              {#if tc.error}
                <span class="text-xs" style="color: var(--error);">{tc.error.slice(0, 120)}</span>
              {/if}
            </div>
          {/each}
        {/if}

        

        {:else}
        {#each $messages as message, msgIdx}
          <div class="group relative flex gap-3" in:fade={{ duration: 150 }}>
            {#if message.role === 'user'}
              <div class="w-7 h-7 rounded-full flex items-center justify-center shrink-0" style="background-color: var(--accent);">
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: #ffffff;">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                </svg>
              </div>
            {:else}
              <div class="w-7 h-7 rounded-full flex items-center justify-center shrink-0" style="background-color: var(--ai-color);">
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: #ffffff;">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
            {/if}

            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-2 mb-1">
                <span class="text-xs font-medium" style="color: {message.role === 'user' ? 'var(--accent)' : 'var(--ai-color)'};">
                  {message.role === 'user' ? 'You' : 'AI'}
                </span>
                <span class="text-xs" style="color: var(--text-muted);">
                  {new Date(message.timestamp).toLocaleTimeString()}
                </span>
              </div>
              <div class="text-sm leading-relaxed" style="color: var(--text-primary);">
                {#if message.role === 'assistant' && message.content.startsWith('Error:')}
                  {@const errInfo = classifyError(message.content)}
                  <div class="panel-card mt-1" style="border-left: 3px solid var(--error); background-color: rgba(248,81,73,0.08); padding: 8px 12px; border-radius: 6px;">
                    <div class="flex items-center gap-2 mb-1">
                      <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--error);">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z" />
                      </svg>
                      <span class="text-sm font-medium" style="color: var(--error);">{errInfo.title}</span>
                    </div>
                    <p class="text-xs" style="color: var(--text-primary);">{errInfo.message}</p>
                    <div class="flex gap-2 mt-2">
                      {#if errInfo.action === 'retry'}
                        <button class="btn btn-primary btn-sm" onclick={retryLastMessage}>{errInfo.actionLabel}</button>
                      {:else if errInfo.action === 'settings'}
                        <button class="btn btn-primary btn-sm" onclick={() => { if (typeof window !== 'undefined') window.dispatchEvent(new CustomEvent('open-settings')) }}>{errInfo.actionLabel}</button>
                      {:else if errInfo.action === 'new_chat'}
                        <button class="btn btn-primary btn-sm" onclick={() => clearMessages()}>{errInfo.actionLabel}</button>
                      {/if}
                    </div>
                  </div>
                {:else}
                  {@html formatMessage(message.content)}
                {/if}
              </div>


            </div>

            <div class="absolute right-0 top-0 opacity-0 group-hover:opacity-100 flex gap-0.5 transition-opacity">
              <button title="å¤åˆ¶" class="btn btn-ghost btn-icon" onclick={() => copyMessage(msgIdx)}>
                <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
              </button>
              {#if message.role === 'assistant'}
                <button title="é‡æ–°ç”Ÿæˆ" class="btn btn-ghost btn-icon" onclick={() => regenerateMessage(msgIdx)}>
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                </button>
              {/if}
              {#if message.role === 'user'}
                <button title="ç¼–è¾‘" class="btn btn-ghost btn-icon" onclick={() => editMessage(msgIdx)}>
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                  </svg>
                </button>
              {/if}
              <button title="åˆ é™¤" class="btn btn-ghost btn-icon" onclick={() => deleteMessage(msgIdx)}>
                <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>
          </div>
        {/each}

        {#if $isGenerating}
          <div class="flex gap-3" in:fade={{ duration: 200 }}>
            <div class="w-7 h-7 rounded-full flex items-center justify-center shrink-0" style="background-color: var(--ai-color);">
              <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: #ffffff;">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
            </div>
            <div class="flex items-center">
              <span class="text-sm" style="color: var(--text-muted);">{$t('ai.generating')}</span>
            </div>
          </div>
        {/if}

        {@const diff = /** @type {{filePath: string, hunks: any[]}|null} */ ($pendingDiff)}
        {#if $diffVisible && diff}
          {@const diffHunks = diff.hunks}
          {@const diffFilePath = diff.filePath}
          <div class="mt-4" in:fade>
            <div class="flex items-center justify-between mb-2">
              <span class="text-xs font-medium" style="color: #e06c75;">Diff Preview</span>
              <div class="flex gap-1">
                <button class="btn btn-success btn-sm" onclick={() => applyDiff(diffFilePath, diffHunks)}>Apply</button>
                <button class="btn btn-ghost btn-sm" onclick={dismissDiff}>Dismiss</button>
              </div>
            </div>
            <DiffViewer hunks={diffHunks} filePath={diffFilePath} />
          </div>
        {/if}

        {#if $isSkillExecuting || $skillResult}
          <div class="panel-card mt-4" style="border-color: var(--selection);" in:fade={{ duration: 200 }}>
            <div class="flex items-center gap-2 mb-2">
              <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--accent);">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
              </svg>
              <span class="text-xs font-medium" style="color: var(--accent);">Skill: {$executingSkillId || '...'}</span>
              {#if $isSkillExecuting}
                <span class="text-xs animate-pulse-subtle" style="color: var(--text-muted);">{$t('ai.skill.executing')}</span>
              {/if}
            </div>
            {#if $skillResult}
              <div class="text-sm leading-relaxed" style="color: var(--text-primary);">
                {@html formatMessage($skillResult)}
              </div>
              {#if !$isSkillExecuting}
                <div class="flex gap-2 mt-3">
                  <button class="btn btn-primary btn-sm" onclick={() => { navigator.clipboard.writeText($skillResult); }}>
                    Copy
                  </button>
                  <button class="btn btn-secondary btn-sm" onclick={() => { insertCode($skillResult); }}>
                    Insert
                  </button>
                  <button class="btn btn-ghost btn-sm" onclick={clearSkillResult}>
                    Dismiss
                  </button>
                </div>
              {/if}
            {/if}
          </div>
        {/if}
      {/if}
    </div>

    <div class="p-3 border-t" style="border-color: var(--border);">
      <!-- Model/Agent/Mode dropdowns -->
      <div class="flex items-center gap-2 mb-2 flex-wrap">
        <div class="relative">
          <button class="dropdown-trigger flex items-center gap-1.5 px-2 py-1 rounded text-xs transition-colors shrink-0" style="background-color: var(--bg-secondary); color: var(--text-primary); border: 1px solid var(--border);" onclick={(e) => { e.stopPropagation(); showAgentDropdown = !$masterMode && !showAgentDropdown; showModelDropdown = false; showModeDropdown = false }}>
            <span>{activeAgent.icon}</span>
            {#if $masterMode}<span>AUTO</span>{:else}<span style="max-width: 60px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{activeAgent.name}</span>{/if}
            {#if !$masterMode}<svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>{/if}
          </button>
          {#if showAgentDropdown}
            <div class="absolute bottom-full left-0 mb-1 z-50 rounded shadow-lg overflow-y-auto" style="background-color: var(--bg-secondary); border: 1px solid var(--border); min-width: 180px; max-height: 260px;">
              {#each $agents as agent}
                <button class="w-full flex items-center gap-2 px-3 py-2 text-sm transition-colors text-left" style="background-color: {$activeAgentId === agent.id ? '#094771' : 'transparent'}; color: {$activeAgentId === agent.id ? '#ffffff' : 'var(--text-primary)'};" onclick={() => selectAgent(agent)}>
                  <span class="text-base">{agent.icon}</span>
                  <div class="flex-1 min-w-0"><div class="font-medium truncate">{agent.name}</div><div class="text-xs truncate" style="color: var(--text-muted);">{agent.description}</div></div>
                </button>
              {/each}
            </div>
          {/if}
        </div>
        <div class="relative">
          <button class="dropdown-trigger flex items-center gap-1.5 px-2 py-1 rounded text-xs transition-colors shrink-0" style="background-color: var(--bg-secondary); color: var(--text-secondary); border: 1px solid var(--border);" onclick={(e) => { e.stopPropagation(); showModelDropdown = !showModelDropdown; showAgentDropdown = false; showModeDropdown = false }}>
            <span style="max-width: 80px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{activeModel?.name || 'No models'}</span>
            <span style="color: var(--text-muted); font-size: 9px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 50px;">{displayProviderName}</span>
            <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
          </button>
          {#if showModelDropdown}
            <div class="absolute bottom-full right-0 mb-1 z-50 rounded shadow-lg overflow-y-auto" style="background-color: var(--bg-secondary); border: 1px solid var(--border); min-width: 180px; max-height: 260px;">
              {#if allModels.length === 0}
                <div class="px-3 py-3 text-xs" style="color: var(--text-muted); text-align: center;">No models configured. Add a provider in Settings.</div>
              {:else}
                {#each [...new Set(allModels.map(m => m.providerId))] as pid}
                  {@const providerName = builtinProviders.find(p => p.id === pid)?.name || allModels.find(m => m.providerId === pid && m.providerName)?.providerName || pid}
                  {@const providerModelList = allModels.filter(m => m.providerId === pid)}
                  <div class="px-3 py-1.5 text-xs font-semibold" style="color: var(--text-muted);">{providerName}</div>
                  {#each providerModelList as model}
                    <button class="w-full flex items-center gap-2 px-3 py-1.5 text-sm transition-colors text-left" style="background-color: {$activeModelId === model.id ? '#094771' : 'transparent'}; color: {$activeModelId === model.id ? '#ffffff' : 'var(--text-primary)'};" onclick={() => selectModel(model)}>
                      <span class="truncate">{model.name}</span>
                      {#if model.supportsThinking}<span class="text-[10px] px-1 rounded" style="background-color: var(--border); color: var(--text-secondary);">thinking</span>{/if}
                    </button>
                  {/each}
                {/each}
              {/if}
            </div>
          {/if}
        </div>
        <div class="relative">
          <button class="dropdown-trigger flex items-center gap-1.5 px-2 py-1 rounded text-xs transition-colors shrink-0" style="background-color: {$aiMode !== 'chat' ? '#094771' : 'var(--bg-secondary)'}; color: {$aiMode !== 'chat' ? '#ffffff' : 'var(--text-secondary)'}; border: 1px solid var(--border);" onclick={(e) => { e.stopPropagation(); showModeDropdown = !showModeDropdown; showAgentDropdown = false; showModelDropdown = false }}>
            {#if $aiMode === 'plan'}<svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" /></svg>{:else if $aiMode === 'build'}<svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" /></svg>{:else}<svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" /></svg>{/if}
            <span style="max-width: 40px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{$masterMode ? ($aiMode === 'plan' ? 'Plan' : 'Build') : 'Chat'}</span>
            {#if $masterMode}
              <span class="text-xs px-1 rounded" style="background-color: #ffcc0030; color: #ffcc00; font-size: 8px;">MASTER</span>
            {/if}
            <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" /></svg>
          </button>
          {#if showModeDropdown}
            <div class="absolute bottom-full right-0 mb-1 z-50 rounded shadow-lg overflow-y-auto" style="background-color: var(--bg-secondary); border: 1px solid var(--border); min-width: 150px;">
              {#if !$masterMode}
              <button class="w-full flex items-center gap-3 px-3 py-2 text-sm transition-colors text-left" style="background-color: {$aiMode === 'chat' ? '#094771' : 'transparent'}; color: {$aiMode === 'chat' ? '#ffffff' : 'var(--text-primary)'};" onclick={() => setMode('chat')}>
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" /></svg>
                <div><div class="font-medium">Chat</div><div class="text-xs" style="color: var(--text-muted);">对话编程</div></div>
              </button>
              {:else}
              <button class="w-full flex items-center gap-3 px-3 py-2 text-sm transition-colors text-left" style="background-color: {$aiMode === 'plan' ? '#094771' : 'transparent'}; color: {$aiMode === 'plan' ? '#ffffff' : 'var(--text-primary)'};" onclick={() => setMode('plan')}>
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" /></svg>
                <div><div class="font-medium">Plan</div><div class="text-xs" style="color: var(--text-muted);">分析规划</div></div>
              </button>
              <button class="w-full flex items-center gap-3 px-3 py-2 text-sm transition-colors text-left" style="background-color: {$aiMode === 'build' ? '#094771' : 'transparent'}; color: {$aiMode === 'build' ? '#ffffff' : 'var(--text-primary)'};" onclick={() => setMode('build')}>
                <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" /></svg>
                <div><div class="font-medium">Build</div><div class="text-xs" style="color: var(--text-muted);">自动编程</div></div>
              </button>
              {/if}
            </div>
          {/if}
        </div>
      </div>

      {#if currentSkill}
        <div class="flex items-center gap-1 px-3 py-1">
          <span style="color: #2ea043; font-size: 12px;">⚡ {currentSkill.icon} {currentSkill.name}</span>
          <button class="text-xs px-1 rounded hover:bg-white/10" style="color: var(--text-muted);" onclick={() => currentSkill = null} title="Cancel skill">✕</button>
        </div>
      {/if}

      <!-- Status indicator -->
      {#if $isGenerating || $isSkillExecuting}
        {@const activeTC = $toolCalls.find(tc => tc.status === 'pending_approval')}
        {@const lastDone = $toolCalls.filter(tc => tc.status === 'completed' || tc.status === 'error').slice(-1)[0]}
        <div class="flex items-center gap-2 px-3 py-1 text-xs status-line">
          <span class="status-spinner"></span>
          {#if activeTC}
            <span class="truncate">⏳ 等待批准: {activeTC.name}</span>
          {:else if lastDone}
            <span class="truncate">{lastDone.name} 完成</span>
          {:else}
            <span>AI 正在处理</span>
          {/if}
          <span class="loading-dots"><span>.</span><span>.</span><span>.</span></span>
        </div>
      {:else if showDone}
        <div class="flex items-center gap-1 px-3 py-1 text-xs" style="color: #2ea043;" transition:fade={{ duration: 300 }}>
          <span>✓</span>
          <span>已完成 — 可以继续输入</span>
        </div>
      {/if}

      <!-- Current model indicator -->
      <div class="px-1 text-[10px]" style="color: var(--text-muted);">
        {#if activeModel}
          <span class="font-medium" style="color: var(--text-secondary);">{displayProviderName}</span>
          <span class="mx-1">·</span>
          <span>{activeModel.name || activeModel.id}</span>
        {:else}
          <span>未选择模型</span>
        {/if}
      </div>

      {#if $pendingAsk}
        <div class="mx-3 p-3 rounded-lg border" style="background-color: var(--bg-secondary); border-color: var(--accent);" transition:fly={{ y: 8, duration: 200 }}>
          <div class="flex items-start gap-2">
            <span class="text-sm mt-0.5">🤔</span>
            <div class="flex-1 space-y-2">
              <p class="text-sm font-medium" style="color: var(--text-primary);">{$pendingAsk.question}</p>
              {#if $pendingAsk.options.length > 0}
                <div class="flex flex-wrap gap-1.5">
                  {#each $pendingAsk.options as opt, i}
                    <button
                      class="px-3 py-1 rounded text-xs font-medium transition-colors"
                      style="background-color: var(--accent); color: #ffffff;"
                      onclick={async () => {
                        const req = $pendingAsk
                        pendingAsk.set(null)
                        if (window.backend?.RespondToAsk) {
                          await window.backend.RespondToAsk({ id: req.id, answer: opt })
                        }
                      }}
                    >{opt}</button>
                  {/each}
                </div>
              {/if}
              <div class="flex gap-2">
                <input
                  type="text"
                  class="flex-1 px-2 py-1 rounded text-xs border"
                  style="background-color: var(--bg-primary); color: var(--text-primary); border-color: var(--border);"
                  placeholder="输入你的回复..."
                  id="ask-user-input"
                />
                <button
                  class="px-3 py-1 rounded text-xs font-medium"
                  style="background-color: var(--accent); color: #ffffff;"
                  onclick={async () => {
                    const input = /** @type {HTMLInputElement} */ (document.getElementById('ask-user-input'))
                    const answer = input?.value?.trim()
                    if (!answer) return
                    const req = $pendingAsk
                    pendingAsk.set(null)
                    if (window.backend?.RespondToAsk) {
                      await window.backend.RespondToAsk({ id: req.id, answer })
                    }
                  }}
                >回复</button>
              </div>
            </div>
          </div>
        </div>
      {/if}

      {#if $loopExhausted}
        <div class="mx-3 p-3 rounded-lg border" style="background-color: var(--bg-secondary); border-color: var(--warning, #f59e0b);" transition:fly={{ y: 8, duration: 200 }}>
          <div class="flex items-start gap-2">
            <span class="text-sm mt-0.5">⚠️</span>
            <div class="flex-1 space-y-2">
              <p class="text-sm font-medium" style="color: var(--text-primary);">
                任务已达到最大执行轮次（{$loopExhausted.maxLoops}轮，{$loopExhausted.mode}模式）
              </p>
              <div class="flex flex-wrap gap-2">
                <button
                  class="px-3 py-1 rounded text-xs font-medium transition-colors"
                  style="background-color: var(--accent); color: #ffffff;"
                  onclick={() => continueLoop(10)}
                >继续 +10轮</button>
                <button
                  class="px-3 py-1 rounded text-xs font-medium transition-colors"
                  style="background-color: var(--accent); color: #ffffff;"
                  onclick={() => continueLoop(20)}
                >继续 +20轮</button>
                <button
                  class="px-3 py-1 rounded text-xs font-medium transition-colors"
                  style="background-color: var(--bg-primary); color: var(--text-secondary); border: 1px solid var(--border);"
                  onclick={() => loopExhausted.set(null)}
                >在新对话中继续</button>
              </div>
            </div>
          </div>
        </div>
      {/if}

      <div class="flex gap-2 relative ai-panel-input" bind:this={inputAreaEl} style="overflow: visible;">
        <div class="flex-1 flex items-end rounded-md overflow-hidden" style="border: 1px solid var(--border); background-color: var(--bg-secondary); transition: border-color 120ms ease;">
          <textarea
            bind:this={textareaEl}
            bind:value={inputValue}
            placeholder={$isGenerating ? 'AI 思考中...' : currentSkill ? '描述你的需求...' : $t('ai.panel.placeholder')}
            class="flex-1 px-3 py-2 text-sm resize-none border-none outline-none placeholder-green"
            style="background-color: transparent; color: var(--text-primary); min-height: 36px; max-height: 120px;"
            rows="1"
            disabled={$isGenerating}
            onkeydown={handleKeyDown}
            onkeyup={handleInputKeyup}
            onfocus={(e) => { const parent = e.currentTarget.parentElement; if (parent) parent.style.borderColor = 'var(--border-focus)'; }}
            onblur={(e) => { const parent = e.currentTarget.parentElement; if (parent) parent.style.borderColor = 'var(--border)'; }}
          ></textarea>
        </div>

        {#if showFilePicker}
          <div class="dropdown-menu fixed overflow-y-auto file-dropdown-menu" style="max-height: 220px; min-height: 40px; min-width: 220px; z-index: 9999; left: {dropdownLeft}px; bottom: {dropdownBottom}px; width: {dropdownWidth}px;" transition:fly={{ y: 8, duration: 150 }}>
            <input
              type="text"
              placeholder="搜索文件..."
              class="input-field input-field-sm"
              bind:value={filePickerQuery}
              onkeydown={(e) => { if (e.key === 'Escape') { showFilePicker = false; } }}
            />
            <div class="mt-1">
              {#each filteredFiles.slice(0, 20) as filePath, i}
                <button
                  class="dropdown-item"
                  style="background-color: {i === focusedFileIndex ? 'var(--bg-hover)' : 'transparent'};"
                  onclick={() => selectFile(filePath)}
                >
                  <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--info);">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                  </svg>
                  <span class="truncate text-xs">{filePath.split(/[\\/]/).pop()}</span>
                  <span class="truncate text-xs ml-auto" style="color: var(--text-muted); max-width: 150px;">{filePath}</span>
                </button>
              {/each}
              {#if filteredFiles.length === 0}
                <div class="px-2 py-1.5 text-xs" style="color: var(--text-muted);">未找到文件</div>
              {/if}
            </div>
          </div>
        {/if}

        {#if showSkillHint}
          {@const query = inputValue.slice(1).toLowerCase()}
          {@const entrySkills = $skills.filter(s => s.category !== 'external' || s.id === 'using-superpowers')}
          {@const matched = query === '' ? entrySkills : $skills.filter(s => s.id.includes(query) || s.name.includes(query))}
          <div class="dropdown-menu fixed overflow-y-auto skill-dropdown-menu" style="max-height: 220px; min-height: 40px; min-width: 220px; z-index: 9999; left: {dropdownLeft}px; bottom: {dropdownBottom}px; width: {dropdownWidth}px;" transition:fly={{ y: 8, duration: 150 }}>
            {#if matched.length > 0}
              {#each matched as sk, i}
                <button
                  class="dropdown-item"
                  style="background-color: {i === focusedSkillIndex ? 'var(--bg-hover)' : 'transparent'};"
                  onclick={() => { currentSkill = { id: sk.id, name: sk.name, icon: sk.icon }; inputValue = ''; showSkillHint = false; focusedSkillIndex = 0; setTimeout(() => { const el = document.querySelector('.ai-panel-input textarea'); if (el) /** @type {HTMLElement} */ (el).focus(); }, 50) }}
                >
                  <span>{sk.icon}</span>
                  <span class="font-medium">{sk.name}</span>
                  <span style="color: var(--text-muted);">/{sk.id}</span>
                </button>
              {/each}
            {:else}
              <div class="px-3 py-2 text-xs" style="color: var(--text-muted);">
                {query ? 'No matching skills' : 'Type to filter skills...'}
              </div>
            {/if}
          </div>
        {/if}

        <button
          class={$isGenerating ? 'btn btn-danger' : (inputValue.trim() ? 'btn btn-primary' : 'btn btn-secondary')}
          onclick={() => { if ($isGenerating) { stopGenerating() } else { handleSend() } }}
          disabled={!inputValue.trim() && !$isGenerating}
        >
          {#if $isGenerating}
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10a1 1 0 011-1h4a1 1 0 011 1v4a1 1 0 01-1 1h-4a1 1 0 01-1-1v-4z" />
            </svg>
          {:else}
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
            </svg>
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
.status-line {
  color: #4ade80;
}

.status-spinner {
  display: inline-block;
  width: 14px;
  height: 14px;
  border: 2px solid rgba(74, 222, 128, 0.25);
  border-top-color: #4ade80;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
  flex-shrink: 0;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.loading-dots span {
  animation: bounce 1.2s ease-in-out infinite;
}
.loading-dots span:nth-child(2) { animation-delay: 0.15s; }
.loading-dots span:nth-child(3) { animation-delay: 0.3s; }

@keyframes bounce {
  0%, 60%, 100% { opacity: 0.2; }
  30% { opacity: 1; }
}

.placeholder-green::placeholder {
  color: #4ade80;
  opacity: 0.6;
}
.ai-resize-handle {
  position: relative;
  width: 5px;
  cursor: col-resize;
  background-color: transparent;
  flex-shrink: 0;
  transition: background-color 120ms ease;
  z-index: 10;
}

.ai-resize-handle::before {
  content: '';
  position: absolute;
  inset: 0 -4px;
  z-index: -1;
}

.ai-resize-handle:hover,
.ai-resize-handle.active {
  background-color: var(--accent);
}

:global(.code-block) {
  background-color: var(--bg-primary);
  border: 1px solid var(--border);
  border-radius: 8px;
  margin: 8px 0;
  overflow: hidden;
}

:global(.code-header) {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background-color: var(--bg-secondary);
  border-bottom: 1px solid var(--border);
}

:global(.code-lang) {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

:global(.code-actions) {
  display: flex;
  gap: 4px;
}

:global(.code-action-btn) {
  padding: 3px 10px;
  border: none;
  border-radius: 4px;
  font-size: 11px;
  cursor: pointer;
  transition: all 120ms cubic-bezier(0.4, 0, 0.2, 1);
  outline: none;
}

:global(.code-action-btn:focus-visible) {
  box-shadow: var(--ring);
}

:global(.copy-btn) {
  background-color: var(--bg-tertiary);
  color: var(--text-primary);
}
:global(.copy-btn:hover) {
  background-color: var(--bg-hover);
}

:global(.insert-btn) {
  background-color: var(--accent);
  color: #ffffff;
}
:global(.insert-btn:hover) {
  background-color: var(--accent-hover);
}

:global(.apply-btn) {
  background-color: var(--ai-color);
  color: var(--bg-primary);
}
:global(.apply-btn:hover) {
  filter: brightness(1.1);
}

:global(.run-btn) {
  background-color: var(--warning);
  color: var(--bg-primary);
}
:global(.run-btn:hover) {
  filter: brightness(1.1);
}

:global(.code-block pre) {
  margin: 0;
  padding: 12px;
  overflow-x: auto;
}

:global(.code-block code) {
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.5;
  color: var(--text-primary);
}

:global(code) {
  background-color: var(--bg-secondary);
  padding: 2px 6px;
  border-radius: 3px;
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--warning);
}

/* Hide all code action buttons in auto modes (Build/Plan) — AI handles everything */
:global(.mode-build .code-actions),
:global(.mode-plan .code-actions) {
  display: none;
}
</style>
