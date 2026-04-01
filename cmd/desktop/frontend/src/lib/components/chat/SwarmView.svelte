<script lang="ts">
  // @ts-ignore - Wails runtime
  import { EventsOn } from 'wailsjs/runtime/runtime';
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

  let saved = usePersistedState('swarm', {
    discoveryMode: true,
    multiRound: false,
    targetLang: 'Java',
    framework: 'Spring Boot',
    integrations: '',
  });

  let messages = $state<Message[]>([]);
  let input = $state('');
  let streaming = $state(false);
  let streamedContent = $state('');
  let sessionId = $state('');
  let agents = $state<AgentState[]>([]);
  let currentRound = $state(0);
  let maxRounds = $state(1);
  let coordinatorDecision = $state<CoordinatorDecisionState | null>(null);
  let synthesizing = $state(false);
  let coordinatorToolCount = $state(0);

  async function sendSwarm() {
    if (!input.trim() || streaming) return;

    const userMsg: Message = { role: 'user', content: input.trim() };
    messages = [...messages, userMsg];
    input = '';
    streaming = true;
    streamedContent = '';
    agents = [];
    currentRound = 0;
    maxRounds = 1;
    coordinatorDecision = null;
    synthesizing = false;
    coordinatorToolCount = 0;

    try {
      const allMsgs = messages.map((m) => ({ role: m.role, content: m.content }));
      // @ts-ignore
      await window.go.main.ChatService.SendSwarm(
        sessionId, allMsgs, saved.discoveryMode, saved.multiRound, saved.targetLang, saved.framework, saved.integrations
      );
    } catch (e: any) {
      messages = [...messages, { role: 'assistant', content: `Error: ${e.message || e}` }];
      streaming = false;
    }
  }

  function cancelSwarm() {
    // @ts-ignore
    window.go.main.ChatService.CancelChat();
  }

  $effect(() => {
    const unsubs = [
      EventsOn('swarm:round_start', (data: any) => {
        currentRound = data?.round ?? 1;
        maxRounds = data?.maxRounds ?? 1;
        coordinatorDecision = null;
      }),
      EventsOn('swarm:round_complete', (data: any) => {
        // Round finished, waiting for coordinator decision or synthesis
        void data;
      }),
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
      EventsOn('swarm:coordinator_tool_result', () => {
        // Tool result received by coordinator during synthesis
      }),
      EventsOn('swarm:text', (data: any) => {
        streamedContent += data?.content || '';
      }),
      EventsOn('swarm:session_created', (data: any) => {
        if (data?.id) sessionId = data.id;
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
        messages = [...messages, { role: 'assistant', content: `Error: ${data?.error}` }];
        streaming = false;
        synthesizing = false;
      }),
    ];

    return () => unsubs.forEach((fn) => fn());
  });

  const languages = ['Java', 'C#', 'Python', 'Go', 'TypeScript', 'Kotlin'];
</script>

<div class="swarm-layout">
  <div class="swarm-main">
    <div class="mode-bar">
      <label class="toggle">
        <input type="checkbox" bind:checked={saved.discoveryMode} />
        <span>{saved.discoveryMode ? 'Discovery' : 'Migration'}</span>
      </label>
      <label class="toggle">
        <input type="checkbox" bind:checked={saved.multiRound} />
        <span>Multi-Round</span>
      </label>
      {#if !saved.discoveryMode}
        <select bind:value={saved.targetLang}>
          {#each languages as lang}
            <option>{lang}</option>
          {/each}
        </select>
      {/if}
    </div>

    <!-- Round progress -->
    {#if streaming && maxRounds > 1 && currentRound > 0}
      <div class="round-bar">
        <span class="round-label">Round {currentRound} / {maxRounds}</span>
        <div class="round-track">
          <div class="round-fill" style="width: {(currentRound / maxRounds) * 100}%"></div>
        </div>
      </div>
    {/if}

    <!-- Agent cards -->
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

    <!-- Coordinator decision -->
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

    <!-- Synthesis indicator -->
    {#if synthesizing && !streamedContent}
      <div class="synthesis-indicator">
        <span class="synthesis-spinner"></span>
        <span>Compiling results...{coordinatorToolCount > 0 ? ` (${coordinatorToolCount} tools used)` : ''}</span>
      </div>
    {/if}

    <!-- Messages -->
    <div class="messages">
      {#each messages as msg}
        <div class="message" class:user={msg.role === 'user'} class:assistant={msg.role === 'assistant'}>
          <div class="role">{msg.role}</div>
          {#if msg.role === 'assistant'}
            <MarkdownContent content={msg.content} />
          {:else}
            <div class="content">{msg.content}</div>
          {/if}
        </div>
      {/each}
      {#if streaming && streamedContent}
        <div class="message assistant streaming">
          <div class="role">coordinator</div>
          <MarkdownContent content={streamedContent} />
        </div>
      {/if}
    </div>

    <!-- Input -->
    <div class="input-bar">
      <textarea
        bind:value={input}
        placeholder="Ask the swarm to investigate..."
        onkeydown={(e) => {
          if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendSwarm();
          }
        }}
        disabled={streaming}
        rows="2"
      ></textarea>
      {#if streaming}
        <button class="btn-danger" onclick={cancelSwarm}>Stop</button>
      {:else}
        <button class="btn-primary" onclick={sendSwarm} disabled={!input.trim()}>Send Swarm</button>
      {/if}
    </div>
  </div>
</div>

<style>
  .swarm-layout {
    height: calc(100vh - 88px);
  }

  .swarm-main {
    display: flex;
    flex-direction: column;
    background: #0d1117;
    border: 1px solid #21262d;
    border-radius: 6px;
    overflow: hidden;
    height: 100%;
  }

  .mode-bar {
    display: flex;
    align-items: center;
    gap: 12px;
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

  .mode-bar select {
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 4px;
    color: #e1e4e8;
    padding: 4px 8px;
    font-size: 12px;
  }

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
  }

  .message.assistant {
    align-self: flex-start;
    background: #161b22;
    border: 1px solid #21262d;
  }

  .message.streaming {
    border-color: #d2a8ff;
  }

  .role {
    font-size: 10px;
    color: #8b949e;
    text-transform: uppercase;
    margin-bottom: 4px;
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
</style>
