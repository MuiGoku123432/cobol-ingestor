<script lang="ts">
  import Sidebar from './lib/components/layout/Sidebar.svelte';
  import StatusBar from './lib/components/layout/StatusBar.svelte';
  import DashboardView from './lib/components/dashboard/DashboardView.svelte';
  import BrowseView from './lib/components/browse/BrowseView.svelte';
  import AnalysisView from './lib/components/analysis/AnalysisView.svelte';
  import IngestView from './lib/components/ingest/IngestView.svelte';
  import ChatView from './lib/components/chat/ChatView.svelte';
  import StrategyView from './lib/components/strategy/StrategyView.svelte';
  import GraphView from './lib/components/graph/GraphView.svelte';
  import SettingsView from './lib/components/settings/SettingsView.svelte';
  import { usePersistedState } from './lib/stores/persisted.svelte';

  let saved = usePersistedState('app', { currentView: 'dashboard' });
</script>

<div class="app">
  <Sidebar bind:currentView={saved.currentView} />
  <main class="content">
    {#if saved.currentView === 'dashboard'}
      <DashboardView />
    {:else if saved.currentView === 'browse'}
      <BrowseView />
    {:else if saved.currentView === 'analysis'}
      <AnalysisView />
    {:else if saved.currentView === 'ingest'}
      <IngestView />
    {:else if saved.currentView === 'chat'}
      <ChatView />
    {:else if saved.currentView === 'strategy'}
      <StrategyView />
    {:else if saved.currentView === 'graph'}
      <GraphView />
    {:else if saved.currentView === 'settings'}
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
