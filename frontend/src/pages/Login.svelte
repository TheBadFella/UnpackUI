<script lang="ts">
  import { Spinner } from '@sveltestrap/sveltestrap'
  import { _ } from '../lib/i18n/Translate.svelte'
  import { login, profile } from '../lib/auth.svelte'
  import icon from '../assets/icon.png'

  let username = $state('admin')
  let password = $state('')
  let loading = $state(false)
  let error = $state('')

  async function onsubmit(e: Event) {
    e.preventDefault()
    if (!password) {
      error = $_('phrases.EnterPassword')
      return
    }
    loading = true
    error = ''
    error = await login(username, password)
    loading = false
  }
</script>

<div class="login-wrap">
  <div class="login-card shadow">
    <div class="login-card-header">
      <img src={icon} alt="" class="brand-logo" />
      <span class="login-card-title">Unpackerr</span>
    </div>
    <div class="login-card-subtitle">
      Sign in to UnpackUI
    </div>

    {#if profile.loginDisabled}
      <div role="status">
        <p class="fw-semibold mb-2" style="color: #ffffff;">{$_('pages.login.Disabled')}</p>
        <p class="mb-0 text-body-secondary" style="color: #a9a9a9 !important;">
          {$_('pages.login.DisabledHint')}
        </p>
      </div>
    {:else}
      <form {onsubmit}>
        <label for="username" class="login-form-label">
          {$_('pages.login.Username')}
        </label>
        <input
          id="username"
          type="text"
          class="login-form-input"
          bind:value={username}
          autocomplete="username"
        />

        <label for="password" class="login-form-label">
          {$_('pages.login.Password')}
        </label>
        <input
          id="password"
          type="password"
          class="login-form-input"
          bind:value={password}
          autocomplete="current-password"
        />

        <button
          type="submit"
          class="login-btn"
          disabled={loading}
        >
          {#if loading}<Spinner size="sm" class="me-2" />{/if}
          <span>{$_('buttons.Login')}</span>
        </button>
      </form>
    {/if}

    {#if error}
      <div
        class="login-error"
        role="alert"
        aria-live="assertive"
      >
        {error}
      </div>
    {/if}
  </div>
</div>
