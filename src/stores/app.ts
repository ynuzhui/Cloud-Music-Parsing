import { defineStore } from "pinia";

type AppState = {
  installChecked: boolean;
  installed: boolean;
};

export const useAppStore = defineStore("app", {
  state: (): AppState => ({
    installChecked: false,
    installed: false
  }),
  actions: {
    setInstallState(installed: boolean) {
      this.installed = installed;
      this.installChecked = true;
    },
    clearInstallState() {
      this.installed = false;
      this.installChecked = false;
    }
  }
});
