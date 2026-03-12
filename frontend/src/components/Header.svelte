<script>
  import { createEventDispatcher } from 'svelte';
  export let status;
  const dispatch = createEventDispatcher();
</script>

<header class="header">
  <div class="header-left" on:click={() => dispatch('home')} on:keydown={() => {}}>
    <span class="logo">🧀</span>
    <span class="title">NachoConnect</span>
    <span class="version">v0.3.2</span>
  </div>
  
  <div class="header-right">
    <div class="xbox-status" class:detected={status.xboxDetected}>
      <span class="dot" class:active={status.xboxDetected}></span>
      <span class="label">
        {#if status.xboxDetected}
          Xbox Detected
        {:else}
          No Xbox Found
        {/if}
      </span>
      {#if status.xboxMAC}
        <span class="mac mono">{status.xboxMAC}</span>
      {/if}
    </div>
  </div>
</header>

<style>
  .header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 24px;
    background: var(--bg-secondary);
    border-bottom: 1px solid var(--border);
    -webkit-app-region: drag;
    height: 52px;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 10px;
    cursor: pointer;
    -webkit-app-region: no-drag;
  }

  .logo {
    font-size: 22px;
  }

  .title {
    font-size: 16px;
    font-weight: 700;
    letter-spacing: -0.3px;
    color: var(--text-primary);
  }

  .version {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-muted);
    background: var(--bg-card);
    padding: 2px 6px;
    border: 1px solid var(--border);
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 16px;
    -webkit-app-region: no-drag;
  }

  .xbox-status {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 12px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    font-size: 12px;
    color: var(--text-secondary);
    transition: all 0.2s;
  }

  .xbox-status.detected {
    border-color: var(--green-dim);
    background: var(--green-glow);
    color: var(--green);
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--text-muted);
    transition: background 0.2s;
  }

  .dot.active {
    background: var(--green);
    box-shadow: 0 0 8px var(--green);
  }

  .mac {
    color: var(--text-muted);
    font-size: 10px;
  }
</style>
