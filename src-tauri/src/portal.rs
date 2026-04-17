use anyhow::{Context, Result};
use regex::Regex;
use reqwest::{
    header::{ACCEPT, ACCEPT_LANGUAGE, LOCATION, USER_AGENT},
    redirect::Policy,
    Client, Url,
};
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use serde_json::Value;
use std::time::Duration;

const REQUEST_TIMEOUT_SECS: u64 = 5;

#[allow(non_snake_case)]
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ActionResponse {
    pub portalconfig: PortalConfig,
    pub serverForm: ServerForm,
}

#[allow(non_snake_case)]
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PortalConfig {
    pub id: i32,
    pub timestamp: i64,
    pub uuid: String,
}

#[allow(non_snake_case)]
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerForm {
    pub portalVer: i32,
    pub serverip: String,
    pub servername: Option<String>,
}

#[allow(non_snake_case)]
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QuickAuthResponse {
    pub code: String,
    pub rec: Option<String>,
    pub message: Option<String>,
    pub wlanacIp: Option<String>,
    pub wlanacIpv6: Option<String>,
    pub version: Option<String>,
    pub usertime: Option<String>,
    pub reccode: Option<String>,
    pub logoutgourl: Option<String>,
    pub selfTicket: Option<String>,
    pub macChange: Option<bool>,
    pub groupId: Option<i32>,
    pub passwdPolicyCheck: Option<bool>,
    pub dropLogCheck: Option<String>,
    pub logoutSsoUrl: Option<String>,
    pub userId: Option<String>,
    pub operatingBindCtrlList: Option<Vec<Value>>,
}

fn build_client() -> Result<Client> {
    Client::builder()
        .no_proxy()
        .timeout(Duration::from_secs(REQUEST_TIMEOUT_SECS))
        .build()
        .context("构建 reqwest 客户端失败")
}

fn build_checker_client() -> Result<Client> {
    Client::builder()
        .no_proxy()
        .timeout(Duration::from_secs(REQUEST_TIMEOUT_SECS))
        .redirect(Policy::none())
        .build()
        .context("构建检查客户端失败")
}

fn parse_json<T: DeserializeOwned>(text: &str) -> Result<T> {
    serde_json::from_str(text).context("反序列化 JSON 失败")
}

/// 对应 Go: TelecomPortalJsonAction
pub async fn telecom_portal_json_action(
    scheme: &str,
    host: &str,
    user_agent: &str,
    wlanuserip: &str,
    wlanacname: &str,
    mac: &str,
    vlan: &str,
    hostname: &str,
    rand: &str,
) -> Result<ActionResponse> {
    let mut url = Url::parse(&format!("{}://{}/PortalJsonAction.do", scheme, host))
        .context("PortalJsonAction.do 地址构建失败")?;

    {
        let mut q = url.query_pairs_mut();
        q.append_pair("wlanuserip", wlanuserip);
        q.append_pair("wlanacname", wlanacname);
        q.append_pair("mac", mac);
        q.append_pair("vlan", vlan);
        q.append_pair("hostname", hostname);
        q.append_pair("rand", rand);
        q.append_pair("viewStatus", "1");
    }

    let client = build_client()?;
    let text = client
        .get(url)
        .header(USER_AGENT, user_agent)
        .header(ACCEPT, "application/json, text/javascript, */*; q=0.01")
        .header(ACCEPT_LANGUAGE, "zh-CN")
        .send()
        .await
        .context("发送 PortalJsonAction 请求失败")?
        .error_for_status()
        .context("PortalJsonAction 返回错误代码")?
        .text()
        .await
        .context("读取 PortalJsonAction 响应失败")?;

    parse_json::<ActionResponse>(&text)
}

/// 对应 Go: TelecomQuickAuth
pub async fn telecom_quick_auth(
    scheme: &str,
    host: &str,
    user_agent: &str,
    userid: &str,
    passwd: &str,
    wlanuserip: &str,
    wlanacname: &str,
    wlanac_ip: &str,
    vlan: &str,
    mac: &str,
    version: i32,
    portalpageid: i32,
    timestamp: i64,
    uuid: &str,
    portaltype: &str,
    hostname: &str,
    rand: &str,
) -> Result<QuickAuthResponse> {
    let mut url = Url::parse(&format!("{}://{}/quickauth.do", scheme, host))
        .context("QuickAuth.do 地址构建失败")?;

    {
        let mut q = url.query_pairs_mut();
        q.append_pair("userid", userid);
        q.append_pair("passwd", passwd);
        q.append_pair("wlanuserip", wlanuserip);
        q.append_pair("wlanacname", wlanacname);
        q.append_pair("wlanacIp", wlanac_ip);
        q.append_pair("vlan", vlan);
        q.append_pair("mac", mac);
        q.append_pair("version", &version.to_string());
        q.append_pair("portalpageid", &portalpageid.to_string());
        q.append_pair("timestamp", &timestamp.to_string());
        q.append_pair("uuid", uuid);
        q.append_pair("portaltype", portaltype);
        q.append_pair("hostname", hostname);
        q.append_pair("rand", rand);
    }

    let client = build_client()?;
    let text = client
        .get(url)
        .header(USER_AGENT, user_agent)
        .header(ACCEPT, "application/json, text/javascript, */*; q=0.01")
        .header(
            ACCEPT_LANGUAGE,
            "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7,ja;q=0.6",
        )
        .send()
        .await
        .context("发送 QuickAuth 请求失败")?
        .error_for_status()
        .context("QuickAuth 返回错误代码")?
        .text()
        .await
        .context("读取 QuickAuth 响应失败")?;

    parse_json::<QuickAuthResponse>(&text)
}

