<script lang="ts">
	import { getStatus, connect, disconnect, getServers, getPreferences, setPreferences, waitForDaemon, formatBytes, formatDuration, type TunnelStatus, type Server } from '$lib/api';
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';

	let status = $state<TunnelStatus | null>(null);
	let servers = $state<Server[]>([]);
	let selectedServer = $state('');
	let killSwitch = $state(false);
	let tunMode = $state(true);
	let autoConnect = $state(false);
	let loading = $state(false);
	let error = $state('');
	let daemonReady = $state(false);
	let daemonStarting = $state(true);
	let pollInterval: ReturnType<typeof setInterval>;

	onMount(async () => {
		daemonStarting = true;
		daemonReady = await waitForDaemon();
		daemonStarting = false;

		if (!daemonReady) {
			error = 'Cannot connect to Burrow daemon';
			return;
		}

		// Load saved preferences
		try {
			const prefs = await getPreferences();
			tunMode = prefs.tun_mode;
			killSwitch = prefs.kill_switch;
			autoConnect = prefs.auto_connect;
		} catch {}

		await refresh();

		// First-run: redirect to servers if none configured
		if (servers.length === 0 && !status?.running) {
			goto('/servers');
			return;
		}

		// Auto-connect if enabled and not already connected
		if (autoConnect && !status?.running && servers.length > 0) {
			await handleToggle();
		}

		pollInterval = setInterval(refresh, 2000);
	});

	onDestroy(() => {
		if (pollInterval) clearInterval(pollInterval);
	});

	async function refresh() {
		try {
			const [s, srv] = await Promise.all([
				getStatus().catch(() => null),
				getServers().catch(() => [])
			]);
			status = s;
			servers = srv;
			if (error === 'Cannot connect to Burrow daemon') error = '';
		} catch {
			// silent
		}
	}

	async function handleToggle() {
		loading = true;
		error = '';
		try {
			if (status?.running) {
				await disconnect();
			} else {
				// Save preferences before connecting
				await setPreferences({ tun_mode: tunMode, kill_switch: killSwitch, auto_connect: autoConnect }).catch(() => {});
				await connect(selectedServer || undefined, killSwitch, tunMode);
			}
			await refresh();
		} catch (e: any) {
			error = e.message;
		} finally {
			loading = false;
		}
	}

	async function toggleTunMode() {
		tunMode = !tunMode;
		await setPreferences({ tun_mode: tunMode }).catch(() => {});
	}

	async function toggleKillSwitch() {
		killSwitch = !killSwitch;
		await setPreferences({ kill_switch: killSwitch }).catch(() => {});
	}

	async function toggleAutoConnect() {
		autoConnect = !autoConnect;
		await setPreferences({ auto_connect: autoConnect }).catch(() => {});
	}

	const connected = $derived(status?.running ?? false);
</script>

