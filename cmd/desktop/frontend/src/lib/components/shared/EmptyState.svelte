<script lang="ts">
  let {
    message = 'No data yet — run ingestion first',
  }: {
    message?: string;
  } = $props();
</script>

<div class="empty-state">
  <div class="icon">&#9682;</div>
  <p>{message}</p>
</div>

<style>
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 60px 20px;
    color: #8b949e;
  }

  .icon {
    font-size: 40px;
    margin-bottom: 12px;
    opacity: 0.4;
  }

  p {
    font-size: 14px;
  }
</style>
