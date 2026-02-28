# syntax=docker/dockerfile:1
FROM oven/bun:1.3.10-alpine AS build

WORKDIR /app

COPY package.json bun.lock ./
RUN --mount=type=cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile

COPY . .

RUN --mount=type=cache,target=/app/.cache \
    bun run build-only

FROM nginx:alpine

COPY --from=build /app/dist /usr/share/nginx/html
COPY ./nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