/// 对应 Go: TelecomQuickAuthDisconn
pub async fn telecom_quick_auth_disconn(
    scheme: &str,
    host: &str,
    user_agent: &str,
    wlanacip: &str,
    wlanuserip: &str,
    wlanacname: &str,
    version: i32,
    portaltype: &str,
    userid: &str,
    mac: &str,
    group_id: i32,
    clear_operator: &str,
) -> Result<QuickAuthResponse> {
    let url = format!("{}://{}/quickauthdisconn.do", scheme, host);

    let form = [
        ("wlanacip", wlanacip.to_string()),
        ("wlanuserip", wlanuserip.to_string()),
        ("wlanacname", wlanacname.to_string()),
        ("version", version.to_string()),
        ("portaltype", portaltype.to_string()),
        ("userid", userid.to_string()),
        ("mac", mac.to_string()),
        ("groupId", group_id.to_string()),
        ("clearOperator", clear_operator.to_string()),
    ];

    let client = build_client()?;
    let text = client
        .post(url)
        .header(USER_AGENT, user_agent)
        .header(ACCEPT, "application/json, text/javascript, */*; q=0.01")
        .header(
            ACCEPT_LANGUAGE,
            "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7,ja;q=0.6",
        )
        .form(&form)
        .send()
        .await
        .context("发送 QuickAuthDisconn 请求失败")?
        .error_for_status()
        .context("QuickAuthDisconn 返回错误代码")?
        .text()
        .await
        .context("读取 QuickAuthDisconn 响应失败")?;

    parse_json::<QuickAuthResponse>(&text)
}

fn contains_portal_keyword(s: &str) -> bool {
    s.contains("portalScript.do") || s.contains("portal.do")
}

fn normalize_url(base: &Url, raw: &str) -> String {
    base.join(raw)
        .map(|u| u.to_string())
        .unwrap_or_else(|_| raw.to_string())
}

fn extract_redirect_from_script(base: &Url, html: &str) -> Option<String> {
    let script_re = Regex::new(r"(?is)<script[^>]*>(.*?)</script>").ok()?;

    let patterns = [
        Regex::new(r#"(?is)(?:window\.)?location\.replace\s*\(\s*["']([^"']+)["']\s*\)"#).ok()?,
        Regex::new(r#"(?is)(?:window\.)?location(?:\.href)?\s*=\s*["']([^"']+)["']"#).ok()?,
        Regex::new(r#"(?is)(?:top|self|parent)\.location(?:\.href)?\s*=\s*["']([^"']+)["']"#)
            .ok()?,
    ];

    for caps in script_re.captures_iter(html) {
        let js = caps.get(1).map(|m| m.as_str()).unwrap_or_default();

        for re in &patterns {
            for one in re.captures_iter(js) {
                if let Some(m) = one.get(1) {
                    let final_url = normalize_url(base, m.as_str());
                    if contains_portal_keyword(&final_url) {
                        return Some(final_url);
                    }
                }
            }
        }
    }

    None
}

/// 对应 Go: TelecomPortalChecker
/// 返回 Some(login_url) 表示需要登录
/// 返回 None 表示当前看起来不需要登录
pub async fn telecom_portal_checker(k_alive_link: &str) -> Result<Option<String>> {
    let client = build_checker_client()?;

    let resp = match client.get(k_alive_link).send().await {
        Ok(resp) => resp,
        Err(_) => {
            return Ok(None);
        }
    };

    let status = resp.status();
    let base_url = resp.url().clone();

    if status.is_redirection() {
        if let Some(loc) = resp.headers().get(LOCATION) {
            if let Ok(loc_str) = loc.to_str() {
                let final_url = normalize_url(&base_url, loc_str);
                if contains_portal_keyword(&final_url) {
                    return Ok(Some(final_url));
                }
            }
        }
        return Ok(None);
    }

    if status.is_success() {
        let html = resp.text().await.context("读取 keep-alive 响应失败")?;

        if let Some(url) = extract_redirect_from_script(&base_url, &html) {
            return Ok(Some(url));
        }
    }

    Ok(None)
}
