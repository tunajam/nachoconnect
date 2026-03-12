<script>
  import { onMount } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  let name = '';
  let game = 'Halo 2';
  let maxPlayers = 8;
  let creating = false;
  let port = 9999;
  let publicIP = '';
  let localIP = '';
  let gatewayIP = '';
  let detectingIP = true;
  let upnpStatus = 'untried'; // untried | trying | success | failed

  const games = [
    'Halo: Combat Evolved',
    'Halo 2',
    'MechAssault',
    'MechAssault 2',
    'Crimson Skies',
    'Star Wars: Battlefront',
    'Star Wars: Battlefront II',
    'Splinter Cell: Pandora Tomorrow',
    'Splinter Cell: Chaos Theory',
    'Counter-Strike',
    'TimeSplitters 2',
    'TimeSplitters: Future Perfect',
    'Doom 3',
    'Rainbow Six 3',
    'Ghost Recon',
    'Burnout 3: Takedown',
    'Forza Motorsport',
    'Other',
  ];

  onMount(async () => {
    try {
      const info = await window.go.main.App.GetPortForwardInfo(port);
      publicIP = info.publicIP || '';
      localIP = info.localIP || '';
      gatewayIP = info.gatewayIP || '';
    } catch (e) {
      console.log('Port forward info fetch failed:', e);
    }
    detectingIP = false;

    // Auto-try UPnP
    tryUPnP();
  });

  async function tryUPnP() {
    upnpStatus = 'trying';
    try {
      const result = await window.go.main.App.TryUPnP(port);
      upnpStatus = result.success ? 'success' : 'failed';
    } catch (e) {
      upnpStatus = 'failed';
    }
  }

  async function createLobby() {
    if (!name.trim()) return;
    creating = true;

    try {
      const lobby = await window.go.main.App.CreateLobby(name, game, maxPlayers, port);
      if (lobby) {
        dispatch('created', lobby);
        return;
      }
    } catch (e) {
      console.log('Create lobby error:', e);
    }

    creating = false;
  }
</script>

