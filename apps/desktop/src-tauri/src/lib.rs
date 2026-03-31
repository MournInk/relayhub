use std::fs;
use std::path::PathBuf;
use std::sync::Mutex;

use serde::{Deserialize, Serialize};
use tauri::{Emitter, Manager};
use tauri_plugin_shell::process::CommandEvent;
use tauri_plugin_shell::ShellExt;

const DEFAULT_BACKEND_URL: &str = "http://127.0.0.1:4317";
const DEFAULT_LISTEN_ADDR: &str = "127.0.0.1:4317";
const DEFAULT_ADMIN_TOKEN: &str = "relayhub-admin";

#[derive(Default, Serialize, Deserialize, Clone)]
struct DesktopConfig {
    backend_url: String,
}

struct DesktopState {
    config: Mutex<DesktopConfig>,
    path: PathBuf,
}

impl DesktopState {
    fn load(app: &tauri::AppHandle) -> Self {
        let app_dir = app
            .path()
            .app_config_dir()
            .expect("failed to resolve app config dir");
        fs::create_dir_all(&app_dir).ok();
        let path = app_dir.join("desktop-config.json");
        let config = fs::read_to_string(&path)
            .ok()
            .and_then(|raw| serde_json::from_str::<DesktopConfig>(&raw).ok())
            .unwrap_or(DesktopConfig {
                backend_url: DEFAULT_BACKEND_URL.to_string(),
            });
        Self {
            config: Mutex::new(config),
            path,
        }
    }

    fn save(&self) -> Result<(), String> {
        let config = self
            .config
            .lock()
            .map_err(|_| "lock poisoned".to_string())?;
        let raw = serde_json::to_string_pretty(&*config).map_err(|err| err.to_string())?;
        fs::write(&self.path, raw).map_err(|err| err.to_string())
    }
}

#[tauri::command]
fn get_desktop_config(state: tauri::State<'_, DesktopState>) -> Result<DesktopConfig, String> {
    state
        .config
        .lock()
        .map(|config| config.clone())
        .map_err(|_| "lock poisoned".to_string())
}

#[tauri::command]
fn set_backend_url(
    state: tauri::State<'_, DesktopState>,
    backend_url: String,
) -> Result<(), String> {
    {
        let mut config = state
            .config
            .lock()
            .map_err(|_| "lock poisoned".to_string())?;
        config.backend_url = backend_url;
    }
    state.save()
}

fn start_sidecar(app: &tauri::AppHandle) -> Result<(), String> {
    let config_dir = app
        .path()
        .app_config_dir()
        .map_err(|err| err.to_string())?;
    let data_dir = app.path().app_data_dir().map_err(|err| err.to_string())?;
    fs::create_dir_all(&config_dir).map_err(|err| err.to_string())?;
    fs::create_dir_all(&data_dir).map_err(|err| err.to_string())?;

    let config_path = config_dir.join("relayhub.json");
    let db_path = data_dir.join("relayhub.db");

    let args = vec![
        "--config".to_string(),
        config_path.to_string_lossy().into_owned(),
        "--listen".to_string(),
        DEFAULT_LISTEN_ADDR.to_string(),
        "--admin-token".to_string(),
        DEFAULT_ADMIN_TOKEN.to_string(),
        "--database".to_string(),
        db_path.to_string_lossy().into_owned(),
        "--instance-name".to_string(),
        "RelayHub Desktop".to_string(),
    ];

    let sidecar_command = app
        .shell()
        .sidecar("relayhub-core")
        .map_err(|err| err.to_string())?
        .args(args);

    let (mut rx, child) = sidecar_command.spawn().map_err(|err| err.to_string())?;
    let app_handle = app.clone();
    tauri::async_runtime::spawn(async move {
        let _sidecar = child;
        while let Some(event) = rx.recv().await {
            match event {
                CommandEvent::Stdout(line) => {
                    let text = String::from_utf8_lossy(&line).to_string();
                    let _ = app_handle.emit("relayhub://backend-log", text);
                }
                CommandEvent::Stderr(line) => {
                    let text = String::from_utf8_lossy(&line).to_string();
                    let _ = app_handle.emit("relayhub://backend-error", text);
                }
                CommandEvent::Error(message) => {
                    let _ = app_handle.emit("relayhub://backend-error", message);
                }
                CommandEvent::Terminated(payload) => {
                    let _ = app_handle.emit(
                        "relayhub://backend-exit",
                        format!("sidecar terminated with {:?}", payload),
                    );
                }
                _ => {}
            }
        }
    });

    Ok(())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_shell::init())
        .setup(|app| {
            let state = DesktopState::load(&app.handle());
            app.manage(state);
            start_sidecar(&app.handle())?;
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![get_desktop_config, set_backend_url])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
