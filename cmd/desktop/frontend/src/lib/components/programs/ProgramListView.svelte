<script lang="ts">
  import DataTable from '../shared/DataTable.svelte';
  import SearchBar from '../shared/SearchBar.svelte';

  let {
    onSelectProgram,
  }: {
    onSelectProgram: (programId: string) => void;
  } = $props();

  let programs = $state<any[]>([]);
  let search = $state('');
  let total = $state(0);
  let page = $state(1);
  let pageSize = 25;
  let loading = $state(false);

  const columns = [
    { key: 'programId', label: 'Program ID', sortable: true },
    { key: 'filePath', label: 'File Path', sortable: true },
    { key: 'language', label: 'Language', sortable: true },
    { key: 'lineCount', label: 'Lines', sortable: true },
    { key: 'callCount', label: 'Calls', sortable: true },
    { key: 'executionMode', label: 'Mode', sortable: true },
    { key: 'deadCode', label: 'Dead Code', sortable: true,
      render: (v: boolean) => v ? 'Yes' : 'No' },
  ];

  async function loadPrograms() {
    loading = true;
    try {
      // @ts-ignore - Wails bindings
      const resp = await window.go.main.BrowserService.ListPrograms(search, '', page, pageSize);
      programs = resp.data || [];
      total = resp.total || 0;
    } catch (e: any) {
      console.error('Failed to load programs:', e);
      programs = [];
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    loadPrograms();
  });

  function handleSearch(query: string) {
    search = query;
    page = 1;
    loadPrograms();
  }

  function handleRowClick(row: any) {
    onSelectProgram(row.programId);
  }

  let totalPages = $derived(Math.max(1, Math.ceil(total / pageSize)));
</script>

<div class="program-list">
  <div class="toolbar">
    <SearchBar placeholder="Search programs..." onSearch={handleSearch} />
    <span class="count">{total} programs</span>
  </div>

  {#if loading}
    <p class="loading">Loading...</p>
  {:else}
    <DataTable {columns} rows={programs} onRowClick={handleRowClick} emptyMessage="No programs found" {pageSize} />
  {/if}

  {#if totalPages > 1}
    <div class="server-pagination">
      <button disabled={page <= 1} onclick={() => { page--; loadPrograms(); }}>Prev</button>
      <span>Page {page} / {totalPages}</span>
      <button disabled={page >= totalPages} onclick={() => { page++; loadPrograms(); }}>Next</button>
    </div>
  {/if}
</div>

<style>
  .program-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .toolbar {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .count {
    font-size: 12px;
    color: #8b949e;
    white-space: nowrap;
  }

  .loading {
    color: #8b949e;
    font-size: 13px;
  }

  .server-pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 8px 0;
    font-size: 12px;
    color: #8b949e;
  }

  .server-pagination button {
    background: #21262d;
    border: 1px solid #30363d;
    color: #c9d1d9;
    padding: 4px 12px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
  }

  .server-pagination button:disabled {
    opacity: 0.4;
    cursor: default;
  }
</style>
