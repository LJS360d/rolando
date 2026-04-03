defmodule RolandoWeb.Router do
  use RolandoWeb, :router

  pipeline :browser do
    plug :accepts, ["html"]
    plug :fetch_session
    plug :fetch_live_flash
    plug :put_root_layout, html: {RolandoWeb.Layouts, :root}
    plug :protect_from_forgery
    plug :put_secure_browser_headers
    plug RolandoWeb.Plugs.FetchCurrentScope
  end

  pipeline :api do
    plug :accepts, ["json"]
  end

  scope "/", RolandoWeb do
    pipe_through :browser

    get "/", PageController, :home
    get "/privacy", PageController, :privacy
    get "/terms", PageController, :terms
  end

  scope "/auth", RolandoWeb do
    pipe_through :browser

    get "/logout", AuthController, :logout
    get "/:provider", AuthController, :request
    get "/:provider/callback", AuthController, :callback
  end

  live_session :operator,
    on_mount: [{RolandoWeb.OperatorAuth, :ensure_operator}] do
    scope "/", RolandoWeb do
      pipe_through :browser

      live "/operator", OperatorLive, :index
    end
  end

  if Application.compile_env(:rolando_web, :dev_routes) do
    import Phoenix.LiveDashboard.Router

    scope "/dev" do
      pipe_through :browser

      live_dashboard "/dashboard", metrics: RolandoWeb.Telemetry
      forward "/mailbox", Plug.Swoosh.MailboxPreview
    end
  end
end
