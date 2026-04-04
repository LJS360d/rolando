defmodule RolandoDiscord.Consumers.SlashCommand do
  @moduledoc """
  Handles `INTERACTION_CREATE` for application (slash) commands only.

  Nostrum broadcasts every gateway event to every registered consumer; this module
  ignores non-slash interactions so work stays scoped per concern.
  """
  use Nostrum.Consumer

  alias Rolando.Messages
  alias Nostrum.Api.Interaction
  alias Nostrum.Cache.GuildCache
  alias Nostrum.Constants.InteractionCallbackType
  alias Nostrum.Struct.Interaction, as: I

  alias Rolando.Analytics
  alias Rolando.LM
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
    # track_analytics(i, "slash_command", meta: %{name: i.data.name})

    case handle_command(i.data.name, i) do
      {:error, err} ->
        track_analytics(i, "slash_command_fail", meta: %{error: inspect(err)}, level: :error)

      :noop ->
        :noop

      _ ->
        track_analytics(i, "slash_command_complete", meta: %{name: i.data.name})
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
      guild_schema = GuildCache.get!(guild_id) |> InteractionHelpers.to_guild_schema()
      {:ok, _guild} = Guilds.get_or_create(guild_schema)

      # Ensure config exists for this guild (creates with defaults if not exists)
      {:ok, config} = GuildConfig.get_or_create(to_string(guild_id))
      operator? = InteractionHelpers.operator_user?(i)
      cd = InteractionHelpers.cooldown_mins()

      cond do
        config.trained_at != nil &&
          InteractionHelpers.train_cooldown_active?(config.trained_at) && !operator? ->
          remaining =
            DateTime.diff(
              DateTime.add(config.trained_at, cd, :minute),
              DateTime.utc_now(),
              :second
            )

          rem_m = div(max(remaining, 0), 60)
          rem_s = rem(max(remaining, 0), 60)
          rem_str = :io_lib.format("~2..0w:~2..0w", [rem_m, rem_s]) |> IO.iodata_to_binary()

          Interaction.create_response(i, %{
            type: InteractionCallbackType.channel_message_with_source(),
            data: %{
              content:
                "Message fetching was last performed on **#{InteractionHelpers.fmt_dt(config.trained_at)}**.\nThe train command has a **#{cd} minutes cooldown** to prevent abuse. Please wait **#{rem_str}** before trying again.",
              flags: 64
            }
          })

        config.trained_at != nil &&
            (!InteractionHelpers.train_cooldown_active?(config.trained_at) || operator?) ->
          trained_fmt = InteractionHelpers.fmt_dt(config.trained_at)

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
      Interaction.create_response(i, %{
        type: InteractionCallbackType.channel_message_with_source(),
        data: %{content: "You are not authorized to use this command.", flags: 64}
      })
    end
  end

  # ---------------------------------------------------------------------------
  # GIF, Image, Video commands (structure only - no implementation)
  # ---------------------------------------------------------------------------

  defp handle_command("gif", %I{guild_id: nil}), do: :noop

  defp handle_command("gif", i) do
    # TODO: Implement GIF retrieval from media store
    Interaction.create_response(i, %{
      type: InteractionCallbackType.channel_message_with_source(),
      data: %{content: "GIF command not yet implemented.", flags: 64}
    })
  end

  defp handle_command("image", %I{guild_id: nil}), do: :noop

  defp handle_command("image", i) do
    # TODO: Implement image retrieval from media store
    Interaction.create_response(i, %{
      type: InteractionCallbackType.channel_message_with_source(),
      data: %{content: "Image command not yet implemented.", flags: 64}
    })
  end

  defp handle_command("video", %I{guild_id: nil}), do: :noop

  defp handle_command("video", i) do
    # TODO: Implement video retrieval from media store
    Interaction.create_response(i, %{
      type: InteractionCallbackType.channel_message_with_source(),
      data: %{content: "Video command not yet implemented.", flags: 64}
    })
  end

  # ---------------------------------------------------------------------------
  # Analytics command
  # ---------------------------------------------------------------------------

  defp handle_command("analytics", %I{guild_id: nil}), do: :noop

  defp handle_command("analytics", %I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      case Interaction.create_response(i, %{
             type: InteractionCallbackType.deferred_channel_message_with_source()
           }) do
        {:ok} ->
          # Fetch stats from Redis Markov Chain
          stats_result =
            LM.get_stats(to_string(guild_id))

          # Fetch guild config
          config = GuildConfig.get_or_default(to_string(guild_id))

          content =
            case stats_result do
              {:ok, %{unique_prefixes: prefixes, message_count: messages}} ->
                """
                **Bot Analytics for this Server**

                **Training Data:**
                • Unique prefixes: #{prefixes}
                • Messages processed: #{messages}

                **Configuration:**
                • Reply rate: #{config.reply_rate}%
                • Reaction rate: #{config.reaction_rate}%
                • Filter pings: #{if config.filter_pings, do: "Enabled", else: "Disabled"}
                • Last trained: #{if config.trained_at, do: InteractionHelpers.fmt_dt(config.trained_at), else: "Never"}
                """

              {:error, _} ->
                """
                **Bot Analytics for this Server**

                **Configuration:**
                • Reply rate: #{config.reply_rate}%
                • Reaction rate: #{config.reaction_rate}%
                • Filter pings: #{if config.filter_pings, do: "Enabled", else: "Disabled"}
                • Last trained: #{if config.trained_at, do: InteractionHelpers.fmt_dt(config.trained_at), else: "Never"}

                *Training data stats unavailable.*
                """
            end

          Interaction.edit_response(i, %{content: content})

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

  # ---------------------------------------------------------------------------
  # TogglePings command
  # ---------------------------------------------------------------------------

  defp handle_command("togglepings", %I{guild_id: nil}), do: :noop

  defp handle_command("togglepings", %I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      case GuildConfig.get_or_create(to_string(guild_id)) do
        {:ok, config} ->
          new_value = !config.filter_pings

          case GuildConfig.update_filter_pings(to_string(guild_id), new_value) do
            {:ok, _updated_config} ->
              Interaction.create_response(i, %{
                type: InteractionCallbackType.channel_message_with_source(),
                data: %{
                  content:
                    "Ping filtering has been **#{if new_value, do: "enabled", else: "disabled"}**.\n#{if new_value, do: "I will no longer mention users in my messages.", else: "I can now mention users in my messages."}",
                  flags: 64
                }
              })

            {:error, _} ->
              Interaction.create_response(i, %{
                type: InteractionCallbackType.channel_message_with_source(),
                data: %{content: "Failed to update ping filter setting.", flags: 64}
              })
          end

        {:error, _} ->
          Interaction.create_response(i, %{
            type: InteractionCallbackType.channel_message_with_source(),
            data: %{content: "Failed to retrieve guild configuration.", flags: 64}
          })
      end
    else
      Interaction.create_response(i, %{
        type: InteractionCallbackType.channel_message_with_source(),
        ephemeral: true,
        data: %{content: "You are not authorized to use this command.", flags: 64}
      })
    end
  end

  # ---------------------------------------------------------------------------
  # ReplyRate command
  # ---------------------------------------------------------------------------

  defp handle_command("replyrate", %I{guild_id: nil}), do: :noop

  defp handle_command("replyrate", %I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      rate_option = get_option_value(i, "rate")

      case GuildConfig.get_or_create(to_string(guild_id)) do
        {:ok, config} ->
          cond do
            is_nil(rate_option) ->
              # View current rate
              Interaction.create_response(i, %{
                type: InteractionCallbackType.channel_message_with_source(),
                data: %{
                  content:
                    "Current reply rate: **#{config.reply_rate}%**\nUse `/replyrate rate:<value>` to set a new rate (0-100).",
                  flags: 64
                }
              })

            true ->
              # Set new rate
              new_rate = rate_option / 100.0

              case GuildConfig.update_reply_rate(to_string(guild_id), new_rate) do
                {:ok, _updated_config} ->
                  Interaction.create_response(i, %{
                    type: InteractionCallbackType.channel_message_with_source(),
                    data: %{
                      content:
                        "Reply rate updated to **#{rate_option}%**.\nI will now reply to approximately #{rate_option}% of messages.",
                      flags: 64
                    }
                  })

                {:error, _} ->
                  Interaction.create_response(i, %{
                    type: InteractionCallbackType.channel_message_with_source(),
                    data: %{content: "Failed to update reply rate.", flags: 64}
                  })
              end
          end

        {:error, _} ->
          Interaction.create_response(i, %{
            type: InteractionCallbackType.channel_message_with_source(),
            data: %{content: "Failed to retrieve guild configuration.", flags: 64}
          })
      end
    else
      Interaction.create_response(i, %{
        type: InteractionCallbackType.channel_message_with_source(),
        ephemeral: true,
        data: %{content: "You are not authorized to use this command.", flags: 64}
      })
    end
  end

  # ---------------------------------------------------------------------------
  # ReactionRate command
  # ---------------------------------------------------------------------------

  defp handle_command("reactionrate", %I{guild_id: nil}), do: :noop

  defp handle_command("reactionrate", %I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      rate_option = get_option_value(i, "rate")

      case GuildConfig.get_or_create(to_string(guild_id)) do
        {:ok, config} ->
          cond do
            is_nil(rate_option) ->
              # View current rate
              Interaction.create_response(i, %{
                type: InteractionCallbackType.channel_message_with_source(),
                data: %{
                  content:
                    "Current reaction rate: **#{config.reaction_rate}%**\nUse `/reactionrate rate:<value>` to set a new rate (0-100).",
                  flags: 64
                }
              })

            true ->
              # Set new rate
              new_rate = rate_option / 100.0

              case GuildConfig.update_reaction_rate(to_string(guild_id), new_rate) do
                {:ok, _updated_config} ->
                  Interaction.create_response(i, %{
                    type: InteractionCallbackType.channel_message_with_source(),
                    data: %{
                      content:
                        "Reaction rate updated to **#{rate_option}%**.\nI will now react to approximately #{rate_option}% of messages.",
                      flags: 64
                    }
                  })

                {:error, _} ->
                  Interaction.create_response(i, %{
                    type: InteractionCallbackType.channel_message_with_source(),
                    data: %{content: "Failed to update reaction rate.", flags: 64}
                  })
              end
          end

        {:error, _} ->
          Interaction.create_response(i, %{
            type: InteractionCallbackType.channel_message_with_source(),
            data: %{content: "Failed to retrieve guild configuration.", flags: 64}
          })
      end
    else
      Interaction.create_response(i, %{
        type: InteractionCallbackType.channel_message_with_source(),
        ephemeral: true,
        data: %{content: "You are not authorized to use this command.", flags: 64}
      })
    end
  end

  # ---------------------------------------------------------------------------
  # Cohesion command (n_gram_size in Redis)
  # ---------------------------------------------------------------------------

  defp handle_command("cohesion", %I{guild_id: nil}), do: :noop

  defp handle_command("cohesion", %I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      value_option = get_option_value(i, "value")

      case LM.get_tier(to_string(guild_id)) do
        {:ok, current_size} ->
          cond do
            is_nil(value_option) ->
              # View current cohesion
              Interaction.create_response(i, %{
                type: InteractionCallbackType.channel_message_with_source(),
                data: %{
                  content:
                    "Current cohesion value: **#{current_size}**\nThis determines how coherent generated sentences are.\nUse `/cohesion value:<2-10>` to set a new value.\n\n**Higher values** = more coherent but repetitive\n**Lower values** = more creative but less coherent",
                  flags: 64
                }
              })

            true ->
              # Set new cohesion
              new_value = value_option
              gid = to_string(guild_id)
              messages = Messages.list_by_guild(gid) |> Enum.map(& &1.content)

              case LM.change_tier(gid, new_value, messages) do
                {:ok, _updated_size} ->
                  Interaction.create_response(i, %{
                    type: InteractionCallbackType.channel_message_with_source(),
                    data: %{
                      content:
                        "Cohesion value updated to **#{new_value}**.\nThis change takes effect immediately for new message generation.\n\n**Note:** Existing training data was trained with the previous cohesion value. To fully apply the new value, you may want to run `/train` again.",
                      flags: 64
                    }
                  })

                {:error, :invalid_n_gram_size} ->
                  Interaction.create_response(i, %{
                    type: InteractionCallbackType.channel_message_with_source(),
                    data: %{
                      content: "Invalid cohesion value. Must be between 2 and 10.",
                      flags: 64
                    }
                  })

                {:error, _} ->
                  Interaction.create_response(i, %{
                    type: InteractionCallbackType.channel_message_with_source(),
                    data: %{content: "Failed to update cohesion value.", flags: 64}
                  })
              end
          end

        {:error, _} ->
          Interaction.create_response(i, %{
            type: InteractionCallbackType.channel_message_with_source(),
            data: %{content: "Failed to retrieve cohesion value.", flags: 64}
          })
      end
    else
      Interaction.create_response(i, %{
        type: InteractionCallbackType.channel_message_with_source(),
        ephemeral: true,
        data: %{content: "You are not authorized to use this command.", flags: 64}
      })
    end
  end

  # ---------------------------------------------------------------------------
  # Opinion command
  # ---------------------------------------------------------------------------

  defp handle_command("opinion", %I{guild_id: nil}), do: :noop

  defp handle_command("opinion", %I{guild_id: guild_id} = i) do
    about_option = get_option_value(i, "about")

    if is_nil(about_option) do
      Interaction.create_response(i, %{
        type: InteractionCallbackType.channel_message_with_source(),
        data: %{content: "Please provide a seed for the opinion.", flags: 64}
      })
    else
      # Generate opinion based on seed using the Markov chain
      case LM.generate(to_string(guild_id), about_option) do
        {:ok, text} ->
          Interaction.create_response(i, %{
            type: InteractionCallbackType.channel_message_with_source(),
            data: %{
              content: "**My opinion on #{about_option}:**\n#{text}",
              flags: 64
            }
          })

        {:error, :no_data} ->
          Interaction.create_response(i, %{
            type: InteractionCallbackType.channel_message_with_source(),
            data: %{
              content:
                "I don't have enough training data to generate an opinion on \"#{about_option}\".\nTry using `/train` to fetch more messages first.",
              flags: 64
            }
          })

        {:error, _} ->
          Interaction.create_response(i, %{
            type: InteractionCallbackType.channel_message_with_source(),
            data: %{
              content: "Failed to generate an opinion. Try again later.",
              flags: 64
            }
          })
      end
    end
  end

  # ---------------------------------------------------------------------------
  # Wipe command
  # ---------------------------------------------------------------------------

  defp handle_command("wipe", %I{guild_id: nil}), do: :noop

  defp handle_command("wipe", %I{guild_id: guild_id} = i) do
    if Permissions.admin_or_owner?(i) do
      data_option = get_option_value(i, "data")

      if is_nil(data_option) do
        Interaction.create_response(i, %{
          type: InteractionCallbackType.channel_message_with_source(),
          data: %{content: "Please provide data to wipe.", flags: 64}
        })
      else
        # Delete the specified data from training data
        case LM.delete_message(
               to_string(guild_id),
               data_option
             ) do
          :ok ->
            Interaction.create_response(i, %{
              type: InteractionCallbackType.channel_message_with_source(),
              data: %{
                content:
                  "Successfully removed \"#{data_option}\" from the training data.\nNote: This removes n-grams associated with this text.",
                flags: 64
              }
            })

          {:error, reason} ->
            Interaction.create_response(i, %{
              type: InteractionCallbackType.channel_message_with_source(),
              data: %{
                content: "Failed to wipe data: #{inspect(reason)}",
                flags: 64
              }
            })
        end
      end
    else
      Interaction.create_response(i, %{
        type: InteractionCallbackType.channel_message_with_source(),
        ephemeral: true,
        data: %{content: "You are not authorized to use this command.", flags: 64}
      })
    end
  end

  # ---------------------------------------------------------------------------
  # VC commands (structure only - no implementation)
  # ---------------------------------------------------------------------------

  defp handle_command("vc", %I{guild_id: nil}), do: :noop

  defp handle_command("vc", i) do
    # VC subcommand handling - structure only
    subcommand = get_subcommand(i)

    case subcommand do
      "join" ->
        handle_vc_join(i)

      "leave" ->
        handle_vc_leave(i)

      "speak" ->
        handle_vc_speak(i)

      "joinrate" ->
        handle_vc_joinrate(i)

      "language" ->
        handle_vc_language(i)

      _ ->
        Interaction.create_response(i, %{
          type: InteractionCallbackType.channel_message_with_source(),
          data: %{content: "Invalid VC subcommand.", flags: 64}
        })
    end
  end

  # VC Subcommand handlers (structure only)

  defp handle_vc_join(i) do
    # TODO: Implement VC join logic
    Interaction.create_response(i, %{
      type: InteractionCallbackType.channel_message_with_source(),
      data: %{content: "VC join command not yet implemented.", flags: 64}
    })
  end

  defp handle_vc_leave(i) do
    # TODO: Implement VC leave logic
    Interaction.create_response(i, %{
      type: InteractionCallbackType.channel_message_with_source(),
      data: %{content: "VC leave command not yet implemented.", flags: 64}
    })
  end

  defp handle_vc_speak(i) do
    # TODO: Implement VC speak logic
    Interaction.create_response(i, %{
      type: InteractionCallbackType.channel_message_with_source(),
      data: %{content: "VC speak command not yet implemented.", flags: 64}
    })
  end

  defp handle_vc_joinrate(i) do
    # TODO: Implement VC joinrate view/set logic
    Interaction.create_response(i, %{
      type: InteractionCallbackType.channel_message_with_source(),
      data: %{content: "VC joinrate command not yet implemented.", flags: 64}
    })
  end

  defp handle_vc_language(i) do
    # TODO: Implement VC language view/set logic
    Interaction.create_response(i, %{
      type: InteractionCallbackType.channel_message_with_source(),
      data: %{content: "VC language command not yet implemented.", flags: 64}
    })
  end

  # ---------------------------------------------------------------------------
  # Helper functions
  # ---------------------------------------------------------------------------

  defp get_option_value(i, option_name) do
    i.data.options
    |> Enum.find(fn opt -> opt.name == option_name end)
    |> case do
      nil -> nil
      opt -> opt.value
    end
  end

  defp get_subcommand(i) do
    i.data.options
    |> Enum.find(fn opt -> opt.type == 1 end)
    |> case do
      nil -> nil
      subcmd -> subcmd.name
    end
  end
end
