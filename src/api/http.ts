import axios from "axios";
import type { AxiosError, InternalAxiosRequestConfig } from "axios";

const http = axios.create({
  baseURL: "/",
  timeout: 20000
});

let isRefreshing = false;
let pendingQueue: Array<{
  resolve: (token: string) => void;
  reject: (err: unknown) => void;
}> = [];

function processPendingQueue(token: string | null, error?: unknown) {
  pendingQueue.forEach(({ resolve, reject }) => {
    if (token) resolve(token);
    else reject(error);
  });
  pendingQueue = [];
}

http.interceptors.request.use((config) => {
  const token = localStorage.getItem("mp_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  config.headers["Cache-Control"] = "no-cache";
  return config;
});

http.interceptors.response.use(
  (response) => {
    const payload = response.data;
    if (payload && typeof payload === "object" && "code" in payload) {
      if (payload.code !== 0) {
        return Promise.reject(new Error(payload.msg || "请求失败"));
      }
      return payload.data;
    }
    return payload;
  },
  async (error: AxiosError) => {
    const status = error?.response?.status;
    const originalRequest = error.config as InternalAxiosRequestConfig & { _retried?: boolean };

    if (status === 401 && originalRequest && !originalRequest._retried) {
      const token = localStorage.getItem("mp_token");
      if (!token || originalRequest.url === "/api/auth/refresh") {
        forceLogout();
        return Promise.reject(error);
      }

      if (isRefreshing) {
        return new Promise<string>((resolve, reject) => {
          pendingQueue.push({ resolve, reject });
        }).then((newToken) => {
          originalRequest.headers.Authorization = `Bearer ${newToken}`;
          originalRequest._retried = true;
          return http(originalRequest);
        });
      }

      isRefreshing = true;
      try {
        const res = await http.post<never, { token: string; expires_at: string; user: unknown }>("/api/auth/refresh");
        const newToken = res.token;
        localStorage.setItem("mp_token", newToken);
        if (res.user) localStorage.setItem("mp_user", JSON.stringify(res.user));
        processPendingQueue(newToken);
        originalRequest.headers.Authorization = `Bearer ${newToken}`;
        originalRequest._retried = true;
        return http(originalRequest);
      } catch (refreshErr) {
        processPendingQueue(null, refreshErr);
        forceLogout();
        return Promise.reject(refreshErr);
      } finally {
        isRefreshing = false;
      }
    }

    if (status === 403) {
      const token = localStorage.getItem("mp_token");
      if (token) {
        forceLogout();
      }
    }

    const message = (error?.response?.data as Record<string, string>)?.msg || error?.message || "网络错误";
    return Promise.reject(new Error(message));
  }
);

function forceLogout() {
  localStorage.removeItem("mp_token");
  localStorage.removeItem("mp_user");
  window.location.href = "/login";
}

export default http;
