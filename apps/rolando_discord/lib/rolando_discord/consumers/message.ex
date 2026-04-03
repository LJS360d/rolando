defmodule RolandoDiscord.Consumers.Message do
  @moduledoc """
  Handles `MESSAGE_CREATE` (and related message events when added).

  Generates text using the configured generator (default: Markov chain backed by Redis or ETS).

  Heavy work should be delegated to `Rolando.TaskSupervisor` or a dedicated pool so
  this process stays responsive across all guilds/shards.
  """
  use Nostrum.Consumer

  alias Nostrum.Cache.Me, as: CacheMe
  alias Nostrum.Struct.Message, as: Msg
  alias Rolando.Contexts.GuildConfig
  alias Rolando.Contexts.GuildWeights
  alias Rolando.Messages
  alias Rolando.Neural.{GRU, LanguageModel, Tokenizer, WordCodebook}

  require Logger

  @default_reply_rate 20

  def handle_event({:MESSAGE_CREATE, %Msg{author: %{bot: true}}, _ws_state}) do
    :noop
  end

  def handle_event({:MESSAGE_CREATE, %Msg{guild_id: nil}, _ws_state}) do
    :noop
  end

  def handle_event({:MESSAGE_CREATE, msg, _ws_state}) do
    spawn(fn ->
      handle_message_async(msg)
    end)
  end

  def handle_event(_event) do
    :noop
  end

  defp handle_message_async(msg) do
    guild_id = to_string(msg.guild_id)
    bot_user_id = resolve_bot_user_id()
    mentioned = mentions_bot?(msg, bot_user_id)
    config = GuildConfig.get_or_default(guild_id)
    reply_rate = config.reply_rate || @default_reply_rate

    if mentioned do
      handle_mention_reply(msg, guild_id)
    end

    if rated_choice(reply_rate) do
      handle_random_message(msg, guild_id)
    end
  end

  defp resolve_bot_user_id do
    case Application.get_env(:rolando_discord, :bot_user_id) do
      nil ->
        case CacheMe.get() do
          %{id: id} -> id
          _ -> nil
        end

      id ->
        id
    end
  end

  defp mentions_bot?(_msg, nil), do: false

  defp mentions_bot?(%Msg{mentions: mentions, content: content}, bot_user_id) do
    bid = to_string(bot_user_id)

    Enum.any?(mentions, fn user -> to_string(user.id) == bid end) or
      (is_binary(content) and
         (String.contains?(content, "<@#{bid}>") or String.contains?(content, "<@!#{bid}>")))
  end

  defp handle_mention_reply(msg, guild_id) do
    case generate_text(guild_id, msg.content) do
      {:ok, generated_text} when generated_text != "" ->
        send_reply(msg, generated_text)

      {:ok, ""} ->
        Logger.debug("Empty generation for mention reply in guild #{guild_id}")

      {:error, reason} ->
        Logger.warning("Failed to generate text for mention: #{inspect(reason)}")
    end
  end

  defp handle_random_message(msg, guild_id) do
    context = get_recent_message_context(guild_id)

    case generate_text(guild_id, context) do
      {:ok, generated_text} when generated_text != "" ->
        if rated_choice(10) do
          send_reply(msg, generated_text)
        else
          send_message(msg.channel_id, generated_text)
        end

      {:ok, ""} ->
        Logger.debug("Empty generation for random message in guild #{guild_id}")

      {:error, reason} ->
        Logger.warning("Failed to generate random text: #{inspect(reason)}")
    end
  end

  defp get_recent_message_context(guild_id) do
    case Messages.get_random_messages(guild_id, 3) do
      [] ->
        ""

      messages ->
        messages
        |> Enum.map(& &1.content)
        |> Enum.join(" ")
        |> String.slice(0, 200)
    end
  end

  def generate_text(guild_id, seed_text) do
    case Application.get_env(:rolando, :text_generator, :markov) do
      :markov ->
        case Rolando.Markov.generate(guild_id, seed_text) do
          {:ok, text} -> {:ok, clean_generated_text(text)}
          {:error, _} = e -> e
        end

      :gru ->
        gru_generate_text(guild_id, seed_text)
    end
  end

  defp gru_generate_text(guild_id, seed_text) do
    case GuildWeights.get(guild_id) do
      {:ok, weights_record} ->
        if weights_record.weight_data == nil or weights_record.weight_data == "" do
          {:error, :no_weights}
        else
          do_generate_text(
            weights_record.weight_data,
            seed_text,
            Map.get(weights_record, :codebook_blob)
          )
        end

      {:error, :not_found} ->
        {:error, :not_trained}
    end
  end

  defp do_generate_text(weights, seed_text, codebook_blob) do
    if LanguageModel.language_model?(weights) do
      cb = WordCodebook.decode(codebook_blob)

      opts =
        [max_tokens: 20, temperature: 0.85]
        |> then(fn o ->
          if cb do
            o |> Keyword.put(:max_tokens, 32) |> Keyword.put(:codebook, cb)
          else
            o
          end
        end)

      case LanguageModel.generate(weights, seed_text, opts) do
        {:ok, text} -> {:ok, clean_generated_text(text)}
        {:error, _} = e -> e
      end
    else
      legacy_do_generate_text(weights, seed_text)
    end
  end

  defp legacy_do_generate_text(weights, seed_text) do
    try do
      token_ids = Tokenizer.tokenize(seed_text)

      token_ids =
        if length(token_ids) == 0 do
          [:rand.uniform(32_000)]
        else
          Enum.take(token_ids, -10)
        end

      input_vectors =
        Enum.map(token_ids, fn token_id ->
          normalize_vector(token_id_to_vector(token_id))
        end)

      hidden_size = 512
      h_prev = GRU.zeros(hidden_size)

      output_states = GRU.gru_forward_sequence(input_vectors, h_prev, weights)
      final_state = List.last(output_states) || h_prev
      generated_token_ids = sample_tokens_from_hidden(final_state, 20)
      generated_text = Tokenizer.detokenize(generated_token_ids)

      {:ok, clean_generated_text(generated_text)}
    rescue
      e ->
        Logger.error("GRU generation error: #{inspect(e)}")
        {:error, :generation_failed}
    end
  end

  defp token_id_to_vector(token_id) do
    for i <- 0..15 do
      bit = (token_id + i) |> :erlang.bsl(1) |> :erlang.band(1)
      if bit == 1, do: 0.5, else: -0.5
    end
  end

  defp normalize_vector(vector) do
    sum = :math.sqrt(Enum.reduce(vector, 0, fn x, acc -> x * x + acc end))

    if sum > 0 do
      Enum.map(vector, fn x -> x / sum end)
    else
      vector
    end
  end

  defp sample_tokens_from_hidden(_hidden_state, count) do
    for _ <- 1..count do
      :rand.uniform(32_000)
    end
  end

  defp clean_generated_text(text) do
    text
    |> String.trim()
    |> String.replace(~r/<@\d+>/, "")
    |> String.replace(~r/<#\d+>/, "")
    |> String.replace(~r/<:[\w]+:\d+>/, "")
    |> String.replace(~r/\s+/, " ")
    |> String.trim()
  end

  defp rated_choice(rate) do
    rate == 1 || (rate > 1 && :rand.uniform(rate) == 1)
  end

  defp send_reply(%Msg{channel_id: channel_id, id: message_id}, text) do
    case Nostrum.Api.Message.create(channel_id,
           content: text,
           message_reference: %{message_id: message_id}
         ) do
      {:ok, _} ->
        :ok

      {:error, reason} ->
        Logger.warning("send_reply failed: #{inspect(reason)}")
    end
  end

  defp send_message(channel_id, text) do
    case Nostrum.Api.Message.create(channel_id, text) do
      {:ok, _} ->
        :ok

      {:error, reason} ->
        Logger.warning("send_message failed: #{inspect(reason)}")
    end
  end
end
