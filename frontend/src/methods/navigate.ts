// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

/**
 * Redirect the user's top-most window to the given URL.
 *
 * This makes sure that the redirect works correctly when the call comes from inside an iframe.
 *
 * @param url The URL to redirect to.
 */
export function redirectToURL(url: string) {
  if (window.top) {
    window.top.location.href = url
  } else {
    window.location.href = url
  }
}
