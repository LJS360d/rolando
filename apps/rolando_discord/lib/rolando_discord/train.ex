defmodule RolandoDiscord.Train do
  @moduledoc false

  require Logger

  alias Nostrum.Api.{Channel, Message}
  alias Rolando.Analytics
  alias Rolando.Chains
  alias Rolando.Messages
  alias RolandoDiscord.Permissions

  def run(opts) do
    do_run(opts)
  rescue
    e ->
      stack = __STACKTRACE__
      Logger.error(Exception.format(:error, e, stack))
      guild_id = Keyword.fetch!(opts, :guild_id)
      channel_id = Keyword.fetch!(opts, :channel_id)
      user_mention = Keyword.fetch!(opts, :user_mention)
      gid = to_string(guild_id)

      _ = Chains.update_trained_at(guild_id, nil)

      Analytics.track_event(%{
        event_type: "train_failed",
        guild_id: gid,
        channel_id: to_string(channel_id),
        meta: %{"exception" => Exception.message(e)}
      })

      _ =
        Message.create(
          channel_id,
          "#{user_mention} Training failed: `#{Exception.message(e)}`"
        )

      :error
  end

  defp do_run(opts) do
    guild_id = Keyword.fetch!(opts, :guild_id)
    channel_id = Keyword.fetch!(opts, :channel_id)
    user_mention = Keyword.fetch!(opts, :user_mention)

    gid = to_string(guild_id)
    max_conc = Application.get_env(:rolando, :train_channel_max_concurrency, 6)
    msg_limit = Application.get_env(:rolando, :train_message_limit_per_channel, 750_000)
    max_err = Application.get_env(:rolando, :train_max_fetch_errors_per_channel, 5)

    Analytics.track_event(%{
      event_type: "train_started",
      guild_id: gid,
      channel_id: to_string(channel_id),
      meta: %{}
    })

    channels = Permissions.list_trainable_text_channels(guild_id)

    started = System.monotonic_time(:millisecond)

    total =
      channels
      |> Task.async_stream(
        fn ch ->
          fetch_one_channel(guild_id, ch.id, msg_limit, max_err)
        end,
        max_concurrency: max_conc,
        timeout: :infinity,
        ordered: false
      )
      |> Enum.reduce(0, fn
        {:ok, n}, acc -> acc + n
        {:exit, reason}, acc ->
          Logger.error("Train channel task exit: #{inspect(reason)}")
          acc
      end)

    elapsed_ms = System.monotonic_time(:millisecond) - started
    elapsed_s = max(elapsed_ms / 1000, 0.001)
    msgs_per_s = Float.round(total / elapsed_s, 2)

    _ = Chains.snapshot_to_repo(guild_id)

    Analytics.track_event(%{
      event_type: "train_completed",
      guild_id: gid,
      channel_id: to_string(channel_id),
      meta: %{
        "message_count" => total,
        "elapsed_ms" => elapsed_ms,
        "messages_per_second" => msgs_per_s
      }
    })

    content =
      "#{user_mention} Finished Fetching messages.\nMessages fetched: `#{total}`\nTime elapsed: `#{format_duration_ms(elapsed_ms)}`\nMessages/Second: `#{msgs_per_s}`"

    _ = Message.create(channel_id, content)

    :ok
  end

  defp format_duration_ms(ms) do
    sec = div(ms, 1000)
    min = div(sec, 60)
    s = rem(sec, 60)
    "#{min}m #{s}s"
  end

  defp fetch_one_channel(guild_id, channel_id, msg_limit, max_err) do
    loop_fetch(guild_id, channel_id, nil, 0, 0, max_err, msg_limit)
  end

  defp loop_fetch(guild_id, channel_id, before_id, err_count, total, max_err, limit) do
    cond do
      err_count > max_err ->
        total

      total >= limit ->
        total

      true ->
        locator = if before_id, do: {:before, before_id}, else: {}

        case Channel.messages(channel_id, 100, locator) do
          {:ok, []} ->
            total

          {:ok, messages} ->
            oldest = List.last(messages)
            next_before = oldest.id
            lines = extract_lines(messages)

            :ok = Messages.insert_training_lines(guild_id, channel_id, lines)
            :ok = Chains.apply_training_batch(guild_id, lines)

            added = length(lines)
            new_total = total + added

            loop_fetch(guild_id, channel_id, next_before, 0, new_total, max_err, limit)

          {:error, _} = err ->
            Logger.warning("Channel #{channel_id} fetch error: #{inspect(err)}")

            if err_count + 1 > max_err do
              total
            else
              loop_fetch(guild_id, channel_id, before_id, err_count + 1, total, max_err, limit)
            end
        end
    end
  end

  defp extract_lines(messages) do
    Enum.flat_map(messages, fn m ->
      if line_acceptable?(m.content) do
        urls = Enum.map(m.attachments || [], & &1.url)
        [m.content | urls]
      else
        []
      end
    end)
  end

  defp line_acceptable?(content) do
    word_count = content |> String.split() |> length()
    word_count > 1 or contains_url?(content)
  end

  defp contains_url?(content) do
    String.contains?(content, "http://") or String.contains?(content, "https://")
  end
end
