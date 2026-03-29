defmodule RolandoDiscord.Consumers.Component do
  @moduledoc """
  Handles `INTERACTION_CREATE` for message components (buttons, selects, etc.).

  See `RolandoDiscord.Consumers.SlashCommand` for notes on Nostrum consumer groups.
  """
  use Nostrum.Consumer

  require Logger

  alias Nostrum.Api.{Interaction, Message}
  alias Nostrum.Cache.GuildCache
  alias Nostrum.Constants.InteractionCallbackType
  alias Nostrum.Struct.Interaction, as: I

  alias Rolando.Contexts.{Guilds, GuildConfig, GuildWeights}
  alias RolandoDiscord.InteractionHelpers
  alias RolandoDiscord.Permissions
  alias RolandoDiscord.Train

  # type 3 = InteractionType.message_component()
  def handle_event({:INTERACTION_CREATE, %{type: 3} = interaction, _ws_state}) do
    case interaction.data do
      %{custom_id: "confirm-train"} -> confirm_train_first(interaction)
      %{custom_id: "confirm-train-again"} -> confirm_train_retrain(interaction)
      _ -> :noop
    end
  end

  def handle_event(_event) do
    :noop
  end

  defp confirm_train_first(%I{guild_id: nil}), do: :noop

  defp confirm_train_first(%I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      _ =
        Interaction.create_response(i, %{
          type: InteractionCallbackType.deferred_channel_message_with_source()
        })

      # Ensure guild exists in our system
      guild = GuildCache.get!(guild_id) |> InteractionHelpers.to_guild_schema()
      {:ok, _guild} = Guilds.get_or_create(guild)

      # Ensure config exists for this guild
      case GuildConfig.get(to_string(guild_id)) do
        {:ok, %{trained_at: t}} when not is_nil(t) ->
          _ = Message.create(i.channel_id, "Training already completed for this server.")
          _ = Interaction.delete_response(i)

        _ ->
          start_training_job(i, guild_id, :first)
      end
    else
      _ =
        Interaction.create_response(i, %{
          type: InteractionCallbackType.channel_message_with_source(),
          data: %{content: "You are not authorized.", flags: 64}
        })
    end

    :ok
  end

  defp confirm_train_retrain(%I{guild_id: nil}), do: :noop

  defp confirm_train_retrain(%I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      _ =
        Interaction.create_response(i, %{
          type: InteractionCallbackType.deferred_channel_message_with_source()
        })

      _ =
        Interaction.edit_response(i, %{
          data: %{content: "Deleting fetched data from this server.\nThis might take a while.."}
        })

      # Ensure guild exists
      guild = GuildCache.get!(guild_id) |> InteractionHelpers.to_guild_schema()
      {:ok, _guild} = Guilds.get_or_create(guild)

      # Delete existing weights (reset for retraining)
      case GuildWeights.delete(to_string(guild_id)) do
        {:ok, _} ->
          start_training_job(i, guild_id, :retrain)

        {:error, reason} ->
          Logger.error("delete_weights failed: #{inspect(reason)}")

          _ =
            Interaction.edit_response(i, %{
              data: %{
                content: "Failed to delete chain data for this server. Please try again later."
              }
            })
      end
    else
      _ =
        Interaction.create_response(i, %{
          type: InteractionCallbackType.channel_message_with_source(),
          data: %{content: "You are not authorized.", flags: 64}
        })
    end

    :ok
  end

  defp start_training_job(%I{} = i, guild_id, mode) do
    mention = InteractionHelpers.user_mention(i)
    now = DateTime.utc_now(:second)

    verb =
      case mode do
        :retrain -> "Refetching"
        _ -> "Fetching"
      end

    # Update trained_at timestamp in config
    case GuildConfig.update_trained_at(to_string(guild_id), now) do
      {:ok, _} ->
        _ =
          Message.create(
            i.channel_id,
            "#{mention} Started #{verb} messages.\nI will send a message when I'm done.\nEstimated Time: `1 Minute per every 5000 Messages in the Server`\nThis might take a while.."
          )

        _ = Interaction.delete_response(i)

        opts = [
          guild_id: guild_id,
          channel_id: i.channel_id,
          user_mention: mention
        ]

        {:ok, _pid} =
          Task.Supervisor.start_child(Rolando.TaskSupervisor, fn -> Train.run(opts) end)

        :ok

      {:error, reason} ->
        Logger.error("update_trained_at: #{inspect(reason)}")
        _ = Interaction.edit_response(i, %{data: %{content: "Failed to start training."}})
    end
  end
end
