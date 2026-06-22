<script>
  import {
    testRunning,
    testSuites,
    testError,
    testSummary,
    runTests,
    clearTestResults,
  } from "../stores/testRunner.js";
  import { t } from "../stores/i18n.js";

  let expandedSuites = $state(new Set());

  function toggleSuite(name) {
    const next = new Set(expandedSuites);
    if (next.has(name)) {
      next.delete(name);
    } else {
      next.add(name);
    }
    expandedSuites = next;
  }

  function statusIcon(status) {
    switch (status) {
      case "passed": return "\u2713";
      case "failed": return "\u2717";
      case "skipped": return "\u25CB";
      default: return "\u25CF";
    }
  }

  function statusClass(status) {
    return `status-${status}`;
  }
</script>

<div class="test-runner">
  <div class="test-toolbar">
    <button
      class="run-btn"
      class:running={$testRunning}
      onclick={() => runTests()}
      disabled={$testRunning}
    >
      {#if $testRunning}
        <svg class="spin" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M8 1.5a6.5 6.5 0 0 1 6.5 6.5" />
        </svg>
      {:else}
        <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M4 2l10 6-10 6z" />
        </svg>
      {/if}
      <span>{$testRunning ? $t('testRunner.running') : $t('testRunner.runAll')}</span>
    </button>

    {#if $testSuites.length > 0}
      <div class="summary">
        <span class="summary-total">{$testSummary.total} {$t('testRunner.tests')}</span>
        {#if $testSummary.passed > 0}
          <span class="summary-passed">{$testSummary.passed} {$t('testRunner.passed')}</span>
        {/if}
        {#if $testSummary.failed > 0}
          <span class="summary-failed">{$testSummary.failed} {$t('testRunner.failed')}</span>
        {/if}
        {#if $testSummary.skipped > 0}
          <span class="summary-skipped">{$testSummary.skipped} {$t('testRunner.skipped')}</span>
        {/if}
      </div>
    {/if}

    <button class="clear-btn" onclick={clearTestResults} title={$t('testRunner.clear')}>
      <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
        <path d="M2 2l12 12M14 2L2 14" />
      </svg>
    </button>
  </div>

  {#if $testError}
    <div class="test-error">{$testError}</div>
  {/if}

  <div class="test-results">
    {#if $testSuites.length === 0 && !$testRunning && !$testError}
      <div class="empty-state">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.2" class="empty-icon">
          <circle cx="12" cy="12" r="9" />
          <path d="M8 12l3 3 5-5" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <span>{$t('testRunner.noResults')}</span>
      </div>
    {/if}

    {#each $testSuites as suite (suite.name)}
      <div class="test-suite">
        <button class="suite-header" onclick={() => toggleSuite(suite.name)}>
          <svg class="chevron" class:expanded={expandedSuites.has(suite.name)} viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5">
            <path d="M6 4l4 4-4 4" />
          </svg>
          {#if suite.failed > 0}
            <span class="suite-icon suite-failed">\u2717</span>
          {:else}
            <span class="suite-icon suite-passed">\u2713</span>
          {/if}
          <span class="suite-name">{suite.name}</span>
          <span class="suite-stats">
            {suite.passed}/{suite.total}
            {#if suite.failed > 0}<span class="fail-count"> ({suite.failed} {$t('testRunner.failed')})</span>{/if}
          </span>
          <span class="suite-duration">{suite.duration}</span>
        </button>

        {#if expandedSuites.has(suite.name) && suite.testCases.length > 0}
          <div class="test-cases">
            {#each suite.testCases as tc}
              <div class="test-case {statusClass(tc.status)}">
                <span class="case-icon">{statusIcon(tc.status)}</span>
                <span class="case-name">{tc.name}</span>
                {#if tc.duration}
                  <span class="case-duration">{tc.duration}</span>
                {/if}
                {#if tc.output}
                  <pre class="case-output">{tc.output}</pre>
                {/if}
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/each}
  </div>
</div>

<style>
  .test-runner {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
  }

  .test-toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    border-bottom: 1px solid var(--bg-tertiary);
    flex-shrink: 0;
  }

  .run-btn {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 3px 10px;
    font-size: 11px;
    color: var(--text-primary);
    background: var(--accent);
    border: none;
    border-radius: 3px;
    cursor: pointer;
    transition: opacity 0.15s;
  }

  .run-btn:hover { opacity: 0.85; }
  .run-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .run-btn svg { width: 12px; height: 12px; }

  .spin { animation: spin 1s linear infinite; }
  @keyframes spin { to { transform: rotate(360deg); } }

  .summary {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11px;
    color: var(--text-muted);
  }

  .summary-passed { color: #4ec9b0; }
  .summary-failed { color: #f14c4c; }
  .summary-skipped { color: #cca700; }

  .clear-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    margin-left: auto;
    color: var(--text-muted);
    background: transparent;
    border: none;
    border-radius: 3px;
    cursor: pointer;
  }
  .clear-btn:hover { background: var(--bg-active); color: var(--text-primary); }
  .clear-btn svg { width: 12px; height: 12px; }

  .test-error {
    padding: 6px 10px;
    font-size: 12px;
    color: #f14c4c;
    background: rgba(241, 76, 76, 0.08);
    border-bottom: 1px solid var(--bg-tertiary);
  }

  .test-results {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
  }

  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--text-muted);
    font-size: 12px;
    gap: 8px;
  }

  .empty-icon { width: 28px; height: 28px; opacity: 0.4; }

  .test-suite {
    border-bottom: 1px solid var(--border);
  }

  .suite-header {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 5px 10px;
    font-size: 12px;
    color: var(--text-primary);
    background: transparent;
    border: none;
    cursor: pointer;
    text-align: left;
    transition: background-color 0.1s;
  }

  .suite-header:hover { background: var(--bg-hover); }

  .chevron {
    width: 12px;
    height: 12px;
    flex-shrink: 0;
    transition: transform 0.15s;
  }
  .chevron.expanded { transform: rotate(90deg); }

  .suite-icon { flex-shrink: 0; width: 14px; text-align: center; }
  .suite-passed { color: #4ec9b0; }
  .suite-failed { color: #f14c4c; }

  .suite-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: "Consolas", "Cascadia Code", monospace;
  }

  .suite-stats { color: var(--text-muted); font-size: 11px; flex-shrink: 0; }
  .fail-count { color: #f14c4c; }
  .suite-duration { color: var(--text-muted); font-size: 11px; flex-shrink: 0; margin-left: 8px; }

  .test-cases {
    padding-left: 28px;
    background: rgba(0, 0, 0, 0.15);
  }

  .test-case {
    display: flex;
    align-items: flex-start;
    gap: 6px;
    padding: 3px 10px;
    font-size: 12px;
    border-bottom: 1px solid var(--border);
    flex-wrap: wrap;
  }

  .case-icon { flex-shrink: 0; width: 14px; text-align: center; margin-top: 1px; }
  .status-passed .case-icon { color: #4ec9b0; }
  .status-failed .case-icon { color: #f14c4c; }
  .status-skipped .case-icon { color: #cca700; }

  .case-name {
    font-family: "Consolas", "Cascadia Code", monospace;
    color: var(--text-primary);
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .case-duration {
    color: var(--text-muted);
    font-size: 11px;
    flex-shrink: 0;
  }

  .case-output {
    width: 100%;
    margin: 2px 0 4px;
    padding: 4px 8px;
    font-size: 11px;
    font-family: "Consolas", "Cascadia Code", monospace;
    color: var(--text-secondary);
    background: rgba(0, 0, 0, 0.2);
    border-radius: 3px;
    max-height: 120px;
    overflow-y: auto;
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>