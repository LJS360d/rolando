defmodule RolandoDiscord.Consumers.Event do
  use Nostrum.Consumer
  alias Nostrum.Struct.Guild
  alias Rolando.Contexts.Guilds
  alias Rolando.Schema.Guild, as: GuildSchema
  alias Nostrum.Shard
  alias RolandoDiscord.Commands
  require Logger

  # fires once when the bot has logged in successfully
  def handle_event({:READY, _data, _ws_state}) do
    Commands.sync()

    guilds_count =
      case Nostrum.Api.Self.guilds() do
        {:ok, guilds} ->
          length(guilds)

        {:error, _} ->
          0
      end

    Shard.Supervisor.update_status(:online, "#{guilds_count} servers", nil, 3)
  end

  # fires when the bot resumes from a disconnected state
  def handle_event({:RESUMED, _data, _ws_state}) do
    Logger.info("Resumed")
  end

  # fires when the bot id added to a server
  def handle_event({:GUILD_CREATE, %Guild{} = guild, _ws_state}) do
    Logger.info("Joined guild #{guild.name}")

    case Guilds.upsert(%GuildSchema{
           id: to_string(guild.id),
           name: guild.name,
           platform: "discord",
           image_url: Nostrum.Struct.Guild.icon_url(guild)
         }) do
      {:ok, _} ->
        :ok

      {:error, err} ->
        Logger.error("failed to store entry for guild #{guild.name}: #{inspect(err)}")
    end
  end

  def handle_event({:GUILD_UPDATE, {old, %Guild{} = new}, _ws_state}) when is_nil(old) do
    Logger.warning("Guild #{new.id} (#{new.name}) updated, but old version was not cached")
  end

  # fires when info about the guild is changed
  def handle_event({:GUILD_UPDATE, {%Guild{} = old, %Guild{} = new}, _ws_state}) do
    Logger.info("Guild #{new.id} updated, #{old.name} -> #{new.name}")

    case Guilds.upsert(%GuildSchema{
           id: to_string(new.id),
           name: new.name,
           platform: "discord",
           image_url: Nostrum.Struct.Guild.icon_url(new)
         }) do
      {:ok, _} ->
        :ok

      {:error, err} ->
        Logger.error("failed to update entry for guild #{old.name}: #{inspect(err)}")
    end
  end

  # fires when the bot is kicked/banned from a server
  def handle_event({:GUILD_DELETE, %Guild{} = guild, _ws_state}) do
    Logger.info("Left guild #{guild.name}")

    case Guilds.delete(guild.id) do
      {:ok, _} ->
        :ok

      {:error, err} ->
        Logger.error("failed to delete entry for guild #{guild.name}: #{inspect(err)}")
    end
  end

  def handle_event(_event), do: :noop
end
