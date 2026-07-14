<script>
  import { onMount, onDestroy } from 'svelte'
  import { toasts, dismissToast } from '../stores/toast.js'

  let visible = $state([])

  toasts.subscribe(v => (visible = v))

  /** @param {Event} e */
  function handleClick(e) {
    const target = e.target
    if (target.closest('.toast-dismiss')) {
      const toast = visible.find(t => t.id === target.closest('[data-toast-id]').dataset.toastId)
      if (toast) dismissToast(toast.id)
    }
  }
</script>

<div class="toast-container" onclick={handleClick}>
  {#each visible as toast (toast.id)}
    <div
      class="toast toast-{toast.level}"
      data-toast-id={toast.id}
      role="alert"
      aria-live="polite"
    >
      <span class="toast-icon">
        {#if toast.level === 'success'}&#x2705;
        {:else if toast.level === 'error'}&#x274C;
        {:else if toast.level === 'warning'}&#x26A0;&#xFE0F;
        {:else}&#x2139;&#xFE0F;
        {/if}
      </span>
      <span class="toast-message">{toast.message}</span>
      {#if toast.dismissible}
        <button class="toast-dismiss" aria-label="Dismiss" tabindex="0">
          &#x2715;
        </button>
      {/if}
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    top: 16px;
    right: 16px;
    z-index: 10000;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-width: 420px;
    pointer-events: none;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    border-radius: 8px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
    color: var(--text-primary);
    font-size: 14px;
    pointer-events: auto;
    animation: toast-in 0.2s ease-out;
    min-width: 200px;
  }

  .toast-error { border-left: 3px solid var(--error); }
  .toast-success { border-left: 3px solid var(--success); }
  .toast-warning { border-left: 3px solid #f0ad4e; }
  .toast-info { border-left: 3px solid var(--accent); }

  .toast-icon { font-size: 16px; flex-shrink: 0; }
  .toast-message { flex: 1; line-height: 1.4; }

  .toast-dismiss {
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    font-size: 16px;
    padding: 2px 4px;
    line-height: 1;
    flex-shrink: 0;
  }

  .toast-dismiss:hover { color: var(--text-primary); }

  @keyframes toast-in {
    from { opacity: 0; transform: translateX(40px); }
    to { opacity: 1; transform: translateX(0); }
  }
</style>
