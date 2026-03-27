defmodule Rolando.Chains do
  @moduledoc false

  import Ecto.Query
  alias Rolando.Repo
  alias Rolando.Schema.{Guild, GuildChain, TrainingMessage}
  alias Rolando.Chains.GuildChainServer

  def get_chain_document(guild_id) do
    gid = to_string(guild_id)

    case Repo.get(GuildChain, gid) do
      nil -> {:error, :not_found}
      row -> {:ok, row}
    end
  end

  def upsert_guild(guild_id, name) do
    gid = to_string(guild_id)

    %Guild{}
    |> Guild.changeset(%{id: gid, name: name})
    |> Repo.insert(
      on_conflict: {:replace, [:name, :updated_at]},
      conflict_target: :id,
      returning: true
    )
  end

  def create_chain(guild_id, name) do
    gid = to_string(guild_id)

    %GuildChain{guild_id: gid}
    |> GuildChain.changeset(%{
      guild_id: gid,
      name: name,
      reply_rate: 10,
      reaction_rate: 30,
      vc_join_rate: 100,
      max_size_mb: 25,
      ngram_size: 2,
      tts_language: "en",
      pings: true,
      premium: false,
      trained_at: nil,
      chain_state: nil
    })
    |> Repo.insert()
  end

  def delete_chain_row(guild_id) do
    gid = to_string(guild_id)
    Repo.delete_all(from(gc in GuildChain, where: gc.guild_id == ^gid))
  end

  def delete_training_messages(guild_id) do
    gid = to_string(guild_id)
    Repo.delete_all(from(tm in TrainingMessage, where: tm.guild_id == ^gid))
  end

  def recreate_chain(guild_id, name) do
    _ = terminate_chain_worker(to_string(guild_id))
    delete_training_messages(guild_id)
    delete_chain_row(guild_id)
    create_chain(guild_id, name)
  end

  def update_trained_at(guild_id, nil) do
    gid = to_string(guild_id)

    q = from(gc in GuildChain, where: gc.guild_id == ^gid)
    {n, _} = Repo.update_all(q, set: [trained_at: nil, updated_at: DateTime.utc_now(:second)])
    {:ok, n}
  end

  def update_trained_at(guild_id, %DateTime{} = dt) do
    gid = to_string(guild_id)

    case Repo.get(GuildChain, gid) do
      nil ->
        {:error, :not_found}

      row ->
        row
        |> GuildChain.changeset(%{trained_at: DateTime.truncate(dt, :second)})
        |> Repo.update()
    end
  end

  def persist_chain_snapshot(guild_id, json) when is_binary(json) do
    gid = to_string(guild_id)

    case Repo.get(GuildChain, gid) do
      nil ->
        {:error, :not_found}

      row ->
        row
        |> GuildChain.changeset(%{chain_state: json})
        |> Repo.update()
    end
  end

  def ensure_chain_worker(guild_id) do
    gid = to_string(guild_id)

    case Registry.lookup(Rolando.Chains.Registry, registry_key(gid)) do
      [{pid, _}] ->
        {:ok, pid}

      [] ->
        case DynamicSupervisor.start_child(
               Rolando.Chains.DynamicSupervisor,
               {GuildChainServer, gid}
             ) do
          {:ok, pid} -> {:ok, pid}
          {:error, {:already_started, pid}} -> {:ok, pid}
          err -> err
        end
    end
  end

  def terminate_chain_worker(guild_id) do
    gid = to_string(guild_id)

    case Registry.lookup(Rolando.Chains.Registry, registry_key(gid)) do
      [{pid, _}] ->
        _ = DynamicSupervisor.terminate_child(Rolando.Chains.DynamicSupervisor, pid)
        :ok

      [] ->
        :ok
    end
  end

  def registry_key(guild_id), do: {:guild_chain, to_string(guild_id)}

  def apply_training_batch(guild_id, lines) when is_list(lines) do
    {:ok, pid} = ensure_chain_worker(guild_id)
    GenServer.call(pid, {:apply_batch, lines}, :infinity)
  end

  def snapshot_to_repo(guild_id) do
    {:ok, pid} = ensure_chain_worker(guild_id)
    GenServer.call(pid, :snapshot, :infinity)
  end

  def reset_worker_state(guild_id) do
    _ = terminate_chain_worker(guild_id)
    ensure_chain_worker(guild_id)
  end
end
