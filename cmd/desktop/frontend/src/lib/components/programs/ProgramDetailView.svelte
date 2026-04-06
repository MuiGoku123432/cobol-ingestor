<script lang="ts">
  import Tabs from '../shared/Tabs.svelte';
  import Badge from '../shared/Badge.svelte';
  import ExportButton from '../shared/ExportButton.svelte';

  let {
    programId,
    onBack,
  }: {
    programId: string;
    onBack: () => void;
  } = $props();

  let detail = $state<any>(null);
  let loading = $state(true);
  let activeTab = $state('overview');
  let error = $state('');

  // Lazy-loaded tab data
  let callChainDown = $state<any[]>([]);
  let callChainUp = $state<any[]>([]);
  let impact = $state<any>(null);
  let effort = $state<any>(null);
  let jclInfo = $state<any>(null);
  let tableAccess = $state<any>(null);

  const tabs = [
    { id: 'overview', label: 'Overview' },
    { id: 'callchain', label: 'Call Chain' },
  ];

  async function loadDetail() {
    loading = true;
    error = '';
    try {
      // @ts-ignore
      detail = await window.go.main.BrowserService.GetProgram(programId);
    } catch (e: any) {
      error = e.message || String(e);
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    if (programId) loadDetail();
  });

  // Lazy load tab data
  $effect(() => {
    if (!programId) return;
    const tab = activeTab;
    (async () => {
      try {
        if (tab === 'callchain' && callChainDown.length === 0) {
          // @ts-ignore
          callChainDown = await window.go.main.BrowserService.GetCallChain(programId, 'down', 3) || [];
          // @ts-ignore
          callChainUp = await window.go.main.BrowserService.GetCallChain(programId, 'up', 3) || [];
        } else if (tab === 'overview' && !impact) {
          try {
            // @ts-ignore
            impact = await window.go.main.BrowserService.GetImpactAnalysis(programId);
          } catch { /* optional */ }
          try {
            // @ts-ignore
            effort = await window.go.main.BrowserService.GetEffortEstimate(programId);
          } catch { /* optional */ }
          try {
            // @ts-ignore
            jclInfo = await window.go.main.BrowserService.GetProgramJCL(programId);
          } catch { /* optional */ }
          try {
            // @ts-ignore
            tableAccess = await window.go.main.BrowserService.GetProgramTableAccess(programId);
          } catch { /* optional */ }
        }
      } catch (e) {
        console.error(`Failed loading ${tab} data:`, e);
      }
    })();
  });

  function flattenCallChain(nodes: any[], result: any[] = [], depth = 0): any[] {
    for (const n of nodes) {
      result.push({ programId: n.programId, depth });
      if (n.children) flattenCallChain(n.children, result, depth + 1);
    }
    return result;
  }

  async function handleExport() {
    // @ts-ignore
    const dir = await window.go.main.ExportService.SelectSaveDirectory();
    if (!dir) return;
    // @ts-ignore
    const path = await window.go.main.ExportService.ExportProgramDetail(programId, dir);
    alert(`Exported to: ${path}`);
  }
</script>

