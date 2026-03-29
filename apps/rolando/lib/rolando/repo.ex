defmodule Rolando.Repo do
  @default_adapter Ecto.Adapters.SQLite3
  @adapter Application.compile_env(:rolando, :repo_adapter, @default_adapter)

  use Ecto.Repo,
    otp_app: :rolando,
    adapter: @adapter
end
