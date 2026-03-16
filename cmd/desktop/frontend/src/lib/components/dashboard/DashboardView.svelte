<script lang="ts">
  let stats = $state<any>(null);
  let error = $state('');
  let loading = $state(true);

  async function loadStats() {
    loading = true;
    error = '';
    try {
      // @ts-ignore - Wails bindings
      stats = await window.go.main.GraphService.GetDashboardStats();
    } catch (e: any) {
      error = e?.message || 'Failed to load dashboard stats';
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    loadStats();
  });

  function statCards(s: any) {
    if (!s) return [];
    return [
      { label: 'Programs', value: s.programs || 0, color: '#58a6ff' },
      { label: 'Copybooks', value: s.copybooks || 0, color: '#3fb950' },
      { label: 'Paragraphs', value: s.paragraphs || 0, color: '#d2a8ff' },
      { label: 'Data Items', value: s.dataItems || 0, color: '#f0883e' },
      { label: 'Relationships', value: s.relationships || 0, color: '#79c0ff' },
      { label: 'Domains', value: s.domains || 0, color: '#56d364' },
      { label: 'JCL Jobs', value: s.jclJobs || 0, color: '#e3b341' },
      { label: 'Dead Code', value: s.deadParagraphs || 0, color: '#f85149' },
    ];
  }
</script>

<div class="dashboard">
  <h1>Dashboard</h1>

  {#if loading}
    <p class="muted">Loading stats...</p>
  {:else if error}
    <div class="error-card">
      <p>{error}</p>
      <p class="hint">Connect to Neo4j in Settings to see dashboard stats.</p>
    </div>
  {:else}
    <div class="cards">
      {#each statCards(stats) as card}
        <div class="card" style="border-top-color: {card.color}">
          <span class="value">{card.value.toLocaleString()}</span>
          <span class="label">{card.label}</span>
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .dashboard h1 {
    font-size: 20px;
    font-weight: 600;
    margin-bottom: 20px;
  }

  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 12px;
  }

  .card {
    background: #0d1117;
    border: 1px solid #21262d;
    border-top: 3px solid;
    border-radius: 6px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .value {
    font-size: 28px;
    font-weight: 700;
    color: #e1e4e8;
  }

  .label {
    font-size: 12px;
    color: #8b949e;
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .muted {
    color: #8b949e;
  }

  .error-card {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 6px;
    padding: 20px;
  }

  .error-card p {
    color: #f85149;
  }

  .hint {
    color: #8b949e !important;
    margin-top: 8px;
    font-size: 13px;
  }
</style>
