<script lang="ts">
  let { name, onBack }: { name: string; onBack: () => void } = $props();

  let detail = $state<any>(null);
  let loading = $state(true);

  $effect(() => {
    (async () => {
      try {
        // @ts-ignore
        detail = await window.go.main.BrowserService.GetBusinessDomain(name);
      } catch (e) {
        console.error(e);
      } finally {
        loading = false;
      }
    })();
  });
</script>

<div class="detail">
  <div class="header">
    <button class="back-btn" onclick={onBack}>&#8592; Back</button>
    <h3>{name}</h3>
  </div>

  {#if loading}
    <p class="loading">Loading...</p>
  {:else if detail}
    {#if detail.description}
      <p class="desc">{detail.description}</p>
    {/if}

    <h4>{detail.programs?.length || 0} Programs</h4>
    <div class="program-list">
      {#each detail.programs || [] as prog}
        <span class="tag">{prog}</span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .header { display: flex; align-items: center; gap: 12px; margin-bottom: 16px; }
  .back-btn { background: #21262d; border: 1px solid #30363d; color: #c9d1d9; padding: 4px 12px; border-radius: 4px; cursor: pointer; font-size: 13px; }
  .back-btn:hover { background: #30363d; }
  h3 { font-size: 16px; font-weight: 600; color: #e1e4e8; }
  h4 { font-size: 13px; font-weight: 600; color: #c9d1d9; margin-bottom: 8px; }
  .desc { font-size: 13px; color: #8b949e; margin-bottom: 12px; }
  .program-list { display: flex; flex-wrap: wrap; gap: 6px; }
  .tag { background: #21262d; padding: 3px 10px; border-radius: 12px; font-size: 12px; color: #58a6ff; }
  .loading { color: #8b949e; font-size: 13px; }
</style>
