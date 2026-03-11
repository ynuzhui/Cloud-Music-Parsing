<script setup lang="ts">
import { computed, h, onMounted } from "vue";
import { RouterLink, RouterView, useRoute, useRouter } from "vue-router";
import { NIcon, type GlobalThemeOverrides, type MenuOption, createDiscreteApi } from "naive-ui";
import { ChartPie, Cookie, Settings, Logout, Music, ExternalLink, Users, Shield } from "@vicons/tabler";
import { useAuthStore } from "@/stores/auth";
import { useSettingsStore } from "@/stores/settings";
import { getSettings } from "@/api/modules/admin";

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();
const settingsStore = useSettingsStore();
const { message } = createDiscreteApi(["message"]);

const siteName = computed(() => settingsStore.siteName);
const isSuperAdmin = computed(() => authStore.user?.role === "super_admin");
const adminThemeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: "#0f6fff",
    primaryColorHover: "#2b80ff",
    primaryColorPressed: "#0d4ed8",
    primaryColorSuppl: "#0f6fff",
    infoColor: "#0f6fff",
    infoColorHover: "#2b80ff",
    infoColorPressed: "#0d4ed8",
    infoColorSuppl: "#0f6fff",
  },
};

onMounted(async () => {
  try {
    const data = await getSettings();
    settingsStore.syncSiteName(data.site.name);
    settingsStore.applyDocumentTitle();
  } catch {
    // 静默处理
  }
});

const menuOptions = computed<MenuOption[]>(() => {
  const options: MenuOption[] = [
    {
      key: "/dashboard",
      label: () => h(RouterLink, { to: "/dashboard" }, { default: () => "统计总览" }),
      icon: renderIcon(ChartPie)
    },
    {
      key: "/dashboard/users",
      label: () => h(RouterLink, { to: "/dashboard/users" }, { default: () => "用户管理" }),
      icon: renderIcon(Users)
    },
    {
      key: "/dashboard/cookies",
      label: () => h(RouterLink, { to: "/dashboard/cookies" }, { default: () => "Cookie 池" }),
      icon: renderIcon(Cookie)
    },
    {
      key: "/dashboard/settings",
      label: "系统设置",
      icon: renderIcon(Settings),
      children: [
        {
          key: "/dashboard/settings",
          label: () => h(RouterLink, { to: "/dashboard/settings" }, { default: () => "站点配置" }),
        },
        {
          key: "/dashboard/settings/redis",
          label: () => h(RouterLink, { to: "/dashboard/settings/redis" }, { default: () => "Redis 配置" }),
        },
        {
          key: "/dashboard/settings/smtp",
          label: () => h(RouterLink, { to: "/dashboard/settings/smtp" }, { default: () => "SMTP 配置" }),
        },
        {
          key: "/dashboard/settings/proxy",
          label: () => h(RouterLink, { to: "/dashboard/settings/proxy" }, { default: () => "代理配置" }),
        }
      ]
    }
  ];
  if (isSuperAdmin.value) {
    options.splice(2, 0, {
      key: "/dashboard/user-groups",
      label: () => h(RouterLink, { to: "/dashboard/user-groups" }, { default: () => "用户组管理" }),
      icon: renderIcon(Shield)
    });
  }
  return options;
});

const selectedKey = computed(() => {
  const p = route.path;
  if (p === "/dashboard/settings/redis") return "/dashboard/settings/redis";
  if (p === "/dashboard/settings/smtp") return "/dashboard/settings/smtp";
  if (p === "/dashboard/settings/proxy") return "/dashboard/settings/proxy";
  if (p.startsWith("/dashboard/user-groups")) return "/dashboard/user-groups";
  if (p.startsWith("/dashboard/users")) return "/dashboard/users";
  if (p.startsWith("/dashboard/settings")) return "/dashboard/settings";
  if (p.startsWith("/dashboard/cookies")) return "/dashboard/cookies";
  return "/dashboard";
});

