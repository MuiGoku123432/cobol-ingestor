<script lang="ts">
  let {
    label = 'Export',
    onExport,
  }: {
    label?: string;
    onExport: () => Promise<void>;
  } = $props();

  let exporting = $state(false);
  let message = $state('');

  async function handleClick() {
    exporting = true;
    message = '';
    try {
      await onExport();
      message = 'Exported successfully';
      setTimeout(() => message = '', 3000);
    } catch (e: any) {
      message = `Error: ${e.message || e}`;
    } finally {
      exporting = false;
    }
  }
</script>

<div class="export-wrapper">
  <button class="export-btn" onclick={handleClick} disabled={exporting}>
    {exporting ? 'Exporting...' : label}
  </button>
  {#if message}
    <span class="message" class:error={message.startsWith('Error')}>{message}</span>
  {/if}
</div>

<style>
  .export-wrapper {
    display: inline-flex;
    align-items: center;
    gap: 8px;
  }

  .export-btn {
    background: #21262d;
    border: 1px solid #30363d;
    color: #c9d1d9;
    padding: 6px 14px;
    border-radius: 6px;
    font-size: 12px;
    cursor: pointer;
  }

  .export-btn:hover:not(:disabled) {
    background: #30363d;
  }

  .export-btn:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .message {
    font-size: 11px;
    color: #3fb950;
  }

  .message.error {
    color: #f85149;
  }
</style>
