<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { darkTheme } from "naive-ui";
import { useSettingsStore } from "@/stores/settings";

const settingsStore = useSettingsStore();

const systemPrefersDark = ref(false);
let mediaQuery: MediaQueryList | null = null;

const naiveTheme = computed(() => {
  const dark = settingsStore.theme === "dark" || (settingsStore.theme === "system" && systemPrefersDark.value);
  return dark ? darkTheme : null;
});

const naiveThemeOverrides = computed(() => {
  const dark = settingsStore.theme === "dark" || (settingsStore.theme === "system" && systemPrefersDark.value);
  return {
    common: dark
      ? {
          primaryColor: "#4d94ff",
          primaryColorHover: "#6aa7ff",
          primaryColorPressed: "#3a7bff",
          primaryColorSuppl: "#4d94ff",
        }
      : {
          primaryColor: "#0f6fff",
          primaryColorHover: "#2b80ff",
          primaryColorPressed: "#0d4ed8",
          primaryColorSuppl: "#0f6fff",
        },
  };
});

onMounted(() => {
  settingsStore.initThemeListener();
  mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
  systemPrefersDark.value = mediaQuery.matches;
  mediaQuery.addEventListener("change", (e) => {
    systemPrefersDark.value = e.matches;
  });
});
</script>

<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="naiveThemeOverrides">
    <router-view />
  </n-config-provider>
</template>
