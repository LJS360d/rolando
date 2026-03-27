# Rolando

![Status](https://img.shields.io/website?url=https%3A%2F%2Fzlejon-middev.de%3A)
![Discord](https://img.shields.io/discord/1122938014637756486)

## Overview

Rolando is a bot that leverages Markov Chains to mimic the speech patterns of users within chat rooms. It guesses the next word in a sentence based on the patterns it has learned.

## Support the Project

Consider supporting the project to help with hosting costs.
[![Buy Me a Coffee](https://img.shields.io/badge/Buy%20Me%20a%20Coffee-Support%20the%20Project-brightgreen)](https://www.buymeacoffee.com/rolandobot)

Thanks for your support!


## Credits

The concept is inspired by [`Fioriktos`](https://github.com/FiorixF1/fioriktos-bot), a Telegram bot using similar principles.

## Contact

If you have any questions or issues with Rolando, you can join the official Discord Server: [Join Here](https://discord.gg/tyrj7wte5b) or DM the creator directly, username: `zlejon`


## Development

### Architecture

- **`apps/rolando`** — Core: Ecto repo, DB schemas and the markov chain engine. Single source of truth for data and pull logic.
- **`apps/rolando_web`** — Phoenix app: admin panel for the core and public website pages.
- **`apps/rolando_discord`** — Discord bot with Nostrum.

Dev: SQLite, one node, no extra services. Prod: configurable DB path and pool, optional DNS-based clustering; the core can be swapped to a sharded or beefier DB by changing Repo config and migrations.

### Requirements

- Elixir and Erlang (e.g. via [mise](https://mise.jdx.dev/) — see `mise.toml`).
- For Discord: a bot token.

### Setup

From the project root:

```bash
mix setup
```

This installs and sets up dependencies for all umbrella apps. Then:

1. Copy `.env.example` to `.env` and set `DISCORD_BOT_TOKEN` if you will run the Discord bot.
2. Run migrations and seeds from the core app:

   ```bash
   mix ecto.setup
   ```

3. Start everything (interactive):

   ```bash
   iex -S mix phx.server
   ```

Web UI: [http://localhost:4000](http://localhost:4000).

### Configuration

- **Development** — SQLite DB path and pool are in `config/dev.exs`. No `DATABASE_PATH` required.
- **Production** — Set in `config/runtime.exs` (or env):
  - `DATABASE_PATH` — path to the SQLite DB file.
  - `SECRET_KEY_BASE` — for Phoenix (e.g. `mix phx.gen.secret`).
  - `DISCORD_BOT_TOKEN` — required if the Discord app is started.
  - Optional: `PORT`, `POOL_SIZE`, `DNS_CLUSTER_QUERY` for clustering.

Secrets and env-based config only; no credentials in the repo.

### Releases

A single OTP release runs all three apps:

```bash
mix release rolando
```

Start with `./_build/prod/rel/rolando/bin/rolando start`. For production, set `DATABASE_PATH`, `SECRET_KEY_BASE`, and `DISCORD_BOT_TOKEN` in the environment.

### Project layout

Run:
```sh
tree -I  '_build|.elixir_ls|deps|node_modules|dist'
```

### Checks

From the root:

```bash
mix precommit
```

Runs compile with warnings-as-errors, dependency cleanup, format, and tests across the umbrella.