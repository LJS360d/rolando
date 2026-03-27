defmodule Rolando.Repo do
  use Ecto.Repo,
    otp_app: :rolando,
    adapter: Ecto.Adapters.SQLite3
end
