<script lang="ts">
  import { onMount } from 'svelte'
  import {
    Spinner,
  } from '@sveltestrap/sveltestrap'
  import { _ } from '../lib/i18n/Translate.svelte'
  import PageIntro from '../components/PageIntro.svelte'
  import { api } from '../lib/api'
  import { profile } from '../lib/auth.svelte'
  import { hashLinkClick } from '../lib/router.svelte'
  import { dateTime } from '../lib/format'
  import { success, failure } from '../lib/toast'
  import type { SystemInfo } from '../lib/types'
  import unpackerrIcon from '../assets/icon.png'
  import githubIcon from '../assets/github.svg'
    import notifiarrIcon from '../assets/notifiarr.svg'
  import xtIcon from '../assets/xt.png'

  const links = [
    {
      href: 'https://unpackerr.zip',
      icon: unpackerrIcon,
      label: 'pages.system.Website',
      hint: 'unpackerr.zip',
    },
    {
      href: 'https://github.com/Unpackerr/unpackerr',
      icon: githubIcon,
      label: 'pages.system.GitHub',
      hint: 'github.com/Unpackerr/unpackerr',
      mono: true,
    },
    
    {
      href: 'https://notifiarr.com',
      icon: notifiarrIcon,
      label: 'pages.system.Notifiarr',
      hint: 'notifiarr.com',
    },
    {
      href: 'https://unpackerr.zip/xt',
      icon: xtIcon,
      label: 'pages.system.Xt',
      hint: 'unpackerr.zip/xt',
    },
  ]

  let info = $state<SystemInfo | null>(null)
  let error = $state('')
  let loading = $state(true)
  let exportText = $state('')
  let exporting = $state(false)

  onMount(async () => {
    const res = await api.get<SystemInfo>('system')
    if (res.ok) info = res.body
    else
      error =
        (res.body as { error?: string })?.error ?? 'failed to load system info'
    loading = false
  })

  async function loadExport() {
    exporting = true
    const res = await api.get<{ text: string }>('system/export')
    if (res.ok) exportText = res.body?.text ?? ''
    else
      failure(
        (res.body as { error?: string })?.error ?? 'failed to load live export',
      )
    exporting = false
  }

  function hostLabel(sys: SystemInfo): string {
    const name = sys.hostname?.trim()
    const os = sys.goos?.trim()
    if (name && os) {
      return `${name} (${os})`
    }
    return name || os || $_('phrases.Empty')
  }

  async function copyExport() {
    try {
      await navigator.clipboard.writeText(exportText)
      success($_('phrases.Copied'))
    } catch {
      // clipboard may be blocked on non-HTTPS
    }
  }
</script>


<h4 class="mb-2">{$_('pages.system.Title')}</h4>
<PageIntro id="pages.system" />

