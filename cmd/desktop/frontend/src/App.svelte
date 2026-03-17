<script lang="ts">
  import Sidebar from './lib/components/layout/Sidebar.svelte';
  import StatusBar from './lib/components/layout/StatusBar.svelte';
  import IngestView from './lib/components/ingest/IngestView.svelte';
  import ChatView from './lib/components/chat/ChatView.svelte';
  import SwarmView from './lib/components/chat/SwarmView.svelte';
  import SettingsView from './lib/components/settings/SettingsView.svelte';
  import GraphView from './lib/components/graph/GraphView.svelte';
  import StrategyView from './lib/components/strategy/StrategyView.svelte';

  let currentView = $state('ingest');
</script>

<div class="app">
  <Sidebar bind:currentView />
  <main class="content">
    {#if currentView === 'ingest'}
      <IngestView />
    {:else if currentView === 'chat'}
      <ChatView />
    {:else if currentView === 'swarm'}
      <SwarmView />
    {:else if currentView === 'strategy'}
      <StrategyView />
    {:else if currentView === 'graph'}
      <GraphView />
    {:else if currentView === 'settings'}
      <SettingsView />
    {/if}
  </main>
  <StatusBar />
</div>

<style>
  :global(*) {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  :global(body) {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen,
      Ubuntu, Cantarell, sans-serif;
    background: #0f1117;
    color: #e1e4e8;
    overflow: hidden;
    height: 100vh;
  }

  .app {
    display: grid;
    grid-template-columns: 200px 1fr;
    grid-template-rows: 1fr 28px;
    height: 100vh;
  }

  .content {
    overflow-y: auto;
    padding: 20px;
    background: #161b22;
  }
</style>
