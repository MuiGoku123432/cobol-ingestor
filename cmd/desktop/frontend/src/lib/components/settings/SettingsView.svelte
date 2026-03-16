<script lang="ts">
  import { refreshNeo4jStatus, refreshAuthStatus } from '../../stores/status';

  let config = $state<any>({});
  let loading = $state(true);
  let saving = $state(false);
  let message = $state('');
  let testingNeo4j = $state(false);
  let dockerStatus = $state<any>(null);

  async function loadConfig() {
    loading = true;
    try {
      // @ts-ignore
      config = await window.go.main.ConfigService.GetConfig();
    } catch (e: any) {
      message = `Failed to load config: ${e.message || e}`;
    } finally {
      loading = false;
    }
  }

  async function saveConfig() {
    saving = true;
    message = '';
    try {
      // @ts-ignore
      await window.go.main.ConfigService.SaveConfig(config);
      message = 'Configuration saved!';
      refreshAuthStatus();
    } catch (e: any) {
      message = `Save failed: ${e.message || e}`;
    } finally {
      saving = false;
    }
  }

  async function testNeo4j() {
    testingNeo4j = true;
    message = '';
    try {
      // @ts-ignore
      await window.go.main.Neo4jService.TestConnection(
        config.neo4jUri, config.neo4jUser, config.neo4jPassword, config.neo4jDatabase
      );
      message = 'Neo4j connection successful!';
    } catch (e: any) {
      message = `Neo4j test failed: ${e.message || e}`;
    } finally {
      testingNeo4j = false;
    }
  }

  async function connectNeo4j() {
    message = '';
    try {
      // @ts-ignore
      await window.go.main.Neo4jService.Connect(
        config.neo4jUri, config.neo4jUser, config.neo4jPassword, config.neo4jDatabase
      );
      message = 'Neo4j connected!';
      refreshNeo4jStatus();
    } catch (e: any) {
      message = `Connect failed: ${e.message || e}`;
    }
  }

  async function checkDocker() {
    try {
      // @ts-ignore
      dockerStatus = await window.go.main.Neo4jService.CheckDocker();
    } catch {
      dockerStatus = null;
    }
  }

  async function startDocker() {
    message = 'Starting Neo4j via Docker...';
    try {
      // @ts-ignore
      await window.go.main.Neo4jService.StartDocker();
      message = 'Neo4j Docker container started!';
      checkDocker();
    } catch (e: any) {
      message = `Docker start failed: ${e.message || e}`;
    }
  }

  $effect(() => {
    loadConfig();
    checkDocker();
  });

  const providers = [
    { value: 'anthropic', label: 'Anthropic (Direct)' },
    { value: 'copilot', label: 'GitHub Copilot' },
    { value: 'vertex', label: 'Google Vertex AI' },
    { value: 'bedrock', label: 'AWS Bedrock' },
    { value: 'openai', label: 'OpenAI' },
  ];
</script>

