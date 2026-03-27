defmodule RolandoDiscord.Permissions do
  @moduledoc false

  alias Nostrum.Cache.{GuildCache, Me}
  alias Nostrum.Constants.ChannelType
  alias Nostrum.Struct.Guild.Member

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

  def bot_can_fetch_text_channel?(guild_id, channel) do
    guild_id = guild_id
    me = Me.get()

    if me == nil or channel == nil do
      false
    else
      bot_id = me.id
      guild = GuildCache.get!(guild_id)
      bot_member = Map.get(guild.members, bot_id)

      if bot_member == nil do
        false
      else
        perms = Member.guild_channel_permissions(bot_member, guild, channel.id)

        textish =
          channel.type == ChannelType.guild_text() or channel.type == ChannelType.guild_announcement()

        textish and :view_channel in perms and :read_message_history in perms and
          :send_messages in perms
      end
    end
  end

  def list_trainable_text_channels(guild_id) do
    guild = GuildCache.get!(guild_id)

    (guild.channels || %{})
    |> Map.values()
    |> Enum.filter(&bot_can_fetch_text_channel?(guild_id, &1))
  end

  def list_channels_for_display(guild_id) do
    guild = GuildCache.get!(guild_id)

    (guild.channels || %{})
    |> Map.values()
    |> Enum.reject(fn ch ->
      ch.type == ChannelType.guild_voice() or ch.type == ChannelType.guild_category()
    end)
    |> Enum.sort_by(& &1.position)
  end
end
