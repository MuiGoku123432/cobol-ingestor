<script lang="ts">
  import NVL from '@neo4j-nvl/base';
  import ContextMenu from './ContextMenu.svelte';
  import CypherPanel from './CypherPanel.svelte';

  let container: HTMLDivElement;
  let minimapContainer: HTMLDivElement;
  let nvl: NVL | null = null;

  let domains: { name: string; description: string }[] = $state([]);
  let selectedDomain = $state('');
  let loading = $state(false);
  let error = $state('');

  let selectedNode: any = $state(null);
  let selectedNodeId: string | null = $state(null);
  let nodeDetail: any = $state(null);
  let detailLoading = $state(false);

  // Context menu state
  let ctxMenuVisible = $state(false);
  let ctxMenuX = $state(0);
  let ctxMenuY = $state(0);
  let ctxMenuNodeId = $state('');
  let ctxMenuNodeLabel = $state('');

  // Navigation breadcrumb trail
  let navHistory: { id: string; label: string; caption: string }[] = $state([]);

  // Minimap
  let showMinimap = $state(false);

  // Cypher panel
  let cypherVisible = $state(false);

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
      domains = await (window as any).go.main.Neo4jService.GetBusinessDomains();
    } catch {
      domains = [];
    }
  }

  function renderGraph(data: { nodes: any[]; relationships: any[] }) {
    if (nvl) {
      nvl.destroy();
      nvl = null;
    }
    if (!container) return;

    // Track labels
    for (const n of data.nodes || []) {
      nodeLabelMap[n.id] = n.label;
    }

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

    const nvlOptions: any = {
      layout: 'force-directed',
      mouseCallbacks: {
        onNodeClick: (_node: any, _nodes: any[], hit: any) => {
          if (hit) handleNodeClick(hit);
        },
        onNodeDoubleClick: (_node: any, _nodes: any[], hit: any) => {
          if (hit) handleNodeDoubleClick(hit);
        },
        onCanvasClick: () => {
          clearSelection();
          ctxMenuVisible = false;
        },
        onNodeRightClick: (_node: any, _nodes: any[], hit: any, evt: any) => {
          if (hit) handleNodeRightClick(hit, evt);
        },
      },
    };

    if (showMinimap && minimapContainer) {
      nvlOptions.minimapContainer = minimapContainer;
    }

    nvl = new NVL(container, nvlNodes, nvlRels, nvlOptions);
  }

  function clearSelection() {
    if (selectedNodeId && nvl) {
      nvl.updateElementsInGraph([{ id: selectedNodeId, activated: false }], []);
    }
    selectedNode = null;
    selectedNodeId = null;
    nodeDetail = null;
  }

  async function handleNodeClick(node: any) {
    // Clear previous selection highlight
    if (selectedNodeId && nvl) {
      nvl.updateElementsInGraph([{ id: selectedNodeId, activated: false }], []);
    }

    selectedNode = node;
    selectedNodeId = node.id;

    // Highlight selected node
    if (nvl) {
      nvl.updateElementsInGraph([{
        id: node.id,
        activated: true,
      }], []);
    }

    detailLoading = true;
    nodeDetail = null;

    try {
      const label = findNodeLabel(node.id);
      nodeDetail = await (window as any).go.main.Neo4jService.GetNodeDetail(node.id, label);
    } catch {
      nodeDetail = { error: 'Failed to load details' };
    } finally {
      detailLoading = false;
    }
  }

  async function handleNodeDoubleClick(node: any) {
    try {
      const label = findNodeLabel(node.id);
      const caption = node.captions?.[0]?.value || node.id;

      // Add to breadcrumb trail
      if (!navHistory.find((h) => h.id === node.id)) {
        navHistory = [...navHistory, { id: node.id, label, caption }];
      }

      const data = await (window as any).go.main.Neo4jService.GetNodeNeighbors(node.id, label);

      // Track new node labels
      for (const n of data.nodes || []) {
        nodeLabelMap[n.id] = n.label;
      }

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

  function handleNodeRightClick(node: any, evt: any) {
    const event = evt?.originalEvent || evt;
    if (event?.preventDefault) event.preventDefault();
    ctxMenuX = event?.clientX || 0;
    ctxMenuY = event?.clientY || 0;
    ctxMenuNodeId = node.id;
    ctxMenuNodeLabel = findNodeLabel(node.id);
    ctxMenuVisible = true;
  }

  function handleContextAction(action: string, nodeId: string, nodeLabel: string) {
    if (!nvl) return;

    switch (action) {
      case 'expand':
        handleNodeDoubleClick({ id: nodeId, captions: [{ value: nodeId }] });
        break;
      case 'details':
        handleNodeClick({ id: nodeId });
        break;
      case 'center':
        nvl.fit([nodeId]);
        break;
      case 'pin':
        nvl.pinNode(nodeId);
        break;
      case 'unpin':
        nvl.unPinNode(nodeId);
        break;
      case 'hide':
        nvl.removeNodesWithIds([nodeId]);
        if (selectedNodeId === nodeId) {
          clearSelection();
        }
        break;
    }
  }

  function navigateToBreadcrumb(id: string) {
    if (nvl) nvl.fit([id]);
  }

  // Track node labels from loaded data
  let nodeLabelMap: Record<string, string> = {};

  function findNodeLabel(id: string): string {
    return nodeLabelMap[id] || 'Program';
  }

  async function loadAndTrackGraph(domain: string) {
    loading = true;
    error = '';
    clearSelection();
    navHistory = [];

    try {
      const data = await (window as any).go.main.Neo4jService.GetCallGraph(domain);
      nodeLabelMap = {};
      renderGraph(data);
    } catch (e: any) {
      error = e?.message || String(e);
    } finally {
      loading = false;
    }
  }

  // Zoom controls
  function zoomIn() {
    if (nvl) nvl.setZoom(nvl.getScale() * 1.25);
  }

  function zoomOut() {
    if (nvl) nvl.setZoom(nvl.getScale() / 1.25);
  }

  function fitGraph() {
    if (nvl) nvl.fit();
  }

  function resetZoom() {
    if (nvl) nvl.resetZoom();
  }

  function resetGraph() {
    loadAndTrackGraph(selectedDomain);
  }

  function toggleMinimap() {
    showMinimap = !showMinimap;
    // Need to re-render to apply minimap container
    if (nvl) {
      resetGraph();
    }
  }

  function handleCypherGraph(data: { nodes: any[]; relationships: any[] }) {
    nodeLabelMap = {};
    renderGraph(data);
  }

  // Keyboard shortcuts
  function handleKeydown(e: KeyboardEvent) {
    const target = e.target as HTMLElement;
    const inInput = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.tagName === 'SELECT';

    // Ctrl/Cmd+Q — toggle Cypher panel (always)
    if ((e.ctrlKey || e.metaKey) && e.key === 'q') {
      e.preventDefault();
      cypherVisible = !cypherVisible;
      return;
    }

    // Skip other shortcuts when in input
    if (inInput) return;

    switch (e.key) {
      case 'f':
      case 'F':
        e.preventDefault();
        fitGraph();
        break;
      case '+':
      case '=':
        e.preventDefault();
        zoomIn();
        break;
      case '-':
        e.preventDefault();
        zoomOut();
        break;
      case 'Escape':
        if (ctxMenuVisible) {
          ctxMenuVisible = false;
        } else if (cypherVisible) {
          cypherVisible = false;
        } else if (selectedNode) {
          clearSelection();
        }
        break;
      case 'Delete':
      case 'Backspace':
        if (selectedNodeId && nvl) {
          nvl.removeNodesWithIds([selectedNodeId]);
          clearSelection();
        }
        break;
    }
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

<svelte:window onkeydown={handleKeydown} />

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

    {#if navHistory.length > 0}
      <div class="breadcrumbs">
        {#each navHistory as crumb, i}
          {#if i > 0}<span class="breadcrumb-sep">&rsaquo;</span>{/if}
          <button class="breadcrumb" onclick={() => navigateToBreadcrumb(crumb.id)} title={crumb.label}>
            {crumb.caption}
          </button>
        {/each}
      </div>
    {/if}

    <div class="toolbar-right">
      <button onclick={() => { cypherVisible = !cypherVisible; }} title="Cypher query (Ctrl+Q)" class:active={cypherVisible}>Cypher</button>
      <button onclick={toggleMinimap} title="Toggle minimap" class:active={showMinimap}>Map</button>
      <button onclick={fitGraph} title="Fit to view (F)">Fit</button>
      <button onclick={resetGraph} title="Reset graph">Reset</button>
    </div>
  </div>

  <div class="graph-content">
    <div class="graph-body">
      <div class="canvas-area" oncontextmenu={(e) => e.preventDefault()}>
        {#if loading}
          <div class="overlay">Loading graph...</div>
        {/if}
        {#if error}
          <div class="overlay error">{error}</div>
        {/if}
        <div class="nvl-container" bind:this={container}></div>

        <!-- Zoom controls -->
        <div class="zoom-controls">
          <button onclick={zoomIn} title="Zoom in (+)">+</button>
          <button onclick={zoomOut} title="Zoom out (-)">-</button>
          <button onclick={fitGraph} title="Fit to view (F)">&#8596;</button>
          <button onclick={resetZoom} title="Reset zoom">&#8634;</button>
        </div>

        <!-- Minimap -->
        {#if showMinimap}
          <div class="minimap" bind:this={minimapContainer}></div>
        {/if}

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
            <button class="close-btn" onclick={() => clearSelection()}>&times;</button>
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

    <CypherPanel bind:visible={cypherVisible} onrungraph={handleCypherGraph} />
  </div>
</div>

<ContextMenu
  x={ctxMenuX}
  y={ctxMenuY}
  nodeId={ctxMenuNodeId}
  nodeLabel={ctxMenuNodeLabel}
  bind:visible={ctxMenuVisible}
  onaction={handleContextAction}
/>

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

  .toolbar button.active {
    background: #58a6ff;
    color: #0d1117;
    border-color: #58a6ff;
  }

  /* Breadcrumbs */
  .breadcrumbs {
    display: flex;
    align-items: center;
    gap: 4px;
    flex: 1;
    min-width: 0;
    padding: 0 12px;
    overflow-x: auto;
  }

  .breadcrumb {
    background: none;
    border: none;
    color: #58a6ff;
    font-size: 12px;
    cursor: pointer;
    padding: 2px 4px;
    border-radius: 3px;
    white-space: nowrap;
  }

  .breadcrumb:hover {
    background: #21262d;
  }

  .breadcrumb-sep {
    color: #484f58;
    font-size: 14px;
  }

  /* Layout */
  .graph-content {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-height: 0;
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

  /* Zoom controls */
  .zoom-controls {
    position: absolute;
    bottom: 50px;
    right: 12px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    z-index: 10;
  }

  .zoom-controls button {
    width: 32px;
    height: 32px;
    background: #21262d;
    color: #e1e4e8;
    border: 1px solid #30363d;
    border-radius: 4px;
    font-size: 16px;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .zoom-controls button:hover {
    background: #30363d;
  }

  /* Minimap */
  .minimap {
    position: absolute;
    bottom: 50px;
    right: 56px;
    width: 150px;
    height: 100px;
    background: rgba(13, 17, 23, 0.85);
    border: 1px solid #30363d;
    border-radius: 4px;
    z-index: 10;
    overflow: hidden;
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
