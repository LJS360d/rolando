defmodule Rolando.Messages.Adapter do
  @moduledoc """
  Behaviour for message storage backends.
  Dev can use SQLite; prod can use a high-throughput store (e.g., TimescaleDB, ClickHouse).
  """

  @callback create(map()) :: {:ok, Ecto.Schema.t()} | {:error, Ecto.Changeset.t()}
  @callback create_many([map()]) :: {Integer.t(), [Ecto.Schema.t()]} | {:error, term()}
  @callback list_by_guild(String.t(), keyword()) :: [Ecto.Schema.t()]
  @callback count_by_guild(String.t()) :: non_neg_integer()
  @callback delete_by_guild(String.t()) :: {non_neg_integer(), nil | [term()]}
  @callback get_random_messages(String.t(), non_neg_integer()) :: [Ecto.Schema.t()]
end
