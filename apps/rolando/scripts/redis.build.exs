defmodule Rolando.RedisBuild do
  @moduledoc """
  Script to load all messages from Ecto DB into Redis for Markov chain training.

  Run with: mix run -e Rolando.RedisBuild.run()
  """

  require Logger
  alias Rolando.Repo
  alias Rolando.Schema.Message
  alias Rolando.Messages
  alias Rolando.LM.Adapters.RedisMarkovChain
  import Ecto.Query

  @default_n_gram_size 2
  @batch_size 1000

  def run do
    # Ensure application is started
    unless Application.started_applications() |> Keyword.has_key?(:rolando) do
      IO.puts("\nERROR: Rolando application is not started!\n")
      IO.puts("Please start the application first, then run:")
      IO.puts("  mix run -e Rolando.RedisBuild.run()\n")
      System.halt(1)
    end

    # Ensure Redis connection is available
    unless Process.whereis(:redix_client) do
      IO.puts("\nERROR: Redis connection (:redix_client) is not available!\n")
      IO.puts("Please ensure Redis is running and the application is properly configured.\n")
      System.halt(1)
    end

    IO.puts("\n=== Starting Markov Chain Build ===")
    IO.puts("Loading all messages from Ecto DB to Redis...\n")

    load_all_messages_to_redis()

    IO.puts("\n=== Markov Chain Build Finished ===\n")
  end

  defp load_all_messages_to_redis do
    Logger.info("Starting to load all messages from Ecto DB to Redis for Markov training...")

    # Get all distinct guild IDs that have messages
    guild_ids = Repo.all(
      from m in Message,
      distinct: true,
      select: m.guild_id
    )
    if Enum.empty?(guild_ids) do
      Logger.info("No guilds with messages found in the Ecto DB.")
      :ok
    else
      Logger.info("Found #{length(guild_ids)} guilds with messages.")

      Enum.each(guild_ids, fn guild_id ->
        guild_id_str = to_string(guild_id)
        IO.puts("\nProcessing guild: #{guild_id_str}")
        load_messages_for_guild_to_redis(guild_id_str)
      end)

      Logger.info("Finished loading messages from Ecto DB to Redis.")
      :ok
    end
  end

  defp load_messages_for_guild_to_redis(guild_id_str) do
    case RedisMarkovChain.get_tier(guild_id_str) do
      {:ok, n_gram_size} ->
        Logger.info("Guild #{guild_id_str}: Using n_gram_size = #{n_gram_size}")
        fetch_and_train_messages_recursive(guild_id_str, nil, n_gram_size)

      {:error, _reason} ->
        Logger.warning("Guild #{guild_id_str}: Could not fetch n_gram_size. Using default: #{@default_n_gram_size}")
        fetch_and_train_messages_recursive(guild_id_str, nil, @default_n_gram_size)
    end
  end

  defp fetch_and_train_messages_recursive(guild_id_str, offset, n_gram_size) do
    current_offset = offset || 0
    Logger.debug("Guild #{guild_id_str}: Fetching messages with offset #{current_offset}")

    messages_batch = Messages.list_by_guild(guild_id_str, limit: @batch_size, offset: current_offset)

    if Enum.empty?(messages_batch) do
      Logger.info("Guild #{guild_id_str}: Finished (offset #{current_offset} reached end).")
      :ok
    else
      message_contents = Enum.map(messages_batch, & &1.content)

      case RedisMarkovChain.train_batch(guild_id_str, message_contents, n_gram_size: n_gram_size) do
        :ok ->
          Logger.info("Guild #{guild_id_str}: Trained #{length(messages_batch)} messages (offset: #{current_offset}).")
          next_offset = current_offset + length(messages_batch)
          fetch_and_train_messages_recursive(guild_id_str, next_offset, n_gram_size)

        {:error, reason} ->
          Logger.error("Guild #{guild_id_str}: Failed to train batch (offset: #{current_offset}). Reason: #{inspect(reason)}")
          next_offset = current_offset + length(messages_batch)
          fetch_and_train_messages_recursive(guild_id_str, next_offset, n_gram_size)
      end
    end
  end
end

# Auto-run when executed with mix run
Rolando.RedisBuild.run()
