use std::fs;
use std::path::PathBuf;
use std::sync::Mutex;

use serde::{Deserialize, Serialize};
use tauri::Manager;

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
                backend_url: "http://127.0.0.1:8080".to_string(),
            });
        Self {
            config: Mutex::new(config),
            path,
        }
    }

    fn save(&self) -> Result<(), String> {
        let config = self.config.lock().map_err(|_| "lock poisoned".to_string())?;
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
fn set_backend_url(state: tauri::State<'_, DesktopState>, backend_url: String) -> Result<(), String> {
    {
        let mut config = state
            .config
            .lock()
            .map_err(|_| "lock poisoned".to_string())?;
        config.backend_url = backend_url;
    }
    state.save()
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_dialog::init())
        .setup(|app| {
            let state = DesktopState::load(&app.handle());
            app.manage(state);
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![get_desktop_config, set_backend_url])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
