<script>
  import { onMount } from 'svelte';
  import { createEventDispatcher } from 'svelte';
  
  const dispatch = createEventDispatcher();
  
  let lobbies = [];
  let joinCode = '';
  let loading = true;

  const gameIcons = {
    'Halo 2': '🎯',
    'Halo: CE': '🔫',
    'Crimson Skies': '✈️',
    'MechAssault': '🤖',
    'Splinter Cell: CT': '🕵️',
    'Star Wars: Battlefront': '⭐',
    'Counter-Strike': '💣',
    'TimeSplitters 2': '⏰',
  };

  onMount(async () => {
    try {
      const result = await window.go.main.App.GetLobbies();
      if (result) lobbies = result;
    } catch (e) {
      // Dev mode - use demo data
      lobbies = [
        { id: 'demo-0', name: 'Friday Fragfest', game: 'Halo 2', host: 'SpartanChief', players: 4, maxPlayers: 8, ping: 32, region: 'NA-East', code: 'NACHO-1337' },
        { id: 'demo-1', name: 'Sky Pirates', game: 'Crimson Skies', host: 'AcePilot99', players: 2, maxPlayers: 4, ping: 45, region: 'EU-West', code: 'NACHO-2468' },
        { id: 'demo-2', name: 'Mech Madness', game: 'MechAssault', host: 'MechWarrior', players: 3, maxPlayers: 4, ping: 78, region: 'NA-West', code: 'NACHO-9001' },
        { id: 'demo-3', name: 'LAN Party', game: 'Halo: CE', host: 'RetroGamer42', players: 6, maxPlayers: 16, ping: 21, region: 'NA-East', code: 'NACHO-4200' },
        { id: 'demo-4', name: 'Spies vs Mercs', game: 'Splinter Cell: CT', host: 'ShadowAgent', players: 2, maxPlayers: 8, ping: 38, region: 'NA-East', code: 'NACHO-0007' },
      ];
    }
    loading = false;
  });

  function getPingColor(ping) {
    if (ping < 50) return 'var(--green)';
    if (ping < 100) return 'var(--yellow)';
    return 'var(--red)';
  }

  function getPingDot(ping) {
    if (ping < 50) return '🟢';
    if (ping < 100) return '🟡';
    return '🔴';
  }

  function joinByCode() {
    if (!joinCode.trim()) return;
    const lobby = lobbies.find(l => l.code === joinCode.trim().toUpperCase());
    if (lobby) {
      dispatch('join', lobby);
    }
  }

  function joinLobby(lobby) {
    dispatch('join', lobby);
  }
</script>

<div class="browser">
  <div class="browser-header">
    <div class="browser-title">
      <h2>🎮 Active Lobbies</h2>
      <span class="count">{lobbies.length} rooms</span>
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

  {#if loading}
    <div class="loading">
      <span class="spinner">🧀</span>
      <p>Finding lobbies...</p>
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
            </div>
          </div>
          <div class="lobby-stats">
            <div class="players">
              <span class="player-count">{lobby.players}/{lobby.maxPlayers}</span>
              <span class="player-label">players</span>
            </div>
            <div class="ping" style="color: {getPingColor(lobby.ping)}">
              <span class="ping-dot">{getPingDot(lobby.ping)}</span>
              <span class="ping-value mono">{lobby.ping}ms</span>
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
