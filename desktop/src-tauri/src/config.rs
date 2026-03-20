use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;
use std::sync::Mutex;
use tauri::State;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProfileMeta {
    pub name: String,
    pub filename: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ProfilesIndex {
    pub active: String,
    pub profiles: Vec<ProfileMeta>,
}

impl Default for ProfilesIndex {
    fn default() -> Self {
        Self {
            active: String::new(),
            profiles: Vec::new(),
        }
    }
}

pub struct ConfigState {
    pub profiles_dir: Mutex<PathBuf>,
}

impl ConfigState {
    pub fn new() -> Self {
        let base = dirs::config_dir()
            .unwrap_or_else(|| PathBuf::from("."))
            .join("remotework")
            .join("profiles");
        fs::create_dir_all(&base).ok();
        Self {
            profiles_dir: Mutex::new(base),
        }
    }

    fn index_path(&self) -> PathBuf {
        self.profiles_dir
            .lock()
            .unwrap()
            .parent()
            .unwrap()
            .join("profiles.json")
    }

    pub fn read_index(&self) -> ProfilesIndex {
        let path = self.index_path();
        match fs::read_to_string(&path) {
            Ok(content) => serde_json::from_str(&content).unwrap_or_default(),
            Err(_) => ProfilesIndex::default(),
        }
    }

    fn write_index(&self, index: &ProfilesIndex) -> Result<(), String> {
        let path = self.index_path();
        let json = serde_json::to_string_pretty(index).map_err(|e| e.to_string())?;
        fs::write(path, json).map_err(|e| e.to_string())
    }

    pub fn profile_path(&self, filename: &str) -> PathBuf {
        self.profiles_dir.lock().unwrap().join(filename)
    }
}

#[tauri::command]
pub fn list_profiles(state: State<ConfigState>) -> Result<ProfilesIndex, String> {
    Ok(state.read_index())
}

#[tauri::command]
pub fn get_profile(state: State<ConfigState>, name: String) -> Result<serde_json::Value, String> {
    let index = state.read_index();
    let meta = index
        .profiles
        .iter()
        .find(|p| p.name == name)
        .ok_or_else(|| format!("profile '{}' not found", name))?;
    let path = state.profile_path(&meta.filename);
    let content = fs::read_to_string(&path).map_err(|e| e.to_string())?;
    serde_json::from_str(&content).map_err(|e| e.to_string())
}

#[tauri::command]
pub fn save_profile(
    state: State<ConfigState>,
    name: String,
    config: serde_json::Value,
) -> Result<(), String> {
    let mut index = state.read_index();
    let filename = match index.profiles.iter().find(|p| p.name == name) {
        Some(meta) => meta.filename.clone(),
        None => {
            let filename = format!("{}.json", sanitize_filename(&name));
            index.profiles.push(ProfileMeta {
                name: name.clone(),
                filename: filename.clone(),
            });
            filename
        }
    };
    let path = state.profile_path(&filename);
    let json = serde_json::to_string_pretty(&config).map_err(|e| e.to_string())?;
    fs::write(path, json).map_err(|e| e.to_string())?;
    state.write_index(&index)
}

#[tauri::command]
pub fn delete_profile(state: State<ConfigState>, name: String) -> Result<(), String> {
    let mut index = state.read_index();
    if let Some(pos) = index.profiles.iter().position(|p| p.name == name) {
        let meta = index.profiles.remove(pos);
        let path = state.profile_path(&meta.filename);
        fs::remove_file(path).ok();
        if index.active == name {
            index.active = String::new();
        }
        state.write_index(&index)?;
    }
    Ok(())
}

#[tauri::command]
pub fn rename_profile(
    state: State<ConfigState>,
    old_name: String,
    new_name: String,
) -> Result<(), String> {
    let mut index = state.read_index();
    let meta = index
        .profiles
        .iter_mut()
        .find(|p| p.name == old_name)
        .ok_or_else(|| format!("profile '{}' not found", old_name))?;
    meta.name = new_name.clone();
    if index.active == old_name {
        index.active = new_name;
    }
    state.write_index(&index)
}

#[tauri::command]
pub fn set_active_profile(state: State<ConfigState>, name: String) -> Result<(), String> {
    let mut index = state.read_index();
    if !name.is_empty() && !index.profiles.iter().any(|p| p.name == name) {
        return Err(format!("profile '{}' not found", name));
    }
    index.active = name;
    state.write_index(&index)
}

#[tauri::command]
pub fn import_profile(
    state: State<ConfigState>,
    name: String,
    content: String,
) -> Result<(), String> {
    let cleaned: String = content
        .lines()
        .map(|line| {
            let trimmed = line.trim_start();
            if trimmed.starts_with("//") {
                ""
            } else {
                line
            }
        })
        .collect::<Vec<_>>()
        .join("\n");

    let config: serde_json::Value =
        serde_json::from_str(&cleaned).map_err(|e| format!("invalid JSON: {}", e))?;

    let mut index = state.read_index();
    let filename = format!("{}.json", sanitize_filename(&name));
    index.profiles.push(ProfileMeta {
        name: name.clone(),
        filename: filename.clone(),
    });
    let path = state.profile_path(&filename);
    let json = serde_json::to_string_pretty(&config).map_err(|e| e.to_string())?;
    fs::write(path, json).map_err(|e| e.to_string())?;
    state.write_index(&index)
}

#[tauri::command]
pub fn export_profile(state: State<ConfigState>, name: String) -> Result<String, String> {
    let index = state.read_index();
    let meta = index
        .profiles
        .iter()
        .find(|p| p.name == name)
        .ok_or_else(|| format!("profile '{}' not found", name))?;
    let path = state.profile_path(&meta.filename);
    fs::read_to_string(&path).map_err(|e| e.to_string())
}

#[tauri::command]
pub fn get_network_interfaces() -> Result<Vec<String>, String> {
    let mut addrs: Vec<String> = vec!["0.0.0.0".into(), "127.0.0.1".into()];
    let ifaces = if_addrs::get_if_addrs().map_err(|e| e.to_string())?;
    for iface in ifaces {
        let ip = iface.addr.ip().to_string();
        if !addrs.contains(&ip) {
            addrs.push(ip);
        }
    }
    Ok(addrs)
}

#[tauri::command]
pub fn get_windows_rdp_port() -> Result<Option<u16>, String> {
    #[cfg(target_os = "windows")]
    {
        use winreg::enums::HKEY_LOCAL_MACHINE;
        use winreg::RegKey;

        let hklm = RegKey::predef(HKEY_LOCAL_MACHINE);
        let path = "SYSTEM\\CurrentControlSet\\Control\\Terminal Server\\WinStations\\RDP-Tcp";
        let key = hklm.open_subkey(path).map_err(|e| e.to_string())?;
        let port: u32 = key.get_value("PortNumber").map_err(|e| e.to_string())?;
        return Ok(u16::try_from(port).ok());
    }

    #[cfg(not(target_os = "windows"))]
    {
        Ok(None)
    }
}

fn sanitize_filename(name: &str) -> String {
    name.chars()
        .map(|c| if c.is_alphanumeric() || c == '-' || c == '_' { c } else { '_' })
        .collect()
}
