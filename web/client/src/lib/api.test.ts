import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

vi.mock('./i18n.svelte', () => ({
	t: (key: string) => key,
	i18n: { locale: 'en', t: (key: string) => key },
}));

import {
	getStatus,
	connect,
	disconnect,
	getServers,
	addServer,
	removeServer,
	pingServer,
	getVersion,
	getPreferences,
	setPreferences,
	waitForDaemon,
	formatBytes,
	formatSpeed,
	formatDuration,
} from './api';

const API = 'http://127.0.0.1:9090';

function mockFetch(body: unknown, ok = true, status = 200) {
	return vi.fn().mockResolvedValue({
		ok,
		status,
		statusText: ok ? 'OK' : 'Bad Request',
		json: () => Promise.resolve(body),
	});
}

describe('api', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.restoreAllMocks();
		vi.useRealTimers();
	});

	describe('getStatus', () => {
		it('calls correct URL and returns parsed response', async () => {
			const data = { running: true, server: 'test' };
			globalThis.fetch = mockFetch(data);

			const result = await getStatus();

			expect(globalThis.fetch).toHaveBeenCalledWith(
				`${API}/api/status`,
				expect.objectContaining({
					headers: { 'Content-Type': 'application/json' },
				}),
			);
			expect(result).toEqual(data);
		});

		it('throws on network error', async () => {
			globalThis.fetch = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'));

			await expect(getStatus()).rejects.toThrow('Failed to fetch');
		});

		it('throws with mapped error on non-ok response', async () => {
			globalThis.fetch = mockFetch({ error: 'already connected' }, false, 400);

			await expect(getStatus()).rejects.toThrow('error.already_connected');
		});

		it('throws with raw error when no mapping exists', async () => {
			globalThis.fetch = mockFetch({ error: 'something weird' }, false, 500);

			await expect(getStatus()).rejects.toThrow('something weird');
		});
	});

	describe('connect', () => {
		it('sends POST with server, killSwitch, tunMode', async () => {
			globalThis.fetch = mockFetch({});

			await connect('my-server', true, false);

			expect(globalThis.fetch).toHaveBeenCalledWith(
				`${API}/api/connect`,
				expect.objectContaining({
					method: 'POST',
					body: JSON.stringify({ server: 'my-server', kill_switch: true, tun_mode: false }),
				}),
			);
		});

		it('uses default tunMode=true', async () => {
			globalThis.fetch = mockFetch({});

			await connect('srv');

			const call = (globalThis.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
			expect(JSON.parse(call[1].body)).toEqual({
				server: 'srv',
				kill_switch: undefined,
				tun_mode: true,
			});
		});
	});

	describe('disconnect', () => {
		it('sends POST to /api/disconnect', async () => {
			globalThis.fetch = mockFetch({});

			await disconnect();

			expect(globalThis.fetch).toHaveBeenCalledWith(
				`${API}/api/disconnect`,
				expect.objectContaining({ method: 'POST' }),
			);
		});
	});

	describe('getServers', () => {
		it('returns array of servers', async () => {
			const servers = [{ name: 'srv1', address: '1.2.3.4', port: 443 }];
			globalThis.fetch = mockFetch(servers);

			const result = await getServers();

			expect(result).toEqual(servers);
			expect(globalThis.fetch).toHaveBeenCalledWith(
				`${API}/api/servers`,
				expect.anything(),
			);
		});
	});

	describe('addServer', () => {
		it('sends invite link in POST body', async () => {
			const server = { name: 'new', address: '5.6.7.8' };
			globalThis.fetch = mockFetch(server);

			const result = await addServer('burrow://connect/abc');

			expect(globalThis.fetch).toHaveBeenCalledWith(
				`${API}/api/servers`,
				expect.objectContaining({
					method: 'POST',
					body: JSON.stringify({ invite: 'burrow://connect/abc' }),
				}),
			);
			expect(result).toEqual(server);
		});
	});

	describe('removeServer', () => {
		it('sends DELETE to correct URL with encoded name', async () => {
			globalThis.fetch = mockFetch({});

			await removeServer('my server');

			expect(globalThis.fetch).toHaveBeenCalledWith(
				`${API}/api/servers/my%20server`,
				expect.objectContaining({ method: 'DELETE' }),
			);
		});
	});

	describe('pingServer', () => {
		it('returns latency object', async () => {
			const ping = { server: 'srv1', reachable: true, latency: 42 };
			globalThis.fetch = mockFetch(ping);

			const result = await pingServer('srv1');

			expect(globalThis.fetch).toHaveBeenCalledWith(
				`${API}/api/servers/srv1/ping`,
				expect.anything(),
			);
			expect(result).toEqual(ping);
		});
	});

	describe('getVersion', () => {
		it('returns version info', async () => {
			const ver = { version: '1.0.0', config_dir: '/etc/burrow' };
			globalThis.fetch = mockFetch(ver);

			const result = await getVersion();

			expect(result).toEqual(ver);
		});
	});

	describe('getPreferences', () => {
		it('returns preferences', async () => {
			const prefs = { tun_mode: true, kill_switch: false, auto_connect: true };
			globalThis.fetch = mockFetch(prefs);

			const result = await getPreferences();

			expect(result).toEqual(prefs);
		});
	});

	describe('setPreferences', () => {
		it('sends partial update via PUT', async () => {
			const updated = { tun_mode: false, kill_switch: true, auto_connect: false };
			globalThis.fetch = mockFetch(updated);

			const result = await setPreferences({ tun_mode: false });

			expect(globalThis.fetch).toHaveBeenCalledWith(
				`${API}/api/preferences`,
				expect.objectContaining({
					method: 'PUT',
					body: JSON.stringify({ tun_mode: false }),
				}),
			);
			expect(result).toEqual(updated);
		});
	});

	describe('waitForDaemon', () => {
		it('resolves true when daemon responds', async () => {
			globalThis.fetch = mockFetch({ running: false });

			const promise = waitForDaemon(3, 100);
			await vi.runAllTimersAsync();
			const result = await promise;

			expect(result).toBe(true);
		});

		it('retries on failure and eventually succeeds', async () => {
			let callCount = 0;
			globalThis.fetch = vi.fn().mockImplementation(() => {
				callCount++;
				if (callCount <= 2) {
					return Promise.reject(new Error('fail'));
				}
				return Promise.resolve({
					ok: true,
					status: 200,
					statusText: 'OK',
					json: () => Promise.resolve({ running: false }),
				});
			});

			const promise = waitForDaemon(5, 100);
			await vi.runAllTimersAsync();
			const result = await promise;

			expect(result).toBe(true);
			expect(callCount).toBe(3);
		});

		it('returns false when max retries exceeded', async () => {
			globalThis.fetch = vi.fn().mockRejectedValue(new Error('fail'));

			const promise = waitForDaemon(3, 50);
			await vi.runAllTimersAsync();
			const result = await promise;

			expect(result).toBe(false);
		});
	});

	describe('formatBytes', () => {
		it('formats 0 as "0 B"', () => {
			expect(formatBytes(0)).toBe('0 B');
		});

		it('formats 1024 as "1.0 KB"', () => {
			expect(formatBytes(1024)).toBe('1.0 KB');
		});

		it('formats 1048576 as "1.0 MB"', () => {
			expect(formatBytes(1048576)).toBe('1.0 MB');
		});

		it('formats 1073741824 as "1.0 GB"', () => {
			expect(formatBytes(1073741824)).toBe('1.0 GB');
		});

		it('formats small values without decimals', () => {
			expect(formatBytes(500)).toBe('500 B');
		});

		it('handles NaN', () => {
			expect(formatBytes(NaN)).toBe('0 B');
		});

		it('handles Infinity', () => {
			expect(formatBytes(Infinity)).toBe('0 B');
		});

		it('handles negative values', () => {
			expect(formatBytes(-100)).toBe('0 B');
		});
	});

	describe('formatSpeed', () => {
		it('formats 0 as "0 B/s"', () => {
			expect(formatSpeed(0)).toBe('0 B/s');
		});

		it('formats 1024 as "1.0 KB/s"', () => {
			expect(formatSpeed(1024)).toBe('1.0 KB/s');
		});

		it('formats 1048576 as "1.0 MB/s"', () => {
			expect(formatSpeed(1048576)).toBe('1.0 MB/s');
		});

		it('handles negative', () => {
			expect(formatSpeed(-1)).toBe('0 B/s');
		});

		it('handles NaN', () => {
			expect(formatSpeed(NaN)).toBe('0 B/s');
		});
	});

	describe('formatDuration', () => {
		it('formats 0 as "0s"', () => {
			expect(formatDuration(0)).toBe('0s');
		});

		it('formats seconds only', () => {
			expect(formatDuration(45)).toBe('45s');
		});

		it('formats minutes and seconds', () => {
			expect(formatDuration(60)).toBe('1m 0s');
			expect(formatDuration(90)).toBe('1m 30s');
		});

		it('formats hours and minutes', () => {
			expect(formatDuration(3661)).toBe('1h 1m');
			expect(formatDuration(7200)).toBe('2h 0m');
		});
	});

	describe('mapError (via request)', () => {
		const knownErrors: [string, string][] = [
			['already connected', 'error.already_connected'],
			['invalid request', 'error.invalid_request'],
			['no server configured', 'error.no_server'],
			['server not found', 'error.server_not_found'],
			['invalid invite link', 'error.invalid_invite'],
			['port 1080 is already in use', 'error.port_in_use'],
			['connection timed out', 'error.timeout'],
		];

		for (const [apiError, i18nKey] of knownErrors) {
			it(`maps "${apiError}" to "${i18nKey}"`, async () => {
				globalThis.fetch = mockFetch({ error: apiError }, false, 400);

				await expect(getStatus()).rejects.toThrow(i18nKey);
			});
		}

		it('passes through unmapped errors', async () => {
			globalThis.fetch = mockFetch({ error: 'custom daemon error' }, false, 500);

			await expect(getStatus()).rejects.toThrow('custom daemon error');
		});

		it('falls back to statusText if JSON parsing fails', async () => {
			globalThis.fetch = vi.fn().mockResolvedValue({
				ok: false,
				status: 503,
				statusText: 'Service Unavailable',
				json: () => Promise.reject(new Error('not json')),
			});

			await expect(getStatus()).rejects.toThrow('Service Unavailable');
		});
	});
});
