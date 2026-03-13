import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
	clearAuth,
	isAuthenticated,
	login,
	getStats,
	getClients,
	getClient,
	revokeClient,
	getInvites,
	createInvite,
	revokeInvite,
	getConfig,
	formatBytes,
	formatDate,
} from './api';

function mockFetchResponse(body: unknown, status = 200, ok = true) {
	return vi.fn().mockResolvedValue({
		ok,
		status,
		statusText: 'Error',
		json: () => Promise.resolve(body),
	});
}

beforeEach(() => {
	Object.defineProperty(document, 'cookie', {
		writable: true,
		value: '',
	});
	vi.restoreAllMocks();
	Object.defineProperty(window, 'location', {
		value: { href: '' },
		writable: true,
		configurable: true,
	});
});

afterEach(() => {
	vi.restoreAllMocks();
});

describe('Auth functions', () => {
	it('clearAuth expires the burrow_authed cookie', () => {
		document.cookie = 'burrow_authed=1';
		clearAuth();
		expect(document.cookie).toContain('Max-Age=0');
	});

	it('isAuthenticated returns true when burrow_authed cookie exists', () => {
		Object.defineProperty(document, 'cookie', {
			writable: true,
			value: 'burrow_authed=1',
		});
		expect(isAuthenticated()).toBe(true);
	});

	it('isAuthenticated returns false when no cookie', () => {
		expect(isAuthenticated()).toBe(false);
	});

	it('login sends password with credentials same-origin', async () => {
		global.fetch = mockFetchResponse({ ok: true });

		const data = await login('secret');

		expect(global.fetch).toHaveBeenCalledWith('/api/auth/login', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ password: 'secret' }),
			credentials: 'same-origin',
		});
		expect(data).toEqual({ ok: true });
	});

	it('login throws on bad password', async () => {
		global.fetch = mockFetchResponse({}, 401, false);

		await expect(login('wrong')).rejects.toThrow('Invalid password');
	});
});

describe('request helper (tested via API functions)', () => {
	it('sends credentials same-origin without Authorization header', async () => {
		global.fetch = mockFetchResponse({ total_clients: 5 });

		await getStats();

		const call = (global.fetch as ReturnType<typeof vi.fn>).mock.calls[0];
		const opts = call[1];
		expect(opts.credentials).toBe('same-origin');
		expect(opts.headers).not.toHaveProperty('Authorization');
	});

	it('redirects to /admin/login on 401', async () => {
		global.fetch = mockFetchResponse({}, 401);

		await expect(getStats()).rejects.toThrow('Unauthorized');
		expect(window.location.href).toBe('/admin/login');
	});

	it('throws on non-ok response with error message from body', async () => {
		global.fetch = mockFetchResponse({ error: 'Not found' }, 404, false);

		await expect(getStats()).rejects.toThrow('Not found');
	});

	it('throws with statusText when body has no error field', async () => {
		global.fetch = vi.fn().mockResolvedValue({
			ok: false,
			status: 500,
			statusText: 'Internal Server Error',
			json: () => Promise.resolve({}),
		});

		await expect(getStats()).rejects.toThrow('Internal Server Error');
	});

	it('throws with statusText when body json parsing fails', async () => {
		global.fetch = vi.fn().mockResolvedValue({
			ok: false,
			status: 500,
			statusText: 'Internal Server Error',
			json: () => Promise.reject(new Error('parse error')),
		});

		await expect(getStats()).rejects.toThrow('Internal Server Error');
	});
});

