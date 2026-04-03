defmodule Rolando.Analytics.Adapter do
  alias Rolando.Analytics.Structs.Event

  @moduledoc """
  Behaviour for analytics backends. Dev can use SQL; prod can use a scalable
  store (e.g. event stream, external analytics service).
  """
  @callback guilds_count() :: non_neg_integer()
  @callback persist_event(Event.t() | map()) ::
              {:ok, Ecto.Schema.t()} | {:error, Ecto.Changeset.t()}
  @callback list_recent_events(pos_integer(), map()) :: [Ecto.Schema.t()]
  @callback event_counts_by_day(pos_integer()) :: [{String.t(), non_neg_integer()}]
end
