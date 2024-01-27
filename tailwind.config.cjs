/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: "class",
  important: true,
  content: [
    './views/**/*.ejs',
    './node_modules/@daisyui/core/dist/**/*.js',
  ],
  theme: {
    extend: {},
  },
  plugins: [require('daisyui')],
  daisyui: {
    themes: [
      "dark",
      "night",
      "light",
    ],
  },
}

