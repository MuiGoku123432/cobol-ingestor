<script lang="ts">
  import DataTable from '../shared/DataTable.svelte';
  import EmptyState from '../shared/EmptyState.svelte';

  let data = $state<any[]>([]);
  let loading = $state(true);

  $effect(() => {
    (async () => {
      try {
        // @ts-ignore
        data = await window.go.main.BrowserService.GetDeadCodeSummary() || [];
      } catch (e) {
        console.error(e);
      } finally {
        loading = false;
      }
    })();
  });

  const columns = [
    { key: 'programId', label: 'Program', sortable: true },
    { key: 'totalParagraphs', label: 'Total Paragraphs', sortable: true },
    { key: 'deadParagraphs', label: 'Dead Paragraphs', sortable: true },
    { key: '_pct', label: '% Dead', sortable: true,
      render: (_: any, row: any) => row.totalParagraphs > 0
        ? `${((row.deadParagraphs / row.totalParagraphs) * 100).toFixed(1)}%`
        : '-' },
  ];
</script>

{#if loading}
  <p class="loading">Loading...</p>
{:else if data.length === 0}
  <EmptyState message="No dead code data — run dead code detection first" />
{:else}
  <DataTable {columns} rows={data} emptyMessage="No dead code data" />
{/if}

<style>
  .loading { color: #8b949e; font-size: 13px; }
</style>
