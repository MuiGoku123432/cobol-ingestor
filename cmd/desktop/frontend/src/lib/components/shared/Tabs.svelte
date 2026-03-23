<script lang="ts">
  type Tab = { id: string; label: string };

  let {
    tabs,
    activeTab = $bindable(),
  }: {
    tabs: Tab[];
    activeTab: string;
  } = $props();
</script>

<div class="tabs">
  {#each tabs as tab}
    <button
      class:active={activeTab === tab.id}
      onclick={() => activeTab = tab.id}
    >
      {tab.label}
    </button>
  {/each}
</div>

<style>
  .tabs {
    display: flex;
    border-bottom: 1px solid #21262d;
    margin-bottom: 16px;
    gap: 0;
  }

  button {
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    color: #8b949e;
    padding: 8px 16px;
    font-size: 13px;
    cursor: pointer;
    transition: color 0.15s, border-color 0.15s;
  }

  button:hover {
    color: #e1e4e8;
  }

  button.active {
    color: #58a6ff;
    border-bottom-color: #58a6ff;
  }
</style>
