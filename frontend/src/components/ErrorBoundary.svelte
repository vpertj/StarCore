<script>
  /**
   * ErrorBoundary catches rendering errors in child components.
   * Instead of crashing the entire app, it shows a fallback UI.
   *
   * Usage:
   * <ErrorBoundary>
   *   <SomeComponent />
   * </ErrorBoundary>
   *
   * With custom fallback:
   * <ErrorBoundary>
   *   <svelte:fragment slot="fallback">
   *     <div class="custom-error">Something went wrong</div>
   *   </svelte:fragment>
   *   <SomeComponent />
   * </ErrorBoundary>
   */
  let hasError = $state(false)
  let error = $state(null)

  /** @param {Error} err */
  function handleError(err) {
    hasError = true
    error = err
    console.error('ErrorBoundary caught:', err)
  }
</script>

{#if hasError}
  <div class="error-boundary" role="alert">
    <div class="error-content">
      <div class="error-icon">&#x26A0;&#xFE0F;</div>
      <h3 class="error-title">组件渲染错误</h3>
      <p class="error-message">
        {error?.message || '未知错误'}
      </p>
      <details class="error-details">
        <summary>详细信息</summary>
        <pre class="error-stack">{error?.stack || 'No stack trace'}</pre>
      </details>
      <!-- svelte-ignore a11y_click_events_have_key_events -->
      <button class="error-retry" onclick={() => { hasError = false; error = null }}>重试</button>
    </div>
  </div>
{:else}
  <slot />
{/if}

<style>
  .error-boundary {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 200px;
    padding: 24px;
  }

  .error-content {
    text-align: center;
    max-width: 500px;
    padding: 24px;
    border-radius: 8px;
    border: 1px solid var(--border);
  }

  .error-icon {
    font-size: 40px;
    margin-bottom: 12px;
  }

  .error-title {
    font-size: 18px;
    font-weight: 600;
    margin: 0 0 8px;
    color: var(--text-primary);
  }

  .error-message {
    font-size: 14px;
    color: var(--text-secondary);
    margin: 0 0 16px;
  }

  .error-details {
    text-align: left;
    margin: 16px 0;
    font-size: 12px;
  }

  .error-details summary {
    cursor: pointer;
    color: var(--accent);
    margin-bottom: 8px;
  }

  .error-stack {
    background: var(--bg-secondary);
    padding: 12px;
    border-radius: 4px;
    overflow-x: auto;
    max-height: 200px;
    overflow-y: auto;
    color: var(--text-muted);
    white-space: pre-wrap;
    word-break: break-all;
  }

  .error-retry {
    padding: 8px 20px;
    border-radius: 6px;
    border: none;
    background: var(--accent);
    color: var(--text-on-accent);
    font-size: 14px;
    cursor: pointer;
    transition: opacity 0.2s;
  }

  .error-retry:hover {
    opacity: 0.9;
  }
</style>
