defmodule RolandoWeb.PageController do
  use RolandoWeb, :controller

  def home(conn, _params) do
    render(conn, :home)
  end
end
