<script lang="ts">
  // @ts-ignore - Wails runtime
  import { EventsOn } from 'wailsjs/runtime/runtime';
  import { usePersistedState } from '../../stores/persisted.svelte';

  interface Question {
    id: string;
    group: string;
    text: string;
    description: string;
    type: string;
    options?: { value: string; label: string; description?: string }[];
    required: boolean;
    dependsOn?: { questionId: string; values: string[] };
  }

  interface AgentState {
    id: string;
    name: string;
    status: string;
    content: string;
  }

  interface WrittenFile {
    path: string;
    name: string;
  }

  type Phase = 'questionnaire' | 'analysis' | 'results';

  let saved = usePersistedState('strategy', {
    phase: 'questionnaire' as Phase,
    answers: {} as Record<string, string[]>,
  });

  let questions = $state<Question[]>([]);
  let validationErrors = $state<string[]>([]);
  let agents = $state<AgentState[]>([]);
  let synthesisContent = $state('');
  let writtenFiles = $state<WrittenFile[]>([]);
  let outputDir = $state('');
  let errorMessage = $state('');
  let loading = $state(false);

  // Load questions on mount
  $effect(() => {
    loadQuestions();

    const unsubs = [
      EventsOn('strategy:start', () => {
        saved.phase = 'analysis';
        agents = [];
        synthesisContent = '';
        writtenFiles = [];
        errorMessage = '';
      }),
      EventsOn('strategy:agent_start', (data: any) => {
        agents = [...agents, {
          id: data?.agentId || '',
          name: data?.agentName || '',
          status: 'running',
          content: ''
        }];
      }),
      EventsOn('strategy:agent_progress', (data: any) => {
        agents = agents.map(a =>
          a.id === data?.agentId
            ? { ...a, content: a.content + (data?.content || '') }
            : a
        );
      }),
      EventsOn('strategy:agent_complete', (data: any) => {
        agents = agents.map(a =>
          a.id === data?.agentId ? { ...a, status: 'done' } : a
        );
      }),
      EventsOn('strategy:synthesis_start', () => {
        synthesisContent = '';
      }),
      EventsOn('strategy:synthesis_progress', (data: any) => {
        synthesisContent += data?.content || '';
      }),
      EventsOn('strategy:file_written', (data: any) => {
        writtenFiles = [...writtenFiles, { path: data?.path || '', name: data?.name || '' }];
      }),
      EventsOn('strategy:complete', (data: any) => {
        outputDir = data?.outputDir || '';
        saved.phase = 'results';
      }),
      EventsOn('strategy:error', (data: any) => {
        errorMessage = data?.error || 'Unknown error';
        if (saved.phase === 'analysis') {
          saved.phase = 'questionnaire';
        }
      }),
    ];

    return () => unsubs.forEach(fn => fn());
  });

  async function loadQuestions() {
    try {
      // @ts-ignore
      const qs = await window.go.main.StrategyService.GetQuestions();
      questions = qs || [];
    } catch (e: any) {
      errorMessage = `Failed to load questions: ${e.message || e}`;
    }
  }

  function shouldShow(q: Question): boolean {
    if (!q.dependsOn) return true;
    const parentAnswers = saved.answers[q.dependsOn.questionId];
    if (!parentAnswers || parentAnswers.length === 0) return false;
    return parentAnswers.some(a => q.dependsOn!.values.includes(a));
  }

  function setAnswer(id: string, value: string) {
    saved.answers = { ...saved.answers, [id]: [value] };
  }

  function toggleMulti(id: string, value: string) {
    const current = saved.answers[id] || [];
    if (current.includes(value)) {
      saved.answers = { ...saved.answers, [id]: current.filter(v => v !== value) };
    } else {
      saved.answers = { ...saved.answers, [id]: [...current, value] };
    }
  }

  function getGroups(): string[] {
    const seen = new Set<string>();
    const groups: string[] = [];
    for (const q of questions) {
      if (!seen.has(q.group)) {
        seen.add(q.group);
        groups.push(q.group);
      }
    }
    return groups;
  }

  function questionsForGroup(group: string): Question[] {
    return questions.filter(q => q.group === group);
  }

  async function startStrategy() {
    loading = true;
    validationErrors = [];
    errorMessage = '';

    try {
      // Validate
      // @ts-ignore
      const [ctx, errors] = await window.go.main.StrategyService.ValidateAnswers(saved.answers);
      if (errors && errors.length > 0) {
        validationErrors = errors;
        loading = false;
        return;
      }

      // Pick output directory
      // @ts-ignore
      const dir = await window.go.main.StrategyService.SelectOutputDirectory();
      if (!dir) {
        loading = false;
        return;
      }

      // Start
      // @ts-ignore
      await window.go.main.StrategyService.StartStrategy(saved.answers, dir);
    } catch (e: any) {
      errorMessage = e.message || String(e);
    } finally {
      loading = false;
    }
  }

  function cancelStrategy() {
    // @ts-ignore
    window.go.main.StrategyService.CancelStrategy();
  }

  function startOver() {
    saved.phase = 'questionnaire';
    agents = [];
    synthesisContent = '';
    writtenFiles = [];
    outputDir = '';
    errorMessage = '';
  }
