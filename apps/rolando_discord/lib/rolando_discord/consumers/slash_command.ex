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

  alias Rolando.Chains
  alias RolandoDiscord.InteractionHelpers
  alias RolandoDiscord.Permissions

  def handle_event({:INTERACTION_CREATE, %{type: 2} = interaction, _ws_state}) do
    # type 2 = InteractionType.application_command()
    case interaction.data do
      %{name: "train"} -> train_slash(interaction)
      %{name: "channels"} -> channels_slash(interaction)
      _ -> :noop
    end
  end

  def handle_event(_event) do
    :noop
  end

  defp channels_slash(%I{guild_id: nil}), do: :noop

  defp channels_slash(%I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      lines =
        Permissions.list_channels_for_display(guild_id)
        |> Enum.map(fn ch ->
          ok = Permissions.bot_can_fetch_text_channel?(guild_id, ch)
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

      _ =
        Interaction.create_response(i, %{
          type: InteractionCallbackType.channel_message_with_source(),
          data: %{content: content}
        })
    else
      _ =
        Interaction.create_response(i, %{
          type: InteractionCallbackType.channel_message_with_source(),
          data: %{content: "You are not authorized to use this command.", flags: 64}
        })
    end

    :ok
  end

  defp train_slash(%I{guild_id: nil}), do: :noop

  defp train_slash(%I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      guild = GuildCache.get!(guild_id)
      guild_name = guild.name
      _ = Chains.upsert_guild(guild_id, guild_name)

      _ =
        case Chains.get_chain_document(guild_id) do
          {:error, :not_found} -> Chains.create_chain(guild_id, guild_name)
          {:ok, _} -> :ok
        end

      {:ok, chain} = Chains.get_chain_document(guild_id)
      owner? = InteractionHelpers.owner_user?(i)
      cd = InteractionHelpers.cooldown_mins()

      cond do
        chain.trained_at != nil &&
            InteractionHelpers.train_cooldown_active?(chain.trained_at) && !owner? ->
          remaining =
            DateTime.diff(
              DateTime.add(chain.trained_at, cd, :minute),
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
                  "Message fetching was last performed on **#{InteractionHelpers.fmt_dt(chain.trained_at)}**.\nThe train command has a **#{cd} minutes cooldown** to prevent abuse. Please wait **#{rem_str}** before trying again.",
                flags: 64
              }
            })

        chain.trained_at != nil &&
            (!InteractionHelpers.train_cooldown_active?(chain.trained_at) || owner?) ->
          trained_fmt = InteractionHelpers.fmt_dt(chain.trained_at)

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
