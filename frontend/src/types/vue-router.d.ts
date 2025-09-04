// Copyright (c) 2025 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import 'vue-router'

declare module 'vue-router' {
  interface RouteMeta {
    title?: string
  }
}
