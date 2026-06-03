// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightUtils from '@lorenzo_lewis/starlight-utils';
import mermaid from 'astro-mermaid';
import tailwindcss from '@tailwindcss/vite';

import mdx from '@astrojs/mdx';

export default defineConfig({
	site: 'https://panix.xyz',
	vite: {
		plugins: [tailwindcss()],
	},
	integrations: [
		mermaid(),
		starlight({
			title: 'Panix',
			logo: {
				src: './public/icon.svg',
			},
			description: 'Universal NixOS Deployment Tool - documentation and wiki',
			customCss: ['./src/styles/panix-theme.css'],
			components: {
				ThemeProvider: './src/components/ForceDarkTheme.astro',
				ThemeSelect: './src/components/EmptyComponent.astro',
				Hero: './src/components/Hero.astro',
			},
			expressiveCode: {
				themes: ['starlight-dark'],
				useStarlightDarkModeSwitch: false,
			},
			social: [
				{
					icon: 'github',
					label: 'GitHub',
					href: 'https://github.com/mihakrumpestar/panix',
				},
				{
					icon: 'heart',
					label: 'Fund me',
					href: 'https://ko-fi.com/mihakrumpestar',
				},
			],
			editLink: {
				baseUrl:
					'https://github.com/mihakrumpestar/panix/edit/main/docs/',
			},
			plugins: [
				starlightUtils({
					navLinks: {
						leading: { useSidebarLabelled: 'leadingNavLinks' },
					},
				}),
			],
			sidebar: [
				{
					label: 'leadingNavLinks',
					items: [
						{ label: 'Docs', link: '/getting-started/' },
					],
				},
				{
					label: 'Getting Started',
					items: [
						{ autogenerate: { directory: 'getting-started' } },
					],
				},
				{
					label: 'Concepts',
					items: [{ autogenerate: { directory: 'concepts' } }],
				},
				{
					label: 'Configuration',
					items: [
						{ autogenerate: { directory: 'configuration' } },
					],
				},
				{
					label: 'Features',
					items: [{ autogenerate: { directory: 'features' } }],
				},
				{
					label: 'Guides',
					items: [
						{
							label: 'Bootstrap',
							items: [
								{ autogenerate: { directory: 'guides/bootstrap' } },
							],
						},
						{ slug: 'guides/reinstall' },
						{ slug: 'guides/secrets' },
						{ slug: 'guides/ssh-config' },
						{ slug: 'guides/snapshots' },
					],
				},
				{
					label: 'TUI',
					items: [{ autogenerate: { directory: 'tui' } }],
				},
				{
					label: 'CLI Reference',
					items: [{ autogenerate: { directory: 'cli' } }],
				},
				{
					label: 'Internals',
					items: [{ autogenerate: { directory: 'internals' } }],
				},
				{
					label: 'Project',
					items: [{ autogenerate: { directory: 'project' } }],
				},
			],
		}),
		mdx(),
	],
});
