// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.
import 'storybook/internal/csf'

import type { SetupWorker } from 'msw/browser'

declare module 'storybook/internal/csf' {
  interface StoryContext {
    msw: SetupWorker
  }
}
