<script lang="ts">
  // @ts-ignore - Wails runtime
  import { EventsOn } from 'wailsjs/runtime/runtime';

  let directory = $state('');
  let selectedPass = $state(0);
  let running = $state(false);
  let logs = $state<string[]>([]);
  let status = $state('idle');

  async function pickDirectory() {
    try {
      // @ts-ignore - Wails bindings
      const dir = await window.go.main.IngestService.SelectDirectory();
      if (dir) directory = dir;
    } catch (e) {
      console.error('Directory picker failed:', e);
    }
  }

  async function startIngestion() {
    if (!directory) return;
    logs = [];
    status = 'running';
    running = true;
    try {
      // @ts-ignore - Wails bindings
      await window.go.main.IngestService.StartIngestion(directory, selectedPass);
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
  <h1>Ingest COBOL Codebase</h1>

  <div class="controls">
    <div class="field">
      <label>Source Directory</label>
      <div class="dir-picker">
        <input type="text" bind:value={directory} placeholder="/path/to/cobol/sources" readonly />
        <button onclick={pickDirectory}>Browse</button>
      </div>
    </div>

    <div class="field">
      <label>Pass</label>
      <select bind:value={selectedPass}>
        {#each passes as p}
          <option value={p.value}>{p.label}</option>
        {/each}
      </select>
    </div>

    <div class="actions">
      {#if running}
        <button class="btn-danger" onclick={cancelIngestion}>Cancel</button>
      {:else}
        <button class="btn-primary" disabled={!directory} onclick={startIngestion}>
          Start Ingestion
        </button>
      {/if}
      <span class="status-badge" class:running class:complete={status === 'complete'} class:error={status === 'error'}>
        {status}
      </span>
    </div>
  </div>

  <div class="log-output">
    <h3>Output</h3>
    <div class="log-scroll">
      {#each logs as line}
        <div class="log-line">{line}</div>
      {/each}
      {#if logs.length === 0}
        <p class="muted">No output yet. Select a directory and start ingestion.</p>
      {/if}
    </div>
  </div>
</div>

<style>
  .ingest h1 {
    font-size: 20px;
    margin-bottom: 16px;
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
    height: calc(100vh - 380px);
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
