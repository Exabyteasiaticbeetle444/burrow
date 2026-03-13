use tauri::Manager;
use tauri_plugin_shell::ShellExt;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_process::init())
        .setup(|app| {
            let window = app.get_webview_window("main").unwrap();
            window.set_title("Burrow VPN").unwrap();

            // Auto-start the daemon sidecar
            let shell = app.shell();
            let sidecar = shell.sidecar("burrow-vpn").unwrap();
            let (mut _rx, _child) = sidecar
                .args(["daemon"])
                .spawn()
                .expect("failed to start burrow daemon");

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
