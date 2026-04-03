defmodule RolandoWeb.OperatorAuth do
  @moduledoc false
  import Phoenix.Component
  import Phoenix.LiveView

  def on_mount(:ensure_operator, _params, session, socket) do
    uid = session["discord_user_id"]
    allow = Application.get_env(:rolando_web, :owner_platform_ids, [])

    cond do
      is_nil(uid) ->
        {:halt,
         socket
         |> put_flash(:error, "Sign in required.")
         |> redirect(to: "/auth/discord")}

      to_string(uid) in allow ->
        scope = %{
          type: :operator,
          user_id: to_string(uid),
          name: session["discord_username"]
        }

        {:cont, assign(socket, :current_scope, scope)}

      true ->
        {:halt,
         socket
         |> put_flash(:error, "You do not have operator access.")
         |> redirect(to: "/")}
    end
  end
end
