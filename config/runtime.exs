import Config

# check if dotEnvy is available before import
if Code.ensure_loaded?(Dotenvy) do
  alias Dotenvy
  Dotenvy.source!([".env", System.get_env()]) |> System.put_env()
end

# config/runtime.exs is executed for all environments, including
# during releases. It is executed after compilation and before the
# system starts, so it is typically used to load production configuration
# and secrets from environment variables or elsewhere. Do not define
# any compile-time configuration in here, as it won't be applied.
# The block below contains prod specific runtime configuration.

config :rolando_web, RolandoWeb.Endpoint,
  http: [port: String.to_integer(System.get_env("PORT", "4000"))]

config :nostrum,
  token:
    System.get_env("DISCORD_BOT_TOKEN") ||
      (if(config_env() == :prod) do
         raise("""
         environment variable DISCORD_BOT_TOKEN is missing.
         """)
       else
         IO.puts("DISCORD_BOT_TOKEN is not set, discord interface will not run")
       end)

config :nostrum, youtubedl: nil
config :nostrum, streamlink: nil

config :ueberauth, Ueberauth.Strategy.Discord.OAuth,
  client_id: System.get_env("DISCORD_OAUTH_CLIENT_ID"),
  client_secret: System.get_env("DISCORD_OAUTH_CLIENT_SECRET")

owner_platform_ids = (System.get_env("OWNER_USER_IDS") || "") |> String.split(",", trim: true)
config :rolando_web, :owner_platform_ids, owner_platform_ids
config :rolando, :owner_platform_ids, owner_platform_ids

if config_env() == :prod do
  database_url =
    System.get_env("DATABASE_URL") ||
      raise """
      environment variable DATABASE_URL is missing.
      For example: ecto://USER:PASS@HOST/DATABASE
      """

  config :rolando, Rolando.Repo,
    url: database_url,
    # SSL is usually required for external DBs, but since we are in
    # the same Docker network as Postgres, we can keep it simple.
    pool_size: String.to_integer(System.get_env("POOL_SIZE") || "5"),
    socket_options: [:inet6]

  # The secret key base is used to sign/encrypt cookies and other secrets.
  # A default value is used in config/dev.exs and config/test.exs but you
  # want to use a different value for prod and you most likely don't want
  # to check this value into version control, so we use an environment
  # variable instead.
  secret_key_base =
    System.get_env("SECRET_KEY_BASE") ||
      raise """
      environment variable SECRET_KEY_BASE is missing.
      You can generate one by calling: mix phx.gen.secret
      """

  config :rolando_web, RolandoWeb.Endpoint,
    http: [
      # Enable IPv6 and bind on all interfaces.
      # Set it to  {0, 0, 0, 0, 0, 0, 0, 1} for local network only access.
      ip: {0, 0, 0, 0, 0, 0, 0, 0}
    ],
    secret_key_base: secret_key_base

  # ## Using releases
  #
  # If you are doing OTP releases, you need to instruct Phoenix
  # to start each relevant endpoint:
  #
  #     config :rolando_web, RolandoWeb.Endpoint, server: true
  #
  # Then you can assemble a release by calling `mix release`.
  # See `mix help release` for more information.

  # ## SSL Support
  #
  # To get SSL working, you will need to add the `https` key
  # to your endpoint configuration:
  #
  #     config :rolando_web, RolandoWeb.Endpoint,
  #       https: [
  #         ...,
  #         port: 443,
  #         cipher_suite: :strong,
  #         keyfile: System.get_env("SOME_APP_SSL_KEY_PATH"),
  #         certfile: System.get_env("SOME_APP_SSL_CERT_PATH")
  #       ]
  #
  # The `cipher_suite` is set to `:strong` to support only the
  # latest and more secure SSL ciphers. This means old browsers
  # and clients may not be supported. You can set it to
  # `:compatible` for wider support.
  #
  # `:keyfile` and `:certfile` expect an absolute path to the key
  # and cert in disk or a relative path inside priv, for example
  # "priv/ssl/server.key". For all supported SSL configuration
  # options, see https://hexdocs.pm/plug/Plug.SSL.html#configure/1
  #
  # We also recommend setting `force_ssl` in your config/prod.exs,
  # ensuring no data is ever sent via http, always redirecting to https:
  #
  #     config :rolando_web, RolandoWeb.Endpoint,
  #       force_ssl: [hsts: true]
  #
  # Check `Plug.SSL` for all available options in `force_ssl`.

  # ## Configuring the mailer
  #
  # In production you need to configure the mailer to use a different adapter.
  # Here is an example configuration for Mailgun:
  #
  #     config :rolando, Rolando.Mailer,
  #       adapter: Swoosh.Adapters.Mailgun,
  #       api_key: System.get_env("MAILGUN_API_KEY"),
  #       domain: System.get_env("MAILGUN_DOMAIN")
  #
  # Most non-SMTP adapters require an API client. Swoosh supports Req, Hackney,
  # and Finch out-of-the-box. This configuration is typically done at
  # compile-time in your config/prod.exs:
  #
  #     config :swoosh, :api_client, Swoosh.ApiClient.Req
  #
  # See https://hexdocs.pm/swoosh/Swoosh.html#module-installation for details.

  config :rolando, :dns_cluster_query, System.get_env("DNS_CLUSTER_QUERY")
end
