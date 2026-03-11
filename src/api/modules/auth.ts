import http from "../http";

export type LoginPayload = {
  email: string;
  password: string;
  remember: boolean;
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

export function getCurrentUser() {
  return http.get<never, LoginResult["user"]>("/api/auth/me");
}
