<script>
  import { onMount, onDestroy } from 'svelte';
  import { createEventDispatcher } from 'svelte';

  export let lobby;
  const dispatch = createEventDispatcher();

  let copied = false;
  let players = lobby?.members || [];
  let tunnelStatus = 'connecting'; // connecting | connected | disconnected | reconnecting
  let refreshInterval;
  let serverPing = 0;

  onMount(() => {
    // Refresh lobby data periodically to get updated player list / pings
    refreshInterval = setInterval(refreshLobby, 5000);

    if (window.runtime) {
      window.runtime.EventsOn('tunnel:connected', () => {
        tunnelStatus = 'connected';
      });
      window.runtime.EventsOn('tunnel:disconnected', () => {
        tunnelStatus = 'disconnected';
      });
      window.runtime.EventsOn('tunnel:reconnecting', (attempt) => {
        tunnelStatus = 'reconnecting';
      });
      window.runtime.EventsOn('tunnel:skipped', (reason) => {
        tunnelStatus = 'connected'; // Show as connected even without tunnel (for browsing)
      });
      window.runtime.EventsOn('ping:update', (ping) => {
        serverPing = ping;
      });
    }

    // Check initial tunnel status
    checkTunnelStatus();
  });

  onDestroy(() => {
    if (refreshInterval) clearInterval(refreshInterval);
  });

  async function checkTunnelStatus() {
    try {
      const status = await window.go.main.App.GetStatus();
      if (status?.tunnelActive) {
        tunnelStatus = 'connected';
      } else {
        // Give tunnel time to connect
        setTimeout(async () => {
          const s = await window.go.main.App.GetStatus();
          tunnelStatus = s?.tunnelActive ? 'connected' : 'disconnected';
        }, 3000);
      }
    } catch (e) {
      tunnelStatus = 'connected'; // Dev mode
    }
  }

  async function refreshLobby() {
    try {
      const updated = await window.go.main.App.RefreshLobby();
      if (updated && updated.members) {
        players = updated.members;
      }
    } catch (e) {}
  }

  function copyCode() {
    const code = lobby?.code || 'NACHO-0000';
    navigator.clipboard?.writeText(code);
    copied = true;
    setTimeout(() => copied = false, 2000);
  }

  async function leaveLobby() {
    try {
      await window.go.main.App.LeaveLobby(lobby?.id);
    } catch (e) {}
    dispatch('leave');
  }

  function getPingColor(ping) {
    if (ping === 0) return 'var(--text-muted)';
    if (ping < 50) return 'var(--green)';
    if (ping < 100) return 'var(--yellow)';
    return 'var(--red)';
  }
</script>

