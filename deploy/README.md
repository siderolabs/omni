## Omni On-Prem Compose File

- Copy `env.template` and edit all fields necessary to match local paths to keys, domain names, etc.
- Run docker compose, supplying the environment file edited above: `docker compose --env-file <path-to-env> up -d`