<script lang="ts">
  import DataTable from '../shared/DataTable.svelte';
  import EmptyState from '../shared/EmptyState.svelte';

  let data = $state<any[]>([]);
  let loading = $state(true);

  $effect(() => {
    (async () => {
      try {
        // @ts-ignore
        data = await window.go.main.BrowserService.GetMigrationSequence() || [];
      } catch (e) {
        console.error(e);
      } finally {
        loading = false;
      }
    })();
  });

  const columns = [
    { key: 'order', label: 'Order', sortable: true },
    { key: 'programId', label: 'Program', sortable: true },
    { key: 'tier', label: 'Tier', sortable: true },
    { key: 'score', label: 'Score', sortable: true, render: (v: number) => v?.toFixed(1) },
    { key: 'domain', label: 'Domain', sortable: true },
    { key: 'approach', label: 'Approach', sortable: true },
    { key: 'blockedBy', label: 'Blocked By', sortable: false,
      render: (v: string[]) => v?.length ? v.join(', ') : '-' },
  ];
</script>

{#if loading}
  <p class="loading">Loading...</p>
{:else if data.length === 0}
  <EmptyState message="No migration sequence available — run analysis passes first" />
{:else}
  <DataTable {columns} rows={data} emptyMessage="No migration steps" />
{/if}

<style>
  .loading { color: #8b949e; font-size: 13px; }
</style>
