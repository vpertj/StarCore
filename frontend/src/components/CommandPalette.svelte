<script>
 import { fly, fade } from 'svelte/transition'
 import { commandPaletteOpen } from '../stores/ui.js'

const commands = [
  { id: 'file.open-folder', label: '打开项目文件夹', category: '文件' },
  { id: 'file.save', label: '保存文件', category: '文件', shortcut: 'Ctrl+S' },
  { id: 'view.toggle-sidebar', label: '切换侧边栏', category: '视图', shortcut: 'Ctrl+B' },
  { id: 'view.toggle-ai-panel', label: '切换 AI 面板', category: '视图', shortcut: 'Ctrl+Shift+A' },
  { id: 'view.toggle-terminal', label: '切换终端', category: '视图', shortcut: 'Ctrl+`' },
  { id: 'view.toggle-bottom-panel', label: '切换底部面板', category: '视图' },
  { id: 'ai.new-chat', label: '新建 AI 对话', category: 'AI', shortcut: 'Ctrl+L' },
  { id: 'ai.agent-selector', label: '切换 Agent', category: 'AI', shortcut: 'Ctrl+Shift+M' },
  { id: 'skill.generate-test', label: 'Skill: 生成单元测试', category: 'Skill' },
  { id: 'skill.code-review', label: 'Skill: 代码审查', category: 'Skill' },
  { id: 'skill.refactor', label: 'Skill: 重构建议', category: 'Skill' },
  { id: 'skill.generate-doc', label: 'Skill: 生成文档', category: 'Skill' },
  { id: 'skill.explain-code', label: 'Skill: 解释代码', category: 'Skill' },
  { id: 'skill.fix-bug', label: 'Skill: 修复 Bug', category: 'Skill' },
  { id: 'skill.commit-message', label: 'Skill: 生成 Commit Message', category: 'Skill' },
  { id: 'skill.sql-optimize', label: 'Skill: SQL 优化', category: 'Skill' },
  { id: 'settings.open', label: '打开设置', category: '偏好' },
  { id: 'editor.format', label: '格式化文档', category: '编辑器' },
  { id: 'view.split-editor', label: '分屏编辑', category: '视图', shortcut: 'Ctrl+\\' },
  { id: 'view.close-split', label: '关闭分屏', category: '视图' },
  { id: 'theme.dark', label: '切换深色主题', category: '外观' },
  { id: 'theme.light', label: '切换浅色主题', category: '外观' },
  { id: 'theme.hc', label: '切换高对比度主题', category: '外观' }
]

let searchQuery = ''
let selectedIndex = 0

$: filteredCommands = commands.filter(cmd =>
  cmd.label.toLowerCase().includes(searchQuery.toLowerCase()) ||
  cmd.category.toLowerCase().includes(searchQuery.toLowerCase())
)

$: if (filteredCommands.length > 0 && selectedIndex >= filteredCommands.length) {
  selectedIndex = 0
}

/** @param {KeyboardEvent} e */
function handleKeydown(e) {
  if (e.key === 'Escape') {
    commandPaletteOpen.set(false)
    return
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    selectedIndex = Math.min(selectedIndex + 1, filteredCommands.length - 1)
    return
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    selectedIndex = Math.max(selectedIndex - 1, 0)
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    executeCommand(filteredCommands[selectedIndex])
    return
  }
}

/** @param {{ id: string }} cmd */
function executeCommand(cmd) {
  if (!cmd) return
  commandPaletteOpen.set(false)
  if (cmd.id.startsWith('skill.')) {
    const skillId = cmd.id.replace('skill.', '')
    window.dispatchEvent(new CustomEvent('skill-trigger', { detail: { id: skillId } }))
    return
  }
  if (cmd.id.startsWith('theme.')) {
    const theme = cmd.id.replace('theme.', '')
    import('../stores/theme.js').then(m => m.setTheme(theme))
    return
  }
  switch (cmd.id) {
    case 'view.toggle-sidebar':
      import('../stores/ui.js').then(m => m.toggleSidebar())
      break
    case 'view.toggle-ai-panel':
      import('../stores/ui.js').then(m => m.toggleAIPanel())
      break
    case 'view.toggle-terminal':
    case 'view.toggle-bottom-panel':
      import('../stores/ui.js').then(m => m.toggleBottomPanel())
      break
    case 'settings.open':
      import('../stores/app.js').then(m => m.settingsVisible.update(v => !v))
      break
    case 'view.split-editor':
      import('../stores/app.js').then(m => m.splitEditor())
      break
    case 'view.close-split':
      import('../stores/app.js').then(m => m.closeSplit())
      break
    case 'file.open-folder':
      window.backend?.OpenFolder().then((p) => { if (p) import('../stores/app.js').then(m => m.currentProject.set(p)) })
      break
    case 'editor.format':
      import('../stores/ai.js').then(m => m.sendMessage('/format code'))
      break
  }
}

$: if ($commandPaletteOpen) {
  searchQuery = ''
  selectedIndex = 0
}
</script>

<svelte:window onkeydown={(e) => {
  if (e.ctrlKey && e.shiftKey && e.key === 'P') {
    e.preventDefault()
    commandPaletteOpen.update(v => !v)
  }
}} />

<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions, a11y_no_noninteractive_element_interactions -->
{#if $commandPaletteOpen}
  <div class="dialog-backdrop justify-center" style="padding-top: 15vh; align-items: flex-start;" transition:fade={{ duration: 100 }} onclick={(e) => { if (e.target === e.currentTarget) commandPaletteOpen.set(false); }}>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="dialog-content w-full max-w-lg overflow-hidden" transition:fly={{ y: -16, duration: 150 }} onkeydown={handleKeydown}>
      <div class="flex items-center px-4 border-b" style="border-color: var(--border);">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--text-muted);">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
        </svg>
        <input
          type="text"
          bind:value={searchQuery}
          placeholder="输入命令..."
          class="flex-1 px-3 py-3 text-sm outline-none border-none"
          style="background-color: transparent; color: var(--text-primary);"
        />
      </div>

      <div class="max-h-64 overflow-y-auto py-1">
        {#each filteredCommands as cmd, i}
          <button
            class="dropdown-item justify-between"
            style="background-color: {i === selectedIndex ? 'var(--selection)' : 'transparent'}; color: {i === selectedIndex ? '#ffffff' : 'var(--text-primary)'};"
            onclick={() => executeCommand(cmd)}
            onmouseenter={() => selectedIndex = i}
          >
            <div class="flex items-center gap-2">
              <span class="chip">{cmd.category}</span>
              <span>{cmd.label}</span>
            </div>
            {#if cmd.shortcut}
              <span class="text-xs" style="color: var(--text-muted);">{cmd.shortcut}</span>
            {/if}
          </button>
        {/each}

        {#if filteredCommands.length === 0}
          <div class="px-4 py-6 text-center text-sm" style="color: var(--text-muted);">
            没有找到匹配的命令
          </div>
        {/if}
      </div>
    </div>
  </div>
{/if}
