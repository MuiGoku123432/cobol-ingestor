<script lang="ts">
  let { currentView = $bindable() }: { currentView: string } = $props();

  const navItems = [
    { id: 'ingest', label: 'Ingest', icon: '&#9654;' },
    { id: 'chat', label: 'Chat', icon: '&#9993;' },
    { id: 'swarm', label: 'Swarm', icon: '&#9881;' },
    { id: 'strategy', label: 'Strategy', icon: '&#9733;' },
    { id: 'graph', label: 'Graph', icon: '&#11052;' },
    { id: 'settings', label: 'Settings', icon: '&#9881;' },
  ];
</script>

<nav class="sidebar">
  <div class="logo">
    <h2>COBOL Graph</h2>
  </div>
  <ul>
    {#each navItems as item}
      <li>
        <button
          class:active={currentView === item.id}
          onclick={() => (currentView = item.id)}
        >
          <span class="icon">{@html item.icon}</span>
          {item.label}
        </button>
      </li>
    {/each}
  </ul>
</nav>

<style>
  .sidebar {
    background: #0d1117;
    border-right: 1px solid #21262d;
    display: flex;
    flex-direction: column;
    grid-row: 1 / -1;
  }

  .logo {
    padding: 16px;
    border-bottom: 1px solid #21262d;
  }

  .logo h2 {
    font-size: 15px;
    font-weight: 600;
    color: #58a6ff;
  }

  ul {
    list-style: none;
    padding: 8px 0;
  }

  button {
    width: 100%;
    background: none;
    border: none;
    color: #8b949e;
    padding: 8px 16px;
    text-align: left;
    font-size: 13px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 8px;
    transition: background 0.15s, color 0.15s;
  }

  button:hover {
    background: #161b22;
    color: #e1e4e8;
  }

  button.active {
    background: #1f2937;
    color: #58a6ff;
    border-left: 2px solid #58a6ff;
  }

  .icon {
    font-size: 14px;
    width: 18px;
    text-align: center;
  }
</style>
