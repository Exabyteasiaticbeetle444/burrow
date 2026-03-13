import { defineConfig } from 'vitest/config';
import { sveltekit } from '@sveltejs/kit/vite';

export default defineConfig({
	plugins: [sveltekit()],
	test: {
		include: ['src/**/*.test.ts'],
		environment: 'jsdom',
		globals: true,
	},
	resolve: {
		alias: {
			'@tauri-apps/api/core': new URL('src/__mocks__/tauri-api-core.ts', import.meta.url).pathname,
		},
	},
});
