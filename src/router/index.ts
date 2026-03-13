import { createRouter, createWebHistory } from "vue-router";
import { useAppStore } from "@/stores/app";
import { useAuthStore } from "@/stores/auth";
import { useSettingsStore } from "@/stores/settings";
import HomeView from "@/views/HomeView.vue";

const InstallView = () => import("@/views/InstallView.vue");
const LoginView = () => import("@/views/LoginView.vue");
const RegisterView = () => import("@/views/RegisterView.vue");
const AdminLayout = () => import("@/layouts/AdminLayout.vue");
const DashboardView = () => import("@/views/admin/DashboardView.vue");
const CookiesView = () => import("@/views/admin/CookiesView.vue");
const SettingsView = () => import("@/views/admin/SettingsView.vue");
const RedisSettingsView = () => import("@/views/admin/RedisSettingsView.vue");
const SmtpSettingsView = () => import("@/views/admin/SmtpSettingsView.vue");
const ProxySettingsView = () => import("@/views/admin/ProxySettingsView.vue");
const UsersView = () => import("@/views/admin/UsersView.vue");
const UserGroupsView = () => import("@/views/admin/UserGroupsView.vue");

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/", component: HomeView },
    { path: "/install", component: InstallView, meta: { guest: true } },
    { path: "/login", component: LoginView, meta: { guest: true } },
    { path: "/register", component: RegisterView, meta: { guest: true } },
    {
      path: "/dashboard",
      component: AdminLayout,
      meta: { auth: true },
      redirect: "/dashboard",
      children: [
        { path: "", name: "dashboard-home", component: DashboardView, meta: { auth: true } },
        { path: "users", name: "dashboard-users", component: UsersView, meta: { auth: true } },
        { path: "user-groups", name: "dashboard-user-groups", component: UserGroupsView, meta: { auth: true, super: true } },
        { path: "cookies", name: "dashboard-cookies", component: CookiesView, meta: { auth: true } },
        { path: "settings", name: "dashboard-settings", component: SettingsView, meta: { auth: true } },
        { path: "settings/redis", name: "dashboard-settings-redis", component: RedisSettingsView, meta: { auth: true } },
        { path: "settings/smtp", name: "dashboard-settings-smtp", component: SmtpSettingsView, meta: { auth: true } },
        { path: "settings/proxy", name: "dashboard-settings-proxy", component: ProxySettingsView, meta: { auth: true } }
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
    return authStore.isAdmin ? "/dashboard" : "/";
  }

  if (to.meta.auth && !authStore.isAuthed) {
    return "/login";
  }
  if (to.path.startsWith("/dashboard") && !authStore.isAdmin) {
    return "/";
  }
  if (to.meta.super && !authStore.isSuperAdmin) {
    return "/dashboard";
  }

  return true;
});

router.afterEach(() => {
  const settingsStore = useSettingsStore();
  settingsStore.applyDocumentTitle();
});

export default router;


