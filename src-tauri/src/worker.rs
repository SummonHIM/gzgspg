use crate::portal::{
    telecom_portal_checker, telecom_portal_json_action, telecom_quick_auth,
    telecom_quick_auth_disconn,
};
use serde::{Deserialize, Serialize};
use std::sync::Arc;
use tauri::{AppHandle, Emitter, State};
use tokio::sync::{watch, Mutex};
use tokio::time::{sleep, Duration};
use url::Url;

const DEFAULT_USER_AGENT: &str =
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36";
const DEFAULT_KALIVE_LINK: &str = "http://3.3.3.3";
const DEFAULT_KEEP_ALIVE_SECS: u64 = 60;
const DEFAULT_RETRY_MAX: u32 = 5;
const DEFAULT_PAUSE_SECS: u64 = 600;

// 登出时的兜底值
const DEFAULT_LOGOUT_SCHEME: &str = "https";
const DEFAULT_LOGOUT_HOST: &str = "10.20.16.5";
const DEFAULT_LOGOUT_WLANACIP: &str = "10.20.16.2";
const DEFAULT_LOGOUT_WLANACNAME: &str = "NFV-BASE-01";
const DEFAULT_LOGOUT_VERSION: i32 = 4;
const DEFAULT_LOGOUT_GROUP_ID: i32 = 19;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
pub enum WorkerStatus {
    Starting,
    NotLoggedIn,
    LoggingIn,
    LoggedIn,
    Paused,
    LoggingOut,
    Stopped,
}

