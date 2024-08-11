import type { OfficeRoute } from './route.type';

export const guestUserRoutes: OfficeRoute[] = [
	{
		path: '/home',
		iconClass: 'fa-solid fa-house',
		label: 'Landing page',
	},
	{
		path: '/tos',
		iconClass: 'fa-solid fa-file-contract',
		label: 'Terms of Service',
	},
	{
		path: '/privacy',
		iconClass: 'fa-solid fa-shield',
		label: 'Privacy Policy',
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
