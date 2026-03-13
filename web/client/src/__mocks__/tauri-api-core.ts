export async function invoke(_cmd: string, _args?: Record<string, unknown>): Promise<unknown> {
	throw new Error('Tauri API not available in test environment');
}
