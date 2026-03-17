<script lang="ts">
  let {
    visible = $bindable(false),
    onrungraph,
  }: {
    visible: boolean;
    onrungraph: (data: { nodes: any[]; relationships: any[] }) => void;
  } = $props();

  let query = $state('');
  let running = $state(false);
  let resultMode: 'auto' | 'table' | 'graph' = $state('auto');
  let columns: string[] = $state([]);
  let rows: Record<string, any>[] = $state([]);
  let hasGraph = $state(false);
  let resultError = $state('');

  // Saved queries
  let savedQueries: any[] = $state([]);
  let showSaveInput = $state(false);
  let saveName = $state('');
  let saveCategory = $state('');

  async function loadSavedQueries() {
    try {
      savedQueries = await (window as any).go.main.QueryStoreService.ListQueries();
    } catch {
      savedQueries = [];
    }
  }

  async function runQuery() {
    if (!query.trim() || running) return;
    running = true;
    resultError = '';
    columns = [];
    rows = [];
    hasGraph = false;

    try {
      const result = await (window as any).go.main.Neo4jService.ExecuteCypher(query);
      columns = result.columns || [];
      rows = result.rows || [];
      hasGraph = result.isGraph;

      if (hasGraph && result.graph && (resultMode === 'auto' || resultMode === 'graph')) {
        onrungraph(result.graph);
      }
    } catch (e: any) {
      resultError = e?.message || String(e);
    } finally {
      running = false;
    }
  }

  async function saveQuery() {
    if (!saveName.trim() || !query.trim()) return;
    try {
      await (window as any).go.main.QueryStoreService.SaveQuery(saveName, query, saveCategory);
      showSaveInput = false;
      saveName = '';
      saveCategory = '';
      await loadSavedQueries();
    } catch (e: any) {
      resultError = e?.message || String(e);
    }
  }

  async function deleteQuery(id: string) {
    try {
      await (window as any).go.main.QueryStoreService.DeleteQuery(id);
      await loadSavedQueries();
    } catch (e: any) {
      resultError = e?.message || String(e);
    }
  }

  function selectSavedQuery(cypher: string) {
    query = cypher;
  }

  function handleKeydown(e: KeyboardEvent) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      runQuery();
    }
  }

  $effect(() => {
    if (visible) {
      loadSavedQueries();
    }
  });
</script>