impl WorkerStatus {
    fn is_running(self) -> bool {
        !matches!(self, WorkerStatus::Stopped)
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct WorkerStatusPayload {
    pub running: bool,
    pub status: WorkerStatus,
    pub message: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct StartWorkerResponse {
    pub started: bool,
    pub already_running: bool,
    pub state: WorkerStatusPayload,
}

#[derive(Debug)]
struct RunningWorker {
    stop_tx: watch::Sender<bool>,
    join_handle: tauri::async_runtime::JoinHandle<()>,
}

#[derive(Debug)]
struct WorkerInner {
    worker: Mutex<Option<RunningWorker>>,
    state: Mutex<WorkerStatusPayload>,
}

#[derive(Clone)]
pub struct WorkerManager {
    inner: Arc<WorkerInner>,
}

impl Default for WorkerManager {
    fn default() -> Self {
        Self {
            inner: Arc::new(WorkerInner {
                worker: Mutex::new(None),
                state: Mutex::new(WorkerStatusPayload {
                    running: false,
                    status: WorkerStatus::Stopped,
                    message: Some("尚未启动".to_string()),
                }),
            }),
        }
    }
}

#[derive(Debug, Clone)]
struct WorkerConfig {
    username: String,
    password: String,
    user_agent: String,
    k_alive_link: String,
    keep_alive_secs: u64,
    retry_max: u32,
    pause_secs: u64,
}

#[derive(Debug, Clone)]
struct LoginRedirectInfo {
    scheme: String,
    host: String,
    wlanuserip: String,
    wlanacname: String,
    mac: String,
    vlan: String,
    hostname: String,
    rand: String,
}

#[derive(Debug, Clone)]
struct LogoutContext {
    scheme: String,
    host: String,
    wlanacip: String,
    wlanuserip: String,
    wlanacname: String,
    version: i32,
    userid: String,
    mac: String,
    group_id: i32,
}

#[derive(Debug)]
enum LoginOutcome {
    AlreadyOnline,
    LoggedIn(LogoutContext),
}

fn host_with_port(url: &Url) -> String {
    match (url.host_str(), url.port()) {
        (Some(host), Some(port)) => format!("{}:{}", host, port),
        (Some(host), None) => host.to_string(),
        _ => String::new(),
    }
}

fn parse_login_redirect(login_url: &str) -> Result<LoginRedirectInfo, String> {
    let url = Url::parse(login_url).map_err(|e| format!("解析登录链接失败: {}", e))?;
    let query = url.query_pairs();

    let mut wlanuserip = String::new();
    let mut wlanacname = String::new();
    let mut mac = String::new();
    let mut vlan = String::new();
    let mut hostname = String::new();
    let mut rand = String::new();

    for (k, v) in query {
        match k.as_ref() {
            "wlanuserip" => wlanuserip = v.to_string(),
            "wlanacname" => wlanacname = v.to_string(),
            "mac" => mac = v.to_string(),
            "vlan" => vlan = v.to_string(),
            "hostname" => hostname = v.to_string(),
            "rand" => rand = v.to_string(),
            _ => {}
        }
    }

    let host = host_with_port(&url);
    if host.is_empty() {
        return Err("登录链接中没有 host".to_string());
    }

    Ok(LoginRedirectInfo {
        scheme: url.scheme().to_string(),
        host,
        wlanuserip,
        wlanacname,
        mac,
        vlan,
        hostname,
        rand,
    })
}

async fn get_state(inner: &Arc<WorkerInner>) -> WorkerStatusPayload {
    inner.state.lock().await.clone()
}

async fn set_state(
    app: &AppHandle,
    inner: &Arc<WorkerInner>,
    status: WorkerStatus,
    message: Option<String>,
) {
    let payload = WorkerStatusPayload {
        running: status.is_running(),
        status,
        message,
    };

    {
        let mut guard = inner.state.lock().await;
        *guard = payload.clone();
    }

    let _ = app.emit("worker-status", payload);
}

async fn wait_or_stop(stop_rx: &mut watch::Receiver<bool>, secs: u64) -> bool {
    tokio::select! {
        changed = stop_rx.changed() => {
            if changed.is_ok() && *stop_rx.borrow() {
                true
            } else {
                false
            }
        }
        _ = sleep(Duration::from_secs(secs)) => false,
    }
}

async fn do_login_once(
    app: &AppHandle,
    inner: &Arc<WorkerInner>,
    cfg: &WorkerConfig,
) -> Result<LoginOutcome, String> {
    let need_login_url = telecom_portal_checker(&cfg.k_alive_link)
        .await
        .map_err(|e| format!("检测登陆链接失败: {}", e))?;

    let login_url = match need_login_url {
        Some(url) => url,
        None => {
            return Ok(LoginOutcome::AlreadyOnline);
        }
    };

    set_state(
        app,
        inner,
        WorkerStatus::LoggingIn,
        Some("检测到需要登陆，开始登录".to_string()),
    )
    .await;

    let redirect = parse_login_redirect(&login_url)?;

    let action = telecom_portal_json_action(
        &redirect.scheme,
        &redirect.host,
        &cfg.user_agent,
        &redirect.wlanuserip,
        &redirect.wlanacname,
        &redirect.mac,
        &redirect.vlan,
        &redirect.hostname,
        &redirect.rand,
    )
    .await
    .map_err(|e| format!("获取 PortalJsonAction 失败: {}", e))?;

    let login = telecom_quick_auth(
        &redirect.scheme,
        &redirect.host,
        &cfg.user_agent,
        &cfg.username,
        &cfg.password,
        &redirect.wlanuserip,
        &redirect.wlanacname,
        &action.serverForm.serverip,
        &redirect.vlan,
        &redirect.mac,
        action.serverForm.portalVer,
        action.portalconfig.id,
        action.portalconfig.timestamp,
        &action.portalconfig.uuid,
        "0",
        &redirect.hostname,
        &redirect.rand,
    )
    .await
    .map_err(|e| format!("登录请求失败: {}", e))?;

    if login.code != "0" {
        return Err(login
            .message
            .unwrap_or_else(|| "登录失败，未知错误".to_string()));
    }

    let logout_ctx = LogoutContext {
        scheme: redirect.scheme,
        host: redirect.host,
        wlanacip: action.serverForm.serverip,
        wlanuserip: redirect.wlanuserip,
        wlanacname: redirect.wlanacname,
        version: action.serverForm.portalVer,
        userid: login.userId.unwrap_or_else(|| cfg.username.clone()),
        mac: redirect.mac,
        group_id: login.groupId.unwrap_or(DEFAULT_LOGOUT_GROUP_ID),
    };

    Ok(LoginOutcome::LoggedIn(logout_ctx))
}

async fn do_logout(cfg: &WorkerConfig, logout_ctx: &Option<LogoutContext>) -> Result<(), String> {
    let ctx = match logout_ctx {
        Some(v) => v.clone(),
        None => {
            return Ok(());
        }
    };

    let resp = telecom_quick_auth_disconn(
        if ctx.scheme.is_empty() {
            DEFAULT_LOGOUT_SCHEME
        } else {
            &ctx.scheme
        },
        if ctx.host.is_empty() {
            DEFAULT_LOGOUT_HOST
        } else {
            &ctx.host
        },
        &cfg.user_agent,
        if ctx.wlanacip.is_empty() {
            DEFAULT_LOGOUT_WLANACIP
        } else {
            &ctx.wlanacip
        },
        &ctx.wlanuserip,
        if ctx.wlanacname.is_empty() {
            DEFAULT_LOGOUT_WLANACNAME
        } else {
            &ctx.wlanacname
        },
        if ctx.version == 0 {
            DEFAULT_LOGOUT_VERSION
        } else {
            ctx.version
        },
        "0",
        &ctx.userid,
        &ctx.mac,
        if ctx.group_id == 0 {
            DEFAULT_LOGOUT_GROUP_ID
        } else {
            ctx.group_id
        },
        "0",
    )
    .await
    .map_err(|e| format!("登出请求失败: {}", e))?;

    if resp.code != "0" {
        return Err(resp
            .message
            .unwrap_or_else(|| "登出失败，未知错误".to_string()));
    }

    Ok(())
}

async fn run_worker(
    app: AppHandle,
    inner: Arc<WorkerInner>,
    cfg: WorkerConfig,
    mut stop_rx: watch::Receiver<bool>,
) {
    set_state(
        &app,
        &inner,
        WorkerStatus::Starting,
        Some("已启动".to_string()),
    )
    .await;

    let mut retry = 0u32;
    let mut logout_ctx: Option<LogoutContext> = None;

    loop {
        if *stop_rx.borrow() {
            break;
        }

        match do_login_once(&app, &inner, &cfg).await {
            Ok(LoginOutcome::AlreadyOnline) => {
                retry = 0;
                set_state(
                    &app,
                    &inner,
                    WorkerStatus::LoggedIn,
                    Some("当前网络已在线".to_string()),
                )
                .await;
            }
            Ok(LoginOutcome::LoggedIn(ctx)) => {
                retry = 0;
                logout_ctx = Some(ctx);
                set_state(
                    &app,
                    &inner,
                    WorkerStatus::LoggedIn,
                    Some("登录成功".to_string()),
                )
                .await;
            }
            Err(err_msg) => {
                retry += 1;
                set_state(&app, &inner, WorkerStatus::NotLoggedIn, Some(err_msg)).await;

                if cfg.retry_max != 0 && retry >= cfg.retry_max {
                    set_state(
                        &app,
                        &inner,
                        WorkerStatus::Paused,
                        Some(format!("连续失败 {} 次，暂停 {} 秒", retry, cfg.pause_secs)),
                    )
                    .await;

                    retry = 0;

                    if wait_or_stop(&mut stop_rx, cfg.pause_secs).await {
                        break;
                    }
                    continue;
                }
            }
        }

        if wait_or_stop(&mut stop_rx, cfg.keep_alive_secs).await {
            break;
        }
    }

    set_state(
        &app,
        &inner,
        WorkerStatus::LoggingOut,
        Some("正在停止".to_string()),
    )
    .await;

    let logout_result = do_logout(&cfg, &logout_ctx).await;

    match logout_result {
        Ok(_) => {
            set_state(
                &app,
                &inner,
                WorkerStatus::Stopped,
                Some("已停止".to_string()),
            )
            .await;
        }
        Err(err_msg) => {
            set_state(
                &app,
                &inner,
                WorkerStatus::Stopped,
                Some(format!("已停止，登出结果: {}", err_msg)),
            )
            .await;
        }
    }

    let mut worker_guard = inner.worker.lock().await;
    *worker_guard = None;
}

#[tauri::command]
pub async fn start_worker(
    username: String,
    password: String,
    app: AppHandle,
    manager: State<'_, WorkerManager>,
) -> Result<StartWorkerResponse, String> {
    let username = username.trim().to_string();
    let password = password.trim().to_string();

    if username.is_empty() {
        return Err("账号不能为空".to_string());
    }
    if password.is_empty() {
        return Err("密码不能为空".to_string());
    }

    let inner = manager.inner.clone();

    {
        let mut guard = inner.worker.lock().await;

        if guard.is_some() {
            drop(guard);
            let state = get_state(&inner).await;
            return Ok(StartWorkerResponse {
                started: false,
                already_running: true,
                state,
            });
        }

        let cfg = WorkerConfig {
            username,
            password,
            user_agent: DEFAULT_USER_AGENT.to_string(),
            k_alive_link: DEFAULT_KALIVE_LINK.to_string(),
            keep_alive_secs: DEFAULT_KEEP_ALIVE_SECS,
            retry_max: DEFAULT_RETRY_MAX,
            pause_secs: DEFAULT_PAUSE_SECS,
        };

        let (stop_tx, stop_rx) = watch::channel(false);
        let app_clone = app.clone();
        let inner_clone = inner.clone();

        let join_handle = tauri::async_runtime::spawn(async move {
            run_worker(app_clone, inner_clone, cfg, stop_rx).await;
        });

        *guard = Some(RunningWorker {
            stop_tx,
            join_handle,
        });
    }

    set_state(
        &app,
        &inner,
        WorkerStatus::Starting,
        Some("启动中".to_string()),
    )
    .await;

    let state = get_state(&inner).await;

    Ok(StartWorkerResponse {
        started: true,
        already_running: false,
        state,
    })
}

#[tauri::command]
pub async fn stop_worker(
    app: AppHandle,
    manager: State<'_, WorkerManager>,
) -> Result<WorkerStatusPayload, String> {
    let inner = manager.inner.clone();

    let worker = {
        let mut guard = inner.worker.lock().await;
        guard.take()
    };

    match worker {
        Some(worker) => {
            let _ = worker.stop_tx.send(true);
            let _ = worker.join_handle.await;
        }
        None => {
            set_state(
                &app,
                &inner,
                WorkerStatus::Stopped,
                Some("未启动".to_string()),
            )
            .await;
        }
    }

    Ok(get_state(&inner).await)
}

#[tauri::command]
pub async fn get_worker_status(
    manager: State<'_, WorkerManager>,
) -> Result<WorkerStatusPayload, String> {
    let inner = manager.inner.clone();
    Ok(get_state(&inner).await)
}
