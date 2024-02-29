// Copyright (c) 2024 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

module.exports = {
  devServer: {
    server: {
      type: 'http',
    },
    client: {
      webSocketURL: {
        hostname: '0.0.0.0',
        pathname: '/ws',
        protocol: 'auto',
        port: 0,
      },
    },
    allowedHosts: 'all',
    host: '127.0.0.1',
    port: 8121,
  }
}
