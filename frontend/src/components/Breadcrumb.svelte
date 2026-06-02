<script>
export let filePath = ''

$: parts = filePath ? filePath.split(/[\\/]/).filter(Boolean) : []
$: fileName = parts.length > 0 ? parts[parts.length - 1] : ''
$: dirParts = parts.slice(0, -1)
</script>

{#if parts.length > 0}
  <div class="flex items-center gap-1 px-3 py-1 text-xs" style="background-color: var(--bg-secondary); color: var(--text-secondary); border-bottom: 1px solid var(--border);">
    {#each dirParts as part, i}
      <span>{part}</span>
      <svg xmlns="http://www.w3.org/2000/svg" class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
      </svg>
    {/each}
    <span style="color: var(--text-primary);">{fileName}</span>
  </div>
{/if}
