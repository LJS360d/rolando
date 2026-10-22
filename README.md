# Rolando#7135

![Status](https://img.shields.io/website?url=https%3A%2F%2Frolando.stables-tech.com%3A)
![Discord](https://img.shields.io/discord/1122938014637756486)

Discord Bot implementation of a MarkovChain to learn how to type like a user of the guild.<br>

## Overview

Rolando is a Discord bot that leverages Markov Chains to mimic the speech patterns of users within the server. The bot guesses the next word in a sentence based on the patterns it has learned.

## Credits

The concept is inspired by [`Fioriktos`](https://github.com/FiorixF1/fioriktos-bot), a Telegram bot using similar principles.

## How to Startup Your Own Rolando

### Locally
- Prerequisites:
  - Go 1.26+, Make, pnpm (versions in `bot/mise.toml`, `web/mise.toml`)
- Step by Step guide:
  1. `cd bot`, copy `.env.example` to `.env` and follow the instructions written on it for the value of each needed environment variable
  2. Run `make dev` (development) or `make build run` (production); `make run` loads `.env` into the environment
  3. Web UI: `cd web && pnpm install && pnpm dev`
  4. Upgrading from the old single `rolando.db`: run `./bin/rolando migrate split-db path/to/rolando.db` once

### Production

Prebuilt docker images: `ghcr.io/ljs360d/rolando` (bot + API) and `ghcr.io/ljs360d/rolando/web` (web UI). Single node: copy `docker-compose.prod.yml` to the server next to a `.env` (vars from `bot/.env.example`), `chown 1000:1000 data`, `docker compose -f docker-compose.prod.yml up -d`.
The repo's `docker-compose.yml` builds from source (`docker compose up --build`, reads `bot/.env`) and is meant for development.

### Troubleshooting:

- If you get an error like `Error: failed to create session: 40001: Unknown account`, make sure you have the `bot` scope enabled in your Discord application.

## Contact

If you have any questions or issues with Rolando, you can join the official Discord Server: [Join Here](https://discord.gg/tyrj7wte5b) or DM the creator directly, username: `zlejon`

## Support the Project

Consider supporting the project to help with hosting costs.
[![Buy Me a Coffee](https://img.shields.io/badge/Buy%20Me%20a%20Coffee-Support%20the%20Project-brightgreen)](https://www.buymeacoffee.com/rolandobot)

Thanks for your support!
