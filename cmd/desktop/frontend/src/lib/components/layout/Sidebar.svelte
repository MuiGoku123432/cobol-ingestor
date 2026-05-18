<script lang="ts">
  let { currentView = $bindable() }: { currentView: string } = $props();

  let searchQuery = $state('');
  let searchResults = $state<any[]>([]);
  let searchOpen = $state(false);
  let searchTimer: ReturnType<typeof setTimeout>;

  const navItems = [
    { id: 'dashboard', label: 'Dashboard', icon: '&#9632;' },
    { id: 'browse', label: 'Browse', icon: '&#9776;' },
    { id: 'analysis', label: 'Analysis', icon: '&#9878;' },
    { id: 'ingest', label: 'Ingest', icon: '&#9654;' },
    { id: 'chat', label: 'Chat', icon: '&#9993;' },
    { id: 'strategy', label: 'Strategy', icon: '&#9733;' },
    { id: 'graph', label: 'Graph', icon: '&#11052;' },
    { id: 'settings', label: 'Settings', icon: '&#9881;' },
  ];

  function handleSearchInput(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    searchQuery = val;
    clearTimeout(searchTimer);
    if (!val.trim()) {
      searchResults = [];
      searchOpen = false;
      return;
    }
    searchTimer = setTimeout(async () => {
      try {
        // @ts-ignore - Wails bindings
        searchResults = await window.go.main.BrowserService.SearchFullText(val, 10) || [];
        searchOpen = searchResults.length > 0;
      } catch {
        searchResults = [];
      }
    }, 300);
  }

  function selectResult(result: any) {
    searchOpen = false;
    searchQuery = '';
    searchResults = [];
    // Navigate to browse tab — the result label tells us what type
    currentView = 'browse';
  }
</script>

<nav class="sidebar">
  <div class="logo">
    <h2>COBOL Graph</h2>
  </div>

  <div class="search-wrapper">
    <input
      type="text"
      class="global-search"
      placeholder="Search..."
      value={searchQuery}
      oninput={handleSearchInput}
      onfocus={() => { if (searchResults.length) searchOpen = true; }}
      onblur={() => setTimeout(() => searchOpen = false, 200)}
    />
    {#if searchOpen}
      <div class="search-dropdown">
        {#each searchResults as result}
          <button class="search-result" onclick={() => selectResult(result)}>
            <span class="result-label">{result.label}</span>
            <span class="result-name">{result.name}</span>
          </button>
        {/each}
      </div>
    {/if}
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

  .search-wrapper {
    padding: 8px 12px;
    position: relative;
  }

  .global-search {
    width: 100%;
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 6px;
    color: #e1e4e8;
    padding: 6px 10px;
    font-size: 12px;
    outline: none;
  }

  .global-search::placeholder {
    color: #484f58;
  }

  .global-search:focus {
    border-color: #58a6ff;
  }

  .search-dropdown {
    position: absolute;
    top: 100%;
    left: 12px;
    right: 12px;
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 6px;
    max-height: 240px;
    overflow-y: auto;
    z-index: 100;
    box-shadow: 0 4px 12px rgba(0,0,0,0.4);
  }

  .search-result {
    display: flex;
    justify-content: space-between;
    width: 100%;
    padding: 6px 10px;
    background: none;
    border: none;
    color: #c9d1d9;
    font-size: 12px;
    cursor: pointer;
    text-align: left;
  }

  .search-result:hover {
    background: #1c2333;
  }

  .result-label {
    color: #8b949e;
    font-size: 10px;
    text-transform: uppercase;
  }

  .result-name {
    color: #58a6ff;
  }

  ul {
    list-style: none;
    padding: 8px 0;
    flex: 1;
    overflow-y: auto;
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
