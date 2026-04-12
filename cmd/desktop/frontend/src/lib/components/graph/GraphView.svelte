<script lang="ts">
  import Graph from 'graphology';
  import Sigma from 'sigma';
  import FA2Layout from 'graphology-layout-forceatlas2/worker';
  import ContextMenu from './ContextMenu.svelte';
  import CypherPanel from './CypherPanel.svelte';
  import { usePersistedState } from '../../stores/persisted.svelte';
  import { onMount, onDestroy } from 'svelte';

  let saved = usePersistedState('graph', {
    selectedDomain: '',
    cypherVisible: false,
  });

  let container: HTMLDivElement;

  // Non-reactive: Sigma and Graphology must NOT be wrapped in $state —
  // Svelte 5 proxies would break Graphology's internal mutation tracking.
  let graph: Graph | null = null;
  let renderer: Sigma | null = null;
  let fa2: FA2Layout | null = null;
  let layoutTimer: ReturnType<typeof setTimeout> | null = null;

  // Focus mode: plain let so the reducer closure reads the current value at render time.
  let focusedNodeId: string | null = null;
  let nodeLabelMap: Record<string, string> = {};

  // Reactive UI state
  let domains: { name: string; description: string }[] = $state([]);
  let loading = $state(false);
  let error = $state('');
  let selectedNodeId: string | null = $state(null);
  let selectedNodeCaption = $state('');
  let nodeDetail: any = $state(null);
  let detailLoading = $state(false);
  let ctxMenuVisible = $state(false);
  let ctxMenuX = $state(0);
  let ctxMenuY = $state(0);
  let ctxMenuNodeId = $state('');
  let ctxMenuNodeLabel = $state('');

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

  function buildGraph(data: { nodes: any[]; relationships: any[] }): Graph {
    const g = new Graph({ multi: true, type: 'directed' });
    nodeLabelMap = {};

    // Seed initial positions by node type — each label gets its own region
    // with random jitter. This mimics how Bloom groups nodes before force
    // simulation, giving FA2 a head start toward organic clusters.
    const typeOffsets: Record<string, { cx: number; cy: number }> = {
      Program:        { cx:    0, cy:    0 },   // center — the hub type
      Copybook:       { cx: -400, cy: -200 },
      Paragraph:      { cx:  400, cy: -200 },
      BusinessDomain: { cx:    0, cy: -450 },
      JCLJob:         { cx:    0, cy:  450 },
    };
    const spread = 200; // jitter radius within each group

    (data.nodes || []).forEach((node: any) => {
      if (g.hasNode(node.id)) return;
      const off = typeOffsets[node.label] ?? { cx: 0, cy: 0 };
      g.addNode(node.id, {
        label: node.caption,
        x: off.cx + (Math.random() - 0.5) * spread * 2,
        y: off.cy + (Math.random() - 0.5) * spread * 2,
        size: Math.max(node.size / 5, 3),
        color: node.color,
        nodeLabel: node.label,
      });
      nodeLabelMap[node.id] = node.label;
    });

    for (const rel of data.relationships || []) {
      if (!g.hasNode(rel.from) || !g.hasNode(rel.to)) continue;
      try {
        g.addDirectedEdge(rel.from, rel.to, {
          label: rel.caption,
          size: 1,
          color: '#484f58',
        });
      } catch {
        // graphology throws on parallel edges for non-multi graphs; safe to ignore.
      }
    }

    return g;
  }

  function startLayout(g: Graph) {
    if (fa2) { fa2.kill(); fa2 = null; }
    if (layoutTimer) { clearTimeout(layoutTimer); layoutTimer = null; }

    // Settings tuned to mimic Neo4j Bloom's force-directed layout:
    // - linLogMode: logarithmic attraction creates well-separated organic clusters
    //   (this is the single biggest factor for Bloom-like hub-and-spoke patterns)
    // - outboundAttractionDistribution: hub nodes sit at the center of their cluster
    //   instead of being pulled toward the periphery
    // - moderate gravity + no strongGravityMode: lets clusters breathe apart
    fa2 = new FA2Layout(g, {
      settings: {
        linLogMode: true,
        outboundAttractionDistribution: true,
        gravity: 0.5,
        scalingRatio: 2,
        strongGravityMode: false,
        barnesHutOptimize: true,
        barnesHutTheta: 0.5,
        slowDown: 2,
      },
    });
    fa2.start();

    // LinLog mode needs a bit longer to converge than standard FA2.
    layoutTimer = setTimeout(() => {
      if (fa2?.isRunning()) fa2.stop();
      renderer?.getCamera().animatedReset({ duration: 500 });
    }, 5000);
  }

  // nodeReducer and edgeReducer: called by Sigma on every render frame.
  // They close over `focusedNodeId` and `graph` (plain lets) — reading current values.
  function nodeReducer(node: string, data: any) {
    if (!focusedNodeId || !graph) return data;
    if (node === focusedNodeId) {
      return { ...data, highlighted: true, zIndex: 2 };
    }
    if (graph.areNeighbors(focusedNodeId, node)) {
      return { ...data, zIndex: 1 };
    }
    // Dim everything that is not the focused node or its neighbors.
    return { ...data, color: '#21262d', label: undefined, zIndex: 0 };
  }

  function edgeReducer(edge: string, data: any) {
    if (!focusedNodeId || !graph) return { ...data, label: undefined };
    const src = graph.source(edge);
    const tgt = graph.target(edge);
    if (src === focusedNodeId || tgt === focusedNodeId) {
      return { ...data, size: 2, color: '#58a6ff' };
    }
    return { ...data, hidden: true };
  }

  function initRenderer(g: Graph) {
    if (renderer) { renderer.kill(); renderer = null; }
    if (!container) return;

    renderer = new Sigma(g, container, {
      defaultNodeColor: '#8b949e',
      defaultEdgeColor: '#484f58',

      // Labels: only show when nodes are large enough on screen, and cap the
      // density so zoomed-out views stay readable instead of a label soup.
      labelColor: { color: '#e1e4e8' },
      labelRenderedSizeThreshold: 8,
      labelDensity: 1,          // roughly 1 label per grid cell
      labelGridCellSize: 120,   // px — larger cells = fewer labels

      // Edge labels appear when zoomed in (renderEdgeLabels + threshold);
      // the edgeReducer still hides unrelated edges in focus mode.
      renderEdgeLabels: true,
      edgeLabelColor: { color: '#6e7681' },
      edgeLabelSize: 10,
      edgeLabelRenderedSizeThreshold: 12,

      // Zoom bounds: prevent losing context when fully zoomed out and
      // keep meaningful detail when fully zoomed in.
      minCameraRatio: 0.02,     // max zoom-in  (~50×)
      maxCameraRatio: 8,        // max zoom-out (~0.125×)

      // Sigma's built-in mouse-wheel zoom + click-drag pan are on by default.
      // zoomDuration controls the animated scroll-wheel zoom smoothness.
      zoomDuration: 200,

      nodeReducer,
      edgeReducer,
    });

    renderer.on('clickNode', ({ node }: { node: string }) => handleNodeClick(node));
    renderer.on('clickStage', () => clearFocus());
    renderer.on('rightClickNode', ({ node, event }: { node: string; event: any }) => {
      const native = event?.original ?? event;
      if (native?.preventDefault) native.preventDefault();
      ctxMenuX = native?.clientX ?? 0;
      ctxMenuY = native?.clientY ?? 0;
      ctxMenuNodeId = node;
      ctxMenuNodeLabel = nodeLabelMap[node] || 'Program';
      ctxMenuVisible = true;
    });
  }

  async function loadFullGraph(domain: string) {
    loading = true;
    error = '';
    clearFocus();

    try {
      const data = await (window as any).go.main.Neo4jService.GetFullGraph(domain);
      const g = buildGraph(data);
      graph = g;
      initRenderer(g);
      startLayout(g);
    } catch (e: any) {
      error = e?.message || String(e);
    } finally {
      loading = false;
    }
  }

  async function handleNodeClick(nodeId: string) {
    focusedNodeId = nodeId;
    renderer?.refresh();

    selectedNodeId = nodeId;
    selectedNodeCaption = graph?.getNodeAttribute(nodeId, 'label') ?? nodeId;

    detailLoading = true;
    nodeDetail = null;

    try {
      const label = nodeLabelMap[nodeId] || 'Program';
      nodeDetail = await (window as any).go.main.Neo4jService.GetNodeDetail(nodeId, label);
    } catch {
      nodeDetail = { error: 'Failed to load details' };
    } finally {
      detailLoading = false;
    }
  }

  function clearFocus() {
    focusedNodeId = null;
    renderer?.refresh();
    selectedNodeId = null;
    selectedNodeCaption = '';
    nodeDetail = null;
    ctxMenuVisible = false;
  }

  function handleContextAction(action: string, nodeId: string, _nodeLabel: string) {
    switch (action) {
      case 'focus':
        handleNodeClick(nodeId);
        break;
      case 'details':
        handleNodeClick(nodeId);
        break;
      case 'center': {
        if (graph?.hasNode(nodeId)) {
          const x = graph.getNodeAttribute(nodeId, 'x') as number;
          const y = graph.getNodeAttribute(nodeId, 'y') as number;
          renderer?.getCamera().animate({ x, y, ratio: 0.3 }, { duration: 400 });
        }
        break;
      }
      case 'hide':
        if (graph?.hasNode(nodeId)) {
          graph.dropNode(nodeId);
          renderer?.refresh();
          if (selectedNodeId === nodeId) clearFocus();
        }
        break;
    }
  }

  function handleCypherGraph(data: { nodes: any[]; relationships: any[] }) {
    const g = buildGraph(data);
    graph = g;
    focusedNodeId = null;
    if (renderer) {
      renderer.setGraph(g);
      renderer.refresh();
    } else {
      initRenderer(g);
    }
    startLayout(g);
  }

  function zoomIn()  { renderer?.getCamera().animatedZoom({ duration: 200 }); }
  function zoomOut() { renderer?.getCamera().animatedUnzoom({ duration: 200 }); }
  function fitGraph() { renderer?.getCamera().animatedReset({ duration: 300 }); }
  function resetGraph() { loadFullGraph(saved.selectedDomain); }

  function handleKeydown(e: KeyboardEvent) {
    const target = e.target as HTMLElement;
    const inInput = ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName);

    if ((e.ctrlKey || e.metaKey) && e.key === 'q') {
      e.preventDefault();
      saved.cypherVisible = !saved.cypherVisible;
      return;
    }
    if (inInput) return;

    switch (e.key) {
      case 'f': case 'F': e.preventDefault(); fitGraph(); break;
      case '+': case '=': e.preventDefault(); zoomIn(); break;
      case '-': e.preventDefault(); zoomOut(); break;
      case 'Escape':
        if (ctxMenuVisible) ctxMenuVisible = false;
        else if (saved.cypherVisible) saved.cypherVisible = false;
        else if (focusedNodeId) clearFocus();
        break;
      case 'Delete': case 'Backspace':
        if (focusedNodeId && graph?.hasNode(focusedNodeId)) {
          graph.dropNode(focusedNodeId);
          renderer?.refresh();
          clearFocus();
        }
        break;
    }
  }

  onMount(() => {
    loadDomains();
    loadFullGraph(saved.selectedDomain);
  });

  onDestroy(() => {
    if (layoutTimer) clearTimeout(layoutTimer);
    fa2?.kill();
    renderer?.kill();
  });
