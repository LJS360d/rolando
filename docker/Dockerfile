FROM node:20.9.0

WORKDIR /app

ADD .env /app/.env
ADD dist /app/dist
ADD package.json /app/package.json
ADD node_modules /app/node_modules
ADD views /app/views
ADD public /app/public

CMD ["npm", "run", "start"]