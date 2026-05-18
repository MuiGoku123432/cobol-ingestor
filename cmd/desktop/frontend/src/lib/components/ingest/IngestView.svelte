<script lang="ts">
  // @ts-ignore - Wails runtime
  import { EventsOn } from 'wailsjs/runtime/runtime';
  import { usePersistedState } from '../../stores/persisted.svelte';

  let saved = usePersistedState('ingest', {
    activeTab: 'cobol' as 'cobol' | 'bw' | 'oracle',
    directory: '',
    selectedPass: 0,
    bwDirectory: '',
    bwExtensions: '',
    contentDetect: false,
  });

  let running = $state(false);
  let logs = $state<string[]>([]);
  let status = $state('idle');
  let config = $state<any>(null);

  // Load config on mount for defaults
  $effect(() => {
    (async () => {
      try {
        // @ts-ignore - Wails bindings
        config = await window.go.main.ConfigService.GetConfig();
        if (config?.bwExtensions && !saved.bwExtensions) saved.bwExtensions = config.bwExtensions;
      } catch (e) {
        console.error('Failed to load config:', e);
      }
    })();
  });

  async function pickDirectory(title: string, target: 'cobol' | 'bw') {
    try {
      // @ts-ignore - Wails bindings
      const dir = await window.go.main.IngestService.SelectDirectory(title);
      if (dir) {
        if (target === 'cobol') saved.directory = dir;
        else saved.bwDirectory = dir;
      }
    } catch (e) {
      console.error('Directory picker failed:', e);
    }
  }

  async function startIngestion() {
    if (!saved.directory) return;
    logs = [];
    status = 'running';
    running = true;
    try {
      // @ts-ignore - Wails bindings
      await window.go.main.IngestService.StartIngestion(saved.directory, saved.selectedPass, saved.contentDetect);
    } catch (e: any) {
      status = 'error';
      logs = [...logs, `Error: ${e.message || e}`];
      running = false;
    }
  }

  async function startBWIngestion() {
    if (!saved.bwDirectory) return;
    logs = [];
    status = 'running';
    running = true;
    try {
      // @ts-ignore - Wails bindings
      await window.go.main.IngestService.StartBWIngestion(saved.bwDirectory, saved.bwExtensions);
    } catch (e: any) {
      status = 'error';
      logs = [...logs, `Error: ${e.message || e}`];
      running = false;
    }
  }

  async function startOracleAnalysis() {
    logs = [];
    status = 'running';
    running = true;
    try {
      // @ts-ignore - Wails bindings
      await window.go.main.IngestService.StartOracleAnalysis();
    } catch (e: any) {
      status = 'error';
      logs = [...logs, `Error: ${e.message || e}`];
      running = false;
    }
  }

  async function cancelIngestion() {
    try {
      // @ts-ignore - Wails bindings
      await window.go.main.IngestService.CancelIngestion();
    } catch (e) {
      console.error('Cancel failed:', e);
    }
  }

  // Subscribe to Wails events
  $effect(() => {
    const unsubs = [
      EventsOn('ingest:progress', (data: any) => {
        const msg = data?.message || JSON.stringify(data);
        logs = [...logs, msg];
      }),
      EventsOn('ingest:complete', () => {
        status = 'complete';
        running = false;
        logs = [...logs, 'Ingestion complete!'];
      }),
      EventsOn('ingest:error', (data: any) => {
        status = 'error';
        running = false;
        logs = [...logs, `Error: ${data?.error || 'Unknown error'}`];
      }),
      EventsOn('ingest:cancelled', () => {
        status = 'cancelled';
        running = false;
        logs = [...logs, 'Ingestion cancelled.'];
      }),
    ];

    return () => unsubs.forEach((fn) => fn());
  });

  const passes = [
    { value: 0, label: 'All Passes' },
    { value: 1, label: 'Pass 1 - Structural' },
    { value: 2, label: 'Pass 2 - Deep Semantic' },
    { value: 3, label: 'Pass 3 - Cross-Cutting' },
    { value: 4, label: 'Pass 4 - Data Flow' },
    { value: 5, label: 'Pass 5 - Validation' },
  ];
</script>

