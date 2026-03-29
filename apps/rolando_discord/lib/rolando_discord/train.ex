defmodule RolandoDiscord.Train do
  @moduledoc false

  require Logger

  alias Nostrum.Api.{Channel, Message}
  alias Rolando.Analytics
  alias Rolando.Contexts.{GuildConfig, MediaStore}
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

      # Reset trained_at timestamp on failure
      _ = GuildConfig.update_trained_at(to_string(guild_id), nil)

      Analytics.track(%{
        name: "train_failed",
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

    Logger.info(
      "Training started for guild #{gid}, channel #{channel_id}, max_concurrency=#{max_conc}, msg_limit=#{msg_limit}"
    )

    Analytics.track(%{
      name: "train_started",
      guild_id: gid,
      channel_id: to_string(channel_id),
      meta: %{}
    })

    channels = Permissions.list_trainable_text_channels(guild_id)
    channel_count = length(channels)
    Logger.info("Found #{channel_count} trainable channels for guild #{gid}")

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
        {:ok, n}, acc ->
          acc + n

        {:exit, reason}, acc ->
          Logger.error("Train channel task exit: #{inspect(reason)}")
          acc
      end)

    elapsed_ms = System.monotonic_time(:millisecond) - started
    elapsed_s = max(elapsed_ms / 1000, 0.001)
    msgs_per_s = Float.round(total / elapsed_s, 2)

    Logger.info(
      "Training completed for guild #{gid}: #{total} messages in #{format_duration_ms(elapsed_ms)} (#{msgs_per_s} msg/s)"
    )

    # TODO: Initialize neural network weights after message collection
    # This will be implemented as part of the neural training pipeline

    Analytics.track(%{
      name: "train_completed",
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
    gid = to_string(guild_id)
    cid = to_string(channel_id)
    Logger.info("Starting fetch for channel #{cid} in guild #{gid}, limit=#{msg_limit}")
    loop_fetch(guild_id, channel_id, nil, 0, 0, max_err, msg_limit)
  end

  defp loop_fetch(guild_id, channel_id, before_id, err_count, total, max_err, limit) do
    cond do
      err_count > max_err ->
        Logger.warning(
          "Channel #{channel_id} exceeded max errors (#{max_err}), stopping at #{total} messages"
        )

        total

      total >= limit ->
        Logger.debug("Channel #{channel_id} reached message limit (#{limit}), stopping")
        total

      true ->
        locator = if before_id, do: {:before, before_id}, else: {}

        case Channel.messages(channel_id, 200, locator) do
          {:ok, []} ->
            Logger.debug("Channel #{channel_id} no more messages (total: #{total})")
            total

          {:ok, messages} ->
            # batch_count = length(messages)
            oldest = List.last(messages)
            next_before = oldest.id
            {text_content, media_urls} = extract_content_and_media(messages)

            # Store extracted media in the media store for neural network training
            store_media_for_training(guild_id, channel_id, media_urls)

            added = length(text_content)
            new_total = total + added

            # Logger.debug(
            #   "Channel #{channel_id} fetched batch of #{batch_count} messages, #{added} text, #{length(media_urls)} media, running total: #{new_total}"
            # )

            loop_fetch(guild_id, channel_id, next_before, 0, new_total, max_err, limit)

          {:error, _} = err ->
            Logger.warning(
              "Channel #{channel_id} fetch error (attempt #{err_count + 1}/#{max_err}): #{inspect(err)}"
            )

            if err_count + 1 > max_err do
              Logger.error(
                "Channel #{channel_id} failed after #{max_err} errors, stopping at #{total} messages"
              )

              total
            else
              loop_fetch(guild_id, channel_id, before_id, err_count + 1, total, max_err, limit)
            end
        end
    end
  end

  defp extract_content_and_media(messages) do
    Enum.reduce(messages, {[], []}, fn message, {text_acc, media_acc} ->
      if line_acceptable?(message.content) do
        urls = Enum.map(message.attachments || [], & &1.url)
        {[message.content | text_acc], urls ++ media_acc}
      else
        {text_acc, media_acc}
      end
    end)
  end

  defp store_media_for_training(guild_id, channel_id, media_urls) do
    gid = to_string(guild_id)
    cid = to_string(channel_id)

    url_count = length(media_urls)

    if url_count > 0 do
      Logger.debug("Storing #{url_count} media URLs for channel #{cid} in guild #{gid}")
    end

    Enum.each(media_urls, fn url ->
      # Store media metadata for neural network processing
      MediaStore.create(%{
        guild_id: gid,
        channel_id: cid,
        url: url,
        media_type: detect_media_type(url),
        context_hash: ""
      })
    end)
  end

  defp detect_media_type(url) do
    cond do
      String.contains?(url, ".jpg") or String.contains?(url, ".jpeg") or
        String.contains?(url, ".png") or String.contains?(url, ".gif") ->
        :image

      String.contains?(url, ".mp4") or String.contains?(url, ".mov") or
          String.contains?(url, ".avi") ->
        :video

      String.contains?(url, ".mp3") or String.contains?(url, ".wav") or
          String.contains?(url, ".ogg") ->
        :gif

      true ->
        :other
    end
  end

  defp line_acceptable?(content) do
    word_count = content |> String.split() |> length()
    word_count > 1 or contains_url?(content)
  end

  defp contains_url?(content) do
    String.contains?(content, "http://") or String.contains?(content, "https://")
  end
end
