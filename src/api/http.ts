import axios from "axios";

const http = axios.create({
  baseURL: "/",
  timeout: 20000
});

http.interceptors.request.use((config) => {
  const token = localStorage.getItem("mp_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
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
  (error) => {
    const message = error?.response?.data?.msg || error?.message || "网络错误";
    return Promise.reject(new Error(message));
  }
);

export default http;
