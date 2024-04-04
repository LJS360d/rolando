import type { OfficeRoute } from './route.type';

export const guestUserRoutes: OfficeRoute[] = [
	{
		path: '/home',
		iconClass: 'fa-solid fa-house',
		label: 'Landing page',
	},
];

export const loggedUserRoutes: OfficeRoute[] = [...guestUserRoutes, ...[]];

export const adminRoutes: OfficeRoute[] = [
	...loggedUserRoutes,
	...[
		{
			path: '/dashboard',
			iconClass: 'fa-solid fa-house-laptop',
			label: 'Admin Dashboard',
		},
		{
			path: '/broadcast',
			iconClass: 'fa-solid fa-bullhorn',
			label: 'Message Broadcast',
		},
	].map((e) => ({ ...e, admin: true })),
];
