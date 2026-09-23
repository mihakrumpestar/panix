// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import starlightUtils from '@lorenzo_lewis/starlight-utils';
import mermaid from 'astro-mermaid';
import tailwindcss from '@tailwindcss/vite';

import mdx from '@astrojs/mdx';

import { REPO_URL } from './src/lib/repo';

export default defineConfig({
	site: 'https://panix.xyz',
	vite: {
		plugins: [tailwindcss()],
		// Allow dev-server imports of files outside docs/ (gen/VERSION via src/lib/repo).
		server: { fs: { allow: ['..'] } },
	},
	integrations: [
		mermaid({
			mermaidConfig: {
				themeVariables: {
					fontSize: '18px',
				},
				flowchart: {
					nodeSpacing: 30,
					rankSpacing: 35,
					padding: 8,
				},
			},
		}),
		starlight({
			title: 'Panix',
			logo: {
				src: './public/icon.svg',
			},
			description: 'Panix - Universal Nix Deployment Orchestrator',
			customCss: ['./src/styles/panix-theme.css'],
			components: {
				ThemeProvider: './src/components/ForceDarkTheme.astro',
				ThemeSelect: './src/components/EmptyComponent.astro',
				Hero: './src/components/Hero.astro',
				SocialIcons: './src/components/HeaderSocial.astro',
			},
			expressiveCode: {
				themes: ['starlight-dark'],
				useStarlightDarkModeSwitch: false,
			},
			social: [
				{
					icon: 'github',
					label: 'GitHub',
					href: REPO_URL,
				},
				{
					icon: 'heart',
					label: 'Fund me',
					href: 'https://ko-fi.com/mihakrumpestar',
				},
			],
			editLink: {
				baseUrl: `${REPO_URL}/edit/main/docs/`,
			},
			plugins: [
				starlightUtils({
					navLinks: {
						leading: { useSidebarLabelled: 'navbar' },
					},
				}),
			],
			sidebar: [
				{
					label: 'navbar',
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
					{ slug: 'guides/packages' },
					{ slug: 'guides/auto-rollback' },
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
		],
		}),
		mdx(),
	],
});
