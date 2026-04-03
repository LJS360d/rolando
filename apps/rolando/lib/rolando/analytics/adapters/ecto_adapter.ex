defmodule Rolando.Analytics.EctoAdapter do
  @behaviour Rolando.Analytics.Adapter
  import Ecto.Query
  alias Rolando.Repo
  alias Rolando.Schema.{AnalyticsEvent, Guild}

  @impl true
  def guilds_count do
    Repo.aggregate(Guild, :count, :id)
  end

  @impl true
  def persist_event(attrs) do
    normalized = Rolando.Analytics.Normalize.event_attrs(attrs)

    %AnalyticsEvent{}
    |> AnalyticsEvent.changeset(normalized)
    |> Repo.insert()
  end

  @impl true
  def list_recent_events(limit, filters)
      when is_integer(limit) and limit > 0 and is_map(filters) do
    q =
      from(e in AnalyticsEvent,
        order_by: [desc: e.inserted_at],
        limit: ^limit
      )

    q =
      case filters[:event_type] do
        t when is_binary(t) and t != "" ->
          from(e in q, where: e.event_type == ^t)

        _ ->
          q
      end

    q =
      case filters[:guild_id] do
        g when is_binary(g) and g != "" ->
          from(e in q, where: e.guild_id == ^g)

        _ ->
          q
      end

    q =
      case filters[:since] do
        %DateTime{} = dt ->
          from(e in q, where: e.inserted_at >= ^dt)

        _ ->
          q
      end

    Repo.all(q)
  end

  @impl true
  def event_counts_by_day(days) when is_integer(days) and days > 0 do
    since = DateTime.utc_now() |> DateTime.add(-days, :day)

    from(e in AnalyticsEvent,
      where: e.inserted_at >= ^since,
      group_by: fragment("strftime('%Y-%m-%d', ?)", e.inserted_at),
      select: {fragment("strftime('%Y-%m-%d', ?)", e.inserted_at), count(e.id)}
    )
    |> Repo.all()
    |> Enum.map(fn {day, n} -> {to_string(day), n} end)
  end
end
