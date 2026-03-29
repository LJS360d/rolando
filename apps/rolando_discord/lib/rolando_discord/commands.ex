defmodule RolandoDiscord.Commands do
  alias Nostrum.Api.ApplicationCommand
  alias Nostrum.Constants.ApplicationCommandOptionType, as: Opt
  require Logger

  def commands do
    [
      %{
        name: "train",
        description: "Fetches all available messages in the server to be used as training data",
        dm_permission: false
      },
      %{name: "gif", description: "Returns a gif from the ones it knows"},
      %{name: "image", description: "Returns an image from the ones it knows"},
      %{name: "video", description: "Returns a video from the ones it knows"},
      %{
        name: "analytics",
        description: "Returns analytics about the bot in this server"
      },
      %{
        name: "togglepings",
        description: "Toggles wether pings are enabled or not"
      },
      %{
        name: "replyrate",
        description: "View or set the reply rate",
        options: [
          %{
            type: Opt.integer(),
            name: "rate",
            description: "the rate to set (leave empty to view)",
            required: false
          }
        ]
      },
      %{
        name: "reactionrate",
        description: "View or set the reaction rate",
        options: [
          %{
            type: Opt.integer(),
            name: "rate",
            description: "the rate to set (leave empty to view)",
            required: false
          }
        ]
      },
      %{
        name: "cohesion",
        description:
          "View or set the cohesion value; A higher value makes sentences more coherent",
        options: [
          %{
            type: Opt.integer(),
            name: "value",
            description: "the value to set, must be at least 2 (leave empty to view)",
            required: false,
            min_value: 2,
            max_value: 10
          }
        ]
      },
      %{
        name: "opinion",
        description: "Generates a random opinion based on the provided seed",
        options: [
          %{
            type: Opt.string(),
            name: "about",
            description: "The seed of the message",
            required: true
          }
        ]
      },
      %{
        name: "wipe",
        description: "Deletes the given argument `data` from the training data",
        options: [
          %{
            type: Opt.string(),
            name: "data",
            description: "The data to be deleted",
            required: true
          }
        ]
      },
      %{
        name: "channels",
        description: "View which channels can be accessed for training data"
      },
      %{
        name: "vc",
        description: "Manage VC features",
        dm_permission: false,
        options: [
          # Subcommand: Join
          %{
            type: Opt.sub_command(),
            name: "join",
            description: "Joins the voice channel you are currently in"
          },
          # Subcommand: Leave
          %{
            type: Opt.sub_command(),
            name: "leave",
            description: "Leaves the voice channel you are currently in"
          },
          # Subcommand: Speak
          %{
            type: Opt.sub_command(),
            name: "speak",
            description: "Speaks words of wisdom in a VC, and then leaves",
            options: [
              %{
                type: Opt.channel(),
                name: "channel",
                description:
                  "Optional: Specific channel to speak in (defaults to the one you are connected to)",
                required: false
              }
            ]
          },
          # Subcommand: Joinrate View/Set
          %{
            type: Opt.sub_command(),
            name: "joinrate",
            description: "View or set the VC random join rate",
            options: [
              %{
                type: Opt.number(),
                name: "rate",
                description: "The rate in % (0-100)",
                required: false,
                min_value: 0.0,
                max_value: 100.0
              },
              %{
                type: Opt.channel(),
                name: "channel",
                description: "Optional: Specific channel to view / update (defaults to current)",
                required: false
              }
            ]
          },
          # Subcommand: Language View/Set
          %{
            type: Opt.sub_command(),
            name: "language",
            description: "View or set the VC language",
            options: [
              %{
                type: Opt.integer(),
                name: "language",
                description: "the language to set (leave empty to view)",
                required: false,
                choices: [
                  %{name: "English", value: 0},
                  %{name: "Italian", value: 1},
                  %{name: "German", value: 2},
                  %{name: "Spanish", value: 3}
                ]
              },
              %{
                type: Opt.channel(),
                name: "channel",
                description: "Optional: Specific channel to update (defaults to current)",
                required: false
              }
            ]
          }
        ]
      }
    ]
  end

  defp needs_resync?(remote, local) do
    cond do
      length(remote) != length(local) ->
        true

      true ->
        local_names = local |> Enum.map(& &1.name) |> Enum.sort()
        remote_names = remote |> Enum.map(& &1.name) |> Enum.sort()
        local_names != remote_names

        # TODO better diffing
    end
  end

  def sync do
    case ApplicationCommand.global_commands() do
      {:ok, remote_commands} ->
        if needs_resync?(remote_commands, commands()) do
          Logger.info("Syncing slash command interactions...")

          case sync_force() do
            {:ok, _} ->
              Logger.info("Commands synced successfully.")

            {:error, reason} ->
              Logger.error("Failed to sync commands: #{inspect(reason)}")
          end
        else
          Logger.info("Slash commands are up to date. Skipping sync.")
        end

      {:error, _} ->
        Logger.error("Could not verify commands")
    end
  end

  def sync_force do
    ApplicationCommand.bulk_overwrite_global_commands(commands())
  end
end
