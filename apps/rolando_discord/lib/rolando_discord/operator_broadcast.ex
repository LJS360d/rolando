defmodule RolandoDiscord.OperatorBroadcast do
  @moduledoc false
  use GenServer

  alias Nostrum.Api.{Message, User}
  alias Rolando.Analytics

  def start_link(opts \\ []) do
    GenServer.start_link(__MODULE__, opts, name: __MODULE__)
  end

  @impl true
  def init(_) do
    Phoenix.PubSub.subscribe(Rolando.PubSub, Rolando.Broadcast.topic())
    {:ok, %{}}
  end

  @impl true
  def handle_info({:operator_broadcast, envelope}, state) do
    _ = spawn(fn -> deliver(envelope) end)
    {:noreply, state}
  end

  def handle_info(_, state), do: {:noreply, state}

  defp deliver(envelope) do
    body = envelope[:body] |> to_string() |> String.trim() |> String.slice(0, 2000)
    correlation_id = envelope[:correlation_id]
    operator_user_id = envelope[:operator_user_id]
    guild_id = envelope[:guild_id]
    channel_ids = List.wrap(envelope[:channel_ids])
    user_ids = List.wrap(envelope[:user_ids])

    results =
      Enum.map(channel_ids, &send_channel(&1, body)) ++
        Enum.map(user_ids, &send_dm(&1, body))

    {ok_n, err_n, details} = summarize_results(results)

    _ =
      Analytics.track("broadcast_sent", %{
        correlation_id: correlation_id,
        operator_id: operator_user_id,
        guild_id: guild_id,
        meta: %{
          ok: ok_n,
          err: err_n,
          details: details
        }
      })
  end

  defp send_channel(raw_id, body) do
    raw_id = String.trim(to_string(raw_id))

    case parse_snowflake(raw_id) do
      {:ok, id} ->
        case Message.create(id, content: body) do
          {:ok, _} -> {:ok, {:channel, raw_id}}
          err -> {:error, {:channel, raw_id, err}}
        end

      e ->
        {:error, {:channel, raw_id, e}}
    end
  end

  defp send_dm(raw_uid, body) do
    raw_uid = String.trim(to_string(raw_uid))

    case parse_snowflake(raw_uid) do
      {:ok, uid} ->
        case User.create_dm(uid) do
          {:ok, dm} ->
            case Message.create(dm.id, content: body) do
              {:ok, _} -> {:ok, {:dm, raw_uid}}
              err -> {:error, {:dm, raw_uid, err}}
            end

          err ->
            {:error, {:dm, raw_uid, err}}
        end

      e ->
        {:error, {:dm, raw_uid, e}}
    end
  end

  defp parse_snowflake(s) do
    case Integer.parse(s) do
      {n, ""} -> {:ok, n}
      _ -> {:error, :invalid_snowflake}
    end
  end

  defp summarize_results(results) do
    {ok_n, err_n} =
      Enum.reduce(results, {0, 0}, fn
        {:ok, _}, {o, e} -> {o + 1, e}
        {:error, _}, {o, e} -> {o, e + 1}
      end)

    details =
      Enum.map(results, fn
        {:ok, tag} -> %{status: :ok, target: inspect(tag)}
        {:error, {kind, id, reason}} -> %{status: :error, kind: kind, id: id, reason: inspect(reason)}
      end)

    {ok_n, err_n, details}
  end
end
