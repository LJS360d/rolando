defmodule RolandoWeb.Plugs.FetchCurrentScope do
  @moduledoc false
  import Plug.Conn

  def init(opts), do: opts

  def call(conn, _opts) do
    uid = get_session(conn, "discord_user_id")
    allow = Application.get_env(:rolando_web, :owner_platform_ids, [])

    scope =
      cond do
        is_nil(uid) ->
          nil

        to_string(uid) in allow ->
          %{type: :operator, user_id: to_string(uid), name: get_session(conn, "discord_username")}

        true ->
          %{type: :user, user_id: to_string(uid), name: get_session(conn, "discord_username")}
      end

    assign(conn, :current_scope, scope)
  end
end
