// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
const preloadReloadedKey = 'vite-preload-reloaded'

export function registerPreloadListener() {
  let navigatingAway = false

  // On firefox, cancelled requests from navigation triggers vite:preloadError.
  // This can cause a redirect loop when unauthenticated and being redirected to auth.
  window.addEventListener('beforeunload', () => {
    navigatingAway = true
  })

  // A preloadError occurs when frontend fails to load hashed assets.
  // This usually means Omni was updated, and we are running on outdated code.
  window.addEventListener('vite:preloadError', () => {
    if (navigatingAway || sessionStorage.getItem(preloadReloadedKey)) return

    // Avoid looping forever if reloading doesn't actually fix the failure.
    sessionStorage.setItem(preloadReloadedKey, 'true')

    window.location.reload()
  })

  // Re-arm the guard once the app has had a reasonable window to finish loading, so a genuine
  // future preload failure (e.g. a later deploy) still gets its one automatic retry.
  setTimeout(() => sessionStorage.removeItem(preloadReloadedKey), 10_000)
}
