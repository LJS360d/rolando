defmodule RolandoDiscord.Consumers.Message do
  @moduledoc """
  Handles `MESSAGE_CREATE` (and related message events when added).

  Generates text using the GRU neural network model when:
  - The bot is mentioned by a user
  - Random chance based on guild's reply_rate setting

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

  # Default reply rate (1 means 50% chance, higher means less frequent)
  @default_reply_rate 20

  def handle_event({:MESSAGE_CREATE, %Msg{author: %{bot: true}}, _ws_state}) do
    # Don't process bot messages
    :noop
  end

  def handle_event({:MESSAGE_CREATE, %Msg{guild_id: nil}, _ws_state}) do
    # Don't process DM messages
    :noop
  end

  def handle_event({:MESSAGE_CREATE, msg, _ws_state}) do
    # Spawn async to not block the consumer
    spawn(fn ->
      handle_message_async(msg)
    end)
  end

  def handle_event(_event) do
    :noop
  end

  # Async handler - runs in separate process
  defp handle_message_async(msg) do
    guild_id = to_string(msg.guild_id)
    bot_user_id = resolve_bot_user_id()

    # Check if bot was mentioned
    mentioned = mentions_bot?(msg, bot_user_id)

    # Get guild config for reply rate
    config = GuildConfig.get_or_default(guild_id)
    reply_rate = config.reply_rate || @default_reply_rate

    # Handle mention reply
    if mentioned do
      handle_mention_reply(msg, guild_id)
    end

    # Handle random message generation (non-reply)
    if rated_choice(reply_rate) do
      handle_random_message(msg, guild_id)
    end

    # Handle reactions (future: add emoji reactions)
    # reaction_rate = config.reaction_rate || @default_reaction_rate
    # if rated_choice(reaction_rate) do
    #   handle_reaction(msg, guild_id)
    # end
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

  # Handle when bot is mentioned - always reply
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

  # Handle random message (not a direct reply)
  defp handle_random_message(msg, guild_id) do
    # Use the last few messages as context if available
    context = get_recent_message_context(guild_id)

    case generate_text(guild_id, context) do
      {:ok, generated_text} when generated_text != "" ->
        # 10% chance to reply to the message, 90% chance to send standalone
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

  # Get some recent messages as context for generation
  defp get_recent_message_context(guild_id) do
    # Get a few random messages to use as seed context
    case Messages.get_random_messages(guild_id, 3) do
      [] ->
        ""

      messages ->
        messages
        |> Enum.map(& &1.content)
        |> Enum.join(" ")
        # Limit context length
        |> String.slice(0, 200)
    end
  end

  # Core text generation using GRU
  def generate_text(guild_id, seed_text) do
    # Load weights from database
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
          [:rand.uniform(32000)]
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

  # Convert token ID to a simple vector representation
  defp token_id_to_vector(token_id) do
    # Simple hash-based vector for demo
    # In production: use learned embeddings
    _vocab_size = 32000

    vector =
      for i <- 0..15 do
        bit = (token_id + i) |> :erlang.bsl(1) |> :erlang.band(1)
        if bit == 1, do: 0.5, else: -0.5
      end

    vector
  end

  # Normalize a vector to have unit length
  defp normalize_vector(vector) do
    sum = :math.sqrt(Enum.reduce(vector, 0, fn x, acc -> x * x + acc end))

    if sum > 0 do
      Enum.map(vector, fn x -> x / sum end)
    else
      vector
    end
  end

  # Sample tokens from hidden state (simplified)
  defp sample_tokens_from_hidden(_hidden_state, count) do
    # Simplified: just generate random token IDs
    # In production: use output projection + softmax + sampling
    for _ <- 1..count do
      :rand.uniform(32000)
    end
  end

  # Clean up generated text
  defp clean_generated_text(text) do
    text
    |> String.trim()
    # Remove Discord mentions that might have been generated
    |> String.replace(~r/<@\d+>/, "")
    |> String.replace(~r/<#\d+>/, "")
    |> String.replace(~r/<:[\w]+:\d+>/, "")
    # Normalize whitespace
    |> String.replace(~r/\s+/, " ")
    |> String.trim()
  end

  # Weighted random choice (same logic as Go code)
  defp rated_choice(rate) do
    rate == 1 || (rate > 1 && :rand.uniform(rate) == 1)
  end

  # Send a reply to a message (with reference)
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

  # Send a standalone message
  defp send_message(channel_id, text) do
    case Nostrum.Api.Message.create(channel_id, text) do
      {:ok, _} ->
        :ok

      {:error, reason} ->
        Logger.warning("send_message failed: #{inspect(reason)}")
    end
  end
end
