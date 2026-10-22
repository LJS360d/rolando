# syntax=docker/dockerfile:1
FROM node:24-alpine AS build
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN --mount=type=cache,target=/pnpm/store pnpm install --frozen-lockfile --store-dir /pnpm/store
COPY . .
ENV SITE_URL=http://site.invalid
RUN pnpm run build

FROM nginx:alpine
ENV API_URL=http://rolando-0:8080 \
    SITE_URL=http://localhost
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
# Build bakes http://site.invalid as Astro's `site`; swap in SITE_URL on container start.
COPY --chmod=755 <<'EOF' /docker-entrypoint.d/40-site-url.sh
#!/bin/sh
find /usr/share/nginx/html -type f \( -name '*.html' -o -name '*.xml' -o -name '*.txt' \) \
	-exec sed -i "s#http://site.invalid#${SITE_URL%/}#g" {} +
EOF
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