<div class="create">
  <div class="create-card">
    <h2>🧀 Host a Lobby</h2>
    <p class="subtitle">You'll host the game — friends connect directly to you (P2P)</p>

    <div class="form">
      <div class="field">
        <label for="name">Lobby Name</label>
        <input 
          id="name" 
          type="text" 
          placeholder="Friday Fragfest" 
          bind:value={name}
          on:keydown={(e) => e.key === 'Enter' && createLobby()}
        />
      </div>

      <div class="field">
        <label for="game">Game</label>
        <select id="game" bind:value={game}>
          {#each games as g}
            <option value={g}>{g}</option>
          {/each}
        </select>
      </div>

      <div class="field">
        <label for="max">Max Players</label>
        <div class="player-selector">
          {#each [2, 4, 8, 16] as n}
            <button 
              class="player-opt" 
              class:active={maxPlayers === n}
              on:click={() => maxPlayers = n}
            >
              {n}
            </button>
          {/each}
        </div>
      </div>

      <div class="field">
        <label>Port Forwarding</label>
        <div class="port-forward-card">
          <div class="pf-status">
            {#if upnpStatus === 'trying'}
              <span class="pf-badge pf-trying">⏳ Trying UPnP auto-forward...</span>
            {:else if upnpStatus === 'success'}
              <span class="pf-badge pf-success">✅ Port forwarded automatically via UPnP</span>
            {:else if upnpStatus === 'failed'}
              <span class="pf-badge pf-manual">⚠️ Manual port forward required</span>
            {:else}
              <span class="pf-badge pf-trying">Checking...</span>
            {/if}
          </div>

          <div class="pf-details">
            <div class="pf-row">
              <span class="pf-label">Your Public IP</span>
              <span class="pf-value mono">
                {#if detectingIP}detecting...{:else}{publicIP || 'unknown'}{/if}
              </span>
            </div>
            <div class="pf-row">
              <span class="pf-label">Your Local IP</span>
              <span class="pf-value mono">{localIP || 'unknown'}</span>
            </div>
            <div class="pf-row">
              <span class="pf-label">UDP Port</span>
              <input 
                type="number" 
                class="port-input" 
                bind:value={port} 
                min="1024" 
                max="65535"
              />
            </div>
            {#if gatewayIP}
              <div class="pf-row">
                <span class="pf-label">Router</span>
                <span class="pf-value">
                  <a href="http://{gatewayIP}" target="_blank" class="router-link mono">{gatewayIP}</a>
                </span>
              </div>
            {/if}
          </div>

          {#if upnpStatus === 'failed'}
            <div class="pf-instructions">
              <p class="pf-heading">📋 Manual Setup:</p>
              <ol>
                <li>Open your router admin page: <a href="http://{gatewayIP}" target="_blank" class="router-link">{gatewayIP}</a></li>
                <li>Find "Port Forwarding" or "NAT" settings</li>
                <li>Add a rule: <strong>UDP port {port}</strong> → <strong class="mono">{localIP}</strong></li>
                <li>Save and come back here</li>
              </ol>
              <p class="pf-help">
                Need help? <a href="https://portforward.com/router.htm" target="_blank" class="help-link">portforward.com</a> has guides for every router.
              </p>
            </div>
          {/if}
        </div>
      </div>

      <div class="actions">
        <button class="btn btn-cancel" on:click={() => dispatch('cancel')}>Cancel</button>
        <button 
          class="btn btn-create" 
          on:click={createLobby}
          disabled={!name.trim() || creating}
        >
          {creating ? 'Starting Hub...' : '⚡ Create & Host'}
        </button>
      </div>
    </div>
  </div>
</div>

<style>
  .create {
    display: flex;
    justify-content: center;
    padding-top: 40px;
  }

  .create-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    padding: 32px;
    width: 480px;
  }

  h2 {
    font-size: 20px;
    font-weight: 700;
    margin-bottom: 4px;
  }

  .subtitle {
    font-size: 13px;
    color: var(--text-muted);
    margin-bottom: 28px;
  }

  .form {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }

  .field {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  label {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .field input, .field select {
    width: 100%;
  }

  .player-selector {
    display: flex;
    gap: 0;
  }

  .player-opt {
    flex: 1;
    padding: 10px;
    background: var(--bg-input);
    color: var(--text-secondary);
    border: 1px solid var(--border);
    font-weight: 500;
    transition: all 0.15s;
  }

  .player-opt:not(:first-child) {
    border-left: none;
  }

  .player-opt.active {
    background: var(--green);
    color: #000;
    border-color: var(--green);
    font-weight: 700;
  }

  .player-opt:hover:not(.active) {
    background: var(--bg-card-hover);
  }

  /* Port Forwarding Card */
  .port-forward-card {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .pf-status {
    margin-bottom: 2px;
  }

  .pf-badge {
    font-size: 12px;
    font-weight: 600;
    padding: 4px 8px;
  }

  .pf-success {
    color: var(--green);
    background: rgba(16, 185, 129, 0.1);
  }

  .pf-manual {
    color: #f59e0b;
    background: rgba(245, 158, 11, 0.1);
  }

  .pf-trying {
    color: var(--text-muted);
  }

  .pf-details {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .pf-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 12px;
  }

  .pf-label {
    color: var(--text-muted);
  }

  .pf-value {
    color: var(--text-primary);
  }

  .port-input {
    width: 80px;
    padding: 4px 8px;
    font-size: 12px;
    text-align: right;
  }

  .router-link {
    color: var(--green);
    text-decoration: none;
  }

  .router-link:hover {
    text-decoration: underline;
  }

  .pf-instructions {
    border-top: 1px solid var(--border);
    padding-top: 12px;
  }

  .pf-heading {
    font-size: 12px;
    font-weight: 600;
    color: var(--text-secondary);
    margin-bottom: 8px;
  }

  .pf-instructions ol {
    margin: 0;
    padding-left: 20px;
    font-size: 12px;
    color: var(--text-secondary);
    line-height: 1.7;
  }

  .pf-instructions li {
    margin-bottom: 2px;
  }

  .pf-help {
    font-size: 11px;
    color: var(--text-muted);
    margin-top: 8px;
  }

  .help-link {
    color: var(--green);
    text-decoration: none;
  }

  .help-link:hover {
    text-decoration: underline;
  }

  .actions {
    display: flex;
    gap: 12px;
    margin-top: 8px;
  }

  .btn {
    padding: 10px 20px;
    font-weight: 600;
    font-size: 13px;
    border: 1px solid var(--border);
    transition: all 0.15s;
  }

  .btn-cancel {
    background: var(--bg-input);
    color: var(--text-secondary);
    flex: 1;
  }

  .btn-cancel:hover {
    color: var(--text-primary);
    border-color: var(--border-hover);
  }

  .btn-create {
    background: var(--green);
    color: #000;
    border-color: var(--green);
    flex: 2;
  }

  .btn-create:hover:not(:disabled) {
    background: #0ea572;
  }

  .btn-create:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
