use std::sync::Mutex;
use std::thread;
use std::time::Duration;

use tauri::menu::{MenuBuilder, MenuItemBuilder};
use tauri::tray::TrayIconEvent;
use tauri::{Manager, RunEvent, WindowEvent};
use tauri_plugin_deep_link::DeepLinkExt;
use tauri_plugin_shell::process::CommandChild;
use tauri_plugin_shell::ShellExt;

const MAX_INVITE_LEN: usize = 4096;

struct DaemonChild(Mutex<Option<CommandChild>>);

fn show_window(app: &tauri::AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.show();
        let _ = window.unminimize();
        let _ = window.set_focus();
    }
}

fn kill_daemon(app: &tauri::AppHandle) {
    if let Some(state) = app.try_state::<DaemonChild>() {
        if let Ok(mut guard) = state.0.lock() {
            if let Some(child) = guard.take() {
                let _ = child.kill();
            }
        }
    }
}

fn handle_deep_link_url(app: &tauri::AppHandle, url: &str) {
    let prefix = "burrow://invite/";
    if let Some(invite_data) = url.strip_prefix(prefix) {
        let invite_data = invite_data.trim_end_matches('/');
        if invite_data.is_empty() || invite_data.len() > MAX_INVITE_LEN {
            return;
        }
        let payload = serde_json::json!({ "invite": invite_data });
        thread::spawn(move || {
            let client = reqwest::blocking::Client::builder()
                .timeout(std::time::Duration::from_secs(10))
                .build()
                .unwrap_or_default();
            let _ = client
                .post("http://127.0.0.1:9090/api/servers")
                .json(&payload)
                .send();
        });
        show_window(app);
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let app = tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_deep_link::init())
        .setup(|app| {
            let window = app.get_webview_window("main").unwrap();
            window.set_title("Burrow VPN").unwrap();

            let shell = app.shell();
            let sidecar = shell.sidecar("burrow-vpn").unwrap();
            let (mut _rx, child) = sidecar
                .args(["daemon"])
                .spawn()
                .expect("failed to start burrow daemon");

            app.manage(DaemonChild(Mutex::new(Some(child))));

            let handle = app.handle().clone();
            app.deep_link().on_open_url(move |event| {
                for url in event.urls() {
                    handle_deep_link_url(&handle, url.as_str());
                }
            });

            let show = MenuItemBuilder::with_id("show", "Show").build(app)?;
            let connect = MenuItemBuilder::with_id("connect", "Connect").build(app)?;
            let disconnect = MenuItemBuilder::with_id("disconnect", "Disconnect").build(app)?;
            let quit = MenuItemBuilder::with_id("quit", "Quit").build(app)?;

            let menu = MenuBuilder::new(app)
                .item(&show)
                .separator()
                .item(&connect)
                .item(&disconnect)
                .separator()
                .item(&quit)
                .build()?;

            let tray = app.tray_by_id("main").expect("no tray icon found");
            tray.set_menu(Some(menu))?;

            tray.on_menu_event(move |app, event| match event.id().as_ref() {
                "show" => {
                    show_window(app);
                }
                "connect" => {
                    thread::spawn(|| {
                        let client = reqwest::blocking::Client::builder()
                            .timeout(std::time::Duration::from_secs(10))
                            .build()
                            .unwrap_or_default();
                        let prefs: serde_json::Value = client
                            .get("http://127.0.0.1:9090/api/preferences")
                            .send()
                            .and_then(|r| r.json())
                            .unwrap_or(serde_json::json!({}));
                        let _ = client
                            .post("http://127.0.0.1:9090/api/connect")
                            .json(&serde_json::json!({
                                "tun_mode": prefs.get("tun_mode").and_then(|v| v.as_bool()).unwrap_or(true),
                                "kill_switch": prefs.get("kill_switch").and_then(|v| v.as_bool()).unwrap_or(false),
                            }))
                            .send();
                    });
                }
                "disconnect" => {
                    thread::spawn(|| {
                        let client = reqwest::blocking::Client::builder()
                            .timeout(std::time::Duration::from_secs(10))
                            .build()
                            .unwrap_or_default();
                        let _ = client
                            .post("http://127.0.0.1:9090/api/disconnect")
                            .send();
                    });
                }
                "quit" => {
                    kill_daemon(app);
                    app.exit(0);
                }
                _ => {}
            });

            tray.on_tray_icon_event(|tray, event| {
                if let TrayIconEvent::Click { .. } = event {
                    show_window(tray.app_handle());
                }
            });

            let tray_handle = app.tray_by_id("main").unwrap();
            thread::spawn(move || {
                let client = reqwest::blocking::Client::builder()
                    .timeout(Duration::from_secs(3))
                    .build()
                    .unwrap_or_default();
                let mut was_connected = false;
                loop {
                    thread::sleep(Duration::from_secs(3));
                    let connected = client
                        .get("http://127.0.0.1:9090/api/status")
                        .send()
                        .and_then(|r| r.json::<serde_json::Value>())
                        .map(|v| v.get("running").and_then(|r| r.as_bool()).unwrap_or(false))
                        .unwrap_or(false);
                    if connected != was_connected {
                        let tooltip = if connected {
                            "Burrow VPN — Connected"
                        } else {
                            "Burrow VPN — Disconnected"
                        };
                        let _ = tray_handle.set_tooltip(Some(tooltip));
                        was_connected = connected;
                    }
                }
            });

            let window_clone = window.clone();
            window.on_window_event(move |event| {
                if let WindowEvent::CloseRequested { api, .. } = event {
                    api.prevent_close();
                    let _ = window_clone.hide();
                }
            });

            Ok(())
        })
        .build(tauri::generate_context!())
        .expect("error while building tauri application");

    app.run(|app_handle, event| {
        if let RunEvent::ExitRequested { .. } = &event {
            kill_daemon(app_handle);
        }
    });
}