{#if loading}
  <Spinner color="primary" />
{:else if error}
  <p class="text-danger">{error}</p>
{:else if info}
  <div class="detail-grid">
    <div class="detail-section">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 18px;">
        <div class="detail-section-title">{$_('pages.system.Instance')}</div>
        <button
          class="action-button"
          onclick={(e) => hashLinkClick(e, '/docs')}
        >
          {$_('nav.API')}
        </button>
      </div>

      <div class="detail-lines">
        <div style="display: flex; gap: 12px;">
          <div style="width: 140px;" class="detail-label">{$_('pages.system.Version')}</div>
          <div class="detail-value">{info.version}</div>
        </div>
        <div style="display: flex; gap: 12px;">
          <div style="width: 140px;" class="detail-label">{$_('pages.system.Host')}</div>
          <div class="detail-value"><code>{hostLabel(info)}</code></div>
        </div>
        <div style="display: flex; gap: 12px;">
          <div style="width: 140px;" class="detail-label">{$_('pages.system.Started')}</div>
          <div class="detail-value">{dateTime(info.started)}</div>
        </div>
        <div style="display: flex; gap: 12px;">
          <div style="width: 140px;" class="detail-label">{$_('pages.system.Uptime')}</div>
          <div class="detail-value">{info.uptime}</div>
        </div>
        <div style="display: flex; gap: 12px;">
          <div style="width: 140px;" class="detail-label">{$_('pages.system.Listen')}</div>
          <div class="detail-value"><code>{info.listenAddr}</code></div>
        </div>
        <div style="display: flex; gap: 12px;">
          <div style="width: 140px;" class="detail-label">{$_('pages.system.ConfigFile')}</div>
          <div class="detail-value"><code>{info.configFile || $_('phrases.Empty')}</code></div>
        </div>
        <div style="display: flex; gap: 12px;">
          <div style="width: 140px;" class="detail-label">{$_('pages.system.Logs')}</div>
          <div class="detail-value"><code>{info.logs || $_('phrases.Empty')}</code></div>
        </div>
        <div style="display: flex; gap: 12px; align-items: center;">
          <div style="width: 140px; margin-bottom:0;" class="detail-label">{$_('pages.system.Auth')}</div>
          <div class="chip-tag chip-auth">{info.auth}</div>
        </div>
        <div style="display: flex; gap: 12px; align-items: center;">
          <div style="width: 140px; margin-bottom:0;" class="detail-label">{$_('pages.system.Metrics')}</div>
          <div class="chip-tag">
            <div class="status-dot {info.metrics ? 'dot-good' : 'dot-muted'}"></div>
            {info.metrics ? $_('pages.system.enabled') : $_('pages.system.disabled')}
          </div>
        </div>
      </div>
    </div>

    <div style="display: flex; flex-direction: column; gap: 18px;">
      <div class="detail-section">
        <div class="detail-section-title" style="margin-bottom: 18px;">{$_('pages.system.You')}</div>
        <div class="detail-lines">
          <div style="display: flex; gap: 12px;">
            <div style="width: 120px;" class="detail-label">{$_('pages.system.Username')}</div>
            <div class="detail-value">{profile.info?.username}</div>
          </div>
          <div style="display: flex; gap: 12px;">
            <div style="width: 120px;" class="detail-label">{$_('pages.system.Via')}</div>
            <div class="detail-value">{profile.info?.via}</div>
          </div>
          <div style="display: flex; gap: 12px; align-items: flex-start;">
            <div style="width: 120px; margin-top: 4px;" class="detail-label">{$_('pages.system.Permissions')}</div>
            <div style="display: flex; flex-wrap: wrap; gap: 6px;">
              {#if profile.info?.permissions.includes('*')}
                <div class="chip-tag chip-admin">
                  <div class="status-dot dot-good"></div>
                  {$_('phrases.AdminAll')}
                </div>
              {:else}
                {#each profile.info?.permissions ?? [] as perm (perm)}
                  <div class="chip-tag">{perm}</div>
                {/each}
              {/if}
            </div>
          </div>
        </div>
      </div>

      <div class="detail-section">
        <div class="detail-section-title" style="margin-bottom: 14px;">{$_('pages.system.Links')}</div>
        <div class="link-grid">
          {#each links as link (link.href)}
            <a
              class="system-link-card"
              href={link.href}
              title={link.hint}
              target="_blank"
              rel="noopener noreferrer"
            >
              <img
                src={link.icon}
                alt=""
                class={['link-ico', link.mono && 'link-ico-mono'].filter(Boolean).join(' ')}
              />
              <span class="min-w-0">
                <span class="link-title">{$_(link.label)}</span>
                <span class="link-hint">{link.hint}</span>
              </span>
            </a>
          {/each}
        </div>
      </div>
    </div>

    <div class="detail-section detail-section-wide">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px;">
        <div class="detail-section-title">{$_('buttons.LiveExport')}</div>
        <div style="display: flex; gap: 8px;">
          <button
            class="action-button"
            disabled={exporting}
            onclick={loadExport}
            style="display: flex; align-items: center; gap: 6px;"
          >
            {#if exporting}<Spinner size="sm" />{/if}
            <span>{$_('buttons.LiveExport')}</span>
          </button>
          {#if exportText}
            <button class="action-button" onclick={copyExport}>
              {$_('buttons.Copy')}
            </button>
          {/if}
        </div>
      </div>
      {#if exportText}
        <pre class="export-pre mb-0">{exportText}</pre>
      {/if}
    </div>
  </div>
{/if}

<style>
  code {
    color: var(--dash-text);
    background: transparent;
    font-family: inherit;
    font-size: 0.86rem;
    padding: 0;
  }

  .link-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
    column-gap: 1rem;
    row-gap: 1rem;
  }

  .system-link-card {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    background: #111;
    border: 1px solid var(--dash-border);
    border-radius: 0;
    padding: 12px 14px;
    text-decoration: none;
    transition: border-color 140ms ease, background 140ms ease, transform 140ms ease;
  }

  .system-link-card:hover {
    border-color: var(--dash-accent);
    background: #1c1210;
    transform: translateY(-1px);
  }

  .link-ico {
    width: 22px;
    height: 22px;
    object-fit: contain;
    flex-shrink: 0;
    filter: brightness(0.9);
  }

  :global([data-bs-theme='dark']) .link-ico-mono {
    filter: brightness(0.9) invert(1);
  }

  .link-title {
    display: block;
    font-size: 0.86rem;
    font-weight: 600;
    color: #fff;
    line-height: 1.2;
  }

  .link-hint {
    display: block;
    font-size: 0.74rem;
    color: var(--dash-muted);
    margin-top: 4px;
  }

  .chip-tag {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 8px;
    border-radius: 0;
    border: 1px solid var(--dash-border);
    background: #111;
    color: var(--dash-text);
    font-size: 0.76rem;
    font-weight: 600;
    letter-spacing: 0.03em;
    text-transform: uppercase;
    white-space: nowrap;
  }

  .chip-tag.chip-auth {
    border-color: #0d5a7d;
    background: #061f2d;
    color: #79d5ff;
  }

  .chip-tag.chip-admin {
    border-color: #1a5639;
    background: #0a2417;
    color: #26e69a;
  }

  .status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .status-dot.dot-good {
    background: #16c784;
    box-shadow: 0 0 6px rgba(22, 199, 132, 0.6);
  }

  .status-dot.dot-muted {
    background: #555;
  }

  .export-pre {
    background: #0f0f0f !important;
    border: 1px solid var(--dash-border) !important;
    color: #f2f2f2 !important;
    border-radius: 0 !important;
    font-family: 'JetBrains Mono', 'Cascadia Mono', Consolas, monospace !important;
    font-size: 0.82rem;
    line-height: 1.5;
    padding: 16px;
    margin-top: 12px;
    white-space: pre-wrap;
    word-break: break-word;
  }
</style>

