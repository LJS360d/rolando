import type { DiscordUserInfo } from 'fonzi2';
import { env } from '../env';
import type { Response } from 'express';

export type RenderOptions = Readonly<{
	themes: string[];
	theme: string;
	title: string;
	version: string;
	userInfo?: DiscordUserInfo;
}>;

export interface Props {
	[x: string]: any;
}

const ThemesIterator = ['night', 'dark', 'light'];

export const baseRenderOptions: RenderOptions = {
	themes: ThemesIterator,
	theme: ThemesIterator[0],
	title: 'Rolando',
	version: env.VERSION,
};

export function render(
	res: Response,
	component: string,
	props: Props,
	options?: Partial<RenderOptions>
) {
	options = { ...baseRenderOptions, ...options };
	res.render('index', {
		component,
		props,
		...options,
	});
}