{#if visible}
  <div class="cypher-panel">
    <div class="panel-header">
      <span class="panel-title">Cypher Query</span>
      <div class="panel-actions">
        <select
          class="saved-select"
          onchange={(e) => {
            const val = (e.target as HTMLSelectElement).value;
            if (val) selectSavedQuery(val);
            (e.target as HTMLSelectElement).value = '';
          }}
        >
          <option value="">Saved Queries...</option>
          {#each savedQueries as sq}
            <option value={sq.cypher}>
              {sq.builtin ? '★ ' : ''}{sq.name}
            </option>
          {/each}
        </select>
        <button class="panel-btn" onclick={() => { showSaveInput = !showSaveInput; }} title="Save query">Save</button>
        <button class="panel-btn" onclick={runQuery} disabled={running} title="Run (Ctrl+Enter)">
          {running ? 'Running...' : 'Run'}
        </button>
        <button class="panel-btn close" onclick={() => { visible = false; }} title="Close">&times;</button>
      </div>
    </div>

    {#if showSaveInput}
      <div class="save-row">
        <input type="text" placeholder="Query name" bind:value={saveName} class="save-input" />
        <input type="text" placeholder="Category" bind:value={saveCategory} class="save-input small" />
        <button class="panel-btn" onclick={saveQuery}>OK</button>
        <button class="panel-btn" onclick={() => { showSaveInput = false; }}>Cancel</button>
      </div>
    {/if}

    <div class="editor-area">
      <textarea
        class="cypher-input"
        placeholder="MATCH (n) RETURN n LIMIT 25"
        bind:value={query}
        onkeydown={handleKeydown}
        spellcheck="false"
      ></textarea>
    </div>

    {#if resultError}
      <div class="result-error">{resultError}</div>
    {/if}

    {#if columns.length > 0}
      <div class="result-bar">
        <span class="result-count">{rows.length} row{rows.length !== 1 ? 's' : ''}</span>
        <div class="mode-toggle">
          <button class:active={resultMode === 'table'} onclick={() => { resultMode = 'table'; }}>Table</button>
          {#if hasGraph}
            <button class:active={resultMode === 'graph'} onclick={() => { resultMode = 'graph'; }}>Graph</button>
          {/if}
        </div>
        {#if savedQueries.length > 0}
          <div class="saved-manage">
            {#each savedQueries.filter(q => !q.builtin) as sq}
              <span class="saved-tag">
                {sq.name}
                <button class="tag-delete" onclick={() => deleteQuery(sq.id)}>&times;</button>
              </span>
            {/each}
          </div>
        {/if}
      </div>

      {#if resultMode !== 'graph'}
        <div class="result-table-wrap">
          <table class="result-table">
            <thead>
              <tr>
                {#each columns as col}
                  <th>{col}</th>
                {/each}
              </tr>
            </thead>
            <tbody>
              {#each rows as row}
                <tr>
                  {#each columns as col}
                    <td>{typeof row[col] === 'object' ? JSON.stringify(row[col]) : String(row[col] ?? '')}</td>
                  {/each}
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    {/if}
  </div>
{/if}

<style>
  .cypher-panel {
    border-top: 1px solid #30363d;
    background: #0d1117;
    display: flex;
    flex-direction: column;
    max-height: 300px;
    min-height: 150px;
  }

  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 12px;
    background: #161b22;
    border-bottom: 1px solid #21262d;
    flex-shrink: 0;
  }

  .panel-title {
    font-size: 12px;
    font-weight: 600;
    color: #e1e4e8;
  }

  .panel-actions {
    display: flex;
    gap: 6px;
    align-items: center;
  }

  .panel-btn {
    background: #21262d;
    color: #e1e4e8;
    border: 1px solid #30363d;
    border-radius: 4px;
    padding: 3px 10px;
    font-size: 12px;
    cursor: pointer;
  }

  .panel-btn:hover {
    background: #30363d;
  }

  .panel-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .panel-btn.close {
    background: none;
    border: none;
    font-size: 16px;
    color: #8b949e;
    padding: 0 4px;
  }

  .panel-btn.close:hover {
    color: #e1e4e8;
  }

  .saved-select {
    background: #161b22;
    color: #e1e4e8;
    border: 1px solid #30363d;
    border-radius: 4px;
    padding: 3px 6px;
    font-size: 12px;
    max-width: 200px;
  }

  .save-row {
    display: flex;
    gap: 6px;
    padding: 6px 12px;
    background: #161b22;
    border-bottom: 1px solid #21262d;
    flex-shrink: 0;
  }

  .save-input {
    background: #0d1117;
    color: #e1e4e8;
    border: 1px solid #30363d;
    border-radius: 4px;
    padding: 3px 8px;
    font-size: 12px;
    flex: 1;
  }

  .save-input.small {
    max-width: 120px;
  }

  .editor-area {
    flex-shrink: 0;
    padding: 6px 12px;
  }

  .cypher-input {
    width: 100%;
    height: 60px;
    background: #161b22;
    color: #e1e4e8;
    border: 1px solid #30363d;
    border-radius: 4px;
    padding: 8px;
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 13px;
    resize: vertical;
    outline: none;
    box-sizing: border-box;
  }

  .cypher-input:focus {
    border-color: #58a6ff;
  }

  .result-error {
    color: #f85149;
    font-size: 12px;
    padding: 6px 12px;
  }

  .result-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 4px 12px;
    background: #161b22;
    border-top: 1px solid #21262d;
    border-bottom: 1px solid #21262d;
    flex-shrink: 0;
  }

  .result-count {
    font-size: 11px;
    color: #8b949e;
  }

  .mode-toggle {
    display: flex;
    gap: 2px;
  }

  .mode-toggle button {
    background: #21262d;
    color: #8b949e;
    border: 1px solid #30363d;
    padding: 2px 8px;
    font-size: 11px;
    cursor: pointer;
  }

  .mode-toggle button:first-child {
    border-radius: 4px 0 0 4px;
  }

  .mode-toggle button:last-child {
    border-radius: 0 4px 4px 0;
  }

  .mode-toggle button.active {
    background: #58a6ff;
    color: #0d1117;
    border-color: #58a6ff;
  }

  .saved-manage {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
    margin-left: auto;
  }

  .saved-tag {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    background: #21262d;
    color: #8b949e;
    font-size: 10px;
    padding: 1px 6px;
    border-radius: 3px;
  }

  .tag-delete {
    background: none;
    border: none;
    color: #8b949e;
    font-size: 12px;
    cursor: pointer;
    padding: 0 2px;
  }

  .tag-delete:hover {
    color: #f85149;
  }

  .result-table-wrap {
    flex: 1;
    overflow: auto;
    min-height: 0;
  }

  .result-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }

  .result-table th {
    background: #161b22;
    color: #8b949e;
    text-align: left;
    padding: 4px 10px;
    border-bottom: 1px solid #21262d;
    position: sticky;
    top: 0;
  }

  .result-table td {
    padding: 4px 10px;
    color: #e1e4e8;
    border-bottom: 1px solid #161b22;
    max-width: 300px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .result-table tbody tr:hover {
    background: #161b22;
  }
</style>
