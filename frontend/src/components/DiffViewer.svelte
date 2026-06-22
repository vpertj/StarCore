<script>
/** @type {{hunks?: any[], filePath?: string}} */
let { hunks = [], filePath = '' } = $props()

let acceptedHunks = $state(new Set())

function toggleHunk(index) {
  if (acceptedHunks.has(index)) {
    acceptedHunks.delete(index)
  } else {
    acceptedHunks.add(index)
  }
  acceptedHunks = acceptedHunks
}

function acceptAll() {
  for (let i = 0; i < hunks.length; i++) acceptedHunks.add(i)
  acceptedHunks = new Set(acceptedHunks)
}

function rejectAll() {
  acceptedHunks = new Set()
}

async function applyAccepted() {
  const accepted = hunks.filter((_, i) => acceptedHunks.has(i))
  if (accepted.length === 0) return
  if (!window.backend?.ApplyDiff) return
  try {
    await window.backend.ApplyDiff({ filePath, hunks: accepted })
    window.dispatchEvent(new CustomEvent('file-changed', { detail: { path: filePath } }))
  } catch (e) { console.error('Diff apply failed:', e) }
}

let totalAdd = $derived(hunks.reduce((s, h) => s + (h.newLines?.length || 0), 0))
let totalDel = $derived(hunks.reduce((s, h) => s + (h.oldLines?.length || 0), 0))
let acceptedCount = $derived(acceptedHunks.size)
</script>

{#if hunks.length > 0}
<div class="diff-viewer rounded overflow-hidden" style="border: 1px solid var(--border); font-size: 13px;">
  <!-- Header -->
  <div class="flex items-center justify-between px-3 py-2 gap-2" style="background-color: var(--bg-secondary); border-bottom: 1px solid var(--border);">
    <div class="flex items-center gap-2 min-w-0">
      <span class="font-mono text-xs truncate" style="color: var(--text-primary);" title={filePath}>{filePath.split(/[\\/]/).pop()}</span>
    </div>
    <div class="flex items-center gap-1 shrink-0">
      <span class="text-xs px-1.5 py-0.5 rounded" style="background-color: #2ea04322; color: var(--success);">+{totalAdd}</span>
      <span class="text-xs px-1.5 py-0.5 rounded" style="background-color: #d73a4922; color: var(--error);">-{totalDel}</span>
      <span class="text-xs" style="color: var(--text-muted);">{hunks.length} hunks</span>
      {#if acceptedCount > 0}
        <span class="text-xs" style="color: var(--accent);">{acceptedCount} selected</span>
      {/if}
    </div>
  </div>

  <!-- Hunk list -->
  <div style="max-height: 400px; overflow-y: auto;">
    {#each hunks as hunk, i}
      {@const accepted = acceptedHunks.has(i)}
      <div
        class="diff-hunk cursor-pointer"
        style="border-bottom: 1px solid var(--border); background-color: {accepted ? '#2ea04310' : 'transparent'};"
        onclick={() => toggleHunk(i)}
        role="button"
        tabindex="0"
        onkeydown={(e) => { if (e.key === ' ') { e.preventDefault(); toggleHunk(i) } }}
      >
        <!-- Hunk header -->
        <div class="flex items-center gap-2 px-3 py-1.5" style="background-color: var(--bg-tertiary);">
          <div class="w-4 h-4 rounded border flex items-center justify-center shrink-0" style="border-color: var(--border); background-color: {accepted ? 'var(--success)' : 'transparent'};">
            {#if accepted}
              <svg class="w-3 h-3" style="color: var(--text-on-accent);" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="3" d="M5 13l4 4L19 7"/></svg>
            {/if}
          </div>
          <span class="text-xs" style="color: var(--text-muted); font-family: monospace;">
            @@ -{hunk.oldStart},{hunk.oldCount} +{hunk.newStart},{hunk.newCount} @@
          </span>
        </div>

        <!-- Diff lines -->
        <div class="font-mono text-xs leading-relaxed">
          {#each hunk.oldLines as line, j}
            {@const newLine = hunk.newLines[j]}
            <div class="flex">
              <div class="shrink-0 w-10 text-right pr-2 select-none" style="color: var(--text-muted); opacity: 0.5;">{hunk.oldStart + j}</div>
              <div class="flex-1 px-2" style="background-color: #d73a4915; color: var(--error);">
                <span class="select-none mr-2">-</span>{line || ''}
              </div>
              {#if newLine !== undefined}
                <div class="shrink-0 w-10 text-right pr-2 select-none" style="color: var(--text-muted); opacity: 0.5;">{hunk.newStart + j}</div>
                <div class="flex-1 px-2" style="background-color: #2ea04315; color: var(--success);">
                  <span class="select-none mr-2">+</span>{newLine || ''}
                </div>
              {:else}
                <div class="flex-1"></div>
              {/if}
            </div>
          {/each}
          {#each hunk.newLines.slice(hunk.oldLines.length) as line, j}
            <div class="flex">
              <div class="shrink-0 w-10"></div>
              <div class="flex-1"></div>
              <div class="shrink-0 w-10 text-right pr-2 select-none" style="color: var(--text-muted); opacity: 0.5;">{hunk.newStart + hunk.oldLines.length + j}</div>
              <div class="flex-1 px-2" style="background-color: #2ea04315; color: var(--success);">
                <span class="select-none mr-2">+</span>{line || ''}
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/each}
  </div>

  <!-- Footer actions -->
  <div class="flex items-center justify-between px-3 py-2" style="background-color: var(--bg-secondary); border-top: 1px solid var(--border);">
    <span class="text-xs" style="color: var(--text-muted);">
      {acceptedCount}/{hunks.length} hunks selected
    </span>
    <div class="flex items-center gap-1.5">
      <button class="diff-btn" style="color: var(--text-secondary);" onclick={rejectAll}>Reject All</button>
      <button class="diff-btn" style="color: var(--success);" onclick={acceptAll}>Accept All</button>
      <button class="diff-btn-apply" onclick={applyAccepted} disabled={acceptedCount === 0}>
        Apply {acceptedCount > 0 ? acceptedCount : ''}
      </button>
    </div>
  </div>
</div>
{/if}

<style>
.diff-hunk:hover { background-color: var(--bg-hover) !important; }
.diff-btn {
  padding: 3px 10px;
  border-radius: 4px;
  font-size: 12px;
  border: 1px solid var(--border);
  background: transparent;
  cursor: pointer;
  transition: background-color 0.15s;
}
.diff-btn:hover { background-color: var(--bg-hover); }
.diff-btn-apply {
  padding: 3px 12px;
  border-radius: 4px;
  font-size: 12px;
  border: none;
  background-color: var(--accent);
  color: white;
  cursor: pointer;
  font-weight: 500;
}
.diff-btn-apply:disabled { opacity: 0.4; cursor: default; }
.diff-btn-apply:not(:disabled):hover { filter: brightness(1.1); }
</style>
