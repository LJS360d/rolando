defmodule Rolando.Analytics.Structs.Event do
  @moduledoc """
  The standard data structure for analytics events.
  """

  defstruct [
    :name,
    :guild_id,
    :channel_id,
    :user_id,
    :level,
    meta: %{}
  ]

  @type t :: %__MODULE__{
          name: String.t(),
          guild_id: String.t() | nil,
          channel_id: String.t() | nil,
          user_id: String.t() | nil,
          # Log Level Enum
          level: :debug | :info | :warn | :error | nil,
          meta: map()
        }
end
