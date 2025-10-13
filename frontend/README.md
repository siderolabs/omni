# Omni frontend

## Recommended IDE Setup

[VSCode](https://code.visualstudio.com/) + [Volar](https://marketplace.visualstudio.com/items?itemName=Vue.volar) (and disable Vetur).

## Project Setup

```sh
npm install
```

### Compile and Hot-Reload for Development

```sh
npm run serve
```

If you want to access the dev server from a host that is not `localhost` or `*.localhost`, create a `.env` file in the frontend root and add `__VITE_ADDITIONAL_SERVER_ALLOWED_HOSTS=your.server`. If you want to enable the vue dev tools, add `ENABLE_DEVTOOLS=true`.

### Run storybook

The latest storybook from `main` will always be available at [https://main--68d664245076e2bb623d98f3.chromatic.com/](https://main--68d664245076e2bb623d98f3.chromatic.com/). To run storybook locally, you can use the command:

> [!WARNING]
> Vue dev tools are not compatible with storybook, so remember to disable them before trying to run storybook if you use them. See the relevant [issue](https://github.com/storybookjs/storybook/issues/32462).

```sh
npm run storybook
```

If you want to publish to storybook's chromatic service manually you may use do so with the following command but it requires the `CHROMATIC_PROJECT_TOKEN` environment variable to be set to our project's token.

```sh
npm run chromatic
```

### Type-Check, Compile and Minify for Production

```sh
npm run build
```

### Type-Check

```sh
npm run tsc
```

### Run Unit Tests with [Vitest](https://vitest.dev/)

```sh
npm run test
```

### Run E2E Tests with [Playwright](https://playwright.dev/)

The E2E tests depend on environment variables to function. Create a `.env` file in the frontend root and add at least these three:

```sh
AUTH_USERNAME=test-user@siderolabs.com
AUTH_PASSWORD=123
BASE_URL=https://omni.local/
```

- `AUTH_*` should be a username/password that already exists in your auth0 configuration
- `BASE_URL` should point to the omni instance under test

Note that `omnictl` related tests may fail if the config downloaded from omni has a different user or URL than the ones configured your environment variables. Make sure these are correctly in sync with in your [docker-compose.override.yml](../hack/compose/docker-compose.override.yml).

Tests can then be run with the `test:e2e` script.

```sh
npm run test:e2e
```

The tests can also be run inside a docker container. Use `-v` to mount the report and results directories if you want to inspect them, but it is not required for the tests to run.

```sh
docker buildx build --load . -t e2etest
docker run --rm \
  --env-file .env \
  -v ./playwright-report:/tmp/test/playwright-report \
  -v ./test-results:/tmp/test/test-results \
  --network=host \
  e2etest
```

### Lint with [ESLint](https://eslint.org/) and [Prettier](https://prettier.io/)

```sh
npm run lint
```

### Attempt to automatically fix lint issues where possible

```sh
npm run lint:fix
```
