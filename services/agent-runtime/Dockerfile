FROM node:22-bookworm-slim AS build

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

COPY tsconfig.json ./
COPY src ./src
RUN npm run build

FROM node:22-bookworm-slim AS production-dependencies

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts --no-audit --no-fund

FROM node:22-bookworm-slim

ARG DIPOLE_VCS_REVISION=unknown
ARG DIPOLE_BUILD_CREATED=unknown
ARG DIPOLE_VCS_DIRTY=unknown

LABEL org.opencontainers.image.revision="${DIPOLE_VCS_REVISION}" \
      org.opencontainers.image.created="${DIPOLE_BUILD_CREATED}" \
      io.dipole.source.dirty="${DIPOLE_VCS_DIRTY}"

WORKDIR /app
ENV NODE_ENV=production

COPY --from=production-dependencies --chown=node:node /app/node_modules ./node_modules
COPY --from=build --chown=node:node /app/dist ./dist

USER node
EXPOSE 8091

CMD ["node", "dist/index.js"]
