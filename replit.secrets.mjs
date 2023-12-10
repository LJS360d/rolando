import { readFileSync, writeFileSync } from 'fs';
import { parse } from 'dotenv';

// Read the .env file
const envContent = readFileSync('.env', 'utf8');

// Parse the .env content
const parsedEnv = parse(envContent);

// Convert the parsed environment variables to JSON
const jsonContent = JSON.stringify(parsedEnv, null, 2);

// Write the JSON content to a file
writeFileSync('replit.secrets.json', jsonContent, 'utf8');

console.log('Parsed .env into replit secrets JSON');