describe('CRUD functions', () => {
	it('getStats returns ServerStats', async () => {
		const stats = {
			total_clients: 10,
			active_clients: 5,
			revoked_clients: 2,
			total_bytes_up: 1024,
			total_bytes_down: 2048,
			total_connections: 15,
		};
		global.fetch = mockFetchResponse(stats);

		const result = await getStats();
		expect(result).toEqual(stats);
	});

	it('getClients returns Client[]', async () => {
		const clients = [
			{ id: '1', name: 'client1', token: 't1', created_at: '2024-01-01', revoked: false, bytes_up: 0, bytes_down: 0 },
			{ id: '2', name: 'client2', token: 't2', created_at: '2024-01-02', revoked: true, bytes_up: 100, bytes_down: 200 },
		];
		global.fetch = mockFetchResponse(clients);

		const result = await getClients();
		expect(result).toEqual(clients);
		expect(global.fetch).toHaveBeenCalledWith('/api/clients', expect.any(Object));
	});

	it('getClient returns a single Client', async () => {
		const client = { id: 'abc', name: 'test', token: 't', created_at: '2024-01-01', revoked: false, bytes_up: 0, bytes_down: 0 };
		global.fetch = mockFetchResponse(client);

		const result = await getClient('abc');
		expect(result).toEqual(client);
		expect(global.fetch).toHaveBeenCalledWith('/api/clients/abc', expect.any(Object));
	});

	it('revokeClient sends DELETE', async () => {
		global.fetch = mockFetchResponse(null);

		await revokeClient('abc');
		expect(global.fetch).toHaveBeenCalledWith('/api/clients/abc', expect.objectContaining({ method: 'DELETE' }));
	});

	it('getInvites returns Invite[]', async () => {
		const invites = [
			{ id: '1', name: 'inv1', token: 'it1', created_at: '2024-01-01', revoked: false },
		];
		global.fetch = mockFetchResponse(invites);

		const result = await getInvites();
		expect(result).toEqual(invites);
		expect(global.fetch).toHaveBeenCalledWith('/api/invites', expect.any(Object));
	});

	it('createInvite sends POST with name', async () => {
		const response = { client: { id: '1' }, invite: 'invite-url' };
		global.fetch = mockFetchResponse(response);

		const result = await createInvite('new-client');
		expect(result).toEqual(response);
		expect(global.fetch).toHaveBeenCalledWith(
			'/api/invites',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ name: 'new-client' }),
			}),
		);
	});

	it('createInvite includes expiresIn in body when provided', async () => {
		const response = { client: { id: '1' }, invite: 'invite-url' };
		global.fetch = mockFetchResponse(response);

		await createInvite('new-client', '24h');
		expect(global.fetch).toHaveBeenCalledWith(
			'/api/invites',
			expect.objectContaining({
				method: 'POST',
				body: JSON.stringify({ name: 'new-client', expires_in: '24h' }),
			}),
		);
	});

	it('revokeInvite sends DELETE', async () => {
		global.fetch = mockFetchResponse(null);

		await revokeInvite('inv-123');
		expect(global.fetch).toHaveBeenCalledWith('/api/invites/inv-123', expect.objectContaining({ method: 'DELETE' }));
	});

	it('getConfig returns config object', async () => {
		const config = { listen: ':8080', dns: '1.1.1.1' };
		global.fetch = mockFetchResponse(config);

		const result = await getConfig();
		expect(result).toEqual(config);
		expect(global.fetch).toHaveBeenCalledWith('/api/config', expect.any(Object));
	});
});

describe('formatBytes', () => {
	it('returns "0 B" for 0', () => {
		expect(formatBytes(0)).toBe('0 B');
	});

	it('returns "1.0 KB" for 1024', () => {
		expect(formatBytes(1024)).toBe('1.0 KB');
	});

	it('returns "1.0 MB" for 1048576', () => {
		expect(formatBytes(1048576)).toBe('1.0 MB');
	});

	it('returns "1.0 GB" for 1073741824', () => {
		expect(formatBytes(1073741824)).toBe('1.0 GB');
	});

	it('returns "1.0 TB" for 1099511627776', () => {
		expect(formatBytes(1099511627776)).toBe('1.0 TB');
	});

	it('returns "0 B" for NaN', () => {
		expect(formatBytes(NaN)).toBe('0 B');
	});

	it('does not crash for Infinity (BUG-25)', () => {
		expect(() => formatBytes(Infinity)).not.toThrow();
		const result = formatBytes(Infinity);
		expect(typeof result).toBe('string');
	});

	it('returns "0 B" for negative numbers', () => {
		expect(formatBytes(-100)).toBe('0 B');
	});

	it('formats intermediate values correctly', () => {
		expect(formatBytes(500)).toBe('500.0 B');
		expect(formatBytes(1536)).toBe('1.5 KB');
	});
});

describe('formatDate', () => {
	it('returns "Never" for empty string', () => {
		expect(formatDate('')).toBe('Never');
	});

	it('formats a valid ISO date string', () => {
		const result = formatDate('2024-01-15T14:30:00Z');
		expect(result).toContain('Jan');
		expect(result).toContain('15');
		expect(result).toContain('2024');
	});

	it('returns "Never" or "Invalid Date" for invalid input', () => {
		const result = formatDate('not-a-date');
		expect(['Never', 'Invalid Date']).toContain(result);
	});
});
