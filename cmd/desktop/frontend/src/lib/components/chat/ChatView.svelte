<script lang="ts">
  // @ts-ignore - Wails runtime
  import { EventsOn } from 'wailsjs/runtime/runtime';
  import { refreshAuthStatus } from '../../stores/status';
  import { usePersistedState } from '../../stores/persisted.svelte';
  import MarkdownContent from './MarkdownContent.svelte';
  interface Message {
    role: string;
    content: string;
  }

  interface AgentState {
    id: string;
    name: string;
    status: string;
    content: string;
    toolCalls: number;
    toolResults: number;
  }

  interface CoordinatorDecisionState {
    round: number;
    satisfied: boolean;
    reasoning: string;
    followUps: Record<string, string> | null;
  }

  let saved = usePersistedState('chat', {
    discoveryMode: true,
    swarmMode: false,
    multiRound: false,
    generateDiagram: false,
    unlimitedIterations: false,
    targetLang: 'Java',
    framework: 'Spring Boot',
    integrations: '',
  });

  let copiedIndex = $state(-1);

  function copyMessage(content: string, index: number) {
    navigator.clipboard.writeText(content).then(() => {
      copiedIndex = index;
      setTimeout(() => { copiedIndex = -1; }, 1500);
    });
  }

  let messages = $state<Message[]>([]);
  let input = $state('');
  let streaming = $state(false);
  let streamedContent = $state('');
  let sessionId = $state('');
  let sessions = $state<any[]>([]);
  let toolCalls = $state<any[]>([]);

  // Diagram state
  interface DiagramEntry {
    filePath: string;
    svgData: string;
  }
  let diagrams = $state<DiagramEntry[]>([]);

  // Swarm-specific state
  let agents = $state<AgentState[]>([]);
  let currentRound = $state(0);
  let maxRounds = $state(1);
  let coordinatorDecision = $state<CoordinatorDecisionState | null>(null);
  let synthesizing = $state(false);
  let coordinatorToolCount = $state(0);

  async function loadSessions() {
    try {
      // @ts-ignore
      sessions = (await window.go.main.ChatService.ListSessions()) || [];
    } catch {
      sessions = [];
    }
  }

  $effect(() => {
    loadSessions();
  });

  async function deleteSession(id: string) {
    try {
      // @ts-ignore
      await window.go.main.ChatService.DeleteSession(id);
      if (sessionId === id) {
        sessionId = '';
        messages = [];
        agents = [];
        coordinatorDecision = null;
      }
      await loadSessions();
    } catch (e) {
      console.error('Delete session failed:', e);
    }
  }

  async function loadSession(id: string) {
    try {
      // @ts-ignore
      const sess = await window.go.main.ChatService.GetSession(id);
      if (sess) {
        sessionId = sess.id;
        messages = sess.messages || [];
      }
    } catch (e) {
      console.error('Load session failed:', e);
    }
  }

  async function sendMessage() {
    if (!input.trim() || streaming) return;

    const userMsg: Message = { role: 'user', content: input.trim() };
    messages = [...messages, userMsg];
    input = '';
    streaming = true;
    streamedContent = '';
    toolCalls = [];
    diagrams = [];
    agents = [];
    currentRound = 0;
    maxRounds = 1;
    coordinatorDecision = null;
    synthesizing = false;
    coordinatorToolCount = 0;

    try {
      const allMsgs = messages.map((m) => ({ role: m.role, content: m.content }));
      if (saved.swarmMode) {
        // @ts-ignore
        await window.go.main.ChatService.SendSwarm(
          sessionId, allMsgs, saved.discoveryMode, saved.multiRound, saved.generateDiagram, saved.unlimitedIterations, saved.targetLang, saved.framework, saved.integrations
        );
      } else {
        // @ts-ignore
        await window.go.main.ChatService.SendChat(
          sessionId, allMsgs, saved.discoveryMode, saved.generateDiagram, saved.unlimitedIterations, saved.targetLang, saved.framework, saved.integrations
        );
      }
    } catch (e: any) {
      messages = [...messages, { role: 'assistant', content: `Error: ${e.message || e}` }];
      streaming = false;
    }
  }

  function cancelChat() {
    // @ts-ignore
    window.go.main.ChatService.CancelChat();
  }

  // Chat event subscriptions
  $effect(() => {
    const unsubs = [
      // --- Chat events ---
      EventsOn('chat:text', (data: any) => {
        streamedContent += data?.content || '';
      }),
      EventsOn('chat:tool_start', (data: any) => {
        toolCalls = [...toolCalls, { name: data?.name, id: data?.id, status: 'running' }];
      }),
      EventsOn('chat:tool_result', (data: any) => {
        toolCalls = toolCalls.map((tc) =>
          tc.id === data?.id ? { ...tc, status: 'done', result: data?.result } : tc
        );
      }),
      EventsOn('chat:diagram_generated', (data: any) => {
        if (data?.svgData) {
          diagrams = [...diagrams, { filePath: data.filePath || '', svgData: data.svgData }];
        }
      }),
      EventsOn('chat:session_created', (data: any) => {
        if (data?.id) sessionId = data.id;
        loadSessions();
      }),
      EventsOn('chat:done', () => {
        if (streamedContent) {
          messages = [...messages, { role: 'assistant', content: streamedContent }];
        }
        streamedContent = '';
        toolCalls = [];
        streaming = false;
      }),
      EventsOn('chat:error', (data: any) => {
        if (streamedContent) {
          messages = [...messages, { role: 'assistant', content: streamedContent }];
          streamedContent = '';
        }
        messages = [...messages, { role: 'error', content: data?.error || 'Unknown error' }];
        streaming = false;
      }),

      // --- Swarm events ---
      EventsOn('swarm:round_start', (data: any) => {
        currentRound = data?.round ?? 1;
        maxRounds = data?.maxRounds ?? 1;
        coordinatorDecision = null;
      }),
      EventsOn('swarm:round_complete', (_data: any) => {}),
      EventsOn('swarm:agent_start', (data: any) => {
        agents = [...agents, { id: data?.id, name: data?.name, status: 'running', content: '', toolCalls: 0, toolResults: 0 }];
      }),
      EventsOn('swarm:agent_progress', (data: any) => {
        agents = agents.map((a) =>
          a.id === data?.id ? { ...a, content: a.content + (data?.content || '') } : a
        );
      }),
      EventsOn('swarm:agent_tool_start', (data: any) => {
        agents = agents.map((a) =>
          a.id === data?.id ? { ...a, toolCalls: a.toolCalls + 1 } : a
        );
      }),
      EventsOn('swarm:agent_tool_result', (data: any) => {
        agents = agents.map((a) =>
          a.id === data?.id ? { ...a, toolResults: a.toolResults + 1 } : a
        );
      }),
      EventsOn('swarm:agent_complete', (data: any) => {
        agents = agents.map((a) =>
          a.id === data?.id ? { ...a, status: 'done', content: data?.summary || a.content } : a
        );
      }),
      EventsOn('swarm:coordinator_decision', (data: any) => {
        coordinatorDecision = {
          round: data?.round ?? 0,
          satisfied: data?.satisfied ?? true,
          reasoning: data?.reasoning ?? '',
          followUps: data?.followUps ?? null,
        };
      }),
      EventsOn('swarm:synthesis_start', () => {
        synthesizing = true;
        coordinatorToolCount = 0;
      }),
      EventsOn('swarm:coordinator_tool_start', () => {
        coordinatorToolCount++;
      }),
      EventsOn('swarm:coordinator_tool_result', () => {}),
      EventsOn('swarm:text', (data: any) => {
        streamedContent += data?.content || '';
      }),
      EventsOn('swarm:diagram_generated', (data: any) => {
        if (data?.svgData) {
          diagrams = [...diagrams, { filePath: data.filePath || '', svgData: data.svgData }];
        }
      }),
      EventsOn('swarm:session_created', (data: any) => {
        if (data?.id) sessionId = data.id;
        loadSessions();
      }),
      EventsOn('swarm:done', () => {
        if (streamedContent) {
          messages = [...messages, { role: 'assistant', content: streamedContent }];
        }
        streamedContent = '';
        streaming = false;
        synthesizing = false;
      }),
      EventsOn('swarm:error', (data: any) => {
        if (streamedContent) {
          messages = [...messages, { role: 'assistant', content: streamedContent }];
          streamedContent = '';
        }
        messages = [...messages, { role: 'error', content: data?.error || 'Unknown error' }];
        streaming = false;
        synthesizing = false;
      }),
    ];

    return () => unsubs.forEach((fn) => fn());
  });

  const languages = ['Java', 'C#', 'Python', 'Go', 'TypeScript', 'Kotlin'];
