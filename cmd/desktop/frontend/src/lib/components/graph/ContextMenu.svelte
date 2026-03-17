<script lang="ts">
  type MenuItem = {
    label: string;
    action: string;
    separator?: boolean;
  };

  let {
    x = 0,
    y = 0,
    nodeId = '',
    nodeLabel = '',
    visible = $bindable(false),
    onaction,
  }: {
    x: number;
    y: number;
    nodeId: string;
    nodeLabel: string;
    visible: boolean;
    onaction: (action: string, nodeId: string, nodeLabel: string) => void;
  } = $props();

  const items: MenuItem[] = [
    { label: 'Expand Neighbors', action: 'expand' },
    { label: 'Show Details', action: 'details' },
    { label: 'Center on Node', action: 'center' },
    { label: '', action: '', separator: true },
    { label: 'Pin Node', action: 'pin' },
    { label: 'Unpin Node', action: 'unpin' },
    { label: 'Hide Node', action: 'hide' },
  ];

  function handleClick(action: string) {
    visible = false;
    onaction(action, nodeId, nodeLabel);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') {
      visible = false;
    }
  }

  function handleClickAway() {
    visible = false;
  }
</script>

<svelte:window onkeydown={handleKeydown} onclick={handleClickAway} />

{#if visible}
  <!-- svelte-ignore a11y_no_static_element_interactions -->
  <div
    class="context-menu"
    style="left: {x}px; top: {y}px"
    onclick|stopPropagation={() => {}}
    oncontextmenu|preventDefault|stopPropagation={() => {}}
  >
    <div class="menu-header">{nodeId}</div>
    {#each items as item}
      {#if item.separator}
        <div class="separator"></div>
      {:else}
        <button class="menu-item" onclick={() => handleClick(item.action)}>
          {item.label}
        </button>
      {/if}
    {/each}
  </div>
{/if}

<style>
  .context-menu {
    position: fixed;
    z-index: 100;
    background: #1c2128;
    border: 1px solid #30363d;
    border-radius: 6px;
    padding: 4px 0;
    min-width: 180px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  }

  .menu-header {
    padding: 4px 12px 6px;
    font-size: 11px;
    color: #8b949e;
    border-bottom: 1px solid #21262d;
    margin-bottom: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .menu-item {
    display: block;
    width: 100%;
    padding: 6px 12px;
    background: none;
    border: none;
    color: #e1e4e8;
    font-size: 13px;
    text-align: left;
    cursor: pointer;
  }

  .menu-item:hover {
    background: #30363d;
  }

  .separator {
    height: 1px;
    background: #21262d;
    margin: 4px 0;
  }
</style>
