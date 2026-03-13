import http from "../http";

export type LoginPayload = {
  email: string;
  password: string;
  remember: boolean;
  captcha?: CaptchaPayload;
};

export type CaptchaPayload = {
  provider: "geetest" | "cloudflare";
  geetest_lot_number?: string;
  geetest_captcha_output?: string;
  geetest_pass_token?: string;
  geetest_gen_time?: string;
  cloudflare_token?: string;
};

export type LoginResult = {
  token: string;
  expires_at: string;
  user: {
    id: number;
    username: string;
    email: string;
    role: "super_admin" | "admin" | "user";
  };
};

export type RegisterPayload = {
  username: string;
  email: string;
  password: string;
  confirm_password: string;
  email_code?: string;
  captcha?: CaptchaPayload;
};

export type RegisterResult = {
  user_id: number;
};

export function login(payload: LoginPayload) {
  return http.post<never, LoginResult>("/api/auth/login", payload);
}

export function register(payload: RegisterPayload) {
  return http.post<never, RegisterResult>("/api/auth/register", payload);
}

export function sendRegisterEmailCode(email: string) {
  return http.post<never, { sent: boolean }>("/api/auth/register/email-code", { email });
}

export function getCurrentUser() {
  return http.get<never, LoginResult["user"]>("/api/auth/me");
}

export function refreshToken() {
  return http.post<never, LoginResult>("/api/auth/refresh");
}
