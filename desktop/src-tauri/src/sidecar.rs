use std::collections::VecDeque;
use std::fmt::Write as _;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;
use std::time::Duration;
use tauri::{AppHandle, Emitter, Manager, State};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;
use tokio::sync::{oneshot, Notify};

// 协议约定见 docs/tauri-sidecar-protocol.md：
// C1 参数/env、C3 stdout 行协议（ready/fatal 控制消息 + slog JSON 日志）、C5 退出码。
const SUPPORTED_PROTOCOL: i64 = 1;
const HANDSHAKE_TIMEOUT: Duration = Duration::from_secs(5);
const STOP_TIMEOUT: Duration = Duration::from_secs(5);
// 强杀后留给 reader 任务处理 Terminated 的宽限，保证 sidecar-exited 的 intended 标记先于 stopping 复位被读取
const KILL_SETTLE_TIMEOUT: Duration = Duration::from_secs(2);
const STDERR_TAIL_LINES: usize = 10;

pub struct SidecarState {
    pub child: Mutex<Option<CommandChild>>,
    pub config_path: Mutex<Option<PathBuf>>,
    pub api_port: Mutex<u16>,
    pub api_token: Mutex<String>,
    /// stderr 最近若干行尾部缓冲，用于启动失败时向用户呈现原因
    pub stderr_tail: Mutex<VecDeque<String>>,
    /// 进程 Terminated 信号，stop_agent 以此等待优雅退出（notify_one 语义，无等待者时留存 permit）
    pub exit_notify: Notify,
    /// stop_agent 发起的停止标记，Terminated 时用于区分正常停止与崩溃
    pub stopping: AtomicBool,
}

impl Default for SidecarState {
    fn default() -> Self {
        Self {
            child: Mutex::new(None),
            config_path: Mutex::new(None),
            api_port: Mutex::new(0),
            api_token: Mutex::new(String::new()),
            stderr_tail: Mutex::new(VecDeque::new()),
            exit_notify: Notify::new(),
            stopping: AtomicBool::new(false),
        }
    }
}

/// 启动握手结果，由输出读取任务经 oneshot 恰好发送一次
enum HandshakeMsg {
    Ready { api: String, protocol: i64 },
    Fatal { stage: String, error: String },
    Exited { code: Option<i32> },
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

    // C2 注入白名单：api.enable=true，api.listen=127.0.0.1:0（端口由 sidecar 自选，ready 行回报）
    let api = config
        .as_object_mut()
        .ok_or("config is not an object")?
        .entry("api")
        .or_insert_with(|| serde_json::json!({}));
    let api_obj = api.as_object_mut().ok_or("api is not an object")?;
    api_obj.insert("enable".into(), serde_json::json!(true));
    api_obj.insert("listen".into(), serde_json::json!("127.0.0.1:0"));

    // Write active config
    let config_dir = dirs::config_dir()
        .unwrap_or_else(|| PathBuf::from("."))
        .join("remotework");
    std::fs::create_dir_all(&config_dir)
        .map_err(|e| format!("create config dir '{}': {}", config_dir.display(), e))?;
    let active_config_path = config_dir.join("active-config.json");
    let json =
        serde_json::to_string_pretty(&config).map_err(|e| format!("serialize config: {}", e))?;
    std::fs::write(&active_config_path, &json).map_err(|e| {
        format!(
            "write active config '{}': {}",
            active_config_path.display(),
            e
        )
    })?;

    // Validate config before spawning agent; any non-zero exit is a hard failure
    let validate_output = app
        .shell()
        .sidecar("remotework")
        .map_err(|e| format!("create sidecar command: {}", e))?
        .args([
            "-mode",
            "validate",
            "-c",
            active_config_path.to_str().unwrap(),
        ])
        .output()
        .await
        .map_err(|e| format!("run config validation: {}", e))?;
    if validate_output.status.code() != Some(0) {
        let stderr = String::from_utf8_lossy(&validate_output.stderr);
        return Err(format!("配置验证失败: {}", stderr.trim()));
    }

