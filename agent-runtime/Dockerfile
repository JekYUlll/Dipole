FROM node:22-bookworm-slim AS build

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

COPY tsconfig.json ./
COPY src ./src
RUN npm run build && npm prune --omit=dev --ignore-scripts

FROM node:22-bookworm-slim

WORKDIR /app
ENV NODE_ENV=production

COPY --from=build --chown=node:node /app/package.json /app/package-lock.json ./
COPY --from=build --chown=node:node /app/node_modules ./node_modules
COPY --from=build --chown=node:node /app/dist ./dist

USER node
EXPOSE 8091

CMD ["node", "dist/index.js"]
