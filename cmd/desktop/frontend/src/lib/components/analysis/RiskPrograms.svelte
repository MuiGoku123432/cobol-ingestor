<script lang="ts">
  import DataTable from '../shared/DataTable.svelte';
  import EmptyState from '../shared/EmptyState.svelte';

  let data = $state<any[]>([]);
  let loading = $state(true);

  $effect(() => {
    (async () => {
      try {
        // @ts-ignore
        data = await window.go.main.BrowserService.ListRiskPrograms(0) || [];
      } catch (e) {
        console.error(e);
      } finally {
        loading = false;
      }
    })();
  });

  function riskVariant(score: number): string {
    if (score >= 0.7) return 'danger';
    if (score >= 0.4) return 'warning';
    return 'success';
  }

  const columns = [
    { key: 'programId', label: 'Program', sortable: true },
    { key: 'riskScore', label: 'Risk Score', sortable: true, render: (v: number) => v?.toFixed(2) },
    { key: 'riskType', label: 'Risk Type', sortable: true },
    { key: 'riskDetails', label: 'Details', sortable: false },
  ];
</script>

{#if loading}
  <p class="loading">Loading...</p>
{:else if data.length === 0}
  <EmptyState message="No risk programs found" />
{:else}
  <DataTable {columns} rows={data} emptyMessage="No risk programs" />
{/if}

<style>
  .loading { color: #8b949e; font-size: 13px; }
</style>