<div class="lobby-view">
  <div class="lobby-header">
    <button class="back-btn" on:click={leaveLobby}>← Back</button>
    <div class="lobby-title">
      <h2>{lobby?.name || 'Lobby'}</h2>
      <span class="lobby-game">{lobby?.game || 'Unknown Game'}</span>
    </div>
    <div class="lobby-actions">
      <button class="btn btn-code" on:click={copyCode}>
        {#if copied}
          ✓ Copied!
        {:else}
          📋 {lobby?.code || 'NACHO-0000'}
        {/if}
      </button>
    </div>
  </div>

  <div class="lobby-body">
    <div class="panel players-panel">
      <div class="panel-header">
        <h3>Players ({players.length}/{lobby?.maxPlayers || 8})</h3>
      </div>
      <div class="player-list">
        {#each players as player}
          <div class="player-row" class:is-host={player.isHost} class:is-you={player.isYou}>
            <div class="player-info">
              {#if player.isHost}
                <span class="host-badge">⭐</span>
              {/if}
              <span class="player-name">{player.name}</span>
              {#if player.isYou}
                <span class="you-badge">you</span>
              {/if}
            </div>
            <span class="player-ping mono" style="color: {getPingColor(player.isYou ? serverPing : player.ping)}">
              {#if player.isYou}
                {serverPing > 0 ? serverPing + 'ms' : '--'}
              {:else}
                {player.ping > 0 ? player.ping + 'ms' : '--'}
              {/if}
            </span>
          </div>
        {/each}
      </div>

      <div class="connection-info">
        <div class="conn-row">
          <span class="conn-label">Mode</span>
          <span class="conn-value">⚡ Direct P2P</span>
        </div>
        <div class="conn-row">
          <span class="conn-label">Connection</span>
          <span class="conn-value">
            {#if tunnelStatus === 'connecting'}
              🟡 Connecting to host...
            {:else if tunnelStatus === 'connected'}
              🟢 Connected
            {:else if tunnelStatus === 'reconnecting'}
              🟡 Reconnecting...
            {:else}
              🔴 Disconnected
            {/if}
          </span>
        </div>
        <div class="conn-row">
          <span class="conn-label">Tunnel</span>
          <span class="conn-value">
            {#if tunnelStatus === 'connecting'}
              🔄 Establishing...
            {:else if tunnelStatus === 'connected'}
              🟢 Active
            {:else if tunnelStatus === 'reconnecting'}
              🔄 Reconnecting...
            {:else}
              ⚠️ Down
            {/if}
          </span>
        </div>
        {#if serverPing > 0}
          <div class="conn-row">
            <span class="conn-label">Ping</span>
            <span class="conn-value mono" style="color: {getPingColor(serverPing)}">
              {serverPing}ms
            </span>
          </div>
        {/if}
      </div>
    </div>

    <div class="panel discord-panel">
      <div class="panel-header">
        <h3>Communication</h3>
      </div>
      <div class="discord-placeholder">
        <div class="discord-icon">💬</div>
        <p class="discord-title">Discord Integration Coming Soon</p>
        <p class="discord-subtitle">Voice chat and text channels will be linked directly to your lobby.</p>
      </div>
    </div>
  </div>

  <div class="lobby-footer">
    <button class="btn btn-leave" on:click={leaveLobby}>Leave Lobby</button>
  </div>
</div>

<style>
  .lobby-view {
    max-width: 900px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    height: calc(100vh - 120px);
  }

  .lobby-header {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 16px;
  }

  .back-btn {
    background: none;
    color: var(--text-secondary);
    padding: 6px 12px;
    border: 1px solid var(--border);
    font-size: 13px;
  }

  .back-btn:hover {
    color: var(--text-primary);
    border-color: var(--border-hover);
  }

  .lobby-title {
    flex: 1;
  }

  .lobby-title h2 {
    font-size: 18px;
    font-weight: 600;
  }

  .lobby-game {
    font-size: 12px;
    color: var(--text-secondary);
  }

  .btn-code {
    background: var(--bg-card);
    color: var(--green);
    border: 1px solid var(--green-dim);
    padding: 6px 14px;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .btn-code:hover {
    background: var(--green-glow);
  }

  .lobby-body {
    display: grid;
    grid-template-columns: 280px 1fr;
    gap: 12px;
    flex: 1;
    min-height: 0;
  }

  .panel {
    background: var(--bg-card);
    border: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .panel-header {
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
  }

  .panel-header h3 {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .player-list {
    flex: 1;
    padding: 8px;
    overflow-y: auto;
  }

  .player-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 10px;
    border-bottom: 1px solid var(--bg-secondary);
  }

  .player-row:last-child {
    border-bottom: none;
  }

  .player-info {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .host-badge {
    font-size: 12px;
  }

  .player-name {
    font-size: 13px;
    font-weight: 500;
  }

  .you-badge {
    font-size: 10px;
    color: var(--text-muted);
    background: var(--bg-secondary);
    padding: 1px 5px;
    border: 1px solid var(--border);
  }

  .player-ping {
    font-size: 12px;
  }

  .connection-info {
    padding: 12px 16px;
    border-top: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .conn-row {
    display: flex;
    justify-content: space-between;
    font-size: 12px;
  }

  .conn-label {
    color: var(--text-muted);
  }

  .conn-value {
    color: var(--text-secondary);
  }

  .discord-panel {
    display: flex;
    flex-direction: column;
  }

  .discord-placeholder {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 32px 24px;
    text-align: center;
    gap: 8px;
  }

  .discord-icon {
    font-size: 32px;
    opacity: 0.5;
    margin-bottom: 8px;
  }

  .discord-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-secondary);
  }

  .discord-subtitle {
    font-size: 12px;
    color: var(--text-muted);
    max-width: 220px;
    line-height: 1.4;
  }

  .lobby-footer {
    padding-top: 12px;
    display: flex;
    justify-content: flex-start;
  }

  .btn-leave {
    background: var(--bg-card);
    color: var(--red);
    border: 1px solid var(--red-dim);
    padding: 8px 16px;
  }

  .btn-leave:hover {
    background: var(--red-dim);
    color: var(--text-primary);
  }
</style>
