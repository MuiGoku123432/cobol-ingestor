<script lang="ts">
  import ProgramDetail from './ProgramDetail.svelte';

  let search = $state('');
  let programs = $state<any[]>([]);
  let totalCount = $state(0);
  let page = $state(1);
  let loading = $state(false);
  let selectedProgram = $state<string | null>(null);

  async function searchPrograms() {
    loading = true;
    try {
      // @ts-ignore - Wails bindings
      const result = await window.go.main.GraphService.ListPrograms(search, page, 30);
      programs = result?.items || [];
      totalCount = result?.total || 0;
    } catch (e: any) {
      programs = [];
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    searchPrograms();
  });

  function handleSearch() {
    page = 1;
    searchPrograms();
  }
</script>

<div class="explorer">
  <div class="list-panel">
    <h2>Programs</h2>
    <div class="search-bar">
      <input
        type="text"
        placeholder="Search programs..."
        bind:value={search}
        onkeydown={(e) => e.key === 'Enter' && handleSearch()}
      />
      <button onclick={handleSearch}>Search</button>
    </div>

    {#if loading}
      <p class="muted">Loading...</p>
    {:else if programs.length === 0}
      <p class="muted">No programs found. Run ingestion first.</p>
    {:else}
      <div class="program-list">
        {#each programs as prog}
          <button
            class="program-item"
            class:selected={selectedProgram === prog.programId}
            onclick={() => (selectedProgram = prog.programId)}
          >
            <span class="prog-id">{prog.programId}</span>
            <span class="prog-meta">{prog.callCount || 0} calls</span>
          </button>
        {/each}
      </div>
      <div class="pagination">
        <span class="muted">{totalCount} programs</span>
        <div class="page-btns">
          <button disabled={page <= 1} onclick={() => { page--; searchPrograms(); }}>Prev</button>
          <span>Page {page}</span>
          <button disabled={programs.length < 30} onclick={() => { page++; searchPrograms(); }}>Next</button>
        </div>
      </div>
    {/if}
  </div>

  <div class="detail-panel">
    {#if selectedProgram}
      <ProgramDetail programId={selectedProgram} />
    {:else}
      <div class="placeholder">
        <p class="muted">Select a program to view details</p>
      </div>
    {/if}
  </div>
</div>

<style>
  .explorer {
    display: grid;
    grid-template-columns: 320px 1fr;
    gap: 16px;
    height: calc(100vh - 88px);
  }

  .list-panel {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 6px;
    padding: 12px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .list-panel h2 {
    font-size: 15px;
    margin-bottom: 8px;
  }

  .search-bar {
    display: flex;
    gap: 6px;
    margin-bottom: 8px;
  }

  .search-bar input {
    flex: 1;
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 4px;
    color: #e1e4e8;
    padding: 6px 8px;
    font-size: 13px;
  }

  .search-bar button {
    background: #21262d;
    border: 1px solid #30363d;
    color: #e1e4e8;
    border-radius: 4px;
    padding: 6px 12px;
    cursor: pointer;
    font-size: 13px;
  }

  .program-list {
    flex: 1;
    overflow-y: auto;
  }

  .program-item {
    width: 100%;
    background: none;
    border: none;
    border-bottom: 1px solid #21262d;
    color: #e1e4e8;
    padding: 8px;
    text-align: left;
    cursor: pointer;
    display: flex;
    justify-content: space-between;
    font-size: 13px;
  }

  .program-item:hover {
    background: #161b22;
  }

  .program-item.selected {
    background: #1f2937;
    color: #58a6ff;
  }

  .prog-meta {
    color: #8b949e;
    font-size: 11px;
  }

  .pagination {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-top: 8px;
    border-top: 1px solid #21262d;
    font-size: 12px;
  }

  .page-btns {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .page-btns button {
    background: #21262d;
    border: 1px solid #30363d;
    color: #e1e4e8;
    border-radius: 4px;
    padding: 4px 8px;
    cursor: pointer;
    font-size: 11px;
  }

  .page-btns button:disabled {
    opacity: 0.4;
    cursor: default;
  }

  .detail-panel {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 6px;
    padding: 16px;
    overflow-y: auto;
  }

  .placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
  }

  .muted {
    color: #8b949e;
  }
</style>
