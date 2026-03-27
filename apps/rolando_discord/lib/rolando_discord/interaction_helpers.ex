defmodule RolandoDiscord.InteractionHelpers do
  @moduledoc false

  alias Nostrum.Struct.Interaction, as: I

  def cooldown_mins, do: 30

  def train_cooldown_active?(%DateTime{} = trained_at) do
    cutoff = DateTime.add(trained_at, cooldown_mins(), :minute)
    DateTime.compare(DateTime.utc_now(), cutoff) == :lt
  end

  def fmt_dt(%DateTime{} = dt) do
    Calendar.strftime(dt, "%d/%m/%Y %H:%M:%S")
  end

  def user_mention(%I{member: %{user_id: uid}}) when not is_nil(uid), do: "<@#{uid}>"
  def user_mention(%I{user: %{id: id}}) when not is_nil(id), do: "<@#{id}>"
  def user_mention(_), do: ""

  def owner_user?(%I{member: %{user_id: uid}}) when not is_nil(uid), do: match_owner?(uid)
  def owner_user?(%I{user: %{id: id}}) when not is_nil(id), do: match_owner?(id)
  def owner_user?(_), do: false

  defp match_owner?(user_id) do
    owners = Application.get_env(:rolando, :owner_platform_ids, [])

    Enum.any?(owners, fn o ->
      String.trim(o) != "" and to_string(user_id) == String.trim(o)
    end)
  end
end
