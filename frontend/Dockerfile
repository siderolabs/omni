FROM mcr.microsoft.com/playwright:v1.56.1-noble

WORKDIR /tmp/test

COPY package.json package-lock.json ./
RUN npm ci

COPY e2e ./e2e
COPY playwright.config.ts ./

CMD ["npm", "run", "test:e2e"]
