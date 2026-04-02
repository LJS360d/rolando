defmodule RolandoDiscord.Train do
  @moduledoc false

  require Logger

  alias Nostrum.Api.{Channel, Message}
  alias Rolando.Analytics
  alias Rolando.Contexts.{GuildConfig, GuildWeights, MediaStore}
  alias Rolando.Messages
  alias Rolando.Neural.{GuildSupervisor, LanguageModel, WordCodebook}
  alias Rolando.Neural.Tokenizer
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

    # Initialize neural network weights and create checkpoint asynchronously
    spawn(fn ->
      create_guild_model_checkpoint(guild_id, gid, total)
    end)

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

            # Store messages for GRU training
            store_messages_for_training(guild_id, channel_id, messages)

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

  defp store_messages_for_training(guild_id, channel_id, messages) do
    gid = to_string(guild_id)
    cid = to_string(channel_id)

    # Filter and prepare messages for storage
    message_attrs =
      Enum.filter(messages, fn msg ->
        line_acceptable?(msg.content)
      end)
      |> Enum.map(fn msg ->
        # Create a simple hash for deduplication
        hash = :crypto.hash(:sha256, "#{gid}:#{msg.id}") |> Base.encode16(case: :lower)

        %{
          guild_id: gid,
          channel_id: cid,
          author_id: to_string(msg.author.id),
          content: msg.content,
          message_hash: hash
        }
      end)

    if length(message_attrs) > 0 do
      # Use bulk insert for efficiency
      case Messages.create_many(message_attrs) do
        {:error, reason} ->
          Logger.warning("Failed to store messages: #{inspect(reason)}")

        {n, _} ->
          Logger.debug("Stored #{n} messages for guild #{gid}")

      end
    end
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

  def create_guild_model_checkpoint(guild_id, gid, message_count) do
    try do
      Logger.info("Starting async language model training for guild #{gid}")

      ensure_guild_supervisor_running()
      start_or_get_guild_model(guild_id)

      vb = Application.get_env(:rolando, :lm_vocab_buckets, 256)
      e = Application.get_env(:rolando, :lm_embed_dim, 64)
      h = Application.get_env(:rolando, :lm_hidden_dim, 256)
      passes = Application.get_env(:rolando, :lm_data_passes, 1) |> max(1)
      batch_size = Application.get_env(:rolando, :lm_message_batch_size, 500)

      lr = Application.get_env(:rolando, :lm_train_lr, 0.05)
      epochs = Application.get_env(:rolando, :lm_train_epochs_per_chunk, 1)
      max_seq_len = Application.get_env(:rolando, :lm_max_seq_len, 128)
      clip = Application.get_env(:rolando, :lm_gradient_clip, 5.0)

      weights =
        LanguageModel.create(
          vocab_buckets: vb,
          embed_dim: e,
          hidden_dim: h
        )

      case GuildWeights.upsert(gid, %{
             weight_data: weights,
             version: 1,
             perplexity: 1.0,
             codebook_blob: nil
           }) do
        {:ok, _} ->
          Logger.debug("Initial language model weights persisted for guild #{gid} (pre-corpus pass)")

        {:error, reason} ->
          Logger.warning("Failed to save initial language model weights: #{inspect(reason)}")
      end

      train_opts = [
        lr: lr,
        epochs: epochs,
        max_seq_len: max_seq_len,
        gradient_clip: clip
      ]

      {weights, avg_loss} =
        Enum.reduce(1..passes, {weights, 0.0}, fn _pass, {w, _acc} ->
          train_language_model_pass(w, gid, batch_size, train_opts)
        end)

      ppl = :math.exp(min(avg_loss, 20.0))

      codebook_blob =
        case build_word_codebook(gid, vb) do
          list when is_list(list) -> WordCodebook.encode(list)
          _ -> nil
        end

      case GuildWeights.upsert(gid, %{
             weight_data: weights,
             version: 1,
             perplexity: ppl,
             codebook_blob: codebook_blob
           }) do
        {:ok, _} -> Logger.debug("Language model weights and word codebook saved")
        {:error, reason} -> Logger.warning("Failed to save weights: #{inspect(reason)}")
      end

      case GuildConfig.update_trained_at(gid, DateTime.utc_now()) do
        {:ok, _} -> Logger.debug("trained_at updated")
        {:error, reason} -> Logger.warning("Failed to update trained_at: #{inspect(reason)}")
      end

      Logger.info(
        "Language model trained for guild #{gid}: #{message_count} messages stored, avg_loss=#{Float.round(avg_loss, 4)}, perplexity≈#{Float.round(ppl, 2)}"
      )

      Analytics.track(%{
        name: "model_checkpoint_created",
        guild_id: gid,
        meta: %{
          "message_count" => message_count,
          "vocab_buckets" => vb,
          "embed_dim" => e,
          "hidden_dim" => h,
          "avg_loss" => avg_loss,
          "perplexity" => ppl
        }
      })
    rescue
      e ->
        stack = __STACKTRACE__

        Logger.error(
          "Failed to create guild model checkpoint: #{Exception.format(:error, e, stack)}"
        )
    end
  end

  defp build_word_codebook(gid, vb) do
    counts_by_bucket =
      Stream.unfold(0, fn offset ->
        case Messages.list_by_guild(gid, limit: 500, offset: offset) do
          [] ->
            nil

          batch ->
            {batch, offset + length(batch)}
        end
      end)
      |> Enum.reduce(%{}, fn batch, acc ->
        Enum.reduce(batch, acc, fn msg, acc_inner ->
          words =
            msg.content
            |> String.split()
            |> Enum.map(&String.trim/1)
            |> Enum.filter(&(&1 != ""))

          Enum.reduce(words, acc_inner, fn word, acc2 ->
            case Tokenizer.tokenize(word) do
              [tid | _] ->
                b = rem(tid, vb)
                inner = Map.get(acc2, b, %{})
                inner = Map.update(inner, word, 1, &(&1 + 1))
                Map.put(acc2, b, inner)

              [] ->
                acc2
            end
          end)
        end)
      end)

    for b <- 0..(vb - 1) do
      case Map.get(counts_by_bucket, b) do
        nil ->
          ""

        inner when map_size(inner) == 0 ->
          ""

        inner ->
          inner |> Enum.max_by(fn {_, c} -> c end) |> elem(0)
      end
    end
  end

  defp train_language_model_pass(weights, gid, batch_size, train_opts) do
    {w_final, losses} =
      Stream.unfold(0, fn offset ->
        case Messages.list_by_guild(gid, limit: batch_size, offset: offset) do
          [] ->
            nil

          batch ->
            {batch, offset + length(batch)}
        end
      end)
      |> Enum.reduce({weights, []}, fn batch, {w, losses} ->
        sequences =
          batch
          |> Enum.map(& &1.content)
          |> Enum.map(&Tokenizer.tokenize/1)
          |> Enum.filter(&(length(&1) >= 2))

        if sequences == [] do
          {w, losses}
        else
          case LanguageModel.train_chunk(w, sequences, train_opts) do
            {:ok, w2, loss} ->
              {w2, [loss | losses]}
          end
        end
      end)

    avg =
      case losses do
        [] -> 0.0
        ls -> Enum.sum(ls) / length(ls)
      end

    {w_final, avg}
  end

  defp ensure_guild_supervisor_running do
    # The supervisor should already be running, but ensure it's started
    # If it's already running, this is a no-op
    case Rolando.Neural.GuildSupervisor.start_link(name: Rolando.Neural.GuildSupervisor) do
      {:ok, _} -> :ok
      {:error, {:already_started, _}} -> :ok
      {:error, reason} -> Logger.warning("GuildSupervisor start error: #{inspect(reason)}")
    end
  end

  defp start_or_get_guild_model(guild_id) do
    # Try to start the guild model, or get it if already running
    case GuildSupervisor.start_guild(guild_id) do
      {:ok, _pid} ->
        Logger.debug("Started new GuildModel for guild #{guild_id}")
        :ok

      {:error, {:already_started, _pid}} ->
        Logger.debug("GuildModel already running for guild #{guild_id}")
        :ok

      {:error, reason} ->
        Logger.warning("Failed to start GuildModel: #{inspect(reason)}")
        :error
    end
  end

end
