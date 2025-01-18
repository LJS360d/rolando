# Rolando#7135

![Status](https://img.shields.io/website?url=https%3A%2F%2Fzlejon-middev.de%3A)
![Discord](https://img.shields.io/discord/1122938014637756486)

Discord Bot implementation of a MarkovChain to learn how to type like a user of the guild.<br>
Created with: [discordgo](https://github.com/bwmarrin/discordgo)

## Overview

Rolando is a Discord bot that leverages Markov Chains to mimic the speech patterns of users within the server. The bot guesses the next word in a sentence based on the patterns it has learned.

## Credits

The concept is inspired by [`Fioriktos`](https://github.com/FiorixF1/fioriktos-bot), a Telegram bot using similar principles.

## How to Startup Your Own Rolando

### Locally
- Prerequisites:
  - Go 1.23.0+
  - Make 4.4.1+
- Step by Step guide:
  - Starting up the Backend
  1. Copy the `.env.example` file to `.env` and follow the instructions written on it for the value of each needed environment variable
  2. Run `go mod download` to download dependencies
  3. Run `make run` to start the application in production mode or `make dev` to start the application in development mode
      - Optionally you can also run `air` to start a the application in dev mode with Hot reloading (you may need to edit the `air.toml` file depending on the OS you are using)
  - Starting up the Frontend (optional)
  1. Run `cd client` to enter the client directory
  2. Run `npm install` to install the required dependencies
  3. Run `npm run dev` to start the development server
  4. Open your browser and navigate to `http://localhost:3000`

### Production

- Prerequisites:
  - Make
  - Docker w/ Docker Compose
- Step by Step guide:
  1. Copy the `.env.example` file to `.env` and follow the instructions written on it for the value of each needed environment variable
      - Optionally you can add a `client/.env` file to set the environment variables for the frontend, refer to the `client/.env.development` file for more information
  2. Run `make run-docker` to build the docker compose file and start the application (both backend and frontend) in production mode

### Troubleshooting:

- If you get an error like `Error: failed to create session: 40001: Unknown account`, make sure you have the `bot` scope enabled in your Discord application.

## Contact

If you have any questions or issues with Rolando, you can join the official Discord Server: [Join Here](https://discord.gg/tyrj7wte5b) or DM the creator directly, username: `zlejon`

## Support the Project

Consider supporting the project to help with hosting costs.
[![Buy Me a Coffee](https://img.shields.io/badge/Buy%20Me%20a%20Coffee-Support%20the%20Project-brightgreen)](https://www.buymeacoffee.com/rolandobot)

Thanks for your support!
