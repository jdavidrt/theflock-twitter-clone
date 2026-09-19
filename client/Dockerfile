# Client image (D-70). Build context is the repo root (npm workspaces: the lockfile is at the
# root, D-45). A Node stage builds the Vite bundle; nginx serves it and proxies /api.

# --- build ---------------------------------------------------------------------------------
FROM node:20-alpine AS build
WORKDIR /app

# Install layer: only re-runs when a manifest or the lockfile changes. This is a throwaway
# build stage, so the extra root devDeps never reach the nginx runtime image.
#
# `npm ci` does not install rollup's platform-specific native binary from a lockfile generated
# on another OS (npm bug #4828 — the Windows-authored lockfile leaves the linux-musl optional
# uninstalled), which makes `vite build` fail with "Cannot find module
# @rollup/rollup-linux-x64-musl". Install that one binary explicitly, pinned to the rollup
# version npm actually resolved, so it can never drift from the lockfile.
COPY package.json package-lock.json ./
COPY client/package.json client/package.json
RUN npm ci \
 && npm install --no-save @rollup/rollup-linux-x64-musl@"$(node -p "require('rollup/package.json').version")"

# Build layer.
COPY client/ client/
RUN npm run build --workspace=client

# --- runtime -------------------------------------------------------------------------------
FROM nginx:alpine
COPY --from=build /app/client/dist /usr/share/nginx/html
COPY client/nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
