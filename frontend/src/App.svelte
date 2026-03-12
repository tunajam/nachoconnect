<script>
  import { onMount } from 'svelte';
  import Header from './components/Header.svelte';
  import Setup from './components/Setup.svelte';
  import LobbyBrowser from './components/LobbyBrowser.svelte';
  import LobbyView from './components/LobbyView.svelte';
  import CreateLobby from './components/CreateLobby.svelte';
  import StatusBar from './components/StatusBar.svelte';

  let currentView = 'setup'; // setup | browser | lobby | create
  let status = {
    xboxDetected: false,
    xboxMAC: '',
    tunnelActive: false,
    connected: false,
    peerCount: 0,
    localIP: '',
    publicIP: '',
    interface: '',
    gamertag: '',
    serverPing: 0,
    error: ''
  };
  let currentLobby = null;
  let errorMessage = '';
  let showError = false;

  onMount(async () => {
    try {
      const s = await window.go.main.App.GetStatus();
      if (s) status = s;

      // Check if gamertag is set — if not, stay on setup
      const tag = await window.go.main.App.GetGamertag();
      if (tag) {
        status.gamertag = tag;
      }
    } catch (e) {
      console.log('Status fetch failed (dev mode):', e);
    }

    // Listen for events
    if (window.runtime) {
      window.runtime.EventsOn('status:update', (s) => {
        status = s;
      });
      window.runtime.EventsOn('xbox:detected', (mac) => {
        status.xboxDetected = true;
        status.xboxMAC = mac;
      });
      window.runtime.EventsOn('ping:update', (ping) => {
        status.serverPing = ping;
      });
      window.runtime.EventsOn('error', (msg) => {
        showErrorBanner(msg);
      });
      window.runtime.EventsOn('tunnel:reconnecting', (attempt) => {
        showErrorBanner(`Tunnel dropped — reconnecting (attempt ${attempt})...`);
      });
      window.runtime.EventsOn('tunnel:disconnected', () => {
        showErrorBanner('Tunnel disconnected');
      });
    }
  });

  function showErrorBanner(msg) {
    errorMessage = msg;
    showError = true;
    setTimeout(() => { showError = false; }, 8000);
  }

  function setupComplete(event) {
    const { interface: iface, mac, gamertag } = event.detail;
    if (iface) {
      status.interface = iface.name;
    }
    if (mac) {
      status.xboxDetected = true;
      status.xboxMAC = mac;
    }
    if (gamertag) {
      status.gamertag = gamertag;
    }
    currentView = 'browser';
  }

  function showBrowser() {
    currentView = 'browser';
    currentLobby = null;
  }

  function showCreate() {
    currentView = 'create';
  }

  function joinLobby(event) {
    currentLobby = event.detail;
    currentView = 'lobby';
  }

  function lobbyCreated(event) {
    currentLobby = event.detail;
    currentView = 'lobby';
  }
</script>

<div id="app">
  <Header {status} on:home={showBrowser} />
  
  {#if showError}
    <div class="error-banner">
      <span>⚠️ {errorMessage}</span>
      <button class="error-close" on:click={() => showError = false}>✕</button>
    </div>
  {/if}

  <main class="main-content">
    {#if currentView === 'setup'}
      <Setup on:ready={setupComplete} />
    {:else if currentView === 'browser'}
      <LobbyBrowser on:join={joinLobby} on:create={showCreate} />
    {:else if currentView === 'lobby'}
      <LobbyView lobby={currentLobby} on:leave={showBrowser} />
    {:else if currentView === 'create'}
      <CreateLobby on:created={lobbyCreated} on:cancel={showBrowser} />
    {/if}
  </main>

  <StatusBar {status} />
</div>

<style>
  .main-content {
    flex: 1;
    overflow-y: auto;
    padding: 20px 24px;
  }

  .error-banner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 24px;
    background: rgba(239, 68, 68, 0.15);
    border-bottom: 1px solid rgba(239, 68, 68, 0.3);
    color: #ef4444;
    font-size: 13px;
    animation: slideDown 0.2s ease-out;
  }

  .error-close {
    background: none;
    color: #ef4444;
    border: none;
    padding: 2px 6px;
    font-size: 14px;
    opacity: 0.7;
  }

  .error-close:hover {
    opacity: 1;
  }

  @keyframes slideDown {
    from { transform: translateY(-100%); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }
</style>
