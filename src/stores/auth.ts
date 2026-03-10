import { defineStore } from "pinia";
import type { LoginResult } from "@/api/modules/auth";

type AuthState = {
  token: string;
  user: LoginResult["user"] | null;
};

export const useAuthStore = defineStore("auth", {
  state: (): AuthState => ({
    token: localStorage.getItem("mp_token") || "",
    user: safeParse(localStorage.getItem("mp_user"))
  }),
  getters: {
    isAuthed: (state) => !!state.token
  },
  actions: {
    setSession(payload: LoginResult) {
      this.token = payload.token;
      this.user = payload.user;
      localStorage.setItem("mp_token", payload.token);
      localStorage.setItem("mp_user", JSON.stringify(payload.user));
    },
    logout() {
      this.token = "";
      this.user = null;
      localStorage.removeItem("mp_token");
      localStorage.removeItem("mp_user");
    }
  }
});

function safeParse(raw: string | null) {
  if (!raw) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
}
