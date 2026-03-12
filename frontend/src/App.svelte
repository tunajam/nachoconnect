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
    interface: ''
  };
  let currentLobby = null;

  onMount(async () => {
    try {
      const s = await window.go.main.App.GetStatus();
      if (s) status = s;
    } catch (e) {
      console.log('Status fetch failed (dev mode):', e);
    }

    // Listen for status updates
    if (window.runtime) {
      window.runtime.EventsOn('status:update', (s) => {
        status = s;
      });
      window.runtime.EventsOn('xbox:detected', (mac) => {
        status.xboxDetected = true;
        status.xboxMAC = mac;
      });
    }
  });

  function setupComplete(event) {
    const { interface: iface, mac } = event.detail;
    if (iface) {
      status.interface = iface.name;
    }
    if (mac) {
      status.xboxDetected = true;
      status.xboxMAC = mac;
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
</style>
