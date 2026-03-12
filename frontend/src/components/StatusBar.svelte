<script>
  export let status;

  function openReportIssue() {
    if (window.runtime) {
      window.runtime.BrowserOpenURL('https://github.com/tunajam/nachoconnect/issues');
    } else {
      window.open('https://github.com/tunajam/nachoconnect/issues', '_blank');
    }
  }
</script>

<footer class="status-bar">
  <div class="status-left">
    {#if status.gamertag}
      <div class="status-item gamertag">
        <span>🧀 {status.gamertag}</span>
      </div>
    {/if}
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
    {#if status.peerPing > 0}
      <span class="mono ping" class:good={status.peerPing < 50} class:ok={status.peerPing >= 50 && status.peerPing < 100} class:bad={status.peerPing >= 100}>
        ⚡ {status.peerPing}ms P2P
      </span>
    {:else if status.serverPing > 0}
      <span class="mono ping" class:good={status.serverPing < 50} class:ok={status.serverPing >= 50 && status.serverPing < 100} class:bad={status.serverPing >= 100}>
        {status.serverPing}ms
      </span>
    {/if}
    {#if status.error}
      <span class="error-indicator">⚠️ {status.error}</span>
    {/if}
    {#if status.localIP}
      <span class="mono">{status.localIP}</span>
    {/if}
    {#if status.interface}
      <span class="mono">{status.interface}</span>
    {/if}
    <button class="report-link" on:click={openReportIssue}>🐛 Report Issue</button>
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

  .gamertag {
    color: var(--green);
    font-weight: 600;
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

  .ping.good { color: var(--green); }
  .ping.ok { color: var(--yellow); }
  .ping.bad { color: var(--red); }

  .error-indicator {
    color: var(--red);
    font-size: 10px;
  }

  .report-link {
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: 11px;
    cursor: pointer;
    padding: 0;
    font-family: inherit;
  }

  .report-link:hover {
    color: var(--text-primary);
  }
</style>
