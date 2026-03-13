import {
	getStatus,
	getServers,
	getPreferences,
	setPreferences,
	waitForDaemon,
	type TunnelStatus,
	type Server,
	type Preferences
} from './api';

let _status = $state<TunnelStatus | null>(null);
let _servers = $state<Server[]>([]);
let _preferences = $state<Preferences>({ tun_mode: true, kill_switch: false, auto_connect: false });
let _daemonReady = $state(false);
let _loading = $state(false);
let _error = $state('');
let _speedUp = $state(0);
let _speedDown = $state(0);

let _prevBytesUp = 0;
let _prevBytesDown = 0;
let _prevTimestamp = 0;
let _pollInterval: ReturnType<typeof setInterval> | null = null;
let _initialized = false;

export const store = {
	get status() {
		return _status;
	},
	set status(v: TunnelStatus | null) {
		_status = v;
	},

	get servers() {
		return _servers;
	},
	set servers(v: Server[]) {
		_servers = v;
	},

	get preferences() {
		return _preferences;
	},

	get daemonReady() {
		return _daemonReady;
	},
	set daemonReady(v: boolean) {
		_daemonReady = v;
	},

	get loading() {
		return _loading;
	},
	set loading(v: boolean) {
		_loading = v;
	},

	get error() {
		return _error;
	},
	set error(v: string) {
		_error = v;
	},

	get connected() {
		return _status?.running ?? false;
	},

	get speedUp() {
		return _speedUp;
	},

	get speedDown() {
		return _speedDown;
	},

	get initialized() {
		return _initialized;
	},

	async refreshStatus() {
		try {
			const [s, srv] = await Promise.all([
				getStatus().catch(() => null),
				getServers().catch(() => [] as Server[])
			]);
			_status = s;
			_servers = srv;
			if (_error === 'Cannot connect to Burrow daemon') _error = '';

			if (s && s.running) {
				const now = Date.now();
				if (_prevTimestamp > 0) {
					const dt = (now - _prevTimestamp) / 1000;
					if (dt > 0) {
						_speedUp = Math.max(0, (s.bytes_up - _prevBytesUp) / dt);
						_speedDown = Math.max(0, (s.bytes_down - _prevBytesDown) / dt);
					}
				}
				_prevBytesUp = s.bytes_up;
				_prevBytesDown = s.bytes_down;
				_prevTimestamp = now;
			} else {
				_speedUp = 0;
				_speedDown = 0;
				_prevBytesUp = 0;
				_prevBytesDown = 0;
				_prevTimestamp = 0;
			}
		} catch {
			// silent
		}
	},

	async refreshPreferences() {
		try {
			const prefs = await getPreferences();
			_preferences = prefs;
		} catch {
			// silent
		}
	},

	async updatePreference(partial: Partial<Preferences>) {
		_preferences = { ..._preferences, ...partial };
		try {
			const updated = await setPreferences(partial);
			_preferences = updated;
		} catch {
			// revert on failure
			await this.refreshPreferences();
		}
	},

	async init() {
		if (_initialized) return;
		_initialized = true;

		_daemonReady = await waitForDaemon();
		if (!_initialized) return;
		if (!_daemonReady) {
			_error = 'Cannot connect to Burrow daemon';
			return;
		}

		await Promise.all([this.refreshStatus(), this.refreshPreferences()]);
		if (!_initialized) return;

		_pollInterval = setInterval(() => {
			this.refreshStatus();
		}, 2000);
	},

	destroy() {
		if (_pollInterval) {
			clearInterval(_pollInterval);
			_pollInterval = null;
		}
		_initialized = false;
	}
};
