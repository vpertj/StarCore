<script>
  import { fade, fly } from 'svelte/transition'
  import { traceEvents, traceHeaders, selectedTraceId, showTraceViewer, traceViewerTab, loadTraceEvents, loadTraces } from '../stores/ai.js'
  import { get } from 'svelte/store'
  import { activeConversationId } from '../stores/memory.js'
  import { t } from '../stores/i18n.js'

  let loading = false
  let loadingEvents = false

  /** @param {string} eventType */
  function eventColor(eventType) {
    switch (eventType) {
      case 'loop_start': return '#4fc1ff'
      case 'loop_end': return '#4ec9b0'
      case 'llm_call': return '#c586c0'
      case 'tool_call': return '#ff8c00'
      case 'tool_result': return '#64b5f6'
      case 'nudge': return '#dcdcaa'
      case 'repetition_detected': return '#f44747'
      case 'stagnation_detected': return '#f44747'
      case 'loop_exhausted': return '#f44747'
      case 'loop_auto_continue': return '#dcdcaa'
      case 'dag_start': return '#4fc1ff'
      case 'dag_done': return '#4ec9b0'
      case 'node_start': return '#ff8c00'
      case 'node_done': return '#4ec9b0'
      case 'node_failed': return '#f44747'
      case 'node_skipped': return '#666'
      default: return 'var(--text-muted, #666)'
    }
  }

  /** @param {string|undefined} ts */
  function formatTime(ts) {
    if (!ts) return ''
    try {
      const d = new Date(ts)
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    } catch { return ts }
  }

  /** @param {number} ms */
  function formatDuration(ms) {
    if (ms < 1000) return `${ms}ms`
    const s = Math.round(ms / 1000)
    if (s < 60) return `${s}s`
    return `${Math.floor(s / 60)}m${s % 60}s`
  }

  /** @param {any} header */
  async function selectTrace(header) {
    loadingEvents = true
    selectedTraceId.set(header.id)
    await loadTraceEvents(header.id)
    loadingEvents = false
  }

  async function refreshTraces() {
    loading = true
    const convId = get(activeConversationId) || ''
    await loadTraces(convId, 20)
    loading = false
  }

  function close() {
    showTraceViewer.set(false)
    selectedTraceId.set(null)
  }
</script>

