defmodule Rolando.LM do
  @moduledoc """
  Language Model facade. Delegates to the configured adapter so dev can use In-Memory markov chains
  and prod can use an external Redis.
  """
  @behaviour Rolando.LM.Adapter
  @default_adapter Rolando.LM.Adapters.ETS

  defp adapter do
    Application.get_env(:rolando, :lm_adapter, @default_adapter)
  end

  @impl true
  def train(guild_id, text, opts \\ []) do
    adapter().train(guild_id, text, opts)
  end

  @impl true
  def train_batch(guild_id, messages, opts \\ []) do
    adapter().train_batch(guild_id, messages, opts)
  end

  @impl true
  def generate(guild_id) do
    adapter().generate(guild_id)
  end

  @impl true
  def generate(guild_id, seed) do
    adapter().generate(guild_id, seed)
  end

  @impl true
  def generate(guild_id, seed, length) do
    adapter().generate(guild_id, seed, length)
  end

  @impl true
  def get_tier(guild_id) do
    adapter().get_tier(guild_id)
  end

  @impl true
  def change_tier(guild_id, tier, messages) do
    adapter().change_tier(guild_id, tier, messages)
  end

  @impl true
  def delete_message(guild_id, data) do
    adapter().delete_message(guild_id, data)
  end

  @impl true
  def get_stats(guild_id) do
    adapter().get_stats(guild_id)
  end

  @impl true
  def delete_guild(guild_id) do
    adapter().delete_guild(guild_id)
  end
end
