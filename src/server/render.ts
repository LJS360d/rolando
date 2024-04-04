import type { DiscordUserInfo } from 'fonzi2';
import { env } from '../env';
import type { Response } from 'express';
import type { OfficeRoute } from './routes/route.type';
import { adminRoutes, guestUserRoutes } from './routes/route.definitions';

export type RenderOptions = {
	themes: string[];
	theme: string;
	title: string;
	version: string;
	userInfo?: DiscordUserInfo;
	routes: OfficeRoute[];
};

export interface Props {
	[x: string]: any;
}

const ThemesIterator = ['night', 'dark', 'light'];

export const baseRenderOptions: RenderOptions = {
	themes: ThemesIterator,
	theme: ThemesIterator[0],
	title: 'Rolando',
	version: env.VERSION,
	routes: guestUserRoutes,
};

export function render(
	res: Response,
	component: string,
	props: Props,
	options?: Partial<RenderOptions>
) {
	options = { ...baseRenderOptions, ...options };
	if (options?.userInfo?.role === 'owner') {
		options.routes = adminRoutes;
	}
	res.render('index', {
		component,
		props,
		...options,
	});
}
