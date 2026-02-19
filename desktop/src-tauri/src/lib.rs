mod config;
mod sidecar;
mod tray;

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_process::init())
        .manage(sidecar::SidecarState::default())
        .manage(config::ConfigState::new())
        .setup(|app| {
            tray::create_tray(app)?;
            Ok(())
        })
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                api.prevent_close();
                let _ = window.hide();
            }
        })
        .invoke_handler(tauri::generate_handler![
            sidecar::start_agent,
            sidecar::stop_agent,
            sidecar::restart_agent,
            sidecar::agent_running,
            config::list_profiles,
            config::get_profile,
            config::save_profile,
            config::delete_profile,
            config::rename_profile,
            config::set_active_profile,
            config::import_profile,
            config::export_profile,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
