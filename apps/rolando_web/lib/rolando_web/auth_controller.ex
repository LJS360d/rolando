defmodule RolandoWeb.AuthController do
  use RolandoWeb, :controller
  plug Ueberauth when action in [:request, :callback]

  def request(conn, _params), do: conn

  def callback(conn, _params) do
    case {conn.assigns[:ueberauth_auth], conn.assigns[:ueberauth_failure]} do
      {%Ueberauth.Auth{} = auth, _} ->
        uid = to_string(auth.uid)
        name = (auth.info && auth.info.name) || uid

        conn
        |> put_session("discord_user_id", uid)
        |> put_session("discord_username", name)
        |> put_flash(:info, "Signed in.")
        |> redirect(to: ~p"/operator")

      {_, %Ueberauth.Failure{}} ->
        conn
        |> put_flash(:error, "Discord sign-in failed.")
        |> redirect(to: ~p"/")

      _ ->
        conn
        |> put_flash(:error, "Discord sign-in failed.")
        |> redirect(to: ~p"/")
    end
  end

  def logout(conn, _params) do
    conn
    |> configure_session(drop: true)
    |> put_flash(:info, "Signed out.")
    |> redirect(to: ~p"/")
  end
end
