defmodule RolandoDiscord.Train do
  @moduledoc false

  require Logger

  alias Rolando.LM
  alias Nostrum.Api.{Channel, Message}
  alias Rolando.Analytics
  alias Rolando.Contexts.{GuildConfig, MediaStore}
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

      # Reset trained_at timestamp on failure
      _ = GuildConfig.update_trained_at(to_string(guild_id), nil)

      Analytics.track(%{
        name: "train_failed",
        guild_id: gid,
        channel_id: to_string(channel_id),
        meta: %{"exception" => Exception.message(e)}
      })

      case Message.create(
             channel_id,
             "#{user_mention} Training failed: `#{Exception.message(e)}`"
           ) do
        {:ok, _} -> :ok
        {:error, r} -> Logger.warning("Failed to send failure message: #{inspect(r)}")
      end

      :error
  end

  defp do_run(opts) do
    guild_id = Keyword.fetch!(opts, :guild_id)
    channel_id = Keyword.fetch!(opts, :channel_id)
    user_mention = Keyword.fetch!(opts, :user_mention)

    gid = to_string(guild_id)
    max_conc = Application.get_env(:rolando, :train_channel_max_concurrency, 2)
    msg_limit = Application.get_env(:rolando, :train_message_limit_per_channel, 750_000)
    max_err = Application.get_env(:rolando, :train_max_fetch_errors_per_channel, 10)

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

    case Message.create(channel_id, content) do
      {:ok, _} -> :ok
      {:error, r} -> Logger.warning("Failed to send completion message: #{inspect(r)}")
    end
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
            store_media(guild_id, channel_id, media_urls)

            store_messages(guild_id, channel_id, messages)

            added = length(text_content)
            new_total = total + added

            # Logger.debug(
            #   "Channel #{channel_id} fetched batch of #{batch_count} messages, #{added} text, #{length(media_urls)} media, running total: #{new_total}"
            # )

            loop_fetch(guild_id, channel_id, next_before, 0, new_total, max_err, limit)

          {:error, %{status_code: 429, retry_after: retry_after}} ->
            # Rate limited - wait and retry
            wait_time = ceil(retry_after * 1000)

            Logger.warning(
              "Channel #{channel_id} rate limited, waiting #{wait_time}ms before retry"
            )

            Process.sleep(wait_time)
            loop_fetch(guild_id, channel_id, before_id, err_count, total, max_err, limit)

          {:error, %{status_code: _}} = error ->
            Logger.warning(
              "Channel #{channel_id} fetch error (attempt #{err_count + 1}/#{max_err}): #{inspect(error)}"
            )

            if err_count + 1 > max_err do
              Logger.error(
                "Channel #{channel_id} failed after #{max_err} errors, stopping at #{total} messages"
              )

              total
            else
              # Add small delay between retries to avoid hammering
              Process.sleep(500)
              loop_fetch(guild_id, channel_id, before_id, err_count + 1, total, max_err, limit)
            end

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
              Process.sleep(500)
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

  defp store_media(guild_id, channel_id, media_urls) do
    gid = to_string(guild_id)
    cid = to_string(channel_id)

    url_count = length(media_urls)

    if url_count > 0 do
      Logger.debug("Storing #{url_count} media URLs for channel #{cid} in guild #{gid}")
    end

    Enum.each(media_urls, fn url ->
      MediaStore.create(%{
        guild_id: gid,
        channel_id: cid,
        url: url,
        media_type: detect_media_type(url)
      })
    end)
  end

  defp store_messages(guild_id, channel_id, messages) do
    gid = to_string(guild_id)
    cid = to_string(channel_id)

    # Filter and prepare messages for storage
    message_attrs =
      Enum.filter(messages, fn msg ->
        line_acceptable?(msg.content)
      end)
      |> Enum.map(fn msg ->
        %{
          guild_id: gid,
          channel_id: cid,
          author_id: to_string(msg.author.id),
          content: msg.content
        }
      end)

    if length(message_attrs) > 0 do
      LM.train_batch(gid, messages |> Enum.map(& &1.content))

      case Messages.create_many(message_attrs) do
        {:error, reason} ->
          Logger.warning("Failed to store messages: #{inspect(reason)}")

        {n, _} ->
          Logger.debug("Stored #{n} messages for guild #{gid}")
      end
    end
  end

  @extensions %{
    gif: ~w(.gif),
    image: ~w(.jpg .jpeg .png .webp .avif),
    video: ~w(.mp4 .mov .avi),
    audio: ~w(.mp3 .wav .ogg)
  }

  @domains %{
    gif: ~w(tenor.com giphy.com),
    image:
      ~w(imgur.com i.imgur.com pinterest.com pin.it pixiv.net pximg.net flickr.com staticflickr.com twitter.com x.com fixvx.com),
    video: ~w(youtube.com youtu.be)
  }

  defp detect_media_type(url) do
    uri = URI.parse(String.downcase(url))
    path = uri.path || ""
    host = uri.host || ""

    type_by_extension =
      Enum.find_value(@extensions, fn {type, exts} ->
        Enum.any?(exts, &String.ends_with?(path, &1)) && type
      end)

    type_by_domain =
      Enum.find_value(@domains, fn {type, domains} ->
        Enum.any?(domains, &(host == &1 or String.ends_with?(host, "." <> &1))) && type
      end)

    type_by_extension || type_by_domain || :other
  end

  defp line_acceptable?(content) do
    word_count = content |> String.split() |> length()
    word_count > 1 or contains_url?(content)
  end

  defp contains_url?(content) do
    String.contains?(content, "http://") or String.contains?(content, "https://")
  end
end
