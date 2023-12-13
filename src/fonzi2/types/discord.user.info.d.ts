export type DiscordUserInfo = {
	id: string;
	username: string;
	avatar: string;
	discriminator: string;
	public_flags: number;
	premium_type: number;
	flags: number;
	banner: string | null;
	accent_color: number;
	global_name: string;
	avatar_decoration_data: any | null; // TODO get proper type inference
	banner_color: string;
	mfa_enabled: boolean;
	locale: string;
	email: string;
	verified: boolean;
};
