import { createApp } from "vue";
import { createPinia } from "pinia";
import { MotionPlugin } from "@vueuse/motion";
import naive from "naive-ui";
import App from "./App.vue";
import router from "./router";
import { useSettingsStore } from "@/stores/settings";
import { useAppStore } from "@/stores/app";
import { getPublicSiteSettings } from "@/api/modules/site";
import "./styles/theme.css";
import "./styles/global.css";

const app = createApp(App);
const pinia = createPinia();
app.use(pinia);
const settingsStore = useSettingsStore(pinia);
const appStore = useAppStore(pinia);
settingsStore.applyDocumentTitle();
app.use(router);
getPublicSiteSettings()
  .then((site) => {
    settingsStore.syncSiteName(site?.name);
    settingsStore.applyDocumentTitle();
    appStore.setInstallState(true);
    if (router.currentRoute.value.path === "/install") {
      router.replace("/");
    }
  })
  .catch((error) => {
    const message = ((error as Error)?.message || "").toLowerCase();
    if (message.includes("not installed")) {
      appStore.setInstallState(false);
      if (router.currentRoute.value.path !== "/install") {
        router.replace("/install");
      }
      return;
    }
    // keep local/default title when public endpoint is unavailable
  });
app.use(MotionPlugin);
app.use(naive);
app.mount("#app");
