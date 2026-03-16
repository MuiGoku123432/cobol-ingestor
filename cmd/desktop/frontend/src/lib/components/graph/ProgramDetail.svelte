<script lang="ts">
  let { programId }: { programId: string } = $props();

  let program = $state<any>(null);
  let callChain = $state<any[]>([]);
  let dataItems = $state<any[]>([]);
  let loading = $state(true);
  let activeTab = $state('overview');

  async function loadProgram(id: string) {
    loading = true;
    try {
      // @ts-ignore - Wails bindings
      program = await window.go.main.GraphService.GetProgram(id);
    } catch (e) {
      program = null;
    } finally {
      loading = false;
    }
  }

  async function loadCallChain(id: string) {
    try {
      // @ts-ignore - Wails bindings
      callChain = (await window.go.main.GraphService.GetCallChain(id, 'both', 3)) || [];
    } catch {
      callChain = [];
    }
  }

  async function loadDataItems(id: string) {
    try {
      // @ts-ignore - Wails bindings
      dataItems = (await window.go.main.GraphService.GetDataItems(id)) || [];
    } catch {
      dataItems = [];
    }
  }

  $effect(() => {
    if (programId) {
      loadProgram(programId);
      loadCallChain(programId);
      loadDataItems(programId);
    }
  });

  const tabs = [
    { id: 'overview', label: 'Overview' },
    { id: 'calls', label: 'Call Chain' },
    { id: 'data', label: 'Data Items' },
  ];
</script>

<div class="detail">
  {#if loading}
    <p class="muted">Loading...</p>
  {:else if !program}
    <p class="muted">Program not found</p>
  {:else}
    <h2>{program.programId}</h2>
    {#if program.filePath}
      <p class="filepath">{program.filePath}</p>
    {/if}

    <div class="tabs">
      {#each tabs as tab}
        <button
          class:active={activeTab === tab.id}
          onclick={() => (activeTab = tab.id)}
        >{tab.label}</button>
      {/each}
    </div>

    {#if activeTab === 'overview'}
      <div class="info-grid">
        {#if program.language}
          <div class="info-item"><span class="lbl">Language</span><span>{program.language}</span></div>
        {/if}
        {#if program.lineCount}
          <div class="info-item"><span class="lbl">Lines</span><span>{program.lineCount}</span></div>
        {/if}
        {#if program.executionMode}
          <div class="info-item"><span class="lbl">Exec Mode</span><span>{program.executionMode}</span></div>
        {/if}
        {#if program.deadCode !== undefined}
          <div class="info-item"><span class="lbl">Dead Code</span><span>{program.deadCode ? 'Yes' : 'No'}</span></div>
        {/if}
      </div>

      {#if program.paragraphs?.length}
        <h3>Paragraphs ({program.paragraphs.length})</h3>
        <ul class="item-list">
          {#each program.paragraphs as p}
            <li>{p.name || p.paragraphId}</li>
          {/each}
        </ul>
      {/if}

      {#if program.copybooks?.length}
        <h3>Copybooks ({program.copybooks.length})</h3>
        <ul class="item-list">
          {#each program.copybooks as cb}
            <li>{cb.name || cb}</li>
          {/each}
        </ul>
      {/if}

      {#if program.calledPrograms?.length}
        <h3>Calls ({program.calledPrograms.length})</h3>
        <ul class="item-list">
          {#each program.calledPrograms as target}
            <li>{target}</li>
          {/each}
        </ul>
      {/if}
    {:else if activeTab === 'calls'}
      <h3>Call Chain</h3>
      {#if callChain.length === 0}
        <p class="muted">No call chain data</p>
      {:else}
        <ul class="item-list">
          {#each callChain as node}
            <li>
              <strong>{node.programId}</strong>
              {#if node.direction}<span class="badge">{node.direction}</span>{/if}
              {#if node.depth}<span class="muted"> (depth {node.depth})</span>{/if}
            </li>
          {/each}
        </ul>
      {/if}
    {:else if activeTab === 'data'}
      <h3>Data Items ({dataItems.length})</h3>
      {#if dataItems.length === 0}
        <p class="muted">No data items</p>
      {:else}
        <div class="data-table">
          <table>
            <thead>
              <tr><th>Level</th><th>Name</th><th>Type</th><th>Size</th></tr>
            </thead>
            <tbody>
              {#each dataItems as item}
                <tr>
                  <td>{item.level || ''}</td>
                  <td>{item.name}</td>
                  <td>{item.dataType || ''}</td>
                  <td>{item.size || ''}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    {/if}
  {/if}
</div>

<style>
  .detail h2 {
    font-size: 18px;
    font-weight: 600;
    color: #58a6ff;
  }

  .filepath {
    font-size: 12px;
    color: #8b949e;
    font-family: monospace;
    margin-bottom: 12px;
  }

  .tabs {
    display: flex;
    gap: 4px;
    margin-bottom: 16px;
    border-bottom: 1px solid #21262d;
    padding-bottom: 4px;
  }

  .tabs button {
    background: none;
    border: none;
    color: #8b949e;
    padding: 6px 12px;
    cursor: pointer;
    font-size: 13px;
    border-bottom: 2px solid transparent;
  }

  .tabs button.active {
    color: #58a6ff;
    border-bottom-color: #58a6ff;
  }

  .info-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
    gap: 8px;
    margin-bottom: 16px;
  }

  .info-item {
    background: #161b22;
    border-radius: 4px;
    padding: 8px;
  }

  .lbl {
    display: block;
    font-size: 11px;
    color: #8b949e;
    text-transform: uppercase;
    margin-bottom: 2px;
  }

  h3 {
    font-size: 14px;
    margin: 12px 0 6px;
    color: #e1e4e8;
  }

  .item-list {
    list-style: none;
    font-size: 13px;
  }

  .item-list li {
    padding: 4px 0;
    border-bottom: 1px solid #21262d;
  }

  .badge {
    background: #1f2937;
    color: #79c0ff;
    padding: 1px 6px;
    border-radius: 8px;
    font-size: 11px;
    margin-left: 6px;
  }

  .data-table {
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 12px;
  }

  th {
    text-align: left;
    padding: 6px 8px;
    border-bottom: 1px solid #30363d;
    color: #8b949e;
    font-weight: 500;
  }

  td {
    padding: 4px 8px;
    border-bottom: 1px solid #21262d;
  }

  .muted {
    color: #8b949e;
  }
</style>