{#if daemonStarting}
	<div class="flex flex-col items-center justify-center gap-4 pt-24 animate-in">
		<div class="spinner text-[var(--accent)]" style="width:40px;height:40px;border-width:3px"></div>
		<p class="text-sm text-[var(--text-secondary)]">Starting Burrow...</p>
	</div>
{:else if !daemonReady}
	<div class="flex flex-col items-center justify-center gap-4 pt-20 animate-in">
		<svg class="w-16 h-16 text-red-400 opacity-60" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
			<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
		</svg>
		<p class="text-sm text-[var(--text-secondary)] text-center px-8">Could not start the VPN daemon.<br>Please restart the application.</p>
	</div>
{:else}
<div class="flex flex-col items-center gap-6 md:gap-8 pt-8 md:pt-12">
	<!-- Connection button -->
	<div class="text-center animate-in-scale">
		<div class="text-sm font-medium mb-4 flex items-center justify-center gap-2">
			{#if connected}
				<span class="w-2 h-2 rounded-full bg-[var(--success)] shadow-[0_0_8px_var(--success-glow)]"></span>
				<span class="text-[var(--success)]">Connected</span>
			{:else}
				<span class="w-2 h-2 rounded-full bg-[var(--text-secondary)]"></span>
				<span class="text-[var(--text-secondary)]">Disconnected</span>
			{/if}
		</div>

		<div class="relative">
			{#if connected}
				<div class="absolute inset-0 rounded-full bg-[var(--success)] opacity-20" style="animation: pulse-ring 2s ease-out infinite"></div>
			{/if}
			<button
				onclick={handleToggle}
				disabled={loading || servers.length === 0}
				class="relative w-32 h-32 md:w-40 md:h-40 rounded-full border-[3px] flex items-center justify-center cursor-pointer transition-all duration-300 select-none active:scale-95 disabled:opacity-50 {connected ? 'border-[var(--success)] bg-[var(--success)]/5 shadow-[0_0_40px_var(--success-glow)]' : 'border-[var(--border)] bg-[var(--bg-card)] hover:border-[var(--accent)] hover:shadow-[0_0_30px_var(--accent-glow)]'}"
			>
				{#if loading}
					<div class="spinner text-[var(--accent)]" style="width:32px;height:32px;border-width:3px"></div>
				{:else}
					<svg class="w-10 h-10 md:w-12 md:h-12 transition-colors duration-300 {connected ? 'text-[var(--success)]' : 'text-[var(--text-secondary)]'}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
						<path stroke-linecap="round" stroke-linejoin="round" d="M5.636 18.364a9 9 0 010-12.728m12.728 0a9 9 0 010 12.728m-9.9-2.829a5 5 0 010-7.07m7.072 0a5 5 0 010 7.07M13 12a1 1 0 11-2 0 1 1 0 012 0z" />
					</svg>
				{/if}
			</button>
		</div>
	</div>

	{#if error}
		<div class="w-full bg-red-500/10 border border-red-500/20 text-red-400 px-4 py-2.5 rounded-xl text-sm text-center flex items-center justify-center gap-2 animate-in">
			<svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2">
				<path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
			</svg>
			{error}
			<button onclick={handleToggle} class="ml-2 text-[var(--accent)] hover:underline font-medium">Retry</button>
		</div>
	{/if}

	{#if connected && status}
		<!-- Stats cards -->
		<div class="grid grid-cols-3 gap-2 md:gap-4 w-full animate-in">
			<div class="bg-[var(--bg-card)] border border-[var(--border)] rounded-xl p-3 md:p-4 text-center">
				<div class="text-[10px] md:text-xs text-[var(--text-secondary)] mb-1 uppercase tracking-wider">Uptime</div>
				<div class="font-mono text-sm md:text-lg">{formatDuration(status.uptime)}</div>
			</div>
			<div class="bg-[var(--bg-card)] border border-[var(--border)] rounded-xl p-3 md:p-4 text-center">
				<div class="text-[10px] md:text-xs text-[var(--text-secondary)] mb-1 uppercase tracking-wider flex items-center justify-center gap-1">
					<svg class="w-3 h-3 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M7 11l5-5m0 0l5 5m-5-5v12" /></svg>
					Upload
				</div>
				<div class="font-mono text-sm md:text-lg text-green-400">{formatBytes(status.bytes_up)}</div>
			</div>
			<div class="bg-[var(--bg-card)] border border-[var(--border)] rounded-xl p-3 md:p-4 text-center">
				<div class="text-[10px] md:text-xs text-[var(--text-secondary)] mb-1 uppercase tracking-wider flex items-center justify-center gap-1">
					<svg class="w-3 h-3 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M17 13l-5 5m0 0l-5-5m5 5V6" /></svg>
					Download
				</div>
				<div class="font-mono text-sm md:text-lg text-blue-400">{formatBytes(status.bytes_down)}</div>
			</div>
		</div>

		<!-- Connection details -->
		<div class="w-full bg-[var(--bg-card)] border border-[var(--border)] rounded-xl p-4 animate-in">
			<div class="space-y-3">
				<div class="flex justify-between items-center text-sm">
					<span class="text-[var(--text-secondary)] flex items-center gap-2">
						<svg class="w-4 h-4 opacity-60" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2" /></svg>
						Server
					</span>
					<span class="font-mono text-xs md:text-sm">{status.server}</span>
				</div>
				<div class="flex justify-between items-center text-sm">
					<span class="text-[var(--text-secondary)] flex items-center gap-2">
						<svg class="w-4 h-4 opacity-60" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" /></svg>
						Protocol
					</span>
					<span class="font-mono text-xs md:text-sm">VLESS+Reality</span>
				</div>
				<div class="flex justify-between items-center text-sm">
					<span class="text-[var(--text-secondary)] flex items-center gap-2">
						<svg class="w-4 h-4 opacity-60" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3" /></svg>
						Mode
					</span>
					<span class="text-xs px-2 py-0.5 rounded-full {status.tun_mode ? 'bg-[var(--accent-glow)] text-[var(--accent)] border border-[var(--accent)]/20' : 'bg-[var(--bg-card-hover)] text-[var(--text-secondary)] border border-[var(--border)]'}">
						{status.tun_mode ? 'VPN (all traffic)' : 'Proxy only'}
					</span>
				</div>
				<div class="flex justify-between items-center text-sm">
					<span class="text-[var(--text-secondary)] flex items-center gap-2">
						<svg class="w-4 h-4 opacity-60" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" /></svg>
						Kill Switch
					</span>
					<span class="text-xs px-2 py-0.5 rounded-full {status.kill_switch ? 'bg-[var(--success-glow)] text-green-400 border border-green-500/20' : 'bg-[var(--bg-card-hover)] text-[var(--text-secondary)] border border-[var(--border)]'}">
						{status.kill_switch ? 'Enabled' : 'Disabled'}
					</span>
				</div>
			</div>
		</div>
	{:else}
		<div class="w-full space-y-3 md:space-y-4 animate-in">
			{#if servers.length > 1}
				<div class="bg-[var(--bg-card)] border border-[var(--border)] rounded-xl p-4">
					<label for="server-select" class="text-xs text-[var(--text-secondary)] mb-2 block uppercase tracking-wider font-medium">Server</label>
					<select
						id="server-select"
						bind:value={selectedServer}
						class="w-full px-3 py-2.5 bg-[var(--bg-primary)] border border-[var(--border)] rounded-lg text-[var(--text-primary)] outline-none transition-all cursor-pointer"
					>
						<option value="">Last used</option>
						{#each servers as server}
							<option value={server.name}>{server.name} ({server.address}:{server.port})</option>
						{/each}
					</select>
				</div>
			{/if}

			<div class="bg-[var(--bg-card)] border border-[var(--border)] rounded-xl p-4 flex items-center justify-between">
				<div>
					<div class="text-sm font-medium flex items-center gap-2">
						<svg class="w-4 h-4 text-[var(--accent)]" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 21a9.004 9.004 0 008.716-6.747M12 21a9.004 9.004 0 01-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9S14.485 3 12 3m0 18c-2.485 0-4.5-4.03-4.5-9S9.515 3 12 3m0 0a8.997 8.997 0 017.843 4.582M12 3a8.997 8.997 0 00-7.843 4.582m15.686 0A11.953 11.953 0 0112 10.5c-2.998 0-5.74-1.1-7.843-2.918m15.686 0A8.959 8.959 0 0121 12c0 .778-.099 1.533-.284 2.253m0 0A17.919 17.919 0 0112 16.5c-3.162 0-6.133-.815-8.716-2.247m0 0A9.015 9.015 0 013 12c0-1.605.42-3.113 1.157-4.418" />
						</svg>
						VPN Mode
					</div>
					<div class="text-xs text-[var(--text-secondary)] mt-0.5">{tunMode ? 'All traffic through VPN' : 'Manual proxy (127.0.0.1:1080)'}</div>
				</div>
				<button
					onclick={toggleTunMode}
					class="w-12 h-7 rounded-full transition-all duration-200 cursor-pointer relative shrink-0"
					class:bg-[var(--accent)]={tunMode}
					class:shadow-[0_0_12px_var(--accent-glow)]={tunMode}
					class:bg-[var(--border)]={!tunMode}
					aria-label="Toggle VPN mode"
				>
					<div class="w-5 h-5 bg-white rounded-full absolute top-1 transition-transform duration-200 shadow-sm" class:translate-x-6={tunMode} class:translate-x-1={!tunMode}></div>
				</button>
			</div>

			<div class="bg-[var(--bg-card)] border border-[var(--border)] rounded-xl p-4 flex items-center justify-between">
				<div>
					<div class="text-sm font-medium flex items-center gap-2">
						<svg class="w-4 h-4 text-[var(--accent)]" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
							<path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
						</svg>
						Kill Switch
					</div>
					<div class="text-xs text-[var(--text-secondary)] mt-0.5">Block all traffic if VPN drops</div>
				</div>
				<button
					onclick={toggleKillSwitch}
					class="w-12 h-7 rounded-full transition-all duration-200 cursor-pointer relative shrink-0"
					class:bg-[var(--accent)]={killSwitch}
					class:shadow-[0_0_12px_var(--accent-glow)]={killSwitch}
					class:bg-[var(--border)]={!killSwitch}
					aria-label="Toggle kill switch"
				>
					<div class="w-5 h-5 bg-white rounded-full absolute top-1 transition-transform duration-200 shadow-sm" class:translate-x-6={killSwitch} class:translate-x-1={!killSwitch}></div>
				</button>
			</div>

			<div class="bg-[var(--bg-card)] border border-[var(--border)] rounded-xl p-4 flex items-center justify-between">
				<div>
					<div class="text-sm font-medium flex items-center gap-2">
						<svg class="w-4 h-4 text-[var(--accent)]" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="1.5">
							<path stroke-linecap="round" stroke-linejoin="round" d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z" />
						</svg>
						Auto-Connect
					</div>
					<div class="text-xs text-[var(--text-secondary)] mt-0.5">Connect automatically when app opens</div>
				</div>
				<button
					onclick={toggleAutoConnect}
					class="w-12 h-7 rounded-full transition-all duration-200 cursor-pointer relative shrink-0"
					class:bg-[var(--accent)]={autoConnect}
					class:shadow-[0_0_12px_var(--accent-glow)]={autoConnect}
					class:bg-[var(--border)]={!autoConnect}
					aria-label="Toggle auto-connect"
				>
					<div class="w-5 h-5 bg-white rounded-full absolute top-1 transition-transform duration-200 shadow-sm" class:translate-x-6={autoConnect} class:translate-x-1={!autoConnect}></div>
				</button>
			</div>

			{#if servers.length === 0}
				<div class="text-center py-4">
					<p class="text-sm text-[var(--text-secondary)]">No servers configured</p>
					<a href="/servers" class="text-sm text-[var(--accent)] hover:text-[var(--accent-hover)] mt-1 inline-flex items-center gap-1 transition-colors">
						Add a server
						<svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" /></svg>
					</a>
				</div>
			{/if}
		</div>
	{/if}
</div>
{/if}
