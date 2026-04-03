# This file is responsible for configuring your umbrella
# and **all applications** and their dependencies with the
# help of the Config module.
#
# Note that all applications in your umbrella share the
# same configuration and dependencies, which is why they
# all use the same configuration file. If you want different
# configurations or dependencies per app, it is best to
# move said applications out of the umbrella.
import Config

# Configure Mix tasks and generators
config :rolando,
  repo_adapter: Ecto.Adapters.SQLite3,
  ecto_repos: [Rolando.Repo],
  analytics_adapter: Rolando.Analytics.EctoAdapter,
  cache_adapter: Rolando.Cache.ETSAdapter,
  lm_adapter: Rolando.LM.Adapters.RedisMarkovChain,
  train_channel_max_concurrency: 6,
  train_message_limit_per_channel: 750_000,
  train_max_fetch_errors_per_channel: 5

# Configure the mailer
#
# By default it uses the "Local" adapter which stores the emails
# locally. You can see the emails in your browser, at "/dev/mailbox".
#
# For production it's recommended to configure a different adapter
# at the `config/runtime.exs`.
config :rolando, Rolando.Mailer, adapter: Swoosh.Adapters.Local

config :rolando_web,
  ecto_repos: [Rolando.Repo],
  generators: [context_app: :rolando]

# Configures the endpoint
config :rolando_web, RolandoWeb.Endpoint,
  url: [host: "localhost"],
  adapter: Bandit.PhoenixAdapter,
  render_errors: [
    formats: [html: RolandoWeb.ErrorHTML, json: RolandoWeb.ErrorJSON],
    layout: false
  ],
  pubsub_server: Rolando.PubSub,
  live_view: [signing_salt: "yCzoMl/U"]

# Configure esbuild (the version is required)
config :esbuild,
  version: "0.25.4",
  rolando_web: [
    args:
      ~w(js/app.js --bundle --target=es2022 --outdir=../priv/static/assets/js --external:/fonts/* --external:/images/* --alias:@=.),
    cd: Path.expand("../apps/rolando_web/assets", __DIR__),
    env: %{"NODE_PATH" => [Path.expand("../deps", __DIR__), Mix.Project.build_path()]}
  ]

# Configure tailwind (the version is required)
config :tailwind,
  version: "4.1.12",
  rolando_web: [
    args: ~w(
      --input=assets/css/app.css
      --output=priv/static/assets/css/app.css
    ),
    cd: Path.expand("../apps/rolando_web", __DIR__)
  ]

# Configure Elixir's Logger
config :logger, :default_formatter,
  format: "$time $metadata[$level] $message\n",
  metadata: [:request_id]

# Use Jason for JSON parsing in Phoenix
config :phoenix, :json_library, Jason

config :ueberauth, Ueberauth,
  providers: [
    discord: {Ueberauth.Strategy.Discord, []}
  ]

# Import environment specific config. This must remain at the bottom
# of this file so it overrides the configuration defined above.
import_config "#{config_env()}.exs"
