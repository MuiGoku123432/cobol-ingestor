<script lang="ts">
  // @ts-ignore - Wails runtime
  import { EventsOn } from 'wailsjs/runtime/runtime';
  import { refreshAuthStatus } from '../../stores/status';

  interface Message {
    role: string;
    content: string;
  }

  let messages = $state<Message[]>([]);
  let input = $state('');
  let streaming = $state(false);
  let streamedContent = $state('');
  let discoveryMode = $state(true);
  let targetLang = $state('Java');
  let framework = $state('Spring Boot');
  let integrations = $state('');
  let sessionId = $state('');
  let sessions = $state<any[]>([]);
  let toolCalls = $state<any[]>([]);

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

    try {
      const allMsgs = messages.map((m) => ({ role: m.role, content: m.content }));
      // @ts-ignore
      await window.go.main.ChatService.SendChat(
        sessionId, allMsgs, discoveryMode, targetLang, framework, integrations
      );
    } catch (e: any) {
      messages = [...messages, { role: 'assistant', content: `Error: ${e.message || e}` }];
      streaming = false;
    }
  }

  function cancelChat() {
    // @ts-ignore
    window.go.main.ChatService.CancelChat();
  }

  // Event subscriptions
  $effect(() => {
    const unsubs = [
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
        const errorContent = streamedContent
          ? streamedContent + `\n\nError: ${data?.error}`
          : `Error: ${data?.error}`;
        messages = [...messages, { role: 'assistant', content: errorContent }];
        streamedContent = '';
        streaming = false;
      }),
    ];

    return () => unsubs.forEach((fn) => fn());
  });

  const languages = ['Java', 'C#', 'Python', 'Go', 'TypeScript', 'Kotlin'];
</script>

<div class="chat-layout">
  <!-- Session sidebar -->
  <div class="session-sidebar">
    <button class="new-session" onclick={() => { sessionId = ''; messages = []; }}>
      + New Chat
    </button>
    {#each sessions as sess}
      <button
        class="session-item"
        class:active={sessionId === sess.id}
        onclick={() => loadSession(sess.id)}
      >
        {sess.title}
      </button>
    {/each}
  </div>

  <div class="chat-main">
    <!-- Mode controls -->
    <div class="mode-bar">
      <label class="toggle">
        <input type="checkbox" bind:checked={discoveryMode} />
        <span>{discoveryMode ? 'Discovery Mode' : 'Migration Mode'}</span>
      </label>
      {#if !discoveryMode}
        <select bind:value={targetLang}>
          {#each languages as lang}
            <option>{lang}</option>
          {/each}
        </select>
        <input type="text" bind:value={framework} placeholder="Framework" class="small-input" />
        <input type="text" bind:value={integrations} placeholder="Integrations" class="small-input" />
      {/if}
    </div>

    <!-- Messages -->
    <div class="messages">
      {#each messages as msg}
        <div class="message" class:user={msg.role === 'user'} class:assistant={msg.role === 'assistant'}>
          <div class="role">{msg.role}</div>
          <div class="content">{msg.content}</div>
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
        {#if streamedContent}
          <div class="message assistant streaming">
            <div class="role">assistant</div>
            <div class="content">{streamedContent}</div>
          </div>
        {/if}
      {/if}
    </div>

    <!-- Input -->
    <div class="input-bar">
      <textarea
        bind:value={input}
        placeholder="Ask about your COBOL codebase..."
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
        <button class="btn-primary" onclick={sendMessage} disabled={!input.trim()}>Send</button>
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

  .session-item {
    background: none;
    border: none;
    color: #8b949e;
    padding: 6px 8px;
    text-align: left;
    cursor: pointer;
    font-size: 12px;
    border-radius: 4px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .session-item:hover {
    background: #161b22;
    color: #e1e4e8;
  }

  .session-item.active {
    background: #1f2937;
    color: #58a6ff;
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

  .small-input,
  .mode-bar select {
    background: #161b22;
    border: 1px solid #30363d;
    border-radius: 4px;
    color: #e1e4e8;
    padding: 4px 8px;
    font-size: 12px;
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
    white-space: pre-wrap;
  }

  .message.user {
    align-self: flex-end;
    background: #1f3a5f;
    color: #e1e4e8;
  }

  .message.assistant {
    align-self: flex-start;
    background: #161b22;
    border: 1px solid #21262d;
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
</style>
