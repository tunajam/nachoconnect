<script>
  import { createEventDispatcher, onMount, onDestroy } from 'svelte';
  const dispatch = createEventDispatcher();

  let interfaces = [];
  let selectedInterface = null;
  let scanning = false;
  let xboxMAC = '';
  let error = '';
  let scanDots = '';
  let dotInterval;

  onMount(async () => {
    try {
      interfaces = await window.go.main.App.GetInterfaces();
    } catch (e) {
      // Dev mode fallback
      interfaces = [
        { name: 'en0', ip: '192.168.1.42', mac: 'aa:bb:cc:dd:ee:ff', description: 'Wi-Fi' },
        { name: 'en1', ip: '192.168.1.43', mac: '11:22:33:44:55:66', description: 'Ethernet' },
      ];
    }

    if (window.runtime) {
      window.runtime.EventsOn('xbox:detected', (mac) => {
        xboxMAC = mac;
        scanning = false;
        clearInterval(dotInterval);
      });
    }
  });

  onDestroy(() => {
    clearInterval(dotInterval);
  });

  async function selectInterface(iface) {
    selectedInterface = iface;
    scanning = true;
    error = '';
    xboxMAC = '';
    scanDots = '';
    
    // Animate dots
    dotInterval = setInterval(() => {
      scanDots = scanDots.length >= 3 ? '' : scanDots + '.';
    }, 500);

    try {
      await window.go.main.App.SelectInterface(iface.name);
    } catch (e) {
      error = `Failed to scan on ${iface.name}: ${e}`;
      scanning = false;
      clearInterval(dotInterval);
    }
  }

  function proceed() {
    dispatch('ready', { interface: selectedInterface, mac: xboxMAC });
  }

  function skipScan() {
    dispatch('ready', { interface: selectedInterface, mac: '' });
  }
</script>

<div class="setup">
  <div class="setup-header">
    <span class="icon">🧀</span>
    <h1>Setup</h1>
    <p class="subtitle">Select your network interface to find your Xbox</p>
  </div>

  {#if error}
    <div class="error-banner">{error}</div>
  {/if}

  <div class="interfaces">
    <h2>Network Interfaces</h2>
    <div class="interface-list">
      {#each interfaces as iface}
        <button
          class="interface-card"
          class:selected={selectedInterface?.name === iface.name}
          class:scanning={scanning && selectedInterface?.name === iface.name}
          on:click={() => selectInterface(iface)}
        >
          <div class="iface-top">
            <span class="iface-name">{iface.name}</span>
            {#if iface.description}
              <span class="iface-desc">{iface.description}</span>
            {/if}
          </div>
          <div class="iface-bottom">
            {#if iface.ip}
              <span class="mono">{iface.ip}</span>
            {/if}
            {#if iface.mac}
              <span class="mono muted">{iface.mac}</span>
            {/if}
          </div>
          {#if scanning && selectedInterface?.name === iface.name}
            <div class="scan-indicator">
              <span class="pulse"></span>
              Scanning for Xbox{scanDots}
            </div>
          {/if}
        </button>
      {/each}
    </div>
  </div>

  {#if xboxMAC}
    <div class="xbox-found">
      <div class="found-icon">🎮</div>
      <div class="found-text">
        <h3>Xbox Detected!</h3>
        <p class="mono">{xboxMAC}</p>
      </div>
      <button class="btn-primary" on:click={proceed}>Continue to Lobbies →</button>
    </div>
  {:else if selectedInterface && !scanning}
    <div class="no-xbox">
      <p>No Xbox detected on <strong>{selectedInterface.name}</strong>. Make sure your Xbox is powered on and connected to this network.</p>
      <div class="no-xbox-actions">
        <button class="btn-secondary" on:click={() => selectInterface(selectedInterface)}>Retry Scan</button>
        <button class="btn-ghost" on:click={skipScan}>Skip — I'll connect later</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .setup {
    max-width: 560px;
    margin: 0 auto;
    padding: 40px 20px;
  }

  .setup-header {
    text-align: center;
    margin-bottom: 32px;
  }

  .icon {
    font-size: 48px;
    display: block;
    margin-bottom: 12px;
  }

  h1 {
    font-size: 24px;
    font-weight: 700;
    color: var(--text-primary);
    margin: 0 0 4px;
  }

  .subtitle {
    color: var(--text-muted);
    font-size: 13px;
    margin: 0;
  }

  h2 {
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 1px;
    color: var(--text-muted);
    margin: 0 0 12px;
  }

  .error-banner {
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #ef4444;
    padding: 8px 12px;
    font-size: 12px;
    margin-bottom: 16px;
  }

  .interface-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .interface-card {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 12px 16px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    cursor: pointer;
    text-align: left;
    color: var(--text-secondary);
    font-family: inherit;
    font-size: 13px;
    transition: all 0.15s;
    width: 100%;
  }

  .interface-card:hover {
    border-color: var(--green-dim);
    background: var(--green-glow);
  }

  .interface-card.selected {
    border-color: var(--green);
    background: var(--green-glow);
  }

  .interface-card.scanning {
    border-color: var(--green);
  }

  .iface-top {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .iface-name {
    font-weight: 700;
    color: var(--text-primary);
    font-family: var(--font-mono);
  }

  .iface-desc {
    color: var(--text-muted);
    font-size: 11px;
  }

  .iface-bottom {
    display: flex;
    gap: 16px;
    font-size: 11px;
  }

  .mono { font-family: var(--font-mono); }
  .muted { color: var(--text-muted); }

  .scan-indicator {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--green);
    font-size: 11px;
    margin-top: 4px;
  }

  .pulse {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--green);
    animation: pulse 1.5s ease-in-out infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 0.4; transform: scale(0.8); }
    50% { opacity: 1; transform: scale(1.2); }
  }

  .xbox-found {
    margin-top: 24px;
    padding: 16px;
    border: 1px solid var(--green);
    background: var(--green-glow);
    display: flex;
    align-items: center;
    gap: 16px;
  }

  .found-icon {
    font-size: 32px;
  }

  .found-text {
    flex: 1;
  }

  .found-text h3 {
    margin: 0;
    color: var(--green);
    font-size: 14px;
    font-weight: 700;
  }

  .found-text p {
    margin: 2px 0 0;
    color: var(--text-muted);
    font-size: 11px;
  }

  .btn-primary {
    padding: 8px 20px;
    background: var(--green);
    color: var(--bg-primary);
    border: none;
    font-weight: 700;
    font-size: 13px;
    cursor: pointer;
    font-family: var(--font-mono);
    white-space: nowrap;
  }

  .btn-primary:hover { opacity: 0.9; }

  .no-xbox {
    margin-top: 24px;
    padding: 16px;
    border: 1px solid var(--border);
    background: var(--bg-card);
    font-size: 13px;
    color: var(--text-secondary);
  }

  .no-xbox p { margin: 0 0 12px; }

  .no-xbox-actions {
    display: flex;
    gap: 8px;
  }

  .btn-secondary {
    padding: 6px 16px;
    background: var(--bg-secondary);
    color: var(--text-primary);
    border: 1px solid var(--border);
    font-size: 12px;
    cursor: pointer;
    font-family: var(--font-mono);
  }

  .btn-secondary:hover { border-color: var(--green-dim); }

  .btn-ghost {
    padding: 6px 16px;
    background: none;
    color: var(--text-muted);
    border: none;
    font-size: 12px;
    cursor: pointer;
    font-family: var(--font-mono);
  }

  .btn-ghost:hover { color: var(--text-primary); }
</style>
