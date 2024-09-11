## Omni On-Prem Compose File

Follow the full documentation in the [Omni docs](https://omni.siderolabs.com/how-to-guides/self_hosted/index).

The template requires the following environment variables to be set (change these for your environment):

```bash
OMNI_VERSION=v0.42.0
OMNI_ACCOUNT_UUID=$(uuidgen)
OMNI_DOMAIN_NAME=omni.siderolabs.com
OMNI_WG_IP=10.10.1.100
OMNI_ADMIN_EMAIL=omni@siderolabs.com
AUTH0_CLIENT_ID=xxxyyyzzz
AUTH0_DOMAIN=dev-aaabbbccc.us.auth0.com
```

You may also want to update certificate paths, etcd storage, and other settings.

- Copy `env.template` and edit all fields necessary
- Run docker compose, supplying the environment file edited above: `docker compose --env-file <path-to-env> up -d`