<div class="settings">
  <h1>Settings</h1>

  {#if loading}
    <p class="muted">Loading configuration...</p>
  {:else}
    {#if message}
      <div class="message-bar" class:success={message.includes('success') || message.includes('saved') || message.includes('connected') || message.includes('started')}>
        {message}
      </div>
    {/if}

    <!-- LLM Provider -->
    <section>
      <h2>LLM Provider</h2>
      <div class="field">
        <label>Provider</label>
        <select bind:value={config.llmProvider}>
          {#each providers as p}
            <option value={p.value}>{p.label}</option>
          {/each}
        </select>
      </div>

      {#if config.llmProvider === 'anthropic'}
        <div class="field">
          <label>Anthropic API Key</label>
          <input type="password" bind:value={config.anthropicApiKey} placeholder="sk-ant-..." />
        </div>
      {:else if config.llmProvider === 'copilot'}
        <div class="field">
          <label>Account Type</label>
          <select bind:value={config.copilotAccountType}>
            <option value="individual">Individual</option>
            <option value="business">Business</option>
            <option value="enterprise">Enterprise</option>
          </select>
        </div>
      {:else if config.llmProvider === 'vertex'}
        <div class="field">
          <label>Project ID</label>
          <input type="text" bind:value={config.vertexProjectId} />
        </div>
        <div class="field">
          <label>Region</label>
          <input type="text" bind:value={config.vertexRegion} placeholder="us-east5" />
        </div>
      {:else if config.llmProvider === 'bedrock'}
        <div class="field">
          <label>Region</label>
          <input type="text" bind:value={config.bedrockRegion} placeholder="us-east-1" />
        </div>
      {:else if config.llmProvider === 'openai'}
        <div class="field">
          <label>API Key</label>
          <input type="password" bind:value={config.openaiApiKey} />
        </div>
        <div class="field">
          <label>Base URL (optional)</label>
          <input type="text" bind:value={config.openaiBaseUrl} placeholder="https://api.openai.com/v1" />
        </div>
        <div class="field">
          <label>Model</label>
          <input type="text" bind:value={config.openaiModel} placeholder="gpt-4o" />
        </div>
      {/if}
    </section>

    <!-- Neo4j -->
    <section>
      <h2>Neo4j Database</h2>
      <div class="field-row">
        <div class="field flex-2">
          <label>URI</label>
          <input type="text" bind:value={config.neo4jUri} placeholder="bolt://localhost:7687" />
        </div>
        <div class="field">
          <label>Database</label>
          <input type="text" bind:value={config.neo4jDatabase} placeholder="cobol" />
        </div>
      </div>
      <div class="field-row">
        <div class="field">
          <label>User</label>
          <input type="text" bind:value={config.neo4jUser} />
        </div>
        <div class="field">
          <label>Password</label>
          <input type="password" bind:value={config.neo4jPassword} />
        </div>
      </div>
      <div class="btn-row">
        <button class="btn-secondary" onclick={testNeo4j} disabled={testingNeo4j}>
          {testingNeo4j ? 'Testing...' : 'Test Connection'}
        </button>
        <button class="btn-primary" onclick={connectNeo4j}>Connect</button>
        {#if dockerStatus}
          {#if dockerStatus.dockerAvailable && !dockerStatus.containerRunning}
            <button class="btn-secondary" onclick={startDocker}>Start Docker Neo4j</button>
          {:else if dockerStatus.containerRunning}
            <span class="docker-ok">Docker Neo4j running</span>
          {/if}
        {/if}
      </div>
    </section>

    <!-- Ingestion -->
    <section>
      <h2>Ingestion</h2>
      <div class="field-row">
        <div class="field">
          <label>Max Workers</label>
          <input type="number" bind:value={config.maxWorkers} min="1" max="50" />
        </div>
        <div class="field">
          <label>Batch Size</label>
          <input type="number" bind:value={config.batchSize} min="1" />
        </div>
        <div class="field">
          <label>Token Limit</label>
          <input type="number" bind:value={config.tokenLimit} min="1000" />
        </div>
      </div>
      <div class="field">
        <label>
          <input type="checkbox" bind:checked={config.stripSeqColumns} />
          Strip sequence columns (cols 1-6 and 73-80)
        </label>
      </div>
    </section>

    <!-- Chat -->
    <section>
      <h2>Chat / Modernize</h2>
      <div class="field-row">
        <div class="field">
          <label>Chat Model</label>
          <input type="text" bind:value={config.chatModel} placeholder="claude-opus-4-6" />
        </div>
        <div class="field">
          <label>Max Tokens</label>
          <input type="number" bind:value={config.chatMaxTokens} min="1000" />
        </div>
      </div>
    </section>

    <div class="save-row">
      <button class="btn-primary" onclick={saveConfig} disabled={saving}>
        {saving ? 'Saving...' : 'Save Configuration'}
      </button>
    </div>
  {/if}
</div>

<style>
  .settings {
    max-width: 700px;
  }

  .settings h1 {
    font-size: 20px;
    margin-bottom: 20px;
  }

  section {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 6px;
    padding: 16px;
    margin-bottom: 16px;
  }

  section h2 {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 12px;
    color: #58a6ff;
  }

  .field {
    margin-bottom: 8px;
  }

  .field label {
    display: block;
    font-size: 12px;
    color: #8b949e;
    margin-bottom: 4px;
  }

  .field input[type='text'],
  .field input[type='password'],
  .field input[type='number'],
  .field select {
    width: 100%;
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 4px;
    color: #e1e4e8;
    padding: 8px;
    font-size: 13px;
  }

  .field-row {
    display: flex;
    gap: 12px;
  }

  .field-row .field {
    flex: 1;
  }

  .flex-2 {
    flex: 2 !important;
  }

  .btn-row {
    display: flex;
    gap: 8px;
    margin-top: 8px;
    align-items: center;
  }

  .btn-primary {
    background: #238636;
    border: 1px solid #2ea043;
    color: #fff;
    border-radius: 6px;
    padding: 8px 16px;
    cursor: pointer;
    font-size: 13px;
    font-weight: 500;
  }

  .btn-primary:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .btn-secondary {
    background: #21262d;
    border: 1px solid #30363d;
    color: #e1e4e8;
    border-radius: 6px;
    padding: 8px 16px;
    cursor: pointer;
    font-size: 13px;
  }

  .btn-secondary:disabled {
    opacity: 0.5;
    cursor: default;
  }

  .save-row {
    margin-top: 16px;
  }

  .message-bar {
    background: #3d1114;
    color: #f85149;
    border: 1px solid #f85149;
    border-radius: 6px;
    padding: 8px 12px;
    margin-bottom: 16px;
    font-size: 13px;
  }

  .message-bar.success {
    background: #0f2f1a;
    color: #3fb950;
    border-color: #238636;
  }

  .docker-ok {
    font-size: 12px;
    color: #3fb950;
  }

  .muted {
    color: #8b949e;
  }
</style>
