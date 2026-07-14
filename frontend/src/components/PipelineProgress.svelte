<script>
  import { fly } from 'svelte/transition'

  let { plan = null } = $props()

  /** @param {string} status */
  function statusColor(status) {
    switch (status) {
      case 'completed': return 'var(--success, #4ec9b0)'
      case 'running': return 'var(--accent, #4fc1ff)'
      case 'failed': return 'var(--error, #f44747)'
      case 'skipped': return 'var(--text-muted, #666)'
      default: return 'var(--text-muted, #666)'
    }
  }

  /** @param {string} status */
  function statusIcon(status) {
    switch (status) {
      case 'completed': return '✓'
      case 'running': return '●'
      case 'failed': return '✗'
      case 'skipped': return '○'
      default: return '○'
    }
  }

  let percent = $derived(plan && plan.total > 0
    ? Math.round(plan.nodes.filter(n => n.status === 'completed').length / plan.total * 100)
    : 0)
</script>

{#if plan}
<div class="panel-card mt-2 overflow-hidden" in:fly={{ y: 8, duration: 200 }}>
  <!-- Header -->
  <div class="flex items-center gap-2 px-3 py-2 border-b" style="border-color: var(--border); background: var(--bg-secondary);">
    <svg xmlns="http://www.w3.org/2000/svg" class="w-3.5 h-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--accent);">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/>
    </svg>
    <span class="text-xs font-medium" style="color: var(--text-primary);">{plan.name || 'Pipeline'}</span>
    <span class="text-[10px] ml-auto" style="color: var(--text-muted);">{percent}%</span>
  </div>

  <!-- Progress bar -->
  <div class="h-1" style="background: var(--bg-tertiary);">
    <div class="h-full transition-all duration-500" style="width: {percent}%; background: var(--accent);"></div>
  </div>

  <!-- Node list -->
  <div class="px-3 py-2 space-y-1 max-h-48 overflow-y-auto">
    {#each plan.nodes as node (node.id)}
      <div class="flex items-center gap-2 text-xs" in:fly={{ y: 4, duration: 150 }}>
        <span class="shrink-0 w-3 text-center" style="color: {statusColor(node.status)};">{statusIcon(node.status)}</span>
        <span class="truncate" style="color: {node.status === 'running' ? 'var(--text-primary)' : 'var(--text-secondary)'};">{node.label}</span>
        {#if node.status === 'running'}
          <span class="ml-auto animate-pulse-subtle shrink-0" style="color: var(--accent);">working...</span>
        {:else if node.mode}
          <span class="ml-auto shrink-0 text-[10px]" style="color: var(--text-muted);">{node.mode}</span>
        {/if}
      </div>
    {/each}
  </div>
</div>
{/if}
