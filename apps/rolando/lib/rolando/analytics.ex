defmodule Rolando.Analytics do
  @moduledoc """
  Analytics facade. Delegates to the configured adapter so dev can use SQL
  and prod can use a scalable backend (event stream, external service, etc.).
  """
  @default_adapter Rolando.Analytics.SQLAdapter

  defp adapter do
    Application.get_env(:rolando, :analytics_adapter, @default_adapter)
  end

  def guilds_count do
    adapter().guilds_count()
  end
end
