<script lang="ts">
  import DataTable from '../shared/DataTable.svelte';

  let { onSelect }: { onSelect: (name: string) => void } = $props();

  let data = $state<any[]>([]);
  let loading = $state(true);

  const columns = [
    { key: 'name', label: 'Domain', sortable: true },
    { key: 'description', label: 'Description', sortable: false },
    { key: 'programCount', label: 'Programs', sortable: true },
  ];

  $effect(() => {
    (async () => {
      try {
        // @ts-ignore
        data = await window.go.main.BrowserService.ListBusinessDomains() || [];
      } catch (e) {
        console.error(e);
      } finally {
        loading = false;
      }
    })();
  });
</script>

{#if loading}
  <p class="loading">Loading...</p>
{:else}
  <DataTable {columns} rows={data} onRowClick={(row) => onSelect(row.name)} emptyMessage="No business domains found" />
{/if}

<style>
  .loading { color: #8b949e; font-size: 13px; }
</style>