    // C1：每次 spawn 重新生成 token，仅经 env 传递（不落盘、不进命令行）
    let token = generate_api_token();
    state.stopping.store(false, Ordering::SeqCst);
    state.stderr_tail.lock().unwrap().clear();

    // Spawn sidecar
    let sidecar_cmd = app
        .shell()
        .sidecar("remotework")
        .map_err(|e| format!("create sidecar command: {}", e))?;
    let (rx, child) = sidecar_cmd
        .args([
            "-mode",
            "agent",
            "-c",
            active_config_path.to_str().unwrap(),
            "-log-format=json",
        ])
        .env("REMOTEWORK_API_TOKEN", &token)
        .spawn()
        .map_err(|e| format!("spawn sidecar: {}", e))?;

    *state.child.lock().unwrap() = Some(child);
    *state.config_path.lock().unwrap() = Some(active_config_path);
    *state.api_token.lock().unwrap() = token;

    // Spawn the output reader task: parses C3 lines, drives the handshake oneshot,
    // forwards structured sidecar-log events, and owns Terminated cleanup.
    let (hs_tx, hs_rx) = oneshot::channel::<HandshakeMsg>();
    let app_handle = app.clone();
    tauri::async_runtime::spawn(async move {
        run_output_reader(rx, hs_tx, app_handle).await;
    });

    // C3 握手：5 秒内 ready / fatal / 进程退出 三选一，不再轮询 /api/v1/status
    match tokio::time::timeout(HANDSHAKE_TIMEOUT, hs_rx).await {
        Ok(Ok(HandshakeMsg::Ready { api, protocol })) => {
            if protocol > SUPPORTED_PROTOCOL {
                kill_child(&state);
                return Err(format!(
                    "sidecar protocol {} is newer than supported version {}; please upgrade the app",
                    protocol, SUPPORTED_PROTOCOL
                ));
            }
            match parse_api_port(&api) {
                Some(port) => {
                    *state.api_port.lock().unwrap() = port;
                    Ok(port)
                }
                None => {
                    kill_child(&state);
                    Err(format!("agent reported invalid api address '{}'", api))
                }
            }
        }
        Ok(Ok(HandshakeMsg::Fatal { stage, error })) => {
            kill_child(&state);
            Err(format!(
                "agent startup failed ({}): {}{}",
                stage,
                error,
                stderr_tail_suffix(&state)
            ))
        }
        Ok(Ok(HandshakeMsg::Exited { code })) => Err(format!(
            "agent exited during startup, exit code {:?}{}",
            code,
            stderr_tail_suffix(&state)
        )),
        Ok(Err(_)) => Err("agent startup handshake channel closed unexpectedly".into()),
        Err(_) => {
            kill_child(&state);
            Err("agent failed to start within 5 seconds".into())
        }
    }
}

