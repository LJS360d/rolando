defmodule Rolando.LM.Adapter do
  @moduledoc """
  Behaviour for language model adapters, Dev can use ETS, distributed Prod an external Redis or distributed ETS approach.
  """

  @type guild_id :: String.t() | integer()
  @type result :: {:ok, String.t()} | {:error, any()}

  @callback train(guild_id(), String.t(), opts :: keyword()) :: :ok | {:error, any()}
  @callback train_batch(guild_id(), [String.t()], opts :: keyword()) :: :ok | {:error, any()}
  @callback change_tier(guild_id(), tier :: integer(), [String.t()]) ::
              :ok | {:error, any()}

  @callback generate(guild_id()) :: result()
  @callback generate(guild_id(), seed :: String.t() | nil) :: result()
  @callback generate(guild_id(), seed :: String.t() | nil, length :: integer() | nil) :: result()

  @callback get_stats(guild_id()) :: {:ok, map()} | {:error, any()}
  @callback delete_message(guild_id(), String.t()) :: :ok | {:error, any()}
  @callback delete_guild(guild_id()) :: :ok | {:error, any()}
end
