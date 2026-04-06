<script lang="ts">
  import DataTable from '../shared/DataTable.svelte';
  import EmptyState from '../shared/EmptyState.svelte';

  let data = $state<any[]>([]);
  let loading = $state(true);

  $effect(() => {
    (async () => {
      try {
        // @ts-ignore
        data = await window.go.main.BrowserService.GetEffortEstimates() || [];
      } catch (e) {
        console.error(e);
      } finally {
        loading = false;
      }
    })();
  });

  const columns = [
    { key: 'programId', label: 'Program', sortable: true },
    { key: 'tShirtSize', label: 'T-Shirt', sortable: true },
    { key: 'complexityScore', label: 'Complexity', sortable: true },
    { key: 'lineCount', label: 'Lines', sortable: true },
    { key: 'paragraphCount', label: 'Paragraphs', sortable: true },
    { key: 'copybookCount', label: 'Copybooks', sortable: true },
    { key: 'sqlStatementCount', label: 'SQL', sortable: true },
    { key: 'cicsTransactionCount', label: 'CICS', sortable: true },
    { key: 'approach', label: 'Approach', sortable: true },
  ];
</script>

{#if loading}
  <p class="loading">Loading...</p>
{:else if data.length === 0}
  <EmptyState message="No effort estimates available" />
{:else}
  <DataTable {columns} rows={data} emptyMessage="No effort estimates" />
{/if}

<style>
  .loading { color: #8b949e; font-size: 13px; }
</style>
