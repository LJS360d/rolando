defmodule Rolando.Analytics.Normalize do
  @moduledoc false

  def event_attrs(attrs) when is_map(attrs) do
    attrs = stringify_keys(attrs)

    event_type =
      attrs["event_type"] || attrs["event"] || attrs["name"] ||
        raise(ArgumentError, "analytics event needs :event_type, :event, or :name")

    guild_id = attrs["guild_id"] || attrs["id"]

    meta0 = attrs["meta"] || %{}

    extra =
      attrs
      |> Map.drop([
        "event_type",
        "event",
        "name",
        "guild_id",
        "channel_id",
        "user_id",
        "level",
        "meta",
        "id"
      ])
      |> Enum.reject(fn {_, v} -> is_nil(v) end)
      |> Map.new()

    level = attrs["level"]

    meta =
      meta0
      |> Map.merge(extra)
      |> maybe_put_string("level", level)

    user_from_meta =
      case attrs["user_id"] do
        nil -> meta
        uid -> Map.put(meta, "user_id", to_string(uid))
      end

    %{
      "event_type" => to_string(event_type),
      "guild_id" => optional_string(guild_id),
      "channel_id" => optional_string(attrs["channel_id"]),
      "meta" => user_from_meta
    }
  end

  defp stringify_keys(map) when is_map(map) do
    Map.new(map, fn
      {k, v} when is_atom(k) -> {Atom.to_string(k), v}
      {k, v} -> {to_string(k), v}
    end)
  end

  defp optional_string(nil), do: nil
  defp optional_string(v), do: to_string(v)

  defp maybe_put_string(meta, _k, nil), do: meta
  defp maybe_put_string(meta, k, v), do: Map.put(meta, k, to_string(v))
end