<div class="ingest">
  <h1>Ingest</h1>

  <div class="tabs">
    <button class="tab" class:active={saved.activeTab === 'cobol'} onclick={() => (saved.activeTab = 'cobol')}>COBOL</button>
    <button class="tab" class:active={saved.activeTab === 'bw'} onclick={() => (saved.activeTab = 'bw')}>BusinessWare</button>
    <button class="tab" class:active={saved.activeTab === 'oracle'} onclick={() => (saved.activeTab = 'oracle')}>Oracle</button>
  </div>

  <div class="controls">
    {#if saved.activeTab === 'cobol'}
      <div class="field">
        <label>Source Directory</label>
        <div class="dir-picker">
          <input type="text" bind:value={saved.directory} placeholder="/path/to/cobol/sources" readonly />
          <button onclick={() => pickDirectory('Select COBOL Source Directory', 'cobol')}>Browse</button>
        </div>
      </div>

      <div class="field">
        <label>Pass</label>
        <select bind:value={saved.selectedPass}>
          {#each passes as p}
            <option value={p.value}>{p.label}</option>
          {/each}
        </select>
      </div>

      <div class="field">
        <label>
          <input type="checkbox" bind:checked={saved.contentDetect} />
          Detect COBOL in .txt files
        </label>
      </div>

      <div class="actions">
        {#if running}
          <button class="btn-danger" onclick={cancelIngestion}>Cancel</button>
        {:else}
          <button class="btn-primary" disabled={!saved.directory} onclick={startIngestion}>
            Start Ingestion
          </button>
        {/if}
        <span class="status-badge" class:running class:complete={status === 'complete'} class:error={status === 'error'}>
          {status}
        </span>
      </div>
    {:else if saved.activeTab === 'bw'}
      <div class="field">
        <label>Source Directory</label>
        <div class="dir-picker">
          <input type="text" bind:value={saved.bwDirectory} placeholder="/path/to/businessware/sources" readonly />
          <button onclick={() => pickDirectory('Select BusinessWare Source Directory', 'bw')}>Browse</button>
        </div>
      </div>

      <div class="field">
        <label>File Extensions</label>
        <input type="text" bind:value={saved.bwExtensions} placeholder=".java,.md,.bw,.txt,.xml" />
      </div>

      <div class="actions">
        {#if running}
          <button class="btn-danger" onclick={cancelIngestion}>Cancel</button>
        {:else}
          <button class="btn-primary" disabled={!saved.bwDirectory} onclick={startBWIngestion}>
            Start BW Ingestion
          </button>
        {/if}
        <span class="status-badge" class:running class:complete={status === 'complete'} class:error={status === 'error'}>
          {status}
        </span>
      </div>
    {:else if saved.activeTab === 'oracle'}
      {#if config}
        <div class="oracle-summary">
          <p><strong>Connection:</strong> {config.oracleHost || 'localhost'}:{config.oraclePort || '1521'}/{config.oracleService || '(not set)'}</p>
          <p><strong>Database:</strong> {config.extDbName || '(not set)'} ({config.extDbType || 'not set'})</p>
          <p class="hint">Configure connection details in Settings.</p>
        </div>
      {/if}

      <div class="actions">
        {#if running}
          <button class="btn-danger" onclick={cancelIngestion}>Cancel</button>
        {:else}
          <button class="btn-primary" onclick={startOracleAnalysis}>
            Run Analysis
          </button>
        {/if}
        <span class="status-badge" class:running class:complete={status === 'complete'} class:error={status === 'error'}>
          {status}
        </span>
      </div>
    {/if}
  </div>

  <div class="log-output">
    <h3>Output</h3>
    <div class="log-scroll">
      {#each logs as line}
        <div class="log-line">{line}</div>
      {/each}
      {#if logs.length === 0}
        <p class="muted">No output yet. Select a tab and start ingestion.</p>
      {/if}
    </div>
  </div>
</div>

<style>
  .ingest h1 {
    font-size: 20px;
    margin-bottom: 16px;
  }

  .tabs {
    display: flex;
    gap: 0;
    margin-bottom: 16px;
    border-bottom: 1px solid #21262d;
  }

  .tab {
    background: none;
    border: none;
    border-bottom: 2px solid transparent;
    color: #8b949e;
    padding: 8px 16px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
    transition: color 0.15s, border-color 0.15s;
  }

  .tab:hover {
    color: #e1e4e8;
  }

  .tab.active {
    color: #58a6ff;
    border-bottom-color: #58a6ff;
  }

  .controls {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 6px;
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-bottom: 16px;
  }

  .field label {
    display: block;
    font-size: 12px;
    color: #8b949e;
    margin-bottom: 4px;
  }

  .field input[type='text'] {
    width: 100%;
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 4px;
    color: #e1e4e8;
    padding: 8px;
    font-size: 13px;
  }

  .dir-picker {
    display: flex;
    gap: 6px;
  }

  .dir-picker input {
    flex: 1;
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 4px;
    color: #e1e4e8;
    padding: 8px;
    font-size: 13px;
    font-family: monospace;
  }

  .dir-picker button,
  select {
    background: #21262d;
    border: 1px solid #30363d;
    color: #e1e4e8;
    border-radius: 4px;
    padding: 8px 12px;
    cursor: pointer;
    font-size: 13px;
  }

  select {
    width: 100%;
  }

  .oracle-summary {
    font-size: 13px;
    color: #e1e4e8;
  }

  .oracle-summary p {
    margin: 4px 0;
  }

  .oracle-summary .hint {
    color: #8b949e;
    font-size: 12px;
    margin-top: 8px;
  }

  .actions {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .btn-primary {
    background: #238636;
    border: 1px solid #2ea043;
    color: #fff;
    border-radius: 6px;
    padding: 8px 20px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
  }

  .btn-primary:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .btn-danger {
    background: #da3633;
    border: 1px solid #f85149;
    color: #fff;
    border-radius: 6px;
    padding: 8px 20px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
  }

  .status-badge {
    font-size: 12px;
    padding: 2px 8px;
    border-radius: 10px;
    background: #21262d;
    color: #8b949e;
    text-transform: uppercase;
  }

  .status-badge.running {
    background: #0c2d6b;
    color: #58a6ff;
  }

  .status-badge.complete {
    background: #0f2f1a;
    color: #3fb950;
  }

  .status-badge.error {
    background: #3d1114;
    color: #f85149;
  }

  .log-output {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 6px;
    padding: 12px;
  }

  .log-output h3 {
    font-size: 13px;
    margin-bottom: 8px;
    color: #8b949e;
  }

  .log-scroll {
    height: calc(100vh - 420px);
    overflow-y: auto;
    font-family: 'SF Mono', 'Fira Code', monospace;
    font-size: 12px;
  }

  .log-line {
    padding: 2px 0;
    border-bottom: 1px solid #161b22;
    white-space: pre-wrap;
    word-break: break-all;
  }

  .muted {
    color: #8b949e;
  }
</style>
