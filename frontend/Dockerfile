FROM mcr.microsoft.com/playwright:v1.55.0-noble

COPY e2e /tmp/test/e2e
COPY package.json /tmp/test
COPY package-lock.json /tmp/test
COPY playwright.config.ts /tmp/test

WORKDIR /tmp/test

RUN npm ci

CMD ["npm", "run", "test:e2e"]