{#if $showTraceViewer}
<div class="fixed inset-0 z-50 flex items-center justify-center" style="background: rgba(0,0,0,0.6);" transition:fade={{ duration: 200 }} onclick={close}>
  <div class="w-[700px] max-w-[90vw] max-h-[80vh] rounded-lg flex flex-col overflow-hidden" style="background: var(--bg-primary); border: 1px solid var(--border);" onclick={(e) => e.stopPropagation()} transition:fly={{ y: 20, duration: 250 }}>
    <!-- Header -->
    <div class="flex items-center gap-3 px-4 py-3 border-b" style="border-color: var(--border);">
      <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" style="color: var(--accent);">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"/>
      </svg>
      <h3 class="text-sm font-medium" style="color: var(--text-primary);">Trace Viewer</h3>

      <!-- Tabs -->
      <div class="flex gap-1 ml-4">
        <button class="px-2 py-0.5 rounded text-[11px]" style="background: {$traceViewerTab === 'current' ? 'var(--accent)' : 'var(--bg-tertiary)'}; color: {$traceViewerTab === 'current' ? 'var(--text-on-accent)' : 'var(--text-secondary)'};" onclick={() => traceViewerTab.set('current')}>Current</button>
        <button class="px-2 py-0.5 rounded text-[11px]" style="background: {$traceViewerTab === 'history' ? 'var(--accent)' : 'var(--bg-tertiary)'}; color: {$traceViewerTab === 'history' ? 'var(--text-on-accent)' : 'var(--text-secondary)'};" onclick={() => traceViewerTab.set('history')}>History</button>
      </div>

      <button class="ml-auto p-1 rounded hover:opacity-80" style="color: var(--text-muted);" onclick={refreshTraces} title="Refresh">
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 {loading ? 'animate-spin' : ''}" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
        </svg>
      </button>
      <button class="p-1 rounded hover:opacity-80" style="color: var(--text-muted);" onclick={close}>
        <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
        </svg>
      </button>
    </div>

    <!-- Body -->
    <div class="flex-1 overflow-y-auto p-4">
      {#if $traceViewerTab === 'current' && $selectedTraceId && $traceEvents.length > 0}
        <!-- Event timeline -->
        <div class="space-y-0">
          {#each $traceEvents as evt, i (evt.id)}
            <div class="flex gap-3 group" in:fly={{ y: 6, duration: 150, delay: Math.min(i * 30, 300) }}>
              <!-- Timeline dot -->
              <div class="flex flex-col items-center shrink-0">
                <div class="w-2 h-2 rounded-full mt-1.5 shrink-0" style="background: {eventColor(evt.type)};"></div>
                {#if i < $traceEvents.length - 1}
                  <div class="flex-1 w-px my-0.5" style="background: var(--border);"></div>
                {/if}
              </div>
              <!-- Content -->
              <div class="flex-1 pb-2 min-w-0">
                <div class="flex items-baseline gap-2">
                  <span class="text-[11px] font-mono" style="color: {eventColor(evt.type)};">{evt.type}</span>
                  <span class="text-[10px]" style="color: var(--text-muted);">{formatTime(evt.timestamp)}</span>
                  {#if evt.loop > 0}
                    <span class="text-[10px]" style="color: var(--text-muted);">loop #{evt.loop}</span>
                  {/if}
                  {#if evt.tool_name}
                    <span class="text-[10px]" style="color: var(--text-secondary);">({evt.tool_name})</span>
                  {/if}
                </div>
                {#if evt.message}
                  <div class="text-xs mt-0.5 truncate" style="color: var(--text-secondary);">{evt.message}</div>
                {/if}
                {#if (evt.token_in || 0) + (evt.token_out || 0) > 0}
                  <div class="text-[10px] mt-0.5" style="color: var(--text-muted);">↑{evt.token_in || 0} ↓{evt.token_out || 0}</div>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      {:else if $traceViewerTab === 'history'}
        <!-- Trace list -->
        {#if $traceHeaders.length === 0}
          <div class="text-center py-8 text-xs" style="color: var(--text-muted);">No traces yet. Start a conversation to generate traces.</div>
        {:else}
          <div class="space-y-2">
            {#each $traceHeaders as header (header.id)}
              <button class="w-full text-left p-3 rounded transition-colors" style="background: {$selectedTraceId === header.id ? 'rgba(79,193,255,0.08)' : 'var(--bg-secondary)'}; border: 1px solid {$selectedTraceId === header.id ? 'var(--accent)' : 'var(--border)'};" onclick={() => selectTrace(header)}>
                <div class="flex items-center gap-2 text-xs">
                  <span class="font-mono" style="color: var(--text-muted);">{header.id.slice(0, 12)}</span>
                  <span style="color: var(--text-secondary);">{header.total_loops} loops</span>
                  <span style="color: var(--text-secondary);">{header.total_tools} tools</span>
                  {#if header.total_errors > 0}
                    <span style="color: var(--error);">{header.total_errors} errors</span>
                  {/if}
                  <span class="ml-auto text-[10px]" style="color: var(--text-muted);">{formatDuration(header.duration_ms)}</span>
                </div>
                <div class="flex items-center gap-3 mt-1 text-[10px]" style="color: var(--text-muted);">
                  <span>↑{header.token_in.toLocaleString()}</span>
                  <span>↓{header.token_out.toLocaleString()}</span>
                  <span>{header.event_count} events</span>
                  <span class="ml-auto">{formatTime(header.start_time)}</span>
                </div>
              </button>
            {/each}
          </div>
        {/if}
      {:else}
        <!-- Current tab placeholder -->
        <div class="text-center py-8 text-xs" style="color: var(--text-muted);">
          {#if loadingEvents}
            Loading events...
          {:else if $traceEvents.length === 0}
            Switch to History tab to browse past traces, or wait for the current generation to populate events.
          {:else}
            <div class="space-y-0">
              {#each $traceEvents as evt, i (evt.id)}
                <div class="flex gap-3" in:fly={{ y: 6, duration: 150, delay: Math.min(i * 30, 300) }}>
                  <div class="flex flex-col items-center shrink-0">
                    <div class="w-2 h-2 rounded-full mt-1.5 shrink-0" style="background: {eventColor(evt.type)};"></div>
                    {#if i < $traceEvents.length - 1}
                      <div class="flex-1 w-px my-0.5" style="background: var(--border);"></div>
                    {/if}
                  </div>
                  <div class="flex-1 pb-2 min-w-0">
                    <div class="flex items-baseline gap-2">
                      <span class="text-[11px] font-mono" style="color: {eventColor(evt.type)};">{evt.type}</span>
                      <span class="text-[10px]" style="color: var(--text-muted);">{formatTime(evt.timestamp)}</span>
                      {#if evt.loop > 0}
                        <span class="text-[10px]" style="color: var(--text-muted);">loop #{evt.loop}</span>
                      {/if}
                    </div>
                    {#if evt.message}
                      <div class="text-xs mt-0.5 truncate" style="color: var(--text-secondary);">{evt.message}</div>
                    {/if}
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <!-- Footer -->
    <div class="flex items-center gap-2 px-4 py-2 border-t" style="border-color: var(--border);">
      <span class="text-[10px]" style="color: var(--text-muted);">{$traceEvents.length} events</span>
      <button class="ml-auto px-2 py-0.5 rounded text-[11px]" style="background: var(--bg-tertiary); color: var(--text-secondary);" onclick={close}>Close</button>
    </div>
  </div>
</div>
{/if}
