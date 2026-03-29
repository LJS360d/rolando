defmodule Rolando.Persistence.StorageProvider do
  @moduledoc """
  Behaviour: save/load/list for weights + media.
  """

  @callback save_weight(guild_id :: String.t(), data :: binary(), opts :: keyword()) ::
              :ok | {:error, term()}
  @callback load_weight(guild_id :: String.t()) :: {:ok, binary()} | {:error, :not_found}
  @callback list_weights() :: {:ok, [String.t()]}
  @callback delete_weight(guild_id :: String.t()) :: :ok | {:error, term()}

  @callback save_media(media :: map()) :: {:ok, integer()} | {:error, term()}
  @callback list_media(guild_id :: String.t(), opts :: keyword()) :: {:ok, [map()]}
  @callback delete_media(id :: integer()) :: :ok | {:error, term()}

  @optional_callbacks [save_media: 1, list_media: 2, delete_media: 1]
end
