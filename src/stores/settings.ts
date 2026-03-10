import { defineStore } from "pinia";

export type ThemeMode = "system" | "light" | "dark";
export type FileNameFormat = "songArtist" | "artistSong";

interface SettingsState {
  siteName: string;
  theme: ThemeMode;
  quality: string;
  searchHistory: string[];
  fileNameFormat: FileNameFormat;
  writeMetadata: boolean;
  zipPackageDownload: boolean;
}

const STORAGE_KEY = "mp_settings";
const DEFAULT_SITE_NAME = "Cloud Music Parsing";

function normalizeSiteName(name: unknown): string {
  const value = typeof name === "string" ? name.trim() : "";
  return value || DEFAULT_SITE_NAME;
}

function loadFromStorage(): Partial<SettingsState> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch {
    return {};
  }
}

function saveToStorage(state: SettingsState) {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
}

function getEffectiveTheme(mode: ThemeMode): "light" | "dark" {
  if (mode === "system") {
    return window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light";
  }
  return mode;
}

export const useSettingsStore = defineStore("settings", {
  state: (): SettingsState => {
    const saved = loadFromStorage();
    return {
      siteName: normalizeSiteName(saved.siteName),
      theme: (saved.theme as ThemeMode) || "system",
      quality: saved.quality || "jymaster",
      searchHistory: saved.searchHistory || [],
      fileNameFormat: (saved.fileNameFormat as FileNameFormat) || "songArtist",
      writeMetadata: saved.writeMetadata !== undefined ? saved.writeMetadata : true,
      zipPackageDownload: saved.zipPackageDownload !== undefined ? saved.zipPackageDownload : false,
    };
  },

  actions: {
    setSiteName(name: string) {
      const normalized = normalizeSiteName(name);
      if (normalized === this.siteName) return;
      this.siteName = normalized;
      this._persist();
    },

    syncSiteName(name?: string | null) {
      if (name === undefined || name === null) return;
      const normalized = normalizeSiteName(name);
      if (normalized === this.siteName) return;
      this.siteName = normalized;
      this._persist();
    },

    buildDocumentTitle(_pageTitle?: string) {
      return this.siteName || DEFAULT_SITE_NAME;
    },

    applyDocumentTitle(_pageTitle?: string) {
      document.title = this.buildDocumentTitle();
    },

    applyTheme() {
      const effective = getEffectiveTheme(this.theme);
      document.documentElement.setAttribute("data-theme", effective);
    },

    setTheme(mode: ThemeMode) {
      this.theme = mode;
      this.applyTheme();
      this._persist();
    },

    setQuality(q: string) {
      this.quality = q;
      this._persist();
    },

    setFileNameFormat(fmt: FileNameFormat) {
      this.fileNameFormat = fmt;
      this._persist();
    },

    setWriteMetadata(val: boolean) {
      this.writeMetadata = val;
      this._persist();
    },

    setZipPackageDownload(val: boolean) {
      this.zipPackageDownload = val;
      this._persist();
    },

    addSearchHistory(keyword: string) {
      const trimmed = keyword.trim();
      if (!trimmed) return;
      this.searchHistory = [
        trimmed,
        ...this.searchHistory.filter((h) => h !== trimmed),
      ].slice(0, 10);
      this._persist();
    },

    clearSearchHistory() {
      this.searchHistory = [];
      this._persist();
    },

    initThemeListener() {
      this.applyTheme();
      window
        .matchMedia("(prefers-color-scheme: dark)")
        .addEventListener("change", () => {
          if (this.theme === "system") {
            this.applyTheme();
          }
        });
    },

    buildFileName(songName: string, artistName: string, ext: string): string {
      const s = songName || "未知歌曲";
      const a = artistName || "未知歌手";
      if (this.fileNameFormat === "artistSong") {
        return `${a} - ${s}.${ext}`;
      }
      return `${s} - ${a}.${ext}`;
    },

    _persist() {
      saveToStorage({
        theme: this.theme,
        siteName: this.siteName,
        quality: this.quality,
        searchHistory: this.searchHistory,
        fileNameFormat: this.fileNameFormat,
        writeMetadata: this.writeMetadata,
        zipPackageDownload: this.zipPackageDownload,
      });
    },
  },
});
