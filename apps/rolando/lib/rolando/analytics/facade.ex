defmodule Rolando.Analytics do
  @moduledoc """
  Analytics facade. Delegates to the configured adapter so dev can use SQL
  and prod can use a scalable backend (event stream, external service, etc.).
  """
  @default_adapter Rolando.Analytics.EctoAdapter
  require Logger

  defp adapter do
    Application.get_env(:rolando, :analytics_adapter, @default_adapter)
  end

  def guilds_count do
    adapter().guilds_count()
  end

  def persist_event(attrs) do
    level = extract_level(attrs)
    log_at_level(level, attrs)

    case adapter().persist_event(attrs) do
      {:ok, _} = result ->
        Rolando.Analytics.Sync.broadcast_ui_refresh()
        result

      other ->
        other
    end
  end

  def track(attrs), do: persist_event(attrs)

  def track(name, attrs), do: persist_event(Map.put(attrs, :name, name))

  def list_recent_events(limit \\ 100, filters \\ %{}) do
    adapter().list_recent_events(limit, filters)
  end

  def event_counts_by_day(days \\ 7) do
    adapter().event_counts_by_day(days)
  end

  defp extract_level(attrs) when is_map(attrs) do
    Map.get(attrs, :level) || Map.get(attrs, "level")
  end

  defp log_at_level(level, attrs) do
    case level do
      :debug -> Logger.debug(inspect(attrs, pretty: true))
      :warn -> Logger.warning(inspect(attrs, pretty: true))
      :error -> Logger.error(inspect(attrs, pretty: true))
      "debug" -> Logger.debug(inspect(attrs, pretty: true))
      "warn" -> Logger.warning(inspect(attrs, pretty: true))
      "error" -> Logger.error(inspect(attrs, pretty: true))
      _ -> Logger.info(inspect(attrs, pretty: true))
    end
  end
end
