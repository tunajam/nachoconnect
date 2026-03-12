<script>
  export let status;
</script>

<footer class="status-bar">
  <div class="status-left">
    <div class="status-item">
      <span class="dot" class:active={status.xboxDetected}></span>
      <span>Xbox: {status.xboxDetected ? 'Connected' : 'Not found'}</span>
    </div>
    <div class="status-item">
      <span class="dot" class:active={status.tunnelActive}></span>
      <span>Tunnel: {status.tunnelActive ? 'Active' : 'Inactive'}</span>
    </div>
    {#if status.peerCount > 0}
      <div class="status-item">
        <span>{status.peerCount} peer{status.peerCount !== 1 ? 's' : ''}</span>
      </div>
    {/if}
  </div>
  <div class="status-right">
    {#if status.localIP}
      <span class="mono">{status.localIP}</span>
    {/if}
    {#if status.interface}
      <span class="mono">{status.interface}</span>
    {/if}
  </div>
</footer>

<style>
  .status-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 24px;
    background: var(--bg-secondary);
    border-top: 1px solid var(--border);
    font-size: 11px;
    color: var(--text-muted);
    height: 32px;
  }

  .status-left, .status-right {
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .status-item {
    display: flex;
    align-items: center;
    gap: 5px;
  }

  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--text-muted);
  }

  .dot.active {
    background: var(--green);
    box-shadow: 0 0 4px var(--green);
  }
</style>
