# syntax=docker/dockerfile:1
FROM node:24-alpine AS build
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN --mount=type=cache,target=/pnpm/store pnpm install --frozen-lockfile --store-dir /pnpm/store
COPY . .
ENV SITE_URL=http://site.invalid \
    INVITE_URL=__INVITE_URL__ \
    REPO_URL=__REPO_URL__
RUN pnpm run build

FROM nginx:alpine
ENV API_URL=http://rolando-0:8080 \
    SITE_URL=http://localhost
COPY nginx.conf.template /etc/nginx/templates/default.conf.template
# Build bakes http://site.invalid as Astro's `site` and placeholders for the
# optional links; swap in the env on container start. An unset link becomes
# empty, which the page treats as absent.
COPY --chmod=755 <<'EOF' /docker-entrypoint.d/40-env.sh
#!/bin/sh
# escape for a sed replacement (# is the delimiter) inside an HTML attribute
esc() { printf '%s' "$1" | sed -e 's/"/%22/g' -e 's/[\\&#]/\\&/g'; }
find /usr/share/nginx/html -type f \( -name '*.html' -o -name '*.xml' -o -name '*.txt' \) \
	-exec sed -i -e "s#http://site.invalid#${SITE_URL%/}#g" \
	-e "s#__INVITE_URL__#$(esc "$INVITE_URL")#g" \
	-e "s#__REPO_URL__#$(esc "$REPO_URL")#g" {} +
EOF
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
