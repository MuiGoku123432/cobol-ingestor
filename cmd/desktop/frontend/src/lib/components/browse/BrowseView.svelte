<script lang="ts">
  import Tabs from '../shared/Tabs.svelte';
  import ProgramListView from '../programs/ProgramListView.svelte';
  import ProgramDetailView from '../programs/ProgramDetailView.svelte';
  import DomainListView from './DomainListView.svelte';
  import DomainDetailView from './DomainDetailView.svelte';

  let activeTab = $state('programs');
  let selectedProgram = $state<string | null>(null);
  let selectedDomain = $state<string | null>(null);

  const tabs = [
    { id: 'programs', label: 'Programs' },
    { id: 'domains', label: 'Domains' },
  ];

  // Clear detail selections when switching tabs
  $effect(() => {
    // Access activeTab to track it
    const _ = activeTab;
    selectedProgram = null;
    selectedDomain = null;
  });
</script>

<div class="browse">
  <h2>Browse</h2>
  <Tabs {tabs} bind:activeTab />

  {#if activeTab === 'programs'}
    {#if selectedProgram}
      <ProgramDetailView programId={selectedProgram} onBack={() => selectedProgram = null} />
    {:else}
      <ProgramListView onSelectProgram={(id) => selectedProgram = id} />
    {/if}

  {:else if activeTab === 'domains'}
    {#if selectedDomain}
      <DomainDetailView name={selectedDomain} onBack={() => selectedDomain = null} />
    {:else}
      <DomainListView onSelect={(name) => selectedDomain = name} />
    {/if}
  {/if}
</div>

<style>
  .browse {
    max-width: 1100px;
  }

  h2 {
    font-size: 18px;
    font-weight: 600;
    color: #e1e4e8;
    margin-bottom: 16px;
  }
</style>
