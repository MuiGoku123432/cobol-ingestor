<script lang="ts">
  import Tabs from '../shared/Tabs.svelte';
  import ExportButton from '../shared/ExportButton.svelte';
  import ModernizationCandidates from './ModernizationCandidates.svelte';
  import RiskPrograms from './RiskPrograms.svelte';
  import MigrationSequence from './MigrationSequence.svelte';
  import DeadCodeSummary from './DeadCodeSummary.svelte';
  import EffortEstimates from './EffortEstimates.svelte';

  let activeTab = $state('modernization');

  const tabs = [
    { id: 'modernization', label: 'Modernization' },
    { id: 'risk', label: 'Risk' },
    { id: 'migration', label: 'Migration' },
    { id: 'deadcode', label: 'Dead Code' },
    { id: 'effort', label: 'Effort' },
  ];

  async function handleExport() {
    // @ts-ignore
    const dir = await window.go.main.ExportService.SelectSaveDirectory();
    if (!dir) return;
    // @ts-ignore
    await window.go.main.ExportService.ExportAnalysisReport(dir);
  }

  async function handleCSVExport() {
    const typeMap: Record<string, string> = {
      modernization: 'modernization',
      risk: 'risk',
      effort: 'effort',
      migration: 'migration',
      deadcode: 'dead-code',
    };
    const reportType = typeMap[activeTab];
    if (!reportType) return;
    // @ts-ignore
    const dir = await window.go.main.ExportService.SelectSaveDirectory();
    if (!dir) return;
    // @ts-ignore
    await window.go.main.ExportService.ExportCSV(reportType, dir);
  }
</script>

<div class="analysis">
  <div class="header">
    <h2>Analysis Reports</h2>
    <div class="actions">
      <ExportButton label="Export CSV" onExport={handleCSVExport} />
      <ExportButton label="Export Full Report" onExport={handleExport} />
    </div>
  </div>

  <Tabs {tabs} bind:activeTab />

  {#if activeTab === 'modernization'}
    <ModernizationCandidates />
  {:else if activeTab === 'risk'}
    <RiskPrograms />
  {:else if activeTab === 'migration'}
    <MigrationSequence />
  {:else if activeTab === 'deadcode'}
    <DeadCodeSummary />
  {:else if activeTab === 'effort'}
    <EffortEstimates />
  {/if}
</div>

<style>
  .analysis {
    max-width: 1100px;
  }

  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 16px;
  }

  h2 {
    font-size: 18px;
    font-weight: 600;
    color: #e1e4e8;
  }

  .actions {
    display: flex;
    gap: 8px;
  }
</style>