#[tauri::command]
pub async fn stop_agent(state: State<'_, SidecarState>) -> Result<(), String> {
    if state.child.lock().unwrap().is_none() {
        *state.api_port.lock().unwrap() = 0;
        state.api_token.lock().unwrap().clear();
        return Ok(());
    }

    state.stopping.store(true, Ordering::SeqCst);

    let (port, token) = {
        let port = *state.api_port.lock().unwrap();
        let token = state.api_token.lock().unwrap().clone();
        (port, token)
    };
    if port > 0 {
        // C4 优雅停止：Bearer token 鉴权
        let client = reqwest::Client::new();
        let _ = client
            .post(format!("http://127.0.0.1:{}/api/v1/stop", port))
            .bearer_auth(&token)
            .send()
            .await;
    }

    // 等待进程 Terminated（D3：禁止固定 sleep 作为停止确认），超时强杀
    if !wait_for_exit(&state, STOP_TIMEOUT).await {
        kill_child(&state);
        // 给 reader 任务留出处理 Terminated 的时间，使 sidecar-exited 的 intended 标记读取到 stopping=true
        let _ = wait_for_exit(&state, KILL_SETTLE_TIMEOUT).await;
    }

    *state.api_port.lock().unwrap() = 0;
    state.api_token.lock().unwrap().clear();
    state.stopping.store(false, Ordering::SeqCst);
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

#[derive(serde::Serialize)]
pub struct ApiEndpoint {
    pub port: u16,
    pub token: String,
}

#[tauri::command]
pub fn get_api_endpoint(state: State<SidecarState>) -> Result<ApiEndpoint, String> {
    if state.child.lock().unwrap().is_none() {
        return Err("agent is not running".into());
    }
    let port = *state.api_port.lock().unwrap();
    let token = state.api_token.lock().unwrap().clone();
    if port == 0 || token.is_empty() {
        return Err("agent is not running".into());
    }
    Ok(ApiEndpoint { port, token })
}

/// 输出读取任务：sidecar stdout/stderr 的第一消费者，常驻至进程 Terminated。
async fn run_output_reader(
    mut rx: tauri::async_runtime::Receiver<CommandEvent>,
    hs_tx: oneshot::Sender<HandshakeMsg>,
    app_handle: AppHandle,
) {
    let mut hs_tx = Some(hs_tx);
    while let Some(event) = rx.recv().await {
        match event {
            CommandEvent::Stdout(line) => {
                let text = String::from_utf8_lossy(&line).trim_end().to_string();
                eprintln!("[agent stdout] {}", &text);
                let (control, payload) = parse_stdout_line(&text);
                let _ = app_handle.emit("sidecar-log", payload);
                if let Some(control) = control {
                    // oneshot 只发一次：ready/fatal 先到即定案，此后 control 为空
                    if let Some(tx) = hs_tx.take() {
                        let _ = tx.send(control);
                    }
                }
            }
            CommandEvent::Stderr(line) => {
                let text = String::from_utf8_lossy(&line).trim_end().to_string();
                eprintln!("[agent stderr] {}", &text);
                push_stderr_tail(&app_handle, &text);
                let _ = app_handle.emit(
                    "sidecar-log",
                    log_payload("stderr", "error", None, None, &text),
                );
            }
            CommandEvent::Error(err) => {
                eprintln!("[agent error] {}", &err);
                push_stderr_tail(&app_handle, &err);
                let _ = app_handle.emit(
                    "sidecar-log",
                    log_payload("stderr", "error", None, None, &err),
                );
            }
            CommandEvent::Terminated(status) => {
                let app_state = app_handle.state::<SidecarState>();
                *app_state.child.lock().unwrap() = None;
                *app_state.api_port.lock().unwrap() = 0;
                app_state.api_token.lock().unwrap().clear();
                // 握手尚未定案时进程先退出：按启动失败处理
                if let Some(tx) = hs_tx.take() {
                    let _ = tx.send(HandshakeMsg::Exited { code: status.code });
                }
                // emit 必须先于 notify：stop_agent 被唤醒后会复位 stopping
                let intended = app_state.stopping.load(Ordering::SeqCst);
                let _ = app_handle.emit(
                    "sidecar-exited",
                    serde_json::json!({ "code": status.code, "intended": intended }),
                );
                app_state.exit_notify.notify_one();
                break;
            }
            _ => {}
        }
    }
}

/// 解析一行 stdout：ready/fatal 为控制消息，其余按 slog JSON 日志或原文转发。
fn parse_stdout_line(text: &str) -> (Option<HandshakeMsg>, serde_json::Value) {
    let parsed: Option<serde_json::Value> = serde_json::from_str(text).ok();
    let obj = parsed.as_ref().and_then(|v| v.as_object());
    let Some(obj) = obj else {
        // 非 JSON 行（如 panic 堆栈）按原文转发
        return (None, log_payload("stdout", "error", None, None, text));
    };

    match obj.get("type").and_then(|t| t.as_str()) {
        Some("ready") => {
            let api = obj
                .get("api")
                .and_then(|a| a.as_str())
                .unwrap_or_default()
                .to_string();
            let protocol = obj.get("protocol").and_then(|p| p.as_i64()).unwrap_or(0);
            let mode = obj.get("mode").and_then(|m| m.as_str()).unwrap_or("");
            let version = obj.get("version").and_then(|v| v.as_str()).unwrap_or("");
            let msg = format!("agent ready: api={} mode={} version={}", api, mode, version);
            (
                Some(HandshakeMsg::Ready { api, protocol }),
                log_payload("stdout", "info", None, Some(&msg), text),
            )
        }
        Some("fatal") => {
            let stage = obj
                .get("stage")
                .and_then(|s| s.as_str())
                .unwrap_or_default()
                .to_string();
            let error = obj
                .get("error")
                .and_then(|e| e.as_str())
                .unwrap_or_default()
                .to_string();
            (
                Some(HandshakeMsg::Fatal {
                    stage,
                    error: error.clone(),
                }),
                log_payload("stdout", "error", None, Some(&error), text),
            )
        }
        _ => {
            // slog JSON 日志记录
            let level = obj
                .get("level")
                .and_then(|l| l.as_str())
                .map(|l| l.to_lowercase())
                .unwrap_or_else(|| "info".into());
            let msg = obj.get("msg").and_then(|m| m.as_str());
            let module = obj.get("module").and_then(|m| m.as_str());
            (None, log_payload("stdout", &level, module, msg, text))
        }
    }
}

fn log_payload(
    source: &str,
    level: &str,
    module: Option<&str>,
    msg: Option<&str>,
    line: &str,
) -> serde_json::Value {
    let mut payload = serde_json::json!({
        "source": source,
        "level": level,
        "line": line,
    });
    if let Some(module) = module {
        payload["module"] = module.into();
    }
    if let Some(msg) = msg {
        payload["msg"] = msg.into();
    }
    payload
}

fn push_stderr_tail(app_handle: &AppHandle, text: &str) {
    let app_state = app_handle.state::<SidecarState>();
    let mut tail = app_state.stderr_tail.lock().unwrap();
    if tail.len() >= STDERR_TAIL_LINES {
        tail.pop_front();
    }
    tail.push_back(text.to_string());
}

fn stderr_tail_suffix(state: &SidecarState) -> String {
    let tail = state.stderr_tail.lock().unwrap();
    if tail.is_empty() {
        String::new()
    } else {
        format!(
            "\nstderr tail:\n{}",
            tail.iter().cloned().collect::<Vec<_>>().join("\n")
        )
    }
}

/// ready 消息的 api 字段形如 127.0.0.1:54321，取最后一个 ':' 后的端口
fn parse_api_port(api: &str) -> Option<u16> {
    api.rsplit(':').next()?.parse::<u16>().ok()
}

fn generate_api_token() -> String {
    let bytes: [u8; 32] = rand::random();
    let mut token = String::with_capacity(64);
    for b in bytes {
        let _ = write!(token, "{:02x}", b);
    }
    token
}

fn kill_child(state: &SidecarState) {
    if let Some(child) = state.child.lock().unwrap().take() {
        let _ = child.kill();
    }
}

/// 等待进程 Terminated（reader 任务会先将 child 置 None 再 notify）。
/// 醒来先复查 child 状态，陈旧的 permit 只会多转一轮，不会误判退出。
async fn wait_for_exit(state: &SidecarState, timeout: Duration) -> bool {
    let deadline = tokio::time::Instant::now() + timeout;
    loop {
        if state.child.lock().unwrap().is_none() {
            return true;
        }
        let now = tokio::time::Instant::now();
        if now >= deadline {
            return false;
        }
        let _ = tokio::time::timeout(deadline - now, state.exit_notify.notified()).await;
    }
}
