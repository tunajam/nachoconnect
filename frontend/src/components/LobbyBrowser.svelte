<script>
  import { onMount, onDestroy } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  
  const dispatch = createEventDispatcher();
  
  let lobbies = [];
  let joinCode = '';
  let loading = true;
  // Server ping removed — meaningless pre-join. P2P ping shown after joining.
  let error = '';
  let refreshInterval;
  let coldStartMessage = '';

  const gameIcons = {
    'Halo 2': '🎯',
    'Halo: CE': '🔫',
    'Halo: Combat Evolved': '🔫',
    'Crimson Skies': '✈️',
    'MechAssault': '🤖',
    'MechAssault 2': '🤖',
    'Splinter Cell: CT': '🕵️',
    'Splinter Cell: Chaos Theory': '🕵️',
    'Splinter Cell: Pandora Tomorrow': '🕵️',
    'Star Wars: Battlefront': '⭐',
    'Star Wars: Battlefront II': '⭐',
    'Counter-Strike': '💣',
    'TimeSplitters 2': '⏰',
    'TimeSplitters: Future Perfect': '⏰',
    'Doom 3': '👹',
    'Rainbow Six 3': '🌈',
    'Ghost Recon': '👻',
    'Burnout 3: Takedown': '🏎️',
    'Forza Motorsport': '🏁',
  };

  onMount(async () => {
    await fetchLobbies();

    // Refresh lobbies every 10 seconds
    refreshInterval = setInterval(fetchLobbies, 10000);

    // No ping listener needed — P2P ping only relevant after joining
  });

  onDestroy(() => {
    if (refreshInterval) clearInterval(refreshInterval);
  });

  async function fetchLobbies(retryCount = 0, maxRetries = 5) {
    try {
      const start = Date.now();
      const resultPromise = window.go.main.App.GetLobbies();
      
      // If first load and taking >5s, show cold start message
      const timeoutId = loading ? setTimeout(() => {
        coldStartMessage = 'Lobby server is waking up, hang tight...';
      }, 5000) : null;

      const result = await resultPromise;
      if (timeoutId) clearTimeout(timeoutId);
      
      if (result) {
        lobbies = result;
        error = '';
        coldStartMessage = '';
      } else {
        lobbies = [];
      }
      loading = false;
    } catch (e) {
      if (retryCount < maxRetries) {
        coldStartMessage = 'Lobby server is waking up, hang tight...';
        setTimeout(() => fetchLobbies(retryCount + 1, maxRetries), 3000);
        return;
      }
      error = 'Unable to reach lobby server';
      coldStartMessage = '';
      lobbies = [];
      loading = false;
      // Emit critical error — server unreachable after retries
      if (window.runtime) {
        window.runtime.EventsEmit('error:critical', 'Lobby server unreachable after multiple attempts');
      }
    }
  }

  function getPingColor(ping) {
    if (ping === 0) return 'var(--text-muted)';
    if (ping < 50) return 'var(--green)';
    if (ping < 100) return 'var(--yellow)';
    return 'var(--red)';
  }

  function getPingDot(ping) {
    if (ping === 0) return '⚪';
    if (ping < 50) return '🟢';
    if (ping < 100) return '🟡';
    return '🔴';
  }

  async function joinByCode() {
    if (!joinCode.trim()) return;
    try {
      const lobby = await window.go.main.App.JoinLobby(joinCode.trim().toUpperCase());
      if (lobby) {
        dispatch('join', lobby);
        return;
      }
    } catch (e) {
      error = `Failed to join: ${e}`;
    }
  }

  async function joinLobby(lobby) {
    try {
      const result = await window.go.main.App.JoinLobby(lobby.code);
      if (result) {
        dispatch('join', result);
        return;
      }
    } catch (e) {
      error = `Failed to join: ${e}`;
    }
    dispatch('join', lobby);
  }
</script>

