defmodule RolandoDiscord.Permissions do
  @moduledoc false

  alias Nostrum.Api.Guild
  alias Nostrum.Cache.{GuildCache, Me}
  alias Nostrum.Constants.ChannelType
  alias Nostrum.Struct.Guild.Member
  alias Nostrum.Struct.Channel

  def admin_or_owner?(%{guild_id: nil}), do: false

  def admin_or_owner?(%{guild_id: guild_id, member: member}) when not is_nil(member) do
    user_id = member.user_id
    owners = Application.get_env(:rolando, :owner_platform_ids, [])

    owner_ids =
      owners
      |> Enum.map(&String.trim/1)
      |> Enum.reject(&(&1 == ""))

    if Enum.any?(owner_ids, &(to_string(user_id) == &1)) do
      true
    else
      guild = GuildCache.get!(guild_id)
      perms = Member.guild_permissions(member, guild)
      :administrator in perms
    end
  end

  def admin_or_owner?(_), do: false

  def can_fetch_text_channel?(guild_id, %Channel{} = channel) do
    guild = GuildCache.get!(guild_id)
    me = Me.get()

    bot_member =
      case Guild.member(guild_id, me.id) do
        {:ok, member} -> member
        _ -> nil
      end

    if bot_member do
      check_channel_permissions(bot_member, guild, channel)
    else
      false
    end
  end

  defp check_channel_permissions(member, guild, %Channel{} = channel) do
    perms = Member.guild_channel_permissions(member, guild, channel.id)

    textish = channel.type in [ChannelType.guild_text(), ChannelType.guild_announcement()]

    textish and
      :view_channel in perms and
      :read_message_history in perms and
      :send_messages in perms
  end

  def list_trainable_text_channels(guild_id) do
    guild = GuildCache.get!(guild_id)

    (guild.channels || %{})
    |> Map.values()
    |> Enum.filter(&can_fetch_text_channel?(guild_id, &1))
  end

  def list_channels_for_display(guild_id) do
    guild = GuildCache.get!(guild_id)

    (guild.channels || %{})
    |> Map.values()
    |> Enum.reject(fn ch ->
      ch.type == ChannelType.guild_voice() or ch.type == ChannelType.guild_category()
    end)
  end
end
