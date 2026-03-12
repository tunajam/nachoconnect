<script>
  import { createEventDispatcher } from 'svelte';
  const dispatch = createEventDispatcher();

  let name = '';
  let game = 'Halo 2';
  let maxPlayers = 8;
  let creating = false;
  let mode = 'direct'; // 'direct' or 'relay'
  let directPort = 9999;
  let publicIP = '';
  let detectingIP = false;

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

  async function detectIP() {
    detectingIP = true;
    try {
      publicIP = await window.go.main.App.DetectPublicIP();
    } catch (e) {
      publicIP = '(could not detect)';
    }
    detectingIP = false;
  }

  // Auto-detect IP when switching to direct mode
  $: if (mode === 'direct' && !publicIP) {
    detectIP();
  }

  async function createLobby() {
    if (!name.trim()) return;
    creating = true;

    try {
      const lobby = await window.go.main.App.CreateLobbyWithMode(name, game, maxPlayers, mode, directPort);
      if (lobby) {
        dispatch('created', lobby);
        return;
      }
    } catch (e) {
      console.log('Create lobby fallback (dev mode)', e);
    }

    creating = false;
  }
</script>

<div class="create">
  <div class="create-card">
    <h2>🧀 Create Lobby</h2>
    <p class="subtitle">Set up a room for your friends to join</p>

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
        <label>Connection Mode</label>
        <div class="mode-selector">
          <button 
            class="mode-opt" 
            class:active={mode === 'direct'}
            on:click={() => mode = 'direct'}
          >
            <span class="mode-icon">⚡</span>
            <span class="mode-label">Direct (Best Latency)</span>
          </button>
          <button 
            class="mode-opt" 
            class:active={mode === 'relay'}
            on:click={() => mode = 'relay'}
          >
            <span class="mode-icon">☁️</span>
            <span class="mode-label">Relay (No Port Forward)</span>
          </button>
        </div>
        {#if mode === 'direct'}
          <div class="direct-info">
            <div class="direct-row">
              <span class="direct-label">Your Public IP</span>
              <span class="direct-value mono">
                {#if detectingIP}detecting...{:else}{publicIP || '--'}{/if}
              </span>
            </div>
            <div class="direct-row">
              <span class="direct-label">UDP Port</span>
              <input 
                type="number" 
                class="port-input" 
                bind:value={directPort} 
                min="1024" 
                max="65535"
              />
            </div>
            <p class="direct-hint">
              ℹ️ Forward UDP port {directPort} on your router to this machine, or let UPnP handle it automatically.
            </p>
          </div>
        {/if}
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

      <div class="actions">
        <button class="btn btn-cancel" on:click={() => dispatch('cancel')}>Cancel</button>
        <button 
          class="btn btn-create" 
          on:click={createLobby}
          disabled={!name.trim() || creating}
        >
          {creating ? 'Creating...' : 'Create Lobby'}
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
    width: 420px;
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

  .mode-selector {
    display: flex;
    gap: 0;
  }

  .mode-opt {
    flex: 1;
    padding: 10px 12px;
    background: var(--bg-input);
    color: var(--text-secondary);
    border: 1px solid var(--border);
    font-weight: 500;
    font-size: 12px;
    display: flex;
    align-items: center;
    gap: 6px;
    justify-content: center;
    transition: all 0.15s;
  }

  .mode-opt:not(:first-child) {
    border-left: none;
  }

  .mode-opt.active {
    background: var(--green);
    color: #000;
    border-color: var(--green);
    font-weight: 700;
  }

  .mode-opt:hover:not(.active) {
    background: var(--bg-card-hover);
  }

  .mode-icon {
    font-size: 14px;
  }

  .direct-info {
    margin-top: 10px;
    padding: 12px;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .direct-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 12px;
  }

  .direct-label {
    color: var(--text-muted);
  }

  .direct-value {
    color: var(--text-primary);
  }

  .port-input {
    width: 80px;
    padding: 4px 8px;
    font-size: 12px;
    text-align: right;
  }

  .direct-hint {
    font-size: 11px;
    color: var(--text-muted);
    line-height: 1.4;
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
