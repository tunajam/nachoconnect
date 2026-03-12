<script>
  import { onMount, onDestroy } from 'svelte';
  import { createEventDispatcher } from 'svelte';

  export let lobby;
  const dispatch = createEventDispatcher();

  let chatMessages = [
    { sender: lobby?.host || 'Host', message: 'Welcome to the lobby! 🧀', time: new Date().toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'}) },
  ];
  let chatInput = '';
  let copied = false;

  // Simulated players for demo
  let players = lobby?.members || [
    { name: lobby?.host || 'SpartanChief', ping: lobby?.ping || 32, isHost: true, isYou: false, connected: true },
    { name: 'You', ping: 0, isHost: false, isYou: true, connected: true },
  ];

  let tunnelStatus = 'connecting'; // connecting | connected | relayed

  onMount(() => {
    // Simulate tunnel connection
    setTimeout(() => {
      tunnelStatus = 'connected';
      addSystemMessage('Tunnel established — direct connection 🟢');
    }, 1500);

    if (window.runtime) {
      window.runtime.EventsOn('chat:message', (msg) => {
        chatMessages = [...chatMessages, msg];
      });
      window.runtime.EventsOn('tunnel:connected', () => {
        tunnelStatus = 'connected';
      });
    }
  });

  function addSystemMessage(text) {
    chatMessages = [...chatMessages, { sender: '🧀', message: text, time: now(), system: true }];
  }

  function sendChat() {
    if (!chatInput.trim()) return;
    const msg = { sender: 'You', message: chatInput, time: now() };
    chatMessages = [...chatMessages, msg];
    try {
      window.go.main.App.SendChat(lobby?.id, chatInput);
    } catch (e) {}
    chatInput = '';
  }

  function copyCode() {
    const code = lobby?.code || 'NACHO-0000';
    navigator.clipboard?.writeText(code);
    copied = true;
    setTimeout(() => copied = false, 2000);
  }

  function leaveLobby() {
    try {
      window.go.main.App.LeaveLobby(lobby?.id);
    } catch (e) {}
    dispatch('leave');
  }

  function now() {
    return new Date().toLocaleTimeString([], {hour: '2-digit', minute:'2-digit'});
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
            <span class="player-ping mono" style="color: {getPingColor(player.ping)}">
              {player.isYou ? '--' : player.ping + 'ms'}
            </span>
          </div>
        {/each}
      </div>

      <div class="connection-info">
        <div class="conn-row">
          <span class="conn-label">Connection</span>
          <span class="conn-value">
            {#if tunnelStatus === 'connecting'}
              🟡 Connecting...
            {:else if tunnelStatus === 'connected'}
              🟢 Direct
            {:else}
              🟡 Relayed
            {/if}
          </span>
        </div>
        <div class="conn-row">
          <span class="conn-label">Tunnel</span>
          <span class="conn-value">
            {#if tunnelStatus === 'connecting'}
              🔄 Establishing...
            {:else}
              🟢 Active
            {/if}
          </span>
        </div>
      </div>
    </div>

    <div class="panel chat-panel">
      <div class="panel-header">
        <h3>Chat</h3>
      </div>
      <div class="chat-messages">
        {#each chatMessages as msg}
          <div class="chat-msg" class:system={msg.system}>
            <span class="chat-time mono">{msg.time}</span>
            <span class="chat-sender" class:system-sender={msg.system}>{msg.sender}</span>
            <span class="chat-text">{msg.message}</span>
          </div>
        {/each}
      </div>
      <div class="chat-input">
        <input 
          type="text" 
          placeholder="Type a message..." 
          bind:value={chatInput}
          on:keydown={(e) => e.key === 'Enter' && sendChat()}
        />
        <button class="btn btn-send" on:click={sendChat}>Send</button>
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

  .chat-panel {
    display: flex;
    flex-direction: column;
  }

  .chat-messages {
    flex: 1;
    overflow-y: auto;
    padding: 12px 16px;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .chat-msg {
    font-size: 13px;
    line-height: 1.5;
  }

  .chat-msg.system {
    color: var(--text-muted);
    font-style: italic;
    font-size: 12px;
  }

  .chat-time {
    color: var(--text-muted);
    font-size: 10px;
    margin-right: 6px;
  }

  .chat-sender {
    font-weight: 600;
    color: var(--green);
    margin-right: 4px;
  }

  .chat-sender.system-sender {
    color: var(--text-muted);
  }

  .chat-text {
    color: var(--text-primary);
  }

  .chat-input {
    display: flex;
    border-top: 1px solid var(--border);
  }

  .chat-input input {
    flex: 1;
    border: none;
    background: var(--bg-input);
    padding: 12px 16px;
  }

  .btn-send {
    background: var(--green);
    color: #000;
    padding: 12px 20px;
    font-weight: 600;
  }

  .btn-send:hover {
    background: #0ea572;
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
