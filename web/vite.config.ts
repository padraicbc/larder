import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [
		tailwindcss(),
		// SvelteKit 3: config lives here (svelte.config.js is gone).
		// SPA build: Go serves build/ and falls back to 200.html for client routes.
		sveltekit({ adapter: adapter({ fallback: '200.html' }) })
	],
	server: {
		proxy: { '/api': 'http://localhost:8080' }
	},
	test: {
		include: ['src/**/*.test.ts']
	}
});
