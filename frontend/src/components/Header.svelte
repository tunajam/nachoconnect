<script>
  import { createEventDispatcher } from 'svelte';
  export let status;
  const dispatch = createEventDispatcher();
  let showAbout = false;

  function openLink(url) {
    if (window.runtime) {
      window.runtime.BrowserOpenURL(url);
    } else {
      window.open(url, '_blank');
    }
  }
</script>

<header class="header">
  <div class="header-left" on:click={() => dispatch('home')} on:keydown={() => {}}>
    <span class="logo">🧀</span>
    <span class="title">NachoConnect</span>
    <button class="version" on:click|stopPropagation={() => showAbout = !showAbout}>v0.3.3</button>
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
  {#if showAbout}
    <div class="about-overlay" on:click|self={() => showAbout = false} on:keydown={() => {}}>
      <div class="about-modal">
        <div class="about-header">
          <span class="about-logo">🧀</span>
          <h2>NachoConnect v0.3.3</h2>
        </div>
        <p class="about-tagline">Original Xbox System Link over the Internet</p>
        <div class="about-links">
          <button class="about-link" on:click={() => openLink('https://github.com/tunajam/nachoconnect')}>📦 GitHub</button>
          <button class="about-link" on:click={() => openLink('https://github.com/tunajam/nachoconnect/issues')}>🐛 Report Issue</button>
          <button class="about-link" on:click={() => openLink('https://github.com/tunajam/nachoconnect/blob/main/PLAYTEST.md')}>📋 PLAYTEST.md</button>
        </div>
        <p class="about-footer">Built by TunaJam</p>
        <button class="about-close" on:click={() => showAbout = false}>✕</button>
      </div>
    </div>
  {/if}
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
    cursor: pointer;
  }

  .version:hover {
    border-color: var(--green-dim);
    color: var(--text-primary);
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

  .about-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
    -webkit-app-region: no-drag;
  }

  .about-modal {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    padding: 32px;
    text-align: center;
    max-width: 320px;
    width: 100%;
    position: relative;
  }

  .about-header {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 10px;
    margin-bottom: 8px;
  }

  .about-logo {
    font-size: 28px;
  }

  .about-header h2 {
    font-size: 16px;
    font-weight: 700;
    color: var(--text-primary);
    margin: 0;
  }

  .about-tagline {
    font-size: 12px;
    color: var(--text-muted);
    margin: 0 0 20px;
  }

  .about-links {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin-bottom: 20px;
  }

  .about-link {
    background: var(--bg-card);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    padding: 8px 16px;
    font-size: 12px;
    cursor: pointer;
    text-align: left;
    font-family: inherit;
  }

  .about-link:hover {
    border-color: var(--green-dim);
    color: var(--text-primary);
  }

  .about-footer {
    font-size: 11px;
    color: var(--text-muted);
    margin: 0;
  }

  .about-close {
    position: absolute;
    top: 8px;
    right: 12px;
    background: none;
    border: none;
    color: var(--text-muted);
    font-size: 16px;
    cursor: pointer;
    padding: 4px;
  }

  .about-close:hover {
    color: var(--text-primary);
  }
</style>
