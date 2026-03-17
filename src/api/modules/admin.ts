import http from "../http";

export type DashboardStats = {
  user_count: number;
  user_new_today: number;
  user_new_prev_day: number;
  parse_count: number;
  parse_today: number;
  cookie_count: number;
  cookie_new_today: number;
  cookie_new_prev_day: number;
  pv_total: number;
  uv_total: number;
  pv_today: number;
  uv_today: number;
  avg_latency_ms: number;
  trend_7days: Array<{ day: string; count: number }>;
  pv_trend_7days: Array<{ day: string; count: number }>;
  uv_trend_7days: Array<{ day: string; count: number }>;
  latency_trend_7days: Array<{ day: string; count: number }>;
};

export type CookieItem = {
  id: number;
  provider: string;
  label: string;
  value: string;
  active: boolean;
  status: "unknown" | "valid" | "invalid";
  nickname: string;
  vip_type: number;
  vip_level: number;
  red_vip_level: number;
  last_checked: string | null;
  call_count: number;
  last_used_at: string | null;
  fail_count: number;
  last_error: string;
  created_at: string;
  updated_at: string;
};

export type CookieVerifyResult = {
  valid: boolean;
  status: "valid" | "invalid";
  nickname: string;
  vip_type: number;
  vip_level: number;
  red_vip_level: number;
  error?: string;
};

export type VerifyCookieResponse = {
  id: number;
  status: "unknown" | "valid" | "invalid";
  nickname: string;
  vip_type: number;
  vip_level: number;
  red_vip_level: number;
  last_checked: string | null;
  fail_count: number;
  last_error: string;
  verify: CookieVerifyResult | null;
};

export type VerifyAllCookiesResponse = {
  total: number;
  valid: number;
  invalid: number;
};

export type CookieQrKeyResponse = {
  code: number;
  unikey: string;
  qr_url: string;
};

export type CookieQrCheckResponse = {
  code: number;
  message: string;
  nickname?: string;
  avatar_url?: string;
  cookie?: string;
  music_u?: string;
};

export type SystemSettings = {
  site: {
    name: string;
    keywords: string;
    description: string;
    icp_no: string;
    police_no: string;
  };
  feature: {
    allow_register: boolean;
    register_email_verify: boolean;
    default_parse_quality: "standard" | "exhigh" | "lossless" | "hires" | "sky" | "jyeffect" | "jymaster";
    parse_require_login: boolean;
    parse_auto_play: boolean;
    default_daily_parse_limit: number;
    default_concurrency_limit: number;
    cookie_auto_verify: boolean;
  };
  captcha: {
    enabled: boolean;
    provider: "geetest" | "cloudflare";
    geetest_captcha_id: string;
    geetest_captcha_key: string;
    cloudflare_site_key: string;
    cloudflare_secret_key: string;
  };
  redis: {
    enabled: boolean;
    host: string;
    port: number;
    pass: string;
    db: number;
  };
  proxy: {
    enabled: boolean;
    host: string;
    port: number;
    username: string;
    password: string;
    protocol: string;
  };
  smtp: {
    host: string;
    port: number;
    user: string;
    pass: string;
    ssl: boolean;
  };
};

export function getDashboardStats() {
  return http.get<never, DashboardStats>("/api/dashboard/stats");
}

export function getSettings() {
  return http.get<never, SystemSettings>("/api/dashboard/settings");
}

export function saveSettings(payload: SystemSettings) {
  return http.put("/api/dashboard/settings", payload);
}

export function testSmtp(to: string) {
  return http.post("/api/dashboard/smtp/test", { to });
}

export function listCookies() {
  return http.get<never, CookieItem[]>("/api/dashboard/cookies");
}

export function createCookie(payload: { provider: string; label: string; value: string; active: boolean }) {
  return http.post("/api/dashboard/cookies", payload);
}

export function updateCookie(id: number, payload: { label?: string; value?: string; active?: boolean }) {
  return http.patch(`/api/dashboard/cookies/${id}`, payload);
}

export function deleteCookie(id: number) {
  return http.delete(`/api/dashboard/cookies/${id}`);
}

export function verifyCookie(id: number) {
  return http.post<never, VerifyCookieResponse>(`/api/dashboard/cookies/${id}/verify`);
}

export function verifyAllCookies() {
  return http.post<never, VerifyAllCookiesResponse>("/api/dashboard/cookies/verify-all");
}

export function getCookieQrKey() {
  return http.get<never, CookieQrKeyResponse>("/api/dashboard/cookies/qr/key");
}

export function checkCookieQrStatus(key: string) {
  return http.post<{ key: string }, CookieQrCheckResponse>("/api/dashboard/cookies/qr/check", { key });
}

export type UserListResult = {
  items: UserItem[];
  total: number;
  page: number;
  page_size: number;
};

export type UserItem = {
  id: number;
  username: string;
  email: string;
  role: "user" | "admin" | "super_admin";
  status: "active" | "disabled";
  group_id: number | null;
  group_name: string;
  group_unlimited: boolean;
  daily_limit: number;
  concurrency_limit: number;
  last_login_at: string | null;
  last_login_ip: string;
  created_at: string;
  updated_at: string;
};

export type UserGroupItem = {
  id: number;
  name: string;
  description: string;
  daily_limit: number;
  concurrency_limit: number;
  unlimited_parse: boolean;
  is_default: boolean;
  member_count: number;
  created_at: string;
  updated_at: string;
};

export function listUsers(params: { page?: number; page_size?: number; keyword?: string; role?: string; status?: string } = {}) {
  return http.get<never, UserListResult>("/api/dashboard/users", { params });
}

export function createUser(payload: {
  username: string;
  email: string;
  password: string;
  role?: "user" | "admin";
  group_id?: number;
  daily_limit?: number;
  concurrency_limit?: number;
  status?: "active" | "disabled";
}) {
  return http.post("/api/dashboard/users", payload);
}

export function updateUser(
  id: number,
  payload: {
    username?: string;
    email?: string;
    group_id?: number;
    daily_limit?: number;
    concurrency_limit?: number;
  }
) {
  return http.patch(`/api/dashboard/users/${id}`, payload);
}

export function updateUserStatus(id: number, active: boolean) {
  return http.patch(`/api/dashboard/users/${id}/status`, { active });
}

export function updateUserRole(id: number, role: "user" | "admin") {
  return http.patch(`/api/dashboard/users/${id}/role`, { role });
}

export function resetUserPassword(id: number, password: string) {
  return http.post(`/api/dashboard/users/${id}/reset-password`, { password });
}

export function deleteUser(id: number) {
  return http.delete(`/api/dashboard/users/${id}`);
}

export function listUserGroups() {
  return http.get<never, UserGroupItem[]>("/api/dashboard/user-groups");
}

export function createUserGroup(payload: {
  name: string;
  description?: string;
  daily_limit?: number;
  concurrency_limit?: number;
  unlimited_parse?: boolean;
  is_default?: boolean;
}) {
  return http.post("/api/dashboard/user-groups", payload);
}

export function updateUserGroup(
  id: number,
  payload: {
    name?: string;
    description?: string;
    daily_limit?: number;
    concurrency_limit?: number;
    unlimited_parse?: boolean;
    is_default?: boolean;
  }
) {
  return http.patch(`/api/dashboard/user-groups/${id}`, payload);
}

export function deleteUserGroup(id: number) {
  return http.delete(`/api/dashboard/user-groups/${id}`);
}