</script>

<svelte:window onkeydown={handleKeydown} />

<div class="graph-view">
  <div class="toolbar">
    <div class="toolbar-left">
      <label for="domain-filter">Domain:</label>
      <select
        id="domain-filter"
        bind:value={saved.selectedDomain}
        onchange={() => loadFullGraph(saved.selectedDomain)}
      >
        <option value="">All</option>
        {#each domains as d}
          <option value={d.name}>{d.name}</option>
        {/each}
      </select>
    </div>

    {#if selectedNodeId}
      <div class="focus-badge">
        <span>Focused: {selectedNodeCaption || selectedNodeId}</span>
        <button class="clear-focus" onclick={clearFocus} title="Clear focus (Esc)">&times;</button>
      </div>
    {/if}

    <div class="toolbar-right">
      <button onclick={() => { saved.cypherVisible = !saved.cypherVisible; }} title="Cypher query (Ctrl+Q)" class:active={saved.cypherVisible}>Cypher</button>
      <button onclick={fitGraph} title="Fit to view (F)">Fit</button>
      <button onclick={resetGraph} title="Reload full graph">Reset</button>
    </div>
  </div>

  <div class="graph-content">
    <div class="graph-body">
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="canvas-area" oncontextmenu={(e) => e.preventDefault()}>
        {#if loading}
          <div class="overlay">Loading graph...</div>
        {/if}
        {#if error}
          <div class="overlay error">{error}</div>
        {/if}

        <div class="sigma-container" bind:this={container}></div>

        <div class="zoom-controls">
          <button onclick={zoomIn} title="Zoom in (+)">+</button>
          <button onclick={zoomOut} title="Zoom out (-)">−</button>
          <button onclick={fitGraph} title="Fit to view (F)">&#8596;</button>
        </div>

        <div class="legend">
          {#each Object.entries(colorMap) as [label, color]}
            <span class="legend-item">
              <span class="legend-dot" style="background:{color}"></span>
              {label}
            </span>
          {/each}
        </div>
      </div>

      {#if selectedNodeId}
        <aside class="detail-panel">
          <div class="detail-header">
            <h3>{selectedNodeCaption || selectedNodeId}</h3>
            <button class="close-btn" onclick={clearFocus}>&times;</button>
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
                        {(value as any[]).length} items
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

    <CypherPanel bind:visible={saved.cypherVisible} onrungraph={handleCypherGraph} />
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
  }

  .toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    background: #0d1117;
    border-bottom: 1px solid #21262d;
    flex-shrink: 0;
    gap: 12px;
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

  .toolbar button:hover { background: #30363d; }

  .toolbar button.active {
    background: #58a6ff;
    color: #0d1117;
    border-color: #58a6ff;
  }

  .focus-badge {
    display: flex;
    align-items: center;
    gap: 6px;
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 4px;
    padding: 3px 8px;
    font-size: 12px;
    color: #58a6ff;
    flex: 1;
    min-width: 0;
    overflow: hidden;
  }

  .focus-badge span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .clear-focus {
    background: none;
    border: none;
    color: #8b949e;
    cursor: pointer;
    font-size: 16px;
    line-height: 1;
    padding: 0;
    flex-shrink: 0;
  }

  .clear-focus:hover { color: #e1e4e8; }

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
  }

  .canvas-area {
    flex: 1;
    position: relative;
    min-width: 0;
  }

  .sigma-container {
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
    pointer-events: none;
  }

  .overlay.error { color: #f85149; }

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

  .zoom-controls button:hover { background: #30363d; }

  .legend {
    position: absolute;
    bottom: 12px;
    left: 12px;
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
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
    flex-shrink: 0;
  }

  .close-btn:hover { color: #e1e4e8; }

  .detail-body {
    padding: 12px;
    font-size: 13px;
  }

  .muted { color: #8b949e; }

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

  /* Sigma injects a canvas — ensure it fills the container */
  :global(.sigma-container canvas) {
    display: block;
  }
</style>
