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

export function login(payload: LoginPayload) {
  return http.post<never, LoginResult>("/api/auth/login", payload);
}

export function getCurrentUser() {
  return http.get<never, LoginResult["user"]>("/api/auth/me");
}