</script>

<div class="chat-layout">
  <!-- Session sidebar -->
  <div class="session-sidebar">
    <button class="new-session" onclick={() => { sessionId = ''; messages = []; agents = []; coordinatorDecision = null; }}>
      + New Chat
    </button>
    {#each sessions as sess}
      <div class="session-row" class:active={sessionId === sess.id}>
        <button
          class="session-item"
          onclick={() => loadSession(sess.id)}
        >
          {sess.title}
        </button>
        <button
          class="session-delete"
          onclick={(e) => { e.stopPropagation(); deleteSession(sess.id); }}
          title="Delete chat"
        >&times;</button>
      </div>
    {/each}
  </div>

  <div class="chat-main">
    <!-- Mode controls -->
    <div class="mode-bar">
      <label class="toggle">
        <input type="checkbox" bind:checked={saved.discoveryMode} />
        <span>{saved.discoveryMode ? 'Discovery Mode' : 'Migration Mode'}</span>
      </label>
      <label class="toggle swarm-toggle">
        <input type="checkbox" bind:checked={saved.swarmMode} />
        <span>Swarm</span>
      </label>
      {#if saved.swarmMode}
        <label class="toggle">
          <input type="checkbox" bind:checked={saved.multiRound} />
          <span>Multi-Round</span>
        </label>
      {/if}
      <label class="toggle">
        <input type="checkbox" bind:checked={saved.generateDiagram} />
        <span>Diagram</span>
      </label>
      <label class="toggle" title="Remove tool-call iteration caps. Use Stop to abort.">
        <input type="checkbox" bind:checked={saved.unlimitedIterations} />
        <span>Unlimited</span>
      </label>
      {#if !saved.discoveryMode}
        <select bind:value={saved.targetLang}>
          {#each languages as lang}
            <option>{lang}</option>
          {/each}
        </select>
        <input type="text" bind:value={saved.framework} placeholder="Framework" class="small-input" />
        <input type="text" bind:value={saved.integrations} placeholder="Integrations" class="small-input" />
      {/if}
    </div>

    <!-- Round progress (swarm multi-round) -->
    {#if streaming && saved.swarmMode && maxRounds > 1 && currentRound > 0}
      <div class="round-bar">
        <span class="round-label">Round {currentRound} / {maxRounds}</span>
        <div class="round-track">
          <div class="round-fill" style="width: {(currentRound / maxRounds) * 100}%"></div>
        </div>
      </div>
    {/if}

    <!-- Agent cards (swarm) -->
    {#if agents.length > 0}
      <div class="agent-grid">
        {#each agents as agent}
          <div class="agent-card" class:done={agent.status === 'done'}>
            <div class="agent-header">
              <span class="agent-name">{agent.name}</span>
              <span class="agent-status">{agent.status === 'done' ? 'Done' : 'Analyzing...'}</span>
            </div>
            <div class="agent-meta">{agent.toolCalls} tools called, {agent.toolResults} results</div>
            {#if agent.content}
              <div class="agent-content">{agent.content.slice(0, 200)}{agent.content.length > 200 ? '...' : ''}</div>
            {/if}
          </div>
        {/each}
      </div>
    {/if}

    <!-- Coordinator decision (swarm) -->
    {#if coordinatorDecision}
      <div class="coordinator-decision" class:satisfied={coordinatorDecision.satisfied}>
        <div class="decision-header">
          <span class="decision-label">Coordinator Decision</span>
          <span class="decision-status">{coordinatorDecision.satisfied ? 'Satisfied' : 'Needs Follow-up'}</span>
        </div>
        {#if coordinatorDecision.reasoning}
          <div class="decision-reasoning">{coordinatorDecision.reasoning}</div>
        {/if}
        {#if coordinatorDecision.followUps && Object.keys(coordinatorDecision.followUps).length > 0}
          <div class="decision-followups">
            {#each Object.entries(coordinatorDecision.followUps) as [agentId, question]}
              <div class="followup-item">
                <span class="followup-agent">{agentId}</span>: {question}
              </div>
            {/each}
          </div>
        {/if}
      </div>
    {/if}

    <!-- Synthesis indicator (swarm) -->
    {#if synthesizing && !streamedContent}
      <div class="synthesis-indicator">
        <span class="synthesis-spinner"></span>
        <span>Compiling results...{coordinatorToolCount > 0 ? ` (${coordinatorToolCount} tools used)` : ''}</span>
      </div>
    {/if}

    <!-- Messages -->
    <div class="messages">
      {#each messages as msg, i}
        <div class="message" class:user={msg.role === 'user'} class:assistant={msg.role === 'assistant'} class:error={msg.role === 'error'}>
          <div class="role">{msg.role}</div>
          {#if msg.role === 'error'}
            <div class="content">{msg.content}</div>
          {:else if msg.role === 'assistant'}
            <MarkdownContent content={msg.content} />
            <button class="copy-btn" title="Copy to clipboard" onclick={() => copyMessage(msg.content, i)}>
              {#if copiedIndex === i}
                <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.75.75 0 0 1 1.06-1.06L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z"/></svg>
              {:else}
                <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M0 6.75C0 5.784.784 5 1.75 5h1.5a.75.75 0 0 1 0 1.5h-1.5a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-1.5a.75.75 0 0 1 1.5 0v1.5A1.75 1.75 0 0 1 9.25 16h-7.5A1.75 1.75 0 0 1 0 14.25Z"/><path d="M5 1.75C5 .784 5.784 0 6.75 0h7.5C15.216 0 16 .784 16 1.75v7.5A1.75 1.75 0 0 1 14.25 11h-7.5A1.75 1.75 0 0 1 5 9.25Zm1.75-.25a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-7.5a.25.25 0 0 0-.25-.25Z"/></svg>
              {/if}
            </button>
          {:else}
            <div class="content">{msg.content}</div>
          {/if}
        </div>
      {/each}

      {#if streaming}
        {#if toolCalls.length > 0}
          <div class="tool-calls">
            {#each toolCalls as tc}
              <div class="tool-call" class:done={tc.status === 'done'}>
                <span class="tool-icon">{tc.status === 'done' ? '&#10003;' : '&#8987;'}</span>
                {tc.name}
              </div>
            {/each}
          </div>
        {/if}
        {#if diagrams.length > 0}
          {#each diagrams as diag}
            <div class="diagram-preview">
              <div class="diagram-svg">{@html diag.svgData}</div>
              {#if diag.filePath}
                <div class="diagram-path">{diag.filePath}</div>
              {/if}
            </div>
          {/each}
        {/if}
        {#if streamedContent}
          <div class="message assistant streaming">
            <div class="role">{saved.swarmMode ? 'coordinator' : 'assistant'}</div>
            <MarkdownContent content={streamedContent} />
            <button class="copy-btn" title="Copy to clipboard" onclick={() => copyMessage(streamedContent, -2)}>
              {#if copiedIndex === -2}
                <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M13.78 4.22a.75.75 0 0 1 0 1.06l-7.25 7.25a.75.75 0 0 1-1.06 0L2.22 9.28a.75.75 0 0 1 1.06-1.06L6 10.94l6.72-6.72a.75.75 0 0 1 1.06 0Z"/></svg>
              {:else}
                <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M0 6.75C0 5.784.784 5 1.75 5h1.5a.75.75 0 0 1 0 1.5h-1.5a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-1.5a.75.75 0 0 1 1.5 0v1.5A1.75 1.75 0 0 1 9.25 16h-7.5A1.75 1.75 0 0 1 0 14.25Z"/><path d="M5 1.75C5 .784 5.784 0 6.75 0h7.5C15.216 0 16 .784 16 1.75v7.5A1.75 1.75 0 0 1 14.25 11h-7.5A1.75 1.75 0 0 1 5 9.25Zm1.75-.25a.25.25 0 0 0-.25.25v7.5c0 .138.112.25.25.25h7.5a.25.25 0 0 0 .25-.25v-7.5a.25.25 0 0 0-.25-.25Z"/></svg>
              {/if}
            </button>
          </div>
        {/if}
      {/if}
    </div>

    <!-- Input -->
    <div class="input-bar">
      <textarea
        bind:value={input}
        placeholder={saved.swarmMode ? 'Ask the swarm to investigate...' : 'Ask about your COBOL codebase...'}
        onkeydown={(e) => {
          if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendMessage();
          }
        }}
        disabled={streaming}
        rows="2"
      ></textarea>
      {#if streaming}
        <button class="btn-danger" onclick={cancelChat}>Stop</button>
      {:else}
        <button class="btn-primary" onclick={sendMessage} disabled={!input.trim()}>
          {saved.swarmMode ? 'Send Swarm' : 'Send'}
        </button>
      {/if}
    </div>
  </div>
</div>

<style>
  .chat-layout {
    display: grid;
    grid-template-columns: 200px 1fr;
    gap: 12px;
    height: calc(100vh - 88px);
  }

  .session-sidebar {
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 6px;
    padding: 8px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .new-session {
    background: #21262d;
    border: 1px solid #30363d;
    color: #58a6ff;
    border-radius: 4px;
    padding: 8px;
    cursor: pointer;
    font-size: 13px;
    margin-bottom: 4px;
  }

  .session-row {
    display: flex;
    align-items: center;
    border-radius: 4px;
  }

  .session-row:hover .session-delete {
    opacity: 1;
  }

  .session-row:hover {
    background: #161b22;
  }

  .session-row.active {
    background: #1f2937;
  }

  .session-row.active .session-item {
    color: #58a6ff;
  }

  .session-item {
    background: none;
    border: none;
    color: #8b949e;
    padding: 6px 8px;
    text-align: left;
    cursor: pointer;
    font-size: 12px;
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }

  .session-row:hover .session-item {
    color: #e1e4e8;
  }

  .session-delete {
    background: none;
    border: none;
    color: #8b949e;
    cursor: pointer;
    font-size: 14px;
    padding: 2px 6px;
    opacity: 0;
    flex-shrink: 0;
    border-radius: 4px;
    line-height: 1;
  }

  .session-delete:hover {
    color: #f85149;
    background: #21262d;
  }

  .chat-main {
    display: flex;
    flex-direction: column;
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 6px;
    overflow: hidden;
  }

  .mode-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border-bottom: 1px solid #21262d;
    font-size: 13px;
  }

  .toggle {
    display: flex;
    align-items: center;
    gap: 6px;
    cursor: pointer;
    color: #8b949e;
  }

  .toggle input {
    cursor: pointer;
  }

  .swarm-toggle span {
    color: #d2a8ff;
    font-weight: 500;
  }

  .small-input,
  .mode-bar select {
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 4px;
    color: #e1e4e8;
    padding: 4px 8px;
    font-size: 12px;
  }

  /* Round progress */
  .round-bar {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 6px 12px;
    border-bottom: 1px solid #21262d;
    font-size: 12px;
    color: #8b949e;
  }

  .round-label {
    white-space: nowrap;
    color: #d2a8ff;
    font-weight: 500;
  }

  .round-track {
    flex: 1;
    height: 4px;
    background: #21262d;
    border-radius: 2px;
    overflow: hidden;
  }

  .round-fill {
    height: 100%;
    background: #d2a8ff;
    border-radius: 2px;
    transition: width 0.3s ease;
  }

  /* Agent cards */
  .agent-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
    gap: 8px;
    padding: 8px 12px;
    border-bottom: 1px solid #21262d;
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
    margin-bottom: 4px;
  }

  .agent-name {
    color: #d2a8ff;
    font-weight: 500;
  }

  .agent-status {
    color: #8b949e;
    font-size: 11px;
  }

  .agent-meta {
    color: #8b949e;
    font-size: 11px;
    margin-bottom: 6px;
  }

  .agent-content {
    color: #c9d1d9;
    font-size: 11px;
    line-height: 1.4;
  }

  /* Coordinator decision */
  .coordinator-decision {
    padding: 8px 12px;
    border-bottom: 1px solid #21262d;
    background: #161b22;
    font-size: 12px;
  }

  .coordinator-decision.satisfied {
    border-left: 3px solid #238636;
  }

  .coordinator-decision:not(.satisfied) {
    border-left: 3px solid #d29922;
  }

  .decision-header {
    display: flex;
    justify-content: space-between;
    margin-bottom: 4px;
  }

  .decision-label {
    color: #d2a8ff;
    font-weight: 500;
  }

  .decision-status {
    font-size: 11px;
    color: #8b949e;
  }

  .decision-reasoning {
    color: #c9d1d9;
    margin-bottom: 4px;
    line-height: 1.4;
  }

  .decision-followups {
    margin-top: 4px;
  }

  .followup-item {
    color: #8b949e;
    font-size: 11px;
    padding: 2px 0;
  }

  .followup-agent {
    color: #d2a8ff;
    font-weight: 500;
  }

  /* Synthesis indicator */
  .synthesis-indicator {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border-bottom: 1px solid #21262d;
    color: #d2a8ff;
    font-size: 12px;
  }

  .synthesis-spinner {
    display: inline-block;
    width: 12px;
    height: 12px;
    border: 2px solid #30363d;
    border-top-color: #d2a8ff;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  .messages {
    flex: 1;
    overflow-y: auto;
    padding: 12px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .message {
    position: relative;
    max-width: 85%;
    padding: 10px 14px;
    border-radius: 8px;
    font-size: 13px;
    line-height: 1.5;
  }

  .message.user {
    white-space: pre-wrap;
    align-self: flex-end;
    background: #1f3a5f;
    color: #e1e4e8;
  }

  .message.assistant {
    align-self: flex-start;
    background: #161b22;
    border: 1px solid #21262d;
  }

  .message.error {
    align-self: flex-start;
    background: #1a0f0f;
    border: 1px solid #6e2020;
    color: #f85149;
  }

  .message.streaming {
    border-color: #58a6ff;
  }

  .role {
    font-size: 10px;
    color: #8b949e;
    text-transform: uppercase;
    margin-bottom: 4px;
  }

  .tool-calls {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    padding: 4px 0;
  }

  .tool-call {
    background: #1c2128;
    border: 1px solid #30363d;
    border-radius: 4px;
    padding: 4px 8px;
    font-size: 11px;
    color: #d2a8ff;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .tool-call.done {
    color: #3fb950;
    border-color: #238636;
  }

  .tool-icon {
    font-size: 12px;
  }

  .input-bar {
    display: flex;
    gap: 8px;
    padding: 8px 12px;
    border-top: 1px solid #21262d;
  }

  textarea {
    flex: 1;
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 6px;
    color: #e1e4e8;
    padding: 8px;
    font-size: 13px;
    font-family: inherit;
    resize: none;
  }

  .btn-primary {
    background: #238636;
    border: 1px solid #2ea043;
    color: #fff;
    border-radius: 6px;
    padding: 8px 16px;
    cursor: pointer;
    font-size: 13px;
    align-self: flex-end;
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
    padding: 8px 16px;
    cursor: pointer;
    font-size: 13px;
    align-self: flex-end;
  }

  .copy-btn {
    position: absolute;
    top: 8px;
    right: 8px;
    background: none;
    border: none;
    color: #8b949e;
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.15s;
    padding: 4px;
    border-radius: 4px;
    display: flex;
    align-items: center;
  }

  .copy-btn:hover {
    color: #58a6ff;
    background: #21262d;
  }

  .message:hover .copy-btn {
    opacity: 1;
  }

  /* Diagram preview */
  .diagram-preview {
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 8px;
    padding: 12px;
    max-width: 85%;
    align-self: flex-start;
  }

  .diagram-svg {
    overflow-x: auto;
    max-width: 100%;
  }

  .diagram-svg :global(svg) {
    max-width: 100%;
    height: auto;
  }

  .diagram-path {
    margin-top: 6px;
    font-size: 10px;
    color: #8b949e;
    font-family: monospace;
  }
</style>
