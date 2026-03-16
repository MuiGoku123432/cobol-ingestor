<script lang="ts">
  import NVL from '@neo4j-nvl/base';

  let container: HTMLDivElement;
  let nvl: NVL | null = null;

  let domains: { name: string; description: string }[] = $state([]);
  let selectedDomain = $state('');
  let loading = $state(false);
  let error = $state('');

  let selectedNode: any = $state(null);
  let nodeDetail: any = $state(null);
  let detailLoading = $state(false);

  const colorMap: Record<string, string> = {
    Program: '#238636',
    Copybook: '#58a6ff',
    DataItem: '#8b949e',
    Paragraph: '#d2a8ff',
    BusinessDomain: '#f0883e',
    JCLJob: '#f85149',
  };

  async function loadDomains() {
    try {
      domains = await window.go.main.Neo4jService.GetBusinessDomains();
    } catch {
      domains = [];
    }
  }

  async function loadGraph(domain: string) {
    loading = true;
    error = '';
    selectedNode = null;
    nodeDetail = null;

    try {
      const data = await window.go.main.Neo4jService.GetCallGraph(domain);
      renderGraph(data);
    } catch (e: any) {
      error = e?.message || String(e);
    } finally {
      loading = false;
    }
  }

  function renderGraph(data: { nodes: any[]; relationships: any[] }) {
    if (nvl) {
      nvl.destroy();
      nvl = null;
    }
    if (!container) return;

    const nvlNodes = (data.nodes || []).map((n: any) => ({
      id: n.id,
      size: n.size,
      color: n.color,
      captions: [{ value: n.caption }],
    }));

    const nvlRels = (data.relationships || []).map((r: any) => ({
      id: r.id,
      from: r.from,
      to: r.to,
      captions: [{ value: r.caption }],
    }));

    nvl = new NVL(container, nvlNodes, nvlRels, {
      layout: 'force-directed',
      mouseCallbacks: {
        onNodeClick: (_node: any, _nodes: any[], hit: any) => {
          if (hit) handleNodeClick(hit);
        },
        onNodeDoubleClick: (_node: any, _nodes: any[], hit: any) => {
          if (hit) handleNodeDoubleClick(hit);
        },
        onCanvasClick: () => {
          selectedNode = null;
          nodeDetail = null;
        },
      },
    });
  }

  async function handleNodeClick(node: any) {
    selectedNode = node;
    detailLoading = true;
    nodeDetail = null;

    try {
      const label = findNodeLabel(node.id);
      nodeDetail = await window.go.main.Neo4jService.GetNodeDetail(node.id, label);
    } catch {
      nodeDetail = { error: 'Failed to load details' };
    } finally {
      detailLoading = false;
    }
  }

  async function handleNodeDoubleClick(node: any) {
    try {
      const label = findNodeLabel(node.id);
      const data = await window.go.main.Neo4jService.GetNodeNeighbors(node.id, label);

      const newNodes = (data.nodes || []).map((n: any) => ({
        id: n.id,
        size: n.size,
        color: n.color,
        captions: [{ value: n.caption }],
      }));

      const newRels = (data.relationships || []).map((r: any) => ({
        id: r.id,
        from: r.from,
        to: r.to,
        captions: [{ value: r.caption }],
      }));

      if (nvl) {
        nvl.addAndUpdateElementsInGraph(newNodes, newRels);
      }
    } catch (e) {
      console.error('expand failed', e);
    }
  }

  // Track node labels from loaded data
  let nodeLabelMap: Record<string, string> = {};

  function findNodeLabel(id: string): string {
    return nodeLabelMap[id] || 'Program';
  }

  // Wrap loadGraph to also track node labels
  async function loadAndTrackGraph(domain: string) {
    loading = true;
    error = '';
    selectedNode = null;
    nodeDetail = null;

    try {
      const data = await window.go.main.Neo4jService.GetCallGraph(domain);
      nodeLabelMap = {};
      for (const n of data.nodes || []) {
        nodeLabelMap[n.id] = n.label;
      }
      renderGraph(data);
    } catch (e: any) {
      error = e?.message || String(e);
    } finally {
      loading = false;
    }
  }

  function fitGraph() {
    if (nvl) nvl.fit();
  }

  function resetGraph() {
    loadAndTrackGraph(selectedDomain);
  }

  $effect(() => {
    loadDomains();
    loadAndTrackGraph('');

    return () => {
      if (nvl) {
        nvl.destroy();
        nvl = null;
      }
    };
  });
</script>