<div class="program-detail">
  <div class="header">
    <button class="back-btn" onclick={onBack}>&#8592; Back</button>
    <h2>{programId}</h2>
    <ExportButton label="Export" onExport={handleExport} />
  </div>

  {#if loading}
    <p class="loading">Loading...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if detail}
    <Tabs {tabs} bind:activeTab />

    {#if activeTab === 'overview'}
      <div class="overview-grid">
        <div class="info-card">
          <h3>Properties</h3>
          <dl>
            <dt>File</dt><dd>{detail.filePath}</dd>
            <dt>Language</dt><dd>{detail.language}</dd>
            <dt>Lines</dt><dd>{detail.lineCount}</dd>
            <dt>Execution Mode</dt><dd>{detail.executionMode || '-'}</dd>
            <dt>Dead Code</dt><dd>{detail.deadCode ? 'Yes' : 'No'}</dd>
            {#if detail.riskScore > 0}
              <dt>Risk</dt><dd><Badge text={`${detail.riskScore.toFixed(1)} ${detail.riskType}`} variant="danger" /></dd>
            {/if}
            {#if detail.domains?.length}
              <dt>Domains</dt><dd>{detail.domains.join(', ')}</dd>
            {/if}
          </dl>
        </div>

        {#if effort}
          <div class="info-card">
            <h3>Effort Estimate</h3>
            <dl>
              <dt>T-Shirt Size</dt><dd><Badge text={effort.tShirtSize} variant={effort.tShirtSize} /></dd>
              <dt>Complexity</dt><dd>{effort.complexityScore}</dd>
              {#if effort.approach}
                <dt>Approach</dt><dd>{effort.approach}</dd>
              {/if}
            </dl>
          </div>
        {/if}

        {#if impact}
          <div class="info-card">
            <h3>Impact Analysis</h3>
            <dl>
              <dt>Total Affected</dt><dd>{impact.totalAffected}</dd>
              <dt>Upstream</dt><dd>{impact.upstreamPrograms?.length || 0}</dd>
              <dt>Downstream</dt><dd>{impact.downstreamPrograms?.length || 0}</dd>
              <dt>Shared Copybooks</dt><dd>{impact.sharedCopybooks?.length || 0}</dd>
            </dl>
          </div>
        {/if}

        <div class="info-card">
          <h3>Relationships</h3>
          <dl>
            <dt>Callers</dt><dd>{detail.callers?.length || 0}</dd>
            <dt>Callees</dt><dd>{detail.callees?.length || 0}</dd>
            <dt>Copybooks</dt><dd>{detail.copybooks?.length || 0}</dd>
            <dt>Paragraphs</dt><dd>{detail.paragraphs?.length || 0}</dd>
          </dl>
        </div>

        {#if jclInfo && (jclInfo.jobs?.length || jclInfo.steps?.length)}
          <div class="info-card">
            <h3>JCL References</h3>
            <dl>
              <dt>Jobs</dt><dd>{jclInfo.jobs?.join(', ') || '-'}</dd>
              <dt>Steps</dt><dd>{jclInfo.steps?.join(', ') || '-'}</dd>
            </dl>
          </div>
        {/if}

        {#if tableAccess?.tables?.length}
          <div class="info-card">
            <h3>DB Tables</h3>
            {#each tableAccess.tables as t}
              <div class="table-item">{t.name}: {t.operations?.join(', ')}</div>
            {/each}
          </div>
        {/if}
      </div>

    {:else if activeTab === 'callchain'}
      <h3>Downstream (calls)</h3>
      {#if callChainDown.length}
        <div class="chain-list">
          {#each flattenCallChain(callChainDown) as node}
            <div class="chain-node" style="padding-left: {node.depth * 20 + 8}px">
              {'  '.repeat(node.depth)}{node.depth > 0 ? '└ ' : ''}{node.programId}
            </div>
          {/each}
        </div>
      {:else}
        <p class="muted">No downstream calls</p>
      {/if}

      <h3 style="margin-top: 16px">Upstream (callers)</h3>
      {#if callChainUp.length}
        <div class="chain-list">
          {#each flattenCallChain(callChainUp) as node}
            <div class="chain-node" style="padding-left: {node.depth * 20 + 8}px">
              {node.depth > 0 ? '└ ' : ''}{node.programId}
            </div>
          {/each}
        </div>
      {:else}
        <p class="muted">No upstream callers</p>
      {/if}
    {/if}
  {/if}
</div>

<style>
  .program-detail {
    max-width: 1100px;
  }

  .header {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 16px;
  }

  .back-btn {
    background: #21262d;
    border: 1px solid #30363d;
    color: #c9d1d9;
    padding: 4px 12px;
    border-radius: 4px;
    cursor: pointer;
    font-size: 13px;
  }

  .back-btn:hover {
    background: #30363d;
  }

  h2 {
    font-size: 18px;
    font-weight: 600;
    color: #e1e4e8;
    flex: 1;
  }

  h3 {
    font-size: 14px;
    font-weight: 600;
    color: #c9d1d9;
    margin-bottom: 8px;
  }

  .overview-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 12px;
  }

  .info-card {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 8px;
    padding: 14px;
  }

  .info-card h3 {
    margin-bottom: 10px;
    font-size: 13px;
    color: #58a6ff;
  }

  dl {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: 4px 12px;
    font-size: 12px;
  }

  dt {
    color: #8b949e;
    font-weight: 500;
  }

  dd {
    color: #c9d1d9;
  }

  .chain-list {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 6px;
    padding: 8px 0;
    font-family: 'SF Mono', monospace;
    font-size: 12px;
  }

  .chain-node {
    padding: 3px 12px;
    color: #c9d1d9;
  }

  .chain-node:hover {
    background: #1c2333;
  }

  .muted {
    color: #8b949e;
    font-size: 13px;
  }

  .loading {
    color: #8b949e;
    font-size: 13px;
    padding: 20px;
  }

  .error {
    color: #f85149;
    font-size: 13px;
    padding: 20px;
  }

  .table-item {
    font-size: 12px;
    color: #c9d1d9;
    padding: 2px 0;
  }
</style>
