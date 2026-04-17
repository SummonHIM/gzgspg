mod portal;
mod worker;
use worker::{get_worker_status, start_worker, stop_worker, WorkerManager};
use tauri::{
    menu::{CheckMenuItemBuilder, MenuBuilder, MenuItemBuilder},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    Manager, WindowEvent,
};
use tauri_plugin_autostart::{MacosLauncher, ManagerExt as AutostartExt};

fn show_main_window(app: &tauri::AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.show();
        let _ = window.unminimize();
        let _ = window.set_focus();
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .manage(WorkerManager::default())
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_single_instance::init(|app, _argv, _cwd| {
            show_main_window(app);
        }))
        .plugin(tauri_plugin_autostart::init(
            MacosLauncher::LaunchAgent,
            Some(vec!["--autostart"]),
        ))
        .invoke_handler(tauri::generate_handler![
            start_worker,
            stop_worker,
            get_worker_status
        ])
        .setup(|app| {
            // 如果是开机自启启动，可以选择默认隐藏
            let is_autostart = std::env::args().any(|arg| arg == "--autostart");
            if is_autostart {
                if let Some(window) = app.get_webview_window("main") {
                    let _ = window.hide();
                }
            }
            let autostart_enabled = app.autolaunch().is_enabled().unwrap_or(false);
            let autostart_item = CheckMenuItemBuilder::with_id("autostart", "开机自启")
                .checked(autostart_enabled)
                .build(app)?;
            let quit_item = MenuItemBuilder::with_id("quit", "退出").build(app)?;
            let menu = MenuBuilder::new(app)
                .items(&[&autostart_item, &quit_item])
                .build()?;
            let autostart_item_handle = autostart_item.clone();
            let mut tray_builder = TrayIconBuilder::with_id("main-tray")
                .tooltip("gzgspg")
                .menu(&menu)
                .show_menu_on_left_click(false)
                // 左键点击托盘图标：恢复主窗口
                .on_tray_icon_event(|tray, event| {
                    if let TrayIconEvent::Click {
                        button: MouseButton::Left,
                        button_state: MouseButtonState::Up,
                        ..
                    } = event
                    {
                        show_main_window(&tray.app_handle());
                    }
                })
                // 右键菜单事件
                .on_menu_event(move |app, event| match event.id().as_ref() {
                    "quit" => {
                        app.exit(0);
                    }
                    "autostart" => {
                        let enabled = app.autolaunch().is_enabled().unwrap_or(false);
                        let result = if enabled {
                            app.autolaunch().disable()
                        } else {
                            app.autolaunch().enable()
                        };
                        if result.is_ok() {
                            let _ = autostart_item_handle.set_checked(!enabled);
                        }
                    }
                    _ => {}
                });
            if let Some(icon) = app.default_window_icon() {
                tray_builder = tray_builder.icon(icon.clone());
            }
            let _tray = tray_builder.build(app)?;
            Ok(())
        })
        .on_window_event(|window, event| {
            if let WindowEvent::CloseRequested { api, .. } = event {
                if window.label() == "main" {
                    api.prevent_close();
                    let _ = window.hide();
                }
            }
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