<div class="graph-view">
  <div class="toolbar">
    <div class="toolbar-left">
      <label for="domain-filter">Domain:</label>
      <select
        id="domain-filter"
        bind:value={selectedDomain}
        onchange={() => loadAndTrackGraph(selectedDomain)}
      >
        <option value="">All Programs</option>
        {#each domains as d}
          <option value={d.name}>{d.name}</option>
        {/each}
      </select>
    </div>
    <div class="toolbar-right">
      <button onclick={fitGraph} title="Fit to view">Fit</button>
      <button onclick={resetGraph} title="Reset graph">Reset</button>
    </div>
  </div>

  <div class="graph-body">
    <div class="canvas-area">
      {#if loading}
        <div class="overlay">Loading graph...</div>
      {/if}
      {#if error}
        <div class="overlay error">{error}</div>
      {/if}
      <div class="nvl-container" bind:this={container}></div>
      <div class="legend">
        {#each Object.entries(colorMap) as [label, color]}
          <span class="legend-item">
            <span class="legend-dot" style="background:{color}"></span>
            {label}
          </span>
        {/each}
      </div>
    </div>

    {#if selectedNode}
      <aside class="detail-panel">
        <div class="detail-header">
          <h3>{selectedNode.id || 'Node Detail'}</h3>
          <button class="close-btn" onclick={() => { selectedNode = null; nodeDetail = null; }}>&times;</button>
        </div>
        <div class="detail-body">
          {#if detailLoading}
            <p class="muted">Loading...</p>
          {:else if nodeDetail}
            {#if nodeDetail.error}
              <p class="muted">{nodeDetail.error}</p>
            {:else}
              <dl>
                {#each Object.entries(nodeDetail) as [key, value]}
                  <dt>{key}</dt>
                  <dd>
                    {#if Array.isArray(value)}
                      {value.length} items
                    {:else if typeof value === 'object' && value !== null}
                      <pre>{JSON.stringify(value, null, 2)}</pre>
                    {:else}
                      {String(value)}
                    {/if}
                  </dd>
                {/each}
              </dl>
            {/if}
          {/if}
        </div>
      </aside>
    {/if}
  </div>
</div>

<style>
  .graph-view {
    display: flex;
    flex-direction: column;
    height: 100%;
    gap: 0;
  }

  .toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    background: #0d1117;
    border-bottom: 1px solid #21262d;
    flex-shrink: 0;
  }

  .toolbar-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .toolbar-right {
    display: flex;
    gap: 6px;
  }

  label {
    color: #8b949e;
    font-size: 13px;
  }

  select {
    background: #161b22;
    color: #e1e4e8;
    border: 1px solid #30363d;
    border-radius: 4px;
    padding: 4px 8px;
    font-size: 13px;
  }

  .toolbar button {
    background: #21262d;
    color: #e1e4e8;
    border: 1px solid #30363d;
    border-radius: 4px;
    padding: 4px 12px;
    font-size: 12px;
    cursor: pointer;
  }

  .toolbar button:hover {
    background: #30363d;
  }

  .graph-body {
    display: flex;
    flex: 1;
    min-height: 0;
    position: relative;
  }

  .canvas-area {
    flex: 1;
    position: relative;
    min-width: 0;
  }

  .nvl-container {
    width: 100%;
    height: 100%;
    background: #0d1117;
  }

  .overlay {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    color: #8b949e;
    font-size: 14px;
    z-index: 10;
  }

  .overlay.error {
    color: #f85149;
  }

  .legend {
    position: absolute;
    bottom: 12px;
    left: 12px;
    display: flex;
    gap: 12px;
    background: rgba(13, 17, 23, 0.85);
    padding: 6px 12px;
    border-radius: 6px;
    border: 1px solid #21262d;
    z-index: 5;
  }

  .legend-item {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    color: #8b949e;
  }

  .legend-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    display: inline-block;
  }

  .detail-panel {
    width: 300px;
    background: #0d1117;
    border-left: 1px solid #21262d;
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    overflow-y: auto;
  }

  .detail-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 10px 12px;
    border-bottom: 1px solid #21262d;
  }

  .detail-header h3 {
    font-size: 14px;
    font-weight: 600;
    color: #e1e4e8;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .close-btn {
    background: none;
    border: none;
    color: #8b949e;
    font-size: 18px;
    cursor: pointer;
    padding: 0 4px;
  }

  .close-btn:hover {
    color: #e1e4e8;
  }

  .detail-body {
    padding: 12px;
    font-size: 13px;
  }

  .muted {
    color: #8b949e;
  }

  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 4px 10px;
  }

  dt {
    color: #8b949e;
    font-size: 12px;
    text-transform: capitalize;
  }

  dd {
    color: #e1e4e8;
    font-size: 12px;
    word-break: break-word;
  }

  dd pre {
    font-size: 11px;
    background: #161b22;
    padding: 4px 6px;
    border-radius: 4px;
    overflow-x: auto;
    max-height: 120px;
    margin: 0;
  }

  :global(.nvl-container canvas) {
    background: #0d1117 !important;
  }
</style>
