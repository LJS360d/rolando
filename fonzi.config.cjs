/** @type {import('fonzi2').Config} */
module.exports = {
  logger: {
    enabled: true,
    levels: "all",
    remote: {
      enabled: true,
      levels: "all"
    },
    file: {
      enabled: true,
      levels: "all",
      path: "logs/"
    }
  }
}