<script>
import { pendingDiffs, multiDiffVisible, activeDiffIndex, toggleDiffAccepted, acceptAllDiffs, rejectAllDiffs, applyAcceptedDiffs, dismissMultiDiff } from '../stores/diffPreview.js'
import DiffViewer from './DiffViewer.svelte'

let diffs = $derived($pendingDiffs)
let visible = $derived($multiDiffVisible)
let activeIdx = $derived($activeDiffIndex)
let activeDiff = $derived(diffs[activeIdx] || null)

let totalFiles = $derived(diffs.length)
let acceptedFiles = $derived(diffs.filter(d => d.accepted).length)

function selectFile(index) {
  activeDiffIndex.set(index)
}
</script>

{#if visible && diffs.length > 0}
<div class="multi-diff-panel" style="border-top: 1px solid var(--border); background-color: var(--bg-primary);">
  <!-- Header -->
  <div class="flex items-center justify-between px-3 py-2" style="background-color: var(--bg-secondary); border-bottom: 1px solid var(--border);">
    <div class="flex items-center gap-2">
      <span class="text-sm font-medium" style="color: var(--text-primary);">Files Changed</span>
      <span class="text-xs px-1.5 py-0.5 rounded" style="background-color: var(--bg-tertiary); color: var(--text-muted);">
        {acceptedFiles}/{totalFiles} files accepted
      </span>
    </div>
    <div class="flex items-center gap-1.5">
      <button class="diff-btn" style="color: var(--text-secondary);" onclick={rejectAllDiffs}>Reject All</button>
      <button class="diff-btn" style="color: var(--success);" onclick={acceptAllDiffs}>Accept All</button>
      <button class="diff-btn-apply" onclick={applyAcceptedDiffs} disabled={acceptedFiles === 0}>
        Apply {acceptedFiles > 0 ? acceptedFiles : ''} File{acceptedFiles !== 1 ? 's' : ''}
      </button>
      <button class="diff-btn-close" onclick={dismissMultiDiff} title="Dismiss">
        <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
      </button>
    </div>
  </div>

  <!-- File tabs -->
  <div class="flex overflow-x-auto" style="background-color: var(--bg-secondary); border-bottom: 1px solid var(--border);">
    {#each diffs as diff, i}
      {@const fileName = diff.filePath.split(/[\\/]/).pop()}
      <button
        class="file-tab"
        class:active={i === activeIdx}
        class:rejected={!diff.accepted}
        onclick={() => selectFile(i)}
      >
        <span class="file-tab-check" style="color: {diff.accepted ? 'var(--success)' : 'var(--text-muted)'};">
          {diff.accepted ? '●' : '○'}
        </span>
        <span class="truncate">{fileName}</span>
      </button>
    {/each}
  </div>

  <!-- Diff content -->
  <div style="max-height: 50vh; overflow-y: auto;">
    {#if activeDiff}
      <DiffViewer hunks={activeDiff.hunks} filePath={activeDiff.filePath} />
    {/if}
  </div>
</div>
{/if}

<style>
.multi-diff-panel {
  max-height: 60vh;
  display: flex;
  flex-direction: column;
}
.file-tab {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  font-size: 12px;
  font-family: monospace;
  border: none;
  background: transparent;
  cursor: pointer;
  border-bottom: 2px solid transparent;
  color: var(--text-secondary);
  white-space: nowrap;
  transition: all 0.15s;
}
.file-tab:hover {
  background-color: var(--bg-hover);
}
.file-tab.active {
  color: var(--text-primary);
  border-bottom-color: var(--accent);
}
.file-tab.rejected {
  opacity: 0.5;
}
.file-tab-check {
  font-size: 8px;
}
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
.diff-btn-close {
  padding: 3px;
  border: none;
  background: transparent;
  cursor: pointer;
  color: var(--text-muted);
  border-radius: 4px;
  display: flex;
  align-items: center;
}
.diff-btn-close:hover { background-color: var(--bg-hover); color: var(--text-primary); }
</style>
