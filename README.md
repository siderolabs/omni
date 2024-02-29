<!-- markdownlint-disable first-line-heading sentences-per-line -->

<p align="center">
  <h1 align="center">Omni</h1>
  <p align="center">SaaS-simple deployment of Kubernetes - on your own hardware.</p>
  <p align="center">
    <a href="https://github.com/siderolabs/omni/releases/latest">
      <img alt="Release" src="https://img.shields.io/github/release/siderolabs/omni.svg?logo=github&logoColor=white&style=flat-square">
    </a>
    <a href="https://github.com/siderolabs/omni/releases/latest">
      <img alt="Pre-release" src="https://img.shields.io/github/release-pre/siderolabs/omni.svg?label=pre-release&logo=GitHub&logoColor=white&style=flat-square">
    </a>
  </p>
</p>

---

Kubernetes is wonderful, but requires scarce time and expertise to set up and manage.
Other solutions demand you have already set up Linux correctly, have all your servers in the same network, and meet their other criteria.
And hybrid clusters that span cloud and data center?
Good luck with that!

Omni allows you to start with bare metal, virtual machines or a cloud provider, and create clusters spanning all of your locations, with a few clicks.

You provide the machines – edge compute, bare metal, VMs, or in your cloud account. Boot from an Omni image. Click to allocate to a cluster. That’s it!

- Vanilla Kubernetes, on your machines, under your control.
- Elegant UI for management and operations
- Security taken care of – ties into your Enterprise ID provider
- Highly Available Kubernetes API endpoint built in
- Firewall friendly. Manage Edge nodes securely
- From single-node clusters to the largest scale
- Support for GPUs and most CSIs.

## Development

For instructions on developing Omni, see [DEVELOPMENT.md](DEVELOPMENT.md).

## Community

- Support: Questions, bugs, feature requests [GitHub Issues](https://github.com/siderolabs/omni/issues)
- Slack: Join our [slack channel](https://slack.dev.talos-systems.io)
- Twitter: [@SideroLabs](https://twitter.com/SideroLabs)
- Email: [info@SideroLabs.com](mailto:info@SideroLabs.com)

If you're interested in this project and would like to help in engineering efforts or have general usage questions, we are happy to have you!
We hold a weekly meeting that all audiences are welcome to attend.

### Office Hours

- When: Mondays at 16:30 UTC.
- Where: [Google Meet](https://meet.google.com/day-pxhv-zky).

You can subscribe to this meeting by joining the community forum above.

> Note: You can convert the meeting hours to your [local time](https://everytimezone.com/s/599e61d6).

## Contributing

Contributions are welcomed and appreciated!
See [Contributing](CONTRIBUTING.md) for our guidelines.

## Licenses

The Omni code is released under a combination of two licenses:

- The main Omni Server code is licensed under [Business Source License 1.1 (BSL-1.1)](LICENSE).
- The [Omni client library](client) is licensed under [Mozilla Public License 2.0 (MPL-2.0)](client/LICENSE).

When contributing to an Omni feature, you can find the relevant license in the comments at the top of each file.
