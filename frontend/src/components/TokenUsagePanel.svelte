<script>
import { onMount } from 'svelte'
import { tokenStats, isLoadingStats, loadTokenUsage, clearTokenUsage, statsError } from '../stores/tokenUsage.js'

let selectedPeriod = 'month'
let clearing = false

onMount(() => { loadTokenUsage(selectedPeriod) })

/** @param {string} p */
function changePeriod(p) {
    selectedPeriod = p
    loadTokenUsage(p)
}

async function handleClear() {
    if (!confirm('确定要清空所有 Token 用量记录吗？此操作不可撤销。')) return
    clearing = true
    await clearTokenUsage()
    clearing = false
}
</script>

<div class="space-y-4">
  <div class="flex items-center justify-between">
    <h3 class="text-sm font-medium" style="color: var(--text-primary, var(--text-primary));">Token 用量</h3>
    <div class="flex gap-1">
      {#each ['today', 'week', 'month', 'all'] as period}
        <button
          class="px-2 py-0.5 rounded text-xs transition-colors"
          style="background-color: {selectedPeriod === period ? 'var(--selection, #094771)' : 'var(--bg-tertiary, var(--border))'}; color: {selectedPeriod === period ? '#ffffff' : 'var(--text-secondary, var(--text-secondary))'};"
          onclick={() => changePeriod(period)}
        >
          {period === 'today' ? '今天' : period === 'week' ? '本周' : period === 'month' ? '本月' : '全部'}
        </button>
      {/each}
    </div>
  </div>

  {#if $isLoadingStats}
    <div class="text-center py-4 text-sm" style="color: var(--text-secondary, var(--text-secondary));">加载中...</div>
  {:else if $tokenStats}
    <div class="grid grid-cols-2 gap-3">
      <div class="p-3 rounded" style="background-color: var(--bg-secondary, var(--bg-secondary)); border: 1px solid var(--border, var(--border));">
        <div class="text-xs" style="color: var(--text-secondary, var(--text-secondary));">输入 Token</div>
        <div class="text-lg font-medium" style="color: var(--info, #4fc1ff);">{($tokenStats.totalTokensIn || 0).toLocaleString()}</div>
      </div>
      <div class="p-3 rounded" style="background-color: var(--bg-secondary, var(--bg-secondary)); border: 1px solid var(--border, var(--border));">
        <div class="text-xs" style="color: var(--text-secondary, var(--text-secondary));">输出 Token</div>
        <div class="text-lg font-medium" style="color: var(--ai-color, #4ec9b0);">{($tokenStats.totalTokensOut || 0).toLocaleString()}</div>
      </div>
    </div>

    {#if $tokenStats.byProvider && Object.keys($tokenStats.byProvider).length > 0}
      <div class="space-y-2">
        <div class="text-xs font-medium" style="color: var(--text-secondary, var(--text-secondary));">按 Provider</div>
        {#each Object.entries($tokenStats.byProvider) as [provider, usage]}
          <div class="flex items-center gap-3 p-2 rounded" style="background-color: var(--bg-secondary, var(--bg-secondary)); border: 1px solid var(--border, var(--border));">
            <span class="text-sm flex-1" style="color: var(--text-primary, var(--text-primary));">{provider}</span>
            <span class="text-xs" style="color: var(--info, #4fc1ff);">{(usage.tokensIn || 0).toLocaleString()} in</span>
            <span class="text-xs" style="color: var(--ai-color, #4ec9b0);">{(usage.tokensOut || 0).toLocaleString()} out</span>
          </div>
        {/each}
      </div>
    {/if}
  {:else if $statsError}
    <div class="text-center py-4 text-sm" style="color: var(--error, #d73a49);">{$statsError}</div>
  {:else}
    <div class="text-center py-4 text-sm" style="color: var(--text-secondary, var(--text-secondary));">暂无用量数据</div>
  {/if}

  <div class="pt-4 border-t" style="border-color: var(--border, var(--border));">
    <button
      class="w-full px-4 py-2 rounded text-xs transition-colors"
      style="background-color: var(--bg-tertiary, var(--border)); color: var(--text-secondary, var(--text-secondary));"
      onclick={handleClear}
      disabled={clearing}
    >
      {clearing ? '清空中...' : '重置用量数据'}
    </button>
  </div>
</div>
