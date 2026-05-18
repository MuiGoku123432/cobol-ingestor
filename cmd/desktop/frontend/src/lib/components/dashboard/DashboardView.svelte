<script lang="ts">
  import EmptyState from '../shared/EmptyState.svelte';
  import ExportButton from '../shared/ExportButton.svelte';

  type DashboardStats = {
    programCount: number;
    copybookCount: number;
    paragraphCount: number;
    sectionCount: number;
    dataItemCount: number;
    fileCount: number;
    sqlStatementCount: number;
    cicsTransactionCount: number;
    externalInterfaceCount: number;
    relationshipCount: number;
    orphanCount: number;
    domainCount: number;
    ddCardCount: number;
    dbTableCount: number;
  };

  let stats = $state<DashboardStats | null>(null);
  let loading = $state(true);
  let error = $state('');

  async function loadStats() {
    loading = true;
    error = '';
    try {
      // @ts-ignore - Wails bindings
      stats = await window.go.main.BrowserService.GetDashboardStats();
    } catch (e: any) {
      error = e.message || String(e);
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    loadStats();
  });

  const cards = $derived(stats ? [
    { label: 'Programs', value: stats.programCount, color: '#238636' },
    { label: 'Copybooks', value: stats.copybookCount, color: '#58a6ff' },
    { label: 'Paragraphs', value: stats.paragraphCount, color: '#d2a8ff' },
    { label: 'Data Items', value: stats.dataItemCount, color: '#8b949e' },
    { label: 'Relationships', value: stats.relationshipCount, color: '#f0883e' },
    { label: 'Business Domains', value: stats.domainCount, color: '#f0883e' },
    { label: 'SQL Statements', value: stats.sqlStatementCount, color: '#58a6ff' },
    { label: 'CICS Transactions', value: stats.cicsTransactionCount, color: '#d29922' },
    { label: 'DB Tables', value: stats.dbTableCount, color: '#58a6ff' },
    { label: 'Files', value: stats.fileCount, color: '#8b949e' },
    { label: 'Orphan Programs', value: stats.orphanCount, color: '#f85149' },
    { label: 'DD Cards', value: stats.ddCardCount, color: '#8b949e' },
  ] : []);

  async function handleExport() {
    // @ts-ignore
    const dir = await window.go.main.ExportService.SelectSaveDirectory();
    if (!dir) return;
    // @ts-ignore
    const path = await window.go.main.ExportService.ExportAnalysisReport(dir);
    alert(`Report saved to: ${path}`);
  }
</script>

<div class="dashboard">
  <div class="header">
    <h2>Dashboard</h2>
    {#if stats}
      <ExportButton label="Export Report" onExport={handleExport} />
    {/if}
  </div>

  {#if loading}
    <p class="loading">Loading dashboard...</p>
  {:else if error}
    <EmptyState message={error} />
  {:else if !stats}
    <EmptyState message="No data yet — connect to Neo4j and run ingestion first" />
  {:else}
    <div class="cards">
      {#each cards as card}
        <div class="card">
          <div class="card-value" style="color: {card.color}">{card.value.toLocaleString()}</div>
          <div class="card-label">{card.label}</div>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .dashboard {
    max-width: 1000px;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;
  }

  h2 {
    font-size: 18px;
    font-weight: 600;
    color: #e1e4e8;
  }

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 12px;
  }

  .card {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 8px;
    padding: 16px;
  }

  .card-value {
    font-size: 28px;
    font-weight: 700;
    margin-bottom: 4px;
  }

  .card-label {
    font-size: 12px;
    color: #8b949e;
  }

  .loading {
    color: #8b949e;
    font-size: 13px;
    padding: 20px;
  }
</style>
