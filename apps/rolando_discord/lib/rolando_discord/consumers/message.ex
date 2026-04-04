defmodule RolandoDiscord.Consumers.Message do
  use Nostrum.Consumer

  alias Rolando.Contexts.MediaStore
  alias Rolando.LM
  alias Nostrum.Cache.Me, as: CacheMe
  alias Nostrum.Api
  alias Nostrum.Struct.Message, as: Msg
  alias Rolando.Contexts.GuildConfig
  alias RolandoDiscord.Data.Emoji
  alias RolandoDiscord.InteractionHelpers

  @default_reply_rate 5
  @default_reaction_rate 0.01

  def handle_event({:MESSAGE_CREATE, %Msg{author: %{bot: true}}, _}), do: :noop
  def handle_event({:MESSAGE_CREATE, %Msg{guild_id: nil}, _}), do: :noop
  def handle_event({:MESSAGE_CREATE, msg, _}), do: handle_message(msg)
  def handle_event(_), do: :noop

  @spec handle_message(Msg.t()) :: :ok
  defp handle_message(msg) do
    guild_id = to_string(msg.guild_id)
    bot_user_id = resolve_bot_user_id()

    ingest(guild_id, msg)

    config = GuildConfig.get_or_default(guild_id)
    reply_rate = config.reply_rate || @default_reply_rate
    reaction_rate = config.reaction_rate || @default_reaction_rate

    if rated?(reaction_rate),
      do: Task.Supervisor.async_nolink(Rolando.TaskSupervisor, fn -> handle_reaction(msg) end)

    if mentions_bot?(msg, bot_user_id) do
      handle_mention_reply(msg, guild_id)
    else
      if rated?(reply_rate), do: handle_random_message(msg, guild_id)
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

  @spec mentions_bot?(Msg.t() | nil, String.t() | nil) :: boolean()
  defp mentions_bot?(_msg, nil), do: false

  defp mentions_bot?(%Msg{mentions: mentions, content: content}, bot_user_id) do
    bid = to_string(bot_user_id)

    Enum.any?(mentions, fn user -> to_string(user.id) == bid end) or
      (is_binary(content) and
         (String.contains?(content, "<@#{bid}>") or String.contains?(content, "<@!#{bid}>")))
  end

  @spec handle_mention_reply(Msg.t(), String.t()) :: :ok
  defp handle_mention_reply(msg, guild_id) do
    case generate_message(guild_id) do
      {:ok, text} when text != "" -> send_reply(msg, text)
      _ -> :ok
    end
  end

  @spec handle_random_message(Msg.t(), String.t()) :: :ok
  defp handle_random_message(msg, guild_id) do
    case generate_message(guild_id) do
      {:ok, text} when text != "" ->
        if rated?(10), do: send_reply_no_ping(msg, text), else: send_message(msg.channel_id, text)

      _ ->
        :ok
    end
  end

  @spec rated?(number()) :: boolean()
  defp rated?(rate) do
    cond do
      is_float(rate) -> :rand.uniform() * 100 <= rate
      rate >= 100 -> true
      true -> :rand.uniform(100) <= rate
    end
  end

  @spec ingest(String.t(), Msg.t()) :: :ok
  defp ingest(guild_id, msg) do
    {messages, media_to_store} = extract_messages_and_media(guild_id, msg)

    if media_to_store != [] do
      store_media(media_to_store)
    end

    if messages != [] do
      case LM.train_batch(guild_id, messages) do
        {:error, reason} ->
          Logger.warning("Failed to update chain state in guild #{guild_id}: #{inspect(reason)}")

        _ ->
          :ok
      end
    end
  end

  @spec extract_messages_and_media(String.t(), Msg.t()) :: {[String.t()], [map()]}
  defp extract_messages_and_media(guild_id, msg) do
    content_acc =
      if is_binary(msg.content) and byte_size(msg.content) > 3,
        do: [msg.content],
        else: []

    media_acc =
      Enum.reduce(msg.attachments, [], fn attachment, acc ->
        if attachment != nil and attachment.url != nil do
          media_type = InteractionHelpers.detect_media_type(attachment.url)

          [
            %{
              guild_id: guild_id,
              channel_id: to_string(msg.channel_id),
              url: attachment.url,
              media_type: media_type
            }
            | acc
          ]
        else
          acc
        end
      end)

    {content_acc, media_acc}
  end

  @spec classify_media_type(String.t()) :: :gif | :image | :video | :generic
  defp classify_media_type(url) do
    cond do
      String.match?(url, ~r/\.(gif)(\?|$)/i) -> :gif
      String.match?(url, ~r/\.(jpg|jpeg|png|webp)(\?|$)/i) -> :image
      String.match?(url, ~r/\.(mp4|webm|mov)(\?|$)/i) -> :video
      true -> :generic
    end
  end

  @spec store_media([map()]) :: :ok
  defp store_media(media_list) do
    Enum.each(media_list, fn media_attrs ->
      case MediaStore.create(media_attrs) do
        {:ok, _} -> :ok
        {:error, reason} -> Logger.warning("Failed to store media: #{inspect(reason)}")
      end
    end)
  end

  @spec handle_reaction(Msg.t()) :: :ok
  defp handle_reaction(msg) do
    emoji_pool = Emoji.unicode_emojis()

    emoji_pool =
      case get_guild_emojis(msg.guild_id) do
        nil ->
          emoji_pool

        guild_emojis ->
          guild_emoji_names =
            Enum.map(guild_emojis, fn emoji ->
              if emoji.animated,
                do: "<a:#{emoji.name}:#{emoji.id}>",
                else: "<:#{emoji.name}:#{emoji.id}>"
            end)

          emoji_pool ++ guild_emoji_names
      end

    rand_emoji = Enum.at(emoji_pool, :rand.uniform(length(emoji_pool)) - 1)

    case Api.Message.react(msg.channel_id, msg.id, rand_emoji) do
      {:error, reason} -> Logger.warning("
Failed to add reaction: #{inspect(reason)}")
      _ -> :ok
    end
  end

  @spec get_guild_emojis(integer() | String.t()) :: [term()] | nil
  defp get_guild_emojis(guild_id) do
    case Api.Guild.emojis(guild_id) do
      {:ok, emojis} ->
        emojis

      {:error, reason} ->
        Logger.warning("Failed to fetch guild emojis: #{inspect(reason)}")
        nil
    end
  end

  @spec generate_message(String.t()) :: {:ok, String.t()} | {:error, term()}
  defp generate_message(guild_id) do
    random = :rand.uniform(22) + 3

    cond do
      random <= 21 -> LM.generate(guild_id)
      random <= 23 -> try_get_media_or_talk(guild_id, :gif)
      random <= 24 -> try_get_media_or_talk(guild_id, :image)
      true -> try_get_media_or_talk(guild_id, :video)
    end
  end

  @spec try_get_media_or_talk(String.t(), atom()) :: {:ok, String.t()} | {:error, term()}
  defp try_get_media_or_talk(guild_id, media_type) do
    case MediaStore.get_random_by_guild(guild_id, media_type) do
      {:ok, media} -> {:ok, media.url}
      _ -> LM.generate(guild_id)
    end
  end

  @spec send_reply(Msg.t(), String.t()) :: :ok
  defp send_reply(%Msg{channel_id: channel_id, id: message_id}, text) do
    case Api.Message.create(channel_id,
           content: text,
           message_reference: %{message_id: message_id}
         ) do
      {:error, reason} -> Logger.warning("send_reply failed: #{inspect(reason)}")
      _ -> :ok
    end
  end

  @spec send_reply_no_ping(Msg.t(), String.t()) :: :ok
  defp send_reply_no_ping(%Msg{channel_id: channel_id, id: message_id}, text) do
    case Api.Message.create(channel_id,
           content: text,
           message_reference: %{message_id: message_id},
           allowed_mentions: %{parse: ["users", "roles", "everyone"], replied_user: false}
         ) do
      {:error, reason} -> Logger.warning("send_reply_no_ping failed: #{inspect(reason)}")
      _ -> :ok
    end
  end

  @spec send_message(integer() | String.t(), String.t()) :: :ok
  defp send_message(channel_id, text) do
    case Nostrum.Api.Message.create(channel_id, text) do
      {:error, reason} -> Logger.warning("send_message failed: #{inspect(reason)}")
      _ -> :ok
    end
  end
end
