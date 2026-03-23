<script lang="ts">
  import DataTable from '../shared/DataTable.svelte';
  import Badge from '../shared/Badge.svelte';
  import EmptyState from '../shared/EmptyState.svelte';

  let data = $state<any[]>([]);
  let loading = $state(true);

  $effect(() => {
    (async () => {
      try {
        // @ts-ignore
        data = await window.go.main.BrowserService.ListModernizationCandidates() || [];
      } catch (e) {
        console.error(e);
      } finally {
        loading = false;
      }
    })();
  });

  const columns = [
    { key: 'programId', label: 'Program', sortable: true },
    { key: 'score', label: 'Score', sortable: true, render: (v: number) => v?.toFixed(1) },
    { key: 'approach', label: 'Approach', sortable: true },
    { key: 'reason', label: 'Reason', sortable: false },
  ];
</script>

{#if loading}
  <p class="loading">Loading...</p>
{:else if data.length === 0}
  <EmptyState message="No modernization candidates found — run analysis passes first" />
{:else}
  <DataTable {columns} rows={data} emptyMessage="No candidates" />
{/if}

<style>
  .loading { color: #8b949e; font-size: 13px; }
</style>
