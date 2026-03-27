defmodule Rolando.Chains.GuildChainServer do
  @moduledoc false
  use GenServer

  alias Rolando.{Chains, Markov, Repo}

  def child_spec(guild_id) do
    %{
      id: {__MODULE__, guild_id},
      start: {__MODULE__, :start_link, [guild_id]},
      restart: :temporary
    }
  end
  alias Rolando.Schema.GuildChain

  def start_link(guild_id) when is_binary(guild_id) do
    GenServer.start_link(__MODULE__, guild_id, name: via(guild_id))
  end

  defp via(guild_id) do
    {:via, Registry, {Rolando.Chains.Registry, Chains.registry_key(guild_id)}}
  end

  @impl true
  def init(guild_id) do
    markov =
      case Repo.get(GuildChain, guild_id) do
        nil ->
          Markov.new()

        row ->
          n = row.ngram_size |> Kernel.||(2) |> max(2)

          if row.chain_state in [nil, ""] do
            Markov.new(ngram_size: n)
          else
            m = Markov.deserialize(row.chain_state)
            %{m | ngram_size: n}
          end
      end

    {:ok, %{guild_id: guild_id, markov: markov}}
  end

  @impl true
  def handle_call({:apply_batch, lines}, _from, state) do
    m = Markov.apply_batch(state.markov, lines)
    {:reply, :ok, %{state | markov: m}}
  end

  @impl true
  def handle_call(:snapshot, _from, state) do
    json = Markov.serialize(state.markov)

    case Chains.persist_chain_snapshot(state.guild_id, json) do
      {:ok, _} -> {:reply, :ok, state}
      err -> {:reply, err, state}
    end
  end
end