function renderIcon(icon: any) {
  return () => h(NIcon, null, { default: () => h(icon) });
}

function logout() {
  authStore.logout();
  message.success("已退出登录");
  router.replace("/login");
}
</script>

<template>
  <n-config-provider :theme-overrides="adminThemeOverrides">
    <main class="admin-shell">
      <n-layout has-sider class="admin-layout">
        <n-layout-sider
          bordered
          collapse-mode="width"
          :collapsed-width="72"
          :width="228"
          :native-scrollbar="false"
          content-class="sider-scroll-content"
          :content-style="{ height: '100%' }"
          class="admin-sider"
        >
          <div class="sider-inner">
            <div class="brand-box">
              <n-icon size="24"><Music /></n-icon>
              <span>{{ siteName }}</span>
            </div>
            <div class="menu-wrap">
              <n-menu :value="selectedKey" :options="menuOptions" :default-expand-all="true" />
            </div>

            <div class="sider-bottom">
              <n-button quaternary block class="sider-btn" tag="a" href="/" target="_blank">
                <template #icon>
                  <n-icon><ExternalLink /></n-icon>
                </template>
                访问前台
              </n-button>
              <n-button quaternary block type="error" class="sider-btn" @click="logout">
                <template #icon>
                  <n-icon><Logout /></n-icon>
                </template>
                退出登录
              </n-button>
            </div>
          </div>
        </n-layout-sider>

        <n-layout>
          <n-layout-header bordered class="admin-header">
            <div class="header-title">{{ siteName }}</div>
            <div class="user-info">
              <strong>{{ authStore.user?.username || "Admin" }}</strong>
              <span>{{ authStore.user?.email }}</span>
            </div>
          </n-layout-header>

          <n-layout-content class="admin-content">
            <router-view v-slot="{ Component }">
              <transition name="fade-slide" mode="out-in">
                <component :is="Component" />
              </transition>
            </router-view>
          </n-layout-content>
        </n-layout>
      </n-layout>
    </main>
  </n-config-provider>
</template>

<style scoped>
.admin-shell {
  height: 100vh;
  overflow: hidden;
  background:
    radial-gradient(42rem 32rem at 100% -8%, #d8e7ff 0%, transparent 56%),
    radial-gradient(32rem 28rem at 0% 110%, #ffe9d9 0%, transparent 52%),
    linear-gradient(135deg, #f5f8ff, #eef3ff);
}

.admin-layout {
  height: 100vh;
  background: transparent;
}

.admin-sider {
  background: rgba(255, 255, 255, 0.88);
  backdrop-filter: blur(10px);
}

.admin-sider :deep(.n-scrollbar),
.admin-sider :deep(.n-scrollbar-container) {
  height: 100%;
}

.admin-sider :deep(.sider-scroll-content) {
  min-height: 100%;
  display: flex;
  flex-direction: column;
}

.sider-inner {
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.menu-wrap {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.brand-box {
  height: 66px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 18px;
  font-weight: 800;
  border-bottom: 1px solid rgba(20, 41, 78, 0.08);
  flex-shrink: 0;
}

.sider-bottom {
  flex-shrink: 0;
  padding: 12px 12px 16px;
  border-top: 1px solid rgba(20, 41, 78, 0.06);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.sider-btn {
  justify-content: center;
}

.admin-header {
  height: 66px;
  padding: 0 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: rgba(255, 255, 255, 0.86);
  backdrop-filter: blur(8px);
  flex-shrink: 0;
}

.header-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--text-1, #1a1a2e);
}

.user-info {
  display: flex;
  flex-direction: column;
  text-align: right;
}

.user-info span {
  font-size: 12px;
  color: var(--text-2);
}

.admin-content {
  padding: 18px;
  height: calc(100vh - 66px);
  overflow-y: auto;
}

.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: all 0.22s ease;
}

.fade-slide-enter-from,
.fade-slide-leave-to {
  opacity: 0;
  transform: translateY(8px);
}
</style>




