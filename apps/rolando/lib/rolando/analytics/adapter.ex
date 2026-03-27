defmodule Rolando.Analytics.Adapter do
  @moduledoc """
  Behaviour for analytics backends. Dev can use SQL; prod can use a scalable
  store (e.g. event stream, external analytics service).
  """
  @callback guilds_count() :: non_neg_integer()
  @callback persist_event(map()) :: {:ok, Ecto.Schema.t()} | {:error, Ecto.Changeset.t()}
end
