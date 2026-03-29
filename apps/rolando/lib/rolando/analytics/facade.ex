defmodule Rolando.Analytics do
  @moduledoc """
  Analytics facade. Delegates to the configured adapter so dev can use SQL
  and prod can use a scalable backend (event stream, external service, etc.).
  """
  @behaviour Rolando.Analytics.Adapter
  @default_adapter Rolando.Analytics.EctoAdapter
  require Logger

  defp adapter do
    Application.get_env(:rolando, :analytics_adapter, @default_adapter)
  end

  def guilds_count do
    adapter().guilds_count()
  end

  def persist_event(attrs) do
    case Map.get(attrs, :level, nil) do
      :debug -> Logger.debug(inspect(attrs, pretty: true))
      :warn -> Logger.warning(inspect(attrs, pretty: true))
      :error -> Logger.error(inspect(attrs, pretty: true))
      _ -> Logger.info(inspect(attrs, pretty: true))
    end

    adapter().persist_event(attrs)
  end

  def track(attrs), do: persist_event(attrs)

  @doc """
  Utility that delegates to persist_event/1
  """
  def track(name, attrs), do: persist_event(Map.put(attrs, :name, name))
end
