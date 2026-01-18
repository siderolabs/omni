# Contributing

## Legal Requirements
To protect the project and its users, we require all contributors to agree to a specific licensing flow and to "sign off" on their work.

### 1. Licensing of Contributions
By contributing to Omni, you agree that your contributions will be licensed under the [Zero-Clause BSD (0BSD)](https://opensource.org/license/0bsd) license.

Why? This ensures that community contributions remain as permissive as possible. While the Omni core is currently licensed under the Business Source License 1.1 (BSL), using 0BSD for inbound contributions allows Sidero Labs the flexibility to re-license the project in the future (e.g., to Apache 2.0) without needing to track down every individual contributor for permission.

The Terms: You can find the full text of the contribution license in the LICENSE-COMMUNITY file (0BSD).

### 2. Developer Certificate of Origin (DCO)
We use the standard [DCO](https://developercertificate.org/) "sign-off" process to ensure that every contribution has a clear chain of legal origin.

By adding a Signed-off-by line to your commit messages, you certify the following:

"The contribution was created in whole or in part by me and I have the right to submit it under the open source license indicated in the file."

In the context of this repository, the "license indicated in the file" refers to the 0BSD license as specified in our licensing guidelines.

### 3. How to Sign Your Work
You must sign off every commit.
If you have your name and email configured in git, you can do this automatically by adding the flag -s or `--signoff` to your commit command:

<pre>
git commit -s -m "feat: add support for new hardware"
</pre>

## Development

The build process for this project is designed to run entirely in containers.
To get started, run `make help` and follow the instructions.
