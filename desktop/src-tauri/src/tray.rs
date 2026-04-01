use tauri::{
    App, Emitter, Listener, Manager,
    menu::{Menu, MenuItem},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
};

pub fn create_tray(app: &App) -> Result<(), Box<dyn std::error::Error>> {
    let show = MenuItem::with_id(app, "show", "显示窗口", true, None::<&str>)?;
    let toggle_mode = MenuItem::with_id(app, "toggle_mode", "进入高级模式", true, None::<&str>)?;
    let quit = MenuItem::with_id(app, "quit", "退出", true, None::<&str>)?;
    let menu = Menu::with_items(app, &[&show, &toggle_mode, &quit])?;

    let toggle_mode_for_label = toggle_mode.clone();
    app.listen("ui-mode-changed", move |event| {
        let payload = event.payload();

        if let Ok(mode) = serde_json::from_str::<String>(payload) {
            let next_label = if mode == "advanced" {
                "进入普通模式"
            } else {
                "进入高级模式"
            };
            let _ = toggle_mode_for_label.set_text(next_label);
        }
    });

    TrayIconBuilder::new()
        .icon(tauri::include_image!("icons/tray-icon.png"))
        .menu(&menu)
        .tooltip("Remotework")
        .on_menu_event(|app, event| match event.id.as_ref() {
            "show" => {
                if let Some(window) = app.get_webview_window("main") {
                    let _ = window.show();
                    let _ = window.set_focus();
                }
            }
            "toggle_mode" => {
                let _ = app.emit("tray-toggle-ui-mode", ());
                if let Some(window) = app.get_webview_window("main") {
                    let _ = window.show();
                    let _ = window.set_focus();
                }
            }
            "quit" => {
                app.exit(0);
            }
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                let app = tray.app_handle();
                if let Some(window) = app.get_webview_window("main") {
                    let _ = window.show();
                    let _ = window.set_focus();
                }
            }
        })
        .build(app)?;

    Ok(())
}
