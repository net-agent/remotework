use std::path::PathBuf;
use std::sync::Mutex;
use tauri::{AppHandle, State};
use tauri_plugin_shell::ShellExt;
use tauri_plugin_shell::process::CommandChild;

pub struct SidecarState {
    pub child: Mutex<Option<CommandChild>>,
    pub config_path: Mutex<Option<PathBuf>>,
    pub api_port: Mutex<u16>,
}

impl Default for SidecarState {
    fn default() -> Self {
        Self {
            child: Mutex::new(None),
            config_path: Mutex::new(None),
            api_port: Mutex::new(0),
        }
    }
}

#[tauri::command]
pub async fn start_agent(
    app: AppHandle,
    state: State<'_, SidecarState>,
    config_state: State<'_, crate::config::ConfigState>,
    profile_name: String,
) -> Result<u16, String> {
    // If already running, return existing port
    if state.child.lock().unwrap().is_some() {
        let port = *state.api_port.lock().unwrap();
        return Ok(port);
    }

    // Read profile config
    let index = config_state.read_index();
    let meta = index
        .profiles
        .iter()
        .find(|p| p.name == profile_name)
        .ok_or_else(|| format!("profile '{}' not found", profile_name))?;
    let profile_path = config_state.profile_path(&meta.filename);
    let content = std::fs::read_to_string(&profile_path)
        .map_err(|e| format!("read profile '{}': {}", profile_path.display(), e))?;
    let mut config: serde_json::Value =
        serde_json::from_str(&content).map_err(|e| format!("parse profile: {}", e))?;

    // Ensure API is enabled and find a free port
    let port = find_available_port().ok_or("no available port")?;
    let api = config
        .as_object_mut()
        .ok_or("config is not an object")?
        .entry("api")
        .or_insert_with(|| serde_json::json!({}));
    let api_obj = api.as_object_mut().ok_or("api is not an object")?;
    api_obj.insert("enable".into(), serde_json::json!(true));
    api_obj.insert("listen".into(), serde_json::json!(format!("127.0.0.1:{}", port)));

    // Write active config
    let config_dir = dirs::config_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join("remotework");
    std::fs::create_dir_all(&config_dir)
        .map_err(|e| format!("create config dir '{}': {}", config_dir.display(), e))?;
    let active_config_path = config_dir.join("active-config.json");
    let json = serde_json::to_string_pretty(&config).map_err(|e| format!("serialize config: {}", e))?;
    std::fs::write(&active_config_path, &json)
        .map_err(|e| format!("write active config '{}': {}", active_config_path.display(), e))?;

    // Spawn sidecar
    let sidecar_cmd = app
        .shell()
        .sidecar("remotework")
        .map_err(|e| format!("create sidecar command: {}", e))?;
    let (mut rx, child) = sidecar_cmd
        .args(["-mode", "agent", "-c", active_config_path.to_str().unwrap()])
        .spawn()
        .map_err(|e| format!("spawn sidecar: {}", e))?;

    *state.child.lock().unwrap() = Some(child);
    *state.config_path.lock().unwrap() = Some(active_config_path);
    *state.api_port.lock().unwrap() = port;

    // Spawn a task to read sidecar output (prevent pipe buffer from filling)
    tauri::async_runtime::spawn(async move {
        use tauri_plugin_shell::process::CommandEvent;
        while let Some(event) = rx.recv().await {
            match event {
                CommandEvent::Stdout(line) => {
                    eprintln!("[agent stdout] {}", String::from_utf8_lossy(&line));
                }
                CommandEvent::Stderr(line) => {
                    eprintln!("[agent stderr] {}", String::from_utf8_lossy(&line));
                }
                CommandEvent::Terminated(_) => break,
                _ => {}
            }
        }
    });

    // Poll health check
    let base = format!("http://127.0.0.1:{}", port);
    let client = reqwest::Client::new();
    for _ in 0..30 {
        tokio::time::sleep(std::time::Duration::from_millis(100)).await;
        if let Ok(resp) = client.get(format!("{}/api/v1/status", base)).send().await {
            if resp.status().is_success() {
                return Ok(port);
            }
        }
    }

    Err("agent failed to start within 3 seconds".into())
}

#[tauri::command]
pub async fn stop_agent(state: State<'_, SidecarState>) -> Result<(), String> {
    let port = *state.api_port.lock().unwrap();
    if port > 0 {
        // Try graceful shutdown
        let client = reqwest::Client::new();
        let _ = client
            .post(format!("http://127.0.0.1:{}/api/v1/stop", port))
            .header("X-Confirm", "yes")
            .send()
            .await;
        tokio::time::sleep(std::time::Duration::from_secs(2)).await;
    }

    // Force kill if still alive
    if let Some(child) = state.child.lock().unwrap().take() {
        let _ = child.kill();
    }
    *state.api_port.lock().unwrap() = 0;
    Ok(())
}

#[tauri::command]
pub async fn restart_agent(
    app: AppHandle,
    state: State<'_, SidecarState>,
    config_state: State<'_, crate::config::ConfigState>,
    profile_name: String,
) -> Result<u16, String> {
    stop_agent(state.clone()).await?;
    start_agent(app, state, config_state, profile_name).await
}

#[tauri::command]
pub fn agent_running(state: State<SidecarState>) -> Result<bool, String> {
    Ok(state.child.lock().unwrap().is_some())
}

#[tauri::command]
pub fn get_agent_port(state: State<SidecarState>) -> Result<u16, String> {
    Ok(*state.api_port.lock().unwrap())
}

fn find_available_port() -> Option<u16> {
    std::net::TcpListener::bind("127.0.0.1:0")
        .ok()
        .map(|l| l.local_addr().unwrap().port())
}
