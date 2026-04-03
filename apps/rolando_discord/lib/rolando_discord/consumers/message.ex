defmodule RolandoDiscord.Consumers.Message do
  @moduledoc """
  Handles `MESSAGE_CREATE` (and related message events when added).

  Generates text using the configured generator (default: Markov chain backed by Redis or ETS).

  Heavy work should be delegated to `Rolando.TaskSupervisor` or a dedicated pool so
  this process stays responsive across all guilds/shards.
  """
  use Nostrum.Consumer

  alias Rolando.LM
  alias Nostrum.Cache.Me, as: CacheMe
  alias Nostrum.Struct.Message, as: Msg
  alias Rolando.Contexts.GuildConfig

  require Logger

  @default_reply_rate 20

  def handle_event({:MESSAGE_CREATE, %Msg{author: %{bot: true}}, _ws_state}) do
    :noop
  end

  def handle_event({:MESSAGE_CREATE, %Msg{guild_id: nil}, _ws_state}) do
    :noop
  end

  def handle_event({:MESSAGE_CREATE, msg, _ws_state}) do
    handle_message(msg)
  end

  def handle_event(_event) do
    :noop
  end

  defp handle_message(msg) do
    guild_id = to_string(msg.guild_id)
    bot_user_id = resolve_bot_user_id()
    mentioned = mentions_bot?(msg, bot_user_id)
    config = GuildConfig.get_or_default(guild_id)
    reply_rate = config.reply_rate || @default_reply_rate

    if mentioned do
      handle_mention_reply(msg, guild_id)
    else
      if rated_choice(reply_rate) do
        handle_random_message(msg, guild_id)
      end
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
    case Rolando.LM.generate(guild_id) do
      {:ok, generated_text} when generated_text != "" ->
        send_reply(msg, generated_text)

      {:ok, ""} ->
        Logger.debug("Empty generation for mention reply in guild #{guild_id}")

      {:error, reason} ->
        Logger.warning("Failed to generate text for mention: #{inspect(reason)}")
    end
  end

  defp handle_random_message(msg, guild_id) do
    # context = get_recent_message_context(guild_id)

    case LM.generate(guild_id) do
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

  # defp get_recent_message_context(guild_id) do
  #   case Messages.get_random_messages(guild_id, 3) do
  #     [] ->
  #       ""

  #     messages ->
  #       messages
  #       |> Enum.map(& &1.content)
  #       |> Enum.join(" ")
  #       |> String.slice(0, 200)
  #   end
  # end

  defp rated_choice(rate) do
    rate >= 100 || (rate > 0 && :rand.uniform(rate) == 1)
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
