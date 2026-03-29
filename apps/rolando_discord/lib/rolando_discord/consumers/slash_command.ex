defmodule RolandoDiscord.Consumers.SlashCommand do
  @moduledoc """
  Handles `INTERACTION_CREATE` for application (slash) commands only.

  Nostrum broadcasts every gateway event to every registered consumer; this module
  ignores non-slash interactions so work stays scoped per concern.
  """
  use Nostrum.Consumer

  alias Nostrum.Api.Interaction
  alias Nostrum.Cache.GuildCache
  alias Nostrum.Constants.InteractionCallbackType
  alias Nostrum.Struct.Interaction, as: I

  alias Rolando.Analytics
  alias Rolando.Contexts.{Guilds, GuildConfig}
  alias RolandoDiscord.InteractionHelpers
  alias RolandoDiscord.Permissions
  require Logger

  defp track_analytics(i, event_name, opts) do
    meta = Keyword.get(opts, :meta, %{})
    level = Keyword.get(opts, :level, :info)

    Analytics.track(%{
      event: event_name,
      guild_id: to_string(i.guild_id),
      channel_id: to_string(i.channel_id),
      user_id: to_string(i.user.id),
      meta: meta,
      level: level
    })
  end

  # type 2 = InteractionType.application_command()
  def handle_event({:INTERACTION_CREATE, %I{type: 2} = i, _ws_state}) do
    track_analytics(i, "slash_command", meta: %{name: i.data.name})

    case handle_command(i.data.name, i) do
      {:ok, _} ->
        track_analytics(i, "slash_command_complete", meta: %{name: i.data.name})

      {:error, err} ->
        track_analytics(i, "slash_command_fail", meta: %{error: inspect(err)}, level: :error)

      :noop ->
        :ok
    end
  end

  def handle_event(_event), do: :noop

  defp handle_command("channels", %I{guild_id: nil}), do: :noop

  defp handle_command("channels", %I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      # defer
      case Interaction.create_response(i, %{
             type: InteractionCallbackType.deferred_channel_message_with_source()
           }) do
        {:ok} ->
          lines =
            Permissions.list_channels_for_display(guild_id)
            |> Enum.map(fn ch ->
              ok = Permissions.can_fetch_text_channel?(guild_id, ch)
              em = if(ok, do: ":green_circle:", else: ":red_circle:")
              "#{em} <##{ch.id}>"
            end)

          header =
            "Channels the bot has access to are marked with: :green_circle:\nWhile channels with no access are marked with: :red_circle:\nMake a channel accessible by giving **ALL** these permissions:\n`View Channel` `Send Messages` `Read Message History`\n\n"

          content =
            case lines do
              [] -> header <> "No available channels to display."
              _ -> header <> Enum.join(lines, "\n")
            end

          Interaction.edit_response(i, %{
            content: content
          })

        err ->
          err
      end
    else
      Interaction.create_response(i, %{
        type: InteractionCallbackType.channel_message_with_source(),
        ephemeral: true,
        data: %{content: "You are not authorized to use this command.", flags: 64}
      })
    end
  end

  defp handle_command("train", %I{guild_id: nil}), do: :noop

  defp handle_command("train", %I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      # Ensure guild exists in our system
      guild = GuildCache.get!(guild_id) |> Map.put(:id, to_string(guild_id))
      {:ok, _guild} = Guilds.get_or_create(guild)

      # Ensure config exists for this guild (creates with defaults if not exists)
      {:ok, config} = GuildConfig.get_or_create(to_string(guild_id))
      owner? = InteractionHelpers.owner_user?(i)
      cd = InteractionHelpers.cooldown_mins()

      cond do
        config.trained_at != nil &&
          InteractionHelpers.train_cooldown_active?(config.trained_at) && !owner? ->
          remaining =
            DateTime.diff(
              DateTime.add(config.trained_at, cd, :minute),
              DateTime.utc_now(),
              :second
            )

          rem_m = div(max(remaining, 0), 60)
          rem_s = rem(max(remaining, 0), 60)
          rem_str = :io_lib.format("~2..0w:~2..0w", [rem_m, rem_s]) |> IO.iodata_to_binary()

          _ =
            Interaction.create_response(i, %{
              type: InteractionCallbackType.channel_message_with_source(),
              data: %{
                content:
                  "Message fetching was last performed on **#{InteractionHelpers.fmt_dt(config.trained_at)}**.\nThe train command has a **#{cd} minutes cooldown** to prevent abuse. Please wait **#{rem_str}** before trying again.",
                flags: 64
              }
            })

        config.trained_at != nil &&
            (!InteractionHelpers.train_cooldown_active?(config.trained_at) || owner?) ->
          trained_fmt = InteractionHelpers.fmt_dt(config.trained_at)

          _ =
            Interaction.create_response(i, %{
              type: InteractionCallbackType.channel_message_with_source(),
              data: %{
                content:
                  "The train command has already been performed at **`#{trained_fmt}`**.\nBy performing it again, you will **delete ALL** the fetched data from this server,\nand it will be fetched again in all accessible text channels,\nyou can use the `/channels` command to see which are accessible.\nIf you wish to exclude specific channels, revoke my typing permissions in those channels.\n\nThis command can only be performed every **#{cd} minutes**. Are you sure?",
                flags: 64,
                components: [
                  %{
                    type: 1,
                    components: [
                      %{
                        type: 2,
                        label: "Confirm Re-train",
                        style: 4,
                        custom_id: "confirm-train-again"
                      }
                    ]
                  }
                ]
              }
            })

        true ->
          _ =
            Interaction.create_response(i, %{
              type: InteractionCallbackType.channel_message_with_source(),
              data: %{
                content:
                  "Are you sure you want to use **ALL SERVER MESSAGES** as training data for me?\nThis will fetch data in all accessible text channels,\nyou can use the `/channels` command to see which are accessible.\nIf you wish to exclude specific channels, revoke my typing permissions in those channels.\n\nThis command can only be performed every **#{cd} minutes**. Are you sure?",
                flags: 64,
                components: [
                  %{
                    type: 1,
                    components: [
                      %{type: 2, label: "Confirm", style: 1, custom_id: "confirm-train"}
                    ]
                  }
                ]
              }
            })
      end
    else
      _ =
        Interaction.create_response(i, %{
          type: InteractionCallbackType.channel_message_with_source(),
          data: %{content: "You are not authorized to use this command.", flags: 64}
        })
    end

    :ok
  end
end
