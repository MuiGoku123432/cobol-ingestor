<script lang="ts">
  type Column = {
    key: string;
    label: string;
    sortable?: boolean;
    render?: (value: any, row: any) => string;
  };

  let {
    columns,
    rows = [],
    onRowClick,
    emptyMessage = 'No data available',
    pageSize = 25,
  }: {
    columns: Column[];
    rows: any[];
    onRowClick?: (row: any) => void;
    emptyMessage?: string;
    pageSize?: number;
  } = $props();

  let sortKey = $state('');
  let sortDir = $state<'asc' | 'desc'>('asc');
  let currentPage = $state(1);

  function toggleSort(key: string) {
    if (sortKey === key) {
      sortDir = sortDir === 'asc' ? 'desc' : 'asc';
    } else {
      sortKey = key;
      sortDir = 'asc';
    }
    currentPage = 1;
  }

  let sortedRows = $derived.by(() => {
    if (!sortKey) return rows;
    return [...rows].sort((a, b) => {
      const av = a[sortKey] ?? '';
      const bv = b[sortKey] ?? '';
      const cmp = typeof av === 'number' ? av - bv : String(av).localeCompare(String(bv));
      return sortDir === 'asc' ? cmp : -cmp;
    });
  });

  let totalPages = $derived(Math.max(1, Math.ceil(sortedRows.length / pageSize)));
  let pagedRows = $derived(sortedRows.slice((currentPage - 1) * pageSize, currentPage * pageSize));

  function getCellValue(row: any, col: Column): string {
    const val = row[col.key];
    if (col.render) return col.render(val, row);
    if (val === null || val === undefined) return '-';
    return String(val);
  }
</script>

{#if rows.length === 0}
  <div class="empty">{emptyMessage}</div>
{:else}
  <div class="table-wrapper">
    <table>
      <thead>
        <tr>
          {#each columns as col}
            <th
              class:sortable={col.sortable}
              onclick={() => col.sortable && toggleSort(col.key)}
            >
              {col.label}
              {#if sortKey === col.key}
                <span class="sort-arrow">{sortDir === 'asc' ? '▲' : '▼'}</span>
              {/if}
            </th>
          {/each}
        </tr>
      </thead>
      <tbody>
        {#each pagedRows as row}
          <tr
            class:clickable={!!onRowClick}
            onclick={() => onRowClick?.(row)}
          >
            {#each columns as col}
              <td>{getCellValue(row, col)}</td>
            {/each}
          </tr>
        {/each}
      </tbody>
    </table>
  </div>

  {#if totalPages > 1}
    <div class="pagination">
      <button disabled={currentPage <= 1} onclick={() => currentPage--}>Prev</button>
      <span>{currentPage} / {totalPages} ({rows.length} rows)</span>
      <button disabled={currentPage >= totalPages} onclick={() => currentPage++}>Next</button>
    </div>
  {/if}
{/if}

<style>
  .table-wrapper {
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }

  th, td {
    padding: 6px 10px;
    text-align: left;
    border-bottom: 1px solid #21262d;
    white-space: nowrap;
  }

  th {
    background: #161b22;
    color: #8b949e;
    font-weight: 600;
    position: sticky;
    top: 0;
    z-index: 1;
  }

  th.sortable {
    cursor: pointer;
    user-select: none;
  }

  th.sortable:hover {
    color: #e1e4e8;
  }

  .sort-arrow {
    font-size: 10px;
    margin-left: 4px;
  }

  tr.clickable {
    cursor: pointer;
  }

  tr.clickable:hover td {
    background: #1c2333;
  }

  td {
    color: #c9d1d9;
  }

  .pagination {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
    padding: 8px 0;
    font-size: 12px;
    color: #8b949e;
  }

  .pagination button {
    background: #21262d;
    border: 1px solid #30363d;
    color: #c9d1d9;
    padding: 4px 12px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 12px;
  }

  .pagination button:disabled {
    opacity: 0.4;
    cursor: default;
  }

  .pagination button:not(:disabled):hover {
    background: #30363d;
  }

  .empty {
    text-align: center;
    padding: 40px 20px;
    color: #8b949e;
    font-size: 13px;
  }
</style>