</script>

<div class="strategy-layout">
  {#if saved.phase === 'questionnaire'}
    <div class="questionnaire">
      <h2>Migration Strategy Planner</h2>
      <p class="subtitle">Answer the questions below to generate a comprehensive migration strategy document set.</p>

      {#if errorMessage}
        <div class="error-banner">{errorMessage}</div>
      {/if}

      {#if validationErrors.length > 0}
        <div class="error-banner">
          {#each validationErrors as err}
            <div>{err}</div>
          {/each}
        </div>
      {/if}

      {#each getGroups() as group}
        <div class="question-group">
          <h3>{group}</h3>
          {#each questionsForGroup(group) as q}
            {#if shouldShow(q)}
              <div class="question">
                <label>
                  {q.text}
                  {#if q.required}<span class="required">*</span>{/if}
                </label>
                {#if q.description}
                  <p class="q-desc">{q.description}</p>
                {/if}

                {#if q.type === 'single_select' && q.options}
                  <div class="options">
                    {#each q.options as opt}
                      <button
                        class="option-btn"
                        class:selected={(saved.answers[q.id] || [])[0] === opt.value}
                        onclick={() => setAnswer(q.id, opt.value)}
                      >
                        <span class="opt-label">{opt.label}</span>
                        {#if opt.description}
                          <span class="opt-desc">{opt.description}</span>
                        {/if}
                      </button>
                    {/each}
                  </div>
                {:else if q.type === 'multi_select' && q.options}
                  <div class="options multi">
                    {#each q.options as opt}
                      <button
                        class="option-btn"
                        class:selected={(saved.answers[q.id] || []).includes(opt.value)}
                        onclick={() => toggleMulti(q.id, opt.value)}
                      >
                        <span class="opt-label">{opt.label}</span>
                      </button>
                    {/each}
                  </div>
                {:else if q.type === 'text'}
                  <input
                    type="text"
                    value={(saved.answers[q.id] || [''])[0]}
                    oninput={(e) => setAnswer(q.id, (e.target as HTMLInputElement).value)}
                    placeholder={q.description || ''}
                  />
                {:else if q.type === 'number'}
                  <input
                    type="number"
                    value={(saved.answers[q.id] || [''])[0]}
                    oninput={(e) => setAnswer(q.id, (e.target as HTMLInputElement).value)}
                    min="0"
                  />
                {/if}
              </div>
            {/if}
          {/each}
        </div>
      {/each}

      <div class="actions">
        <button class="btn-primary" onclick={startStrategy} disabled={loading}>
          {loading ? 'Starting...' : 'Generate Strategy'}
        </button>
      </div>
    </div>

  {:else if saved.phase === 'analysis'}
    <div class="analysis">
      <h2>Generating Migration Strategy</h2>

      {#if errorMessage}
        <div class="error-banner">{errorMessage}</div>
      {/if}

      <div class="agent-grid">
        {#each agents as agent}
          <div class="agent-card" class:done={agent.status === 'done'}>
            <div class="agent-header">
              <span class="agent-name">{agent.name}</span>
              <span class="agent-status">{agent.status === 'done' ? 'Done' : 'Analyzing...'}</span>
            </div>
            {#if agent.content}
              <div class="agent-content">{agent.content.slice(0, 200)}{agent.content.length > 200 ? '...' : ''}</div>
            {/if}
          </div>
        {/each}
      </div>

      {#if synthesisContent}
        <div class="synthesis">
          <h3>Coordinator Synthesis</h3>
          <div class="synthesis-content">{synthesisContent}</div>
        </div>
      {/if}

      <div class="actions">
        <button class="btn-danger" onclick={cancelStrategy}>Cancel</button>
      </div>
    </div>

  {:else if saved.phase === 'results'}
    <div class="results">
      <h2>Strategy Generated</h2>
      <p class="subtitle">Your migration strategy documents have been written to: <code>{outputDir}</code></p>

      <div class="file-list">
        {#each writtenFiles as file}
          <div class="file-item">
            <span class="file-icon">&#128196;</span>
            <span class="file-name">{file.name}</span>
          </div>
        {/each}
      </div>

      {#if synthesisContent}
        <details class="preview">
          <summary>Preview: Executive Summary</summary>
          <div class="preview-content">{synthesisContent.slice(0, 2000)}{synthesisContent.length > 2000 ? '...' : ''}</div>
        </details>
      {/if}

      <div class="actions">
        <button class="btn-primary" onclick={startOver}>Start Over</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .strategy-layout {
    height: calc(100vh - 88px);
    overflow-y: auto;
  }

  h2 {
    font-size: 18px;
    font-weight: 600;
    color: #e1e4e8;
    margin-bottom: 4px;
  }

  .subtitle {
    color: #8b949e;
    font-size: 13px;
    margin-bottom: 20px;
  }

  .error-banner {
    background: #3d1f1f;
    border: 1px solid #f85149;
    border-radius: 6px;
    padding: 10px 14px;
    color: #f85149;
    font-size: 13px;
    margin-bottom: 16px;
  }

  .question-group {
    margin-bottom: 24px;
  }

  .question-group h3 {
    font-size: 14px;
    font-weight: 600;
    color: #58a6ff;
    margin-bottom: 12px;
    padding-bottom: 6px;
    border-bottom: 1px solid #21262d;
  }

  .question {
    margin-bottom: 16px;
  }

  .question label {
    display: block;
    font-size: 13px;
    font-weight: 500;
    color: #e1e4e8;
    margin-bottom: 4px;
  }

  .required {
    color: #f85149;
    margin-left: 2px;
  }

  .q-desc {
    font-size: 12px;
    color: #8b949e;
    margin-bottom: 8px;
  }

  .options {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .option-btn {
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 6px;
    padding: 6px 12px;
    color: #c9d1d9;
    cursor: pointer;
    font-size: 12px;
    text-align: left;
    transition: border-color 0.15s, background 0.15s;
  }

  .option-btn:hover {
    border-color: #58a6ff;
  }

  .option-btn.selected {
    background: #1f3a5f;
    border-color: #58a6ff;
    color: #e1e4e8;
  }

  .opt-label {
    display: block;
    font-weight: 500;
  }

  .opt-desc {
    display: block;
    font-size: 11px;
    color: #8b949e;
    margin-top: 2px;
  }

  input[type="text"],
  input[type="number"] {
    width: 100%;
    max-width: 400px;
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 6px;
    padding: 6px 10px;
    color: #e1e4e8;
    font-size: 13px;
  }

  .actions {
    margin-top: 20px;
    padding-top: 16px;
    border-top: 1px solid #21262d;
  }

  .btn-primary {
    background: #238636;
    border: 1px solid #2ea043;
    color: #fff;
    border-radius: 6px;
    padding: 8px 20px;
    cursor: pointer;
    font-size: 13px;
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
  }

  .agent-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
    gap: 8px;
    margin-bottom: 16px;
  }

  .agent-card {
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 6px;
    padding: 10px;
    font-size: 12px;
  }

  .agent-card.done {
    border-color: #238636;
  }

  .agent-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 6px;
  }

  .agent-name {
    color: #d2a8ff;
    font-weight: 500;
  }

  .agent-status {
    color: #8b949e;
    font-size: 11px;
  }

  .agent-content {
    color: #c9d1d9;
    font-size: 11px;
    line-height: 1.4;
  }

  .synthesis {
    background: #161b22;
    border: 1px solid #d2a8ff;
    border-radius: 6px;
    padding: 12px;
    margin-bottom: 16px;
  }

  .synthesis h3 {
    font-size: 13px;
    color: #d2a8ff;
    margin-bottom: 8px;
  }

  .synthesis-content {
    font-size: 12px;
    color: #c9d1d9;
    line-height: 1.5;
    white-space: pre-wrap;
    max-height: 300px;
    overflow-y: auto;
  }

  .file-list {
    margin: 16px 0;
  }

  .file-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 0;
    font-size: 13px;
    color: #c9d1d9;
  }

  .file-icon {
    font-size: 16px;
  }

  code {
    background: #161b22;
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 12px;
    color: #79c0ff;
  }

  .preview {
    margin-top: 16px;
  }

  .preview summary {
    cursor: pointer;
    font-size: 13px;
    color: #58a6ff;
    margin-bottom: 8px;
  }

  .preview-content {
    background: #161b22;
    border: 1px solid #21262d;
    border-radius: 6px;
    padding: 12px;
    font-size: 12px;
    color: #c9d1d9;
    line-height: 1.5;
    white-space: pre-wrap;
    max-height: 400px;
    overflow-y: auto;
  }
</style>
