# Rolando#7135

![Status](https://img.shields.io/website?url=http%3A%2F%2F136.243.177.154%3A25576)
![Discord](https://img.shields.io/discord/1122938014637756486)

Discord Bot implementation of a MarkovChain to learn how to type like a user of the guild.<br>
Created with: [Fonzi2 framework](https://github.com/LJS360d/fonzi2)

## Overview

Rolando is a Discord bot that leverages Markov Chains to mimic the speech patterns of users within the server. The bot guesses the next word in a sentence based on the patterns it has learned.

## Credits

The concept is inspired by [`Fioriktos`](https://github.com/FiorixF1/fioriktos-bot), a Telegram bot using similar principles.

## How to Startup Your Own Rolando

- Prerequisites:
  - Node.js Version 18+
- Reccomended for development:
  - nvm (Node version manager)
  - pnpm
  - docker-compose
  - docker desktop

### Development:

1. Rename the file `example.env` in `.env` and follow the instructions written on it for the value of each needed enviroment variable
2. Go to the `docker` folder and run `docker-compose up -d` to create the local database connection <br> (ignore the node container for development)
2. Run `npm run dev` to start the local dev server
2. Run `npm run dev:css` to watch for changes in the tailwind classes used in the frontend pages
3. You now have a running rolando dev environment.

### Production:

1. Rename the file `example.env` in `.env` and follow the instructions written on it for the value of each needed enviroment variable for production
2. Run `npm run build` to build the project with webpack
3. Run `npm run start` to start the application in production mode

## Contact

If you have any questions or issues with Rolando, you can join the official Discord Server: [Join Here](https://discord.gg/tyrj7wte5b) or DM the creator directly, username: `zlejon`

## Support the Project

Consider supporting the project to help with hosting costs.  
[![Buy Me a Coffee](https://img.shields.io/badge/Buy%20Me%20a%20Coffee-Support%20the%20Project-brightgreen)](https://www.buymeacoffee.com/rolandobot)

Thanks for your support!
