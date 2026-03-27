defmodule Rolando.Messages do
  @moduledoc false

  alias Rolando.Repo
  alias Rolando.Schema.TrainingMessage

  @chunk 400

  def insert_training_lines(_guild_id, _channel_id, []), do: :ok

  def insert_training_lines(guild_id, channel_id, lines) when is_list(lines) do
    gid = to_string(guild_id)
    cid = to_string(channel_id)
    now = DateTime.utc_now(:second)

    lines
    |> Enum.chunk_every(@chunk)
    |> Enum.each(fn chunk ->
      rows =
        Enum.map(chunk, fn content ->
          %{
            guild_id: gid,
            channel_id: cid,
            content: content,
            inserted_at: now
          }
        end)

      Repo.insert_all(TrainingMessage, rows)
    end)

    :ok
  end

  def delete_for_guild(guild_id) do
    Rolando.Chains.delete_training_messages(guild_id)
  end
end
