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

  async function sendSwarm() {
    if (!input.trim() || streaming) return;

    const userMsg: Message = { role: 'user', content: input.trim() };
    messages = [...messages, userMsg];
    input = '';
    streaming = true;
    streamedContent = '';
    agents = [];

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
      EventsOn('swarm:agent_start', (data: any) => {
        agents = [...agents, { id: data?.id, name: data?.name, status: 'running', content: '', toolCalls: 0 }];
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
      EventsOn('swarm:agent_complete', (data: any) => {
        agents = agents.map((a) =>
          a.id === data?.id ? { ...a, status: 'done', content: data?.summary || a.content } : a
        );
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
      }),
      EventsOn('swarm:error', (data: any) => {
        messages = [...messages, { role: 'assistant', content: `Error: ${data?.error}` }];
        streaming = false;
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

    <!-- Agent cards -->
    {#if agents.length > 0}
      <div class="agent-grid">
        {#each agents as agent}
          <div class="agent-card" class:done={agent.status === 'done'}>
            <div class="agent-header">
              <span class="agent-name">{agent.name}</span>
              <span class="agent-status">{agent.status === 'done' ? 'Done' : 'Analyzing...'}</span>
            </div>
            <div class="agent-meta">{agent.toolCalls} tool calls</div>
            {#if agent.content}
              <div class="agent-content">{agent.content.slice(0, 200)}{agent.content.length > 200 ? '...' : ''}</div>
            {/if}
          </div>
        {/each}
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
