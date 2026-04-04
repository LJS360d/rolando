defmodule RolandoDiscord.InteractionHelpers do
  @moduledoc false

  alias Nostrum.Struct.Interaction, as: I
  alias Nostrum.Struct.Guild
  alias Rolando.Schema.Guild, as: GuildSchema

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

  def operator_user?(%I{} = i), do: owner_user?(i)

  defp match_owner?(user_id) do
    owners = Application.get_env(:rolando, :owner_platform_ids, [])

    Enum.any?(owners, fn o ->
      String.trim(o) != "" and to_string(user_id) == String.trim(o)
    end)
  end

  def to_guild_schema(%Guild{} = guild) do
    %GuildSchema{
      id: to_string(guild.id),
      name: guild.name,
      platform: "discord",
      image_url: Guild.icon_url(guild)
    }
  end

  @extensions %{
    gif: ~w(.gif),
    image: ~w(.jpg .jpeg .png .webp .avif),
    video: ~w(.mp4 .mov .avi),
    audio: ~w(.mp3 .wav .ogg)
  }

  @domains %{
    gif: ~w(tenor.com giphy.com),
    image:
      ~w(imgur.com i.imgur.com pinterest.com pin.it pixiv.net pximg.net flickr.com staticflickr.com twitter.com x.com fixvx.com),
    video: ~w(youtube.com youtu.be)
  }

  def detect_media_type(url) do
    uri = URI.parse(String.downcase(url))
    path = uri.path || ""
    host = uri.host || ""

    type_by_extension =
      Enum.find_value(@extensions, fn {type, exts} ->
        Enum.any?(exts, &String.ends_with?(path, &1)) && type
      end)

    type_by_domain =
      Enum.find_value(@domains, fn {type, domains} ->
        Enum.any?(domains, &(host == &1 or String.ends_with?(host, "." <> &1))) && type
      end)

    type_by_extension || type_by_domain || :other
  end
end