<div class="browser">
  <div class="browser-header">
    <div class="browser-title">
      <h2>🎮 Active Lobbies</h2>
      <span class="count">{lobbies.length} room{lobbies.length !== 1 ? 's' : ''}</span>
      <span class="p2p-badge">⚡ Direct P2P</span>
    </div>
    <div class="browser-actions">
      <div class="join-code">
        <input 
          type="text" 
          placeholder="Enter invite code..." 
          bind:value={joinCode}
          on:keydown={(e) => e.key === 'Enter' && joinByCode()}
        />
        <button class="btn btn-secondary" on:click={joinByCode}>Join</button>
      </div>
      <button class="btn btn-primary" on:click={() => dispatch('create')}>
        + Create Lobby
      </button>
    </div>
  </div>

  {#if error}
    <div class="error-msg">{error}</div>
  {/if}

  {#if loading}
    <div class="loading">
      <span class="spinner">🧀</span>
      <p>{coldStartMessage || 'Finding lobbies...'}</p>
    </div>
  {:else if lobbies.length === 0}
    <div class="empty">
      <span class="empty-icon">🧀</span>
      <h3>No active lobbies</h3>
      <p>Create one and invite your friends!</p>
      <button class="btn btn-primary" on:click={() => dispatch('create')}>
        Create Lobby
      </button>
    </div>
  {:else}
    <div class="lobby-list">
      {#each lobbies as lobby}
        <button class="lobby-card" on:click={() => joinLobby(lobby)}>
          <div class="lobby-game-icon">
            {gameIcons[lobby.game] || '🎮'}
          </div>
          <div class="lobby-info">
            <div class="lobby-name">{lobby.name}</div>
            <div class="lobby-meta">
              <span class="game-name">{lobby.game}</span>
              <span class="separator">•</span>
              <span class="host">Hosted by {lobby.host}</span>
              <span class="separator">•</span>
              <span class="mode-badge">⚡ Direct P2P</span>
            </div>
          </div>
          <div class="lobby-stats">
            <div class="players">
              <span class="player-count">{lobby.players}/{lobby.maxPlayers}</span>
              <span class="player-label">players</span>
            </div>
            <div class="p2p-indicator">
              <span class="mode-badge">⚡ P2P</span>
            </div>
            <div class="region mono">{lobby.region}</div>
          </div>
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .browser {
    max-width: 900px;
    margin: 0 auto;
  }

  .browser-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 20px;
    flex-wrap: wrap;
    gap: 12px;
  }

  .browser-title {
    display: flex;
    align-items: baseline;
    gap: 12px;
  }

  .browser-title h2 {
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary);
  }

  .count {
    font-size: 12px;
    color: var(--text-muted);
  }

  .p2p-badge {
    font-size: 11px;
    color: var(--green);
    font-weight: 600;
  }

  .p2p-indicator {
    display: flex;
    align-items: center;
    min-width: 65px;
  }

  .browser-actions {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .join-code {
    display: flex;
    gap: 0;
  }

  .join-code input {
    width: 180px;
    border-right: none;
  }

  .join-code .btn {
    border-left: none;
  }

  .btn {
    padding: 8px 16px;
    font-weight: 500;
    font-size: 13px;
    border: 1px solid var(--border);
    transition: all 0.15s;
  }

  .btn-primary {
    background: var(--green);
    color: #000;
    border-color: var(--green);
    font-weight: 600;
  }

  .btn-primary:hover {
    background: #0ea572;
  }

  .btn-secondary {
    background: var(--bg-card);
    color: var(--text-primary);
  }

  .btn-secondary:hover {
    background: var(--bg-card-hover);
    border-color: var(--border-hover);
  }

  .error-msg {
    background: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: #ef4444;
    padding: 8px 12px;
    font-size: 12px;
    margin-bottom: 12px;
  }

  .lobby-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .lobby-card {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 14px 18px;
    background: var(--bg-card);
    border: 1px solid var(--border);
    transition: all 0.15s;
    width: 100%;
    text-align: left;
    color: var(--text-primary);
  }

  .lobby-card:hover {
    background: var(--bg-card-hover);
    border-color: var(--border-hover);
  }

  .lobby-game-icon {
    font-size: 28px;
    width: 44px;
    height: 44px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    flex-shrink: 0;
  }

  .lobby-info {
    flex: 1;
    min-width: 0;
  }

  .lobby-name {
    font-size: 14px;
    font-weight: 600;
    margin-bottom: 3px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .lobby-meta {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--text-secondary);
  }

  .separator {
    color: var(--text-muted);
  }

  .mode-badge {
    font-size: 10px;
    padding: 1px 6px;
    border: 1px solid;
    font-weight: 600;
    color: var(--green);
    border-color: var(--green-dim, rgba(16, 185, 129, 0.3));
  }

  .lobby-stats {
    display: flex;
    align-items: center;
    gap: 20px;
    flex-shrink: 0;
  }

  .players {
    display: flex;
    flex-direction: column;
    align-items: center;
  }

  .player-count {
    font-weight: 600;
    font-size: 14px;
  }

  .player-label {
    font-size: 10px;
    color: var(--text-muted);
  }

  .ping {
    display: flex;
    align-items: center;
    gap: 4px;
    min-width: 65px;
  }

  .ping-dot {
    font-size: 8px;
  }

  .ping-value {
    font-size: 12px;
  }

  .region {
    font-size: 11px;
    color: var(--text-muted);
    min-width: 55px;
    text-align: right;
  }

  .loading, .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 60px 20px;
    color: var(--text-secondary);
    gap: 12px;
  }

  .spinner {
    font-size: 32px;
    animation: spin 2s linear infinite;
  }

  .empty-icon {
    font-size: 48px;
    opacity: 0.5;
  }

  .empty h3 {
    font-size: 16px;
    color: var(--text-primary);
  }

  .empty p {
    font-size: 13px;
    color: var(--text-muted);
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>
