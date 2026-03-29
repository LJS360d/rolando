defmodule Rolando.Cache.Adapter do
  @moduledoc """
  Behaviour for memory cache adapters, Dev can use ETS, distributed Prod an external Redis or distributed ETS approach.
  """
  @callback init() :: :ok | {:error, any()}
  @callback get(table :: atom(), key :: String.t()) :: {:ok, any()} | {:error, :not_found}
  @callback put(table :: atom(), key :: String.t(), value: any()) :: :ok
  @callback delete(table :: atom(), key :: String.t()) :: :ok
end
