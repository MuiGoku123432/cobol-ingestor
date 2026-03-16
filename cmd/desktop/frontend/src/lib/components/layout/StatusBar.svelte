<script lang="ts">
  import { neo4jStatus, authStatus } from '../../stores/status';
</script>

<footer class="status-bar">
  <span class="status-item">
    <span class="dot" class:connected={$neo4jStatus.connected}></span>
    Neo4j: {$neo4jStatus.connected ? 'Connected' : 'Disconnected'}
  </span>
  <span class="status-item">
    LLM: {$authStatus.ready ? $authStatus.model || $authStatus.provider : 'Not configured'}
  </span>
</footer>

<style>
  .status-bar {
    grid-column: 1 / -1;
    background: #0d1117;
    border-top: 1px solid #21262d;
    padding: 0 12px;
    display: flex;
    align-items: center;
    gap: 16px;
    font-size: 11px;
    color: #8b949e;
  }

  .status-item {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: #f85149;
  }

  .dot.connected {
    background: #3fb950;
  }
</style>
