import { createRouter, createWebHistory } from "vue-router";
import { useAppStore } from "@/stores/app";
import { useAuthStore } from "@/stores/auth";
import { useSettingsStore } from "@/stores/settings";
import HomeView from "@/views/HomeView.vue";
import InstallView from "@/views/InstallView.vue";
import LoginView from "@/views/LoginView.vue";
import RegisterView from "@/views/RegisterView.vue";
import AdminLayout from "@/layouts/AdminLayout.vue";
import DashboardView from "@/views/admin/DashboardView.vue";
import CookiesView from "@/views/admin/CookiesView.vue";
import SettingsView from "@/views/admin/SettingsView.vue";
import RedisSettingsView from "@/views/admin/RedisSettingsView.vue";
import SmtpSettingsView from "@/views/admin/SmtpSettingsView.vue";
import ProxySettingsView from "@/views/admin/ProxySettingsView.vue";
import UsersView from "@/views/admin/UsersView.vue";
import UserGroupsView from "@/views/admin/UserGroupsView.vue";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: HomeView },
    { path: "/install", component: InstallView, meta: { guest: true } },
    { path: "/login", component: LoginView, meta: { guest: true } },
    { path: "/register", component: RegisterView, meta: { guest: true } },
    {
      path: "/admin",
      component: AdminLayout,
      meta: { auth: true },
      redirect: "/admin",
      children: [
        { path: "", name: "admin-dashboard", component: DashboardView, meta: { auth: true } },
        { path: "users", name: "admin-users", component: UsersView, meta: { auth: true } },
        { path: "user-groups", name: "admin-user-groups", component: UserGroupsView, meta: { auth: true, super: true } },
        { path: "cookies", name: "admin-cookies", component: CookiesView, meta: { auth: true } },
        { path: "settings", name: "admin-settings", component: SettingsView, meta: { auth: true } },
        { path: "settings/redis", name: "admin-settings-redis", component: RedisSettingsView, meta: { auth: true } },
        { path: "settings/smtp", name: "admin-settings-smtp", component: SmtpSettingsView, meta: { auth: true } },
        { path: "settings/proxy", name: "admin-settings-proxy", component: ProxySettingsView, meta: { auth: true } }
      ]
    }
  ]
});

router.beforeEach(async (to) => {
  const appStore = useAppStore();
  const authStore = useAuthStore();

  if (appStore.installChecked) {
    if (!appStore.installed && to.path !== "/install") {
      return "/install";
    }
    if (appStore.installed && to.path === "/install") {
      return "/";
    }
  }

  if ((to.path === "/login" || to.path === "/register") && authStore.isAuthed) {
    return authStore.isAdmin ? "/admin" : "/";
  }

  if (to.meta.auth && !authStore.isAuthed) {
    return "/login";
  }
  if (to.path.startsWith("/admin") && !authStore.isAdmin) {
    return "/";
  }
  if (to.meta.super && !authStore.isSuperAdmin) {
    return "/admin";
  }

  return true;
});

router.afterEach(() => {
  const settingsStore = useSettingsStore();
  settingsStore.applyDocumentTitle();
});

export default router;


