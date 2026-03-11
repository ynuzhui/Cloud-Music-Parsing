<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import { createDiscreteApi, darkTheme, type MessageReactive } from "naive-ui";
import { Music, Headphones, Settings, Download, FileText, Photo } from "@vicons/tabler";
import {
  parseMusic,
  searchSong,
  fetchPlaylist,
  fetchLyric,
  downloadLyricAsset,
  downloadCoverAsset,
  type ParseResult,
  type SearchSongItem,
  type PlaylistInfo,
  type LyricResult,
} from "@/api/modules/music";
import { getPublicSiteSettings } from "@/api/modules/site";
import { getUserQuotaToday, getUserUsageTrend, type UserQuotaToday, type UserUsageTrend } from "@/api/modules/user";
import { useAuthStore } from "@/stores/auth";
import { useSettingsStore } from "@/stores/settings";
import { useRouter } from "vue-router";
import APlayer from "aplayer";
import { ID3Writer } from "browser-id3-writer";
import { writeFlacMetadata } from "@/utils/flacMetadata";
import "aplayer/dist/APlayer.min.css";

const router = useRouter();
const authStore = useAuthStore();
const settingsStore = useSettingsStore();
const { message } = createDiscreteApi(["message"]);
const ICP_RECORD_LINK = "https://beian.miit.gov.cn/";
const POLICE_RECORD_BASE_LINK = "https://beian.mps.gov.cn/#/query/webSearch";

type TabMode = "song" | "id" | "playlist";
const activeTab = ref<TabMode>("song");

const songKeyword = ref("");
const searching = ref(false);
const searchResults = ref<SearchSongItem[]>([]);
const hasSearched = ref(false);
const parsing = ref(false);
const parseResult = ref<ParseResult | null>(null);

const idInput = ref("");
const idParsing = ref(false);

const playlistId = ref("");
const loadingPlaylist = ref(false);
const playlistInfo = ref<PlaylistInfo | null>(null);
const playlistParsing = ref<Record<number, boolean>>({});
const playlistResults = ref<Record<number, ParseResult>>({});

const downloading = ref(false);
const lyricDownloading = ref(false);
const coverDownloading = ref(false);
const showSettings = ref(false);
const parseResultRef = ref<HTMLElement | null>(null);
const footerRecord = ref<{ icpNo: string; policeNo: string }>({
  icpNo: "",
  policeNo: "",
});
const hasFooterRecord = computed(() => !!(footerRecord.value.icpNo || footerRecord.value.policeNo));
const parseRequireLogin = ref(true);
const quotaLoading = ref(false);
const quotaToday = ref<UserQuotaToday | null>(null);
const quotaTrend = ref<UserUsageTrend["items"]>([]);
const policeRecordLink = computed(() => {
  const no = (footerRecord.value.policeNo || "").trim();
  if (!no) return POLICE_RECORD_BASE_LINK;
  return `${POLICE_RECORD_BASE_LINK}?police=${encodeURIComponent(no)}`;
});

const systemPrefersDark = ref(false);
let mediaQuery: MediaQueryList | null = null;
let mediaQueryListener: ((event: MediaQueryListEvent) => void) | null = null;

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

const quality = computed({
  get: () => settingsStore.quality,
  set: (v: string) => settingsStore.setQuality(v),
});

const qualityOptions = [
  { label: "超清母带", value: "jymaster", short: "超清母带" },
  { label: "高解析度无损", value: "hires", short: "Hi-Res" },
  { label: "无损", value: "lossless", short: "无损" },
  { label: "极高", value: "exhigh", short: "极高" },
  { label: "标准", value: "standard", short: "标准" },
];

const aplayerRef = ref<HTMLElement | null>(null);
let aplayerInstance: any = null;
let topLoadingMessage: MessageReactive | null = null;

function showTopLoading(content: string) {
  if (topLoadingMessage) topLoadingMessage.destroy();
  topLoadingMessage = message.loading(content, { duration: 0, closable: false });
}

function hideTopLoading() {
  if (!topLoadingMessage) return;
  topLoadingMessage.destroy();
  topLoadingMessage = null;
}

function scrollToParseResult() {
  if (!parseResultRef.value) return;
  parseResultRef.value.scrollIntoView({ behavior: "smooth", block: "start" });
}

function requireAuth(): boolean {
  if (!parseRequireLogin.value) {
    return true;
  }
  if (!authStore.isAuthed) {
    message.warning("请先登录后再使用");
    router.push("/login");
    return false;
  }
  return true;
}

async function loadUserQuota() {
  if (!authStore.isAuthed) {
    quotaToday.value = null;
    quotaTrend.value = [];
    return;
  }
  quotaLoading.value = true;
  try {
    const [today, trend] = await Promise.all([getUserQuotaToday(), getUserUsageTrend(7)]);
    quotaToday.value = today;
    quotaTrend.value = trend.items || [];
  } catch {
    quotaToday.value = null;
    quotaTrend.value = [];
  } finally {
    quotaLoading.value = false;
  }
}

async function onSearch() {
  if (!requireAuth()) return;
  if (searching.value) return;
  const keyword = songKeyword.value.trim();
  if (!keyword) {
    message.warning("请输入歌曲名称");
    return;
  }
  searching.value = true;
  showTopLoading("正在搜索...");
  hasSearched.value = true;
  searchResults.value = [];
  parseResult.value = null;
  destroyPlayer();
  try {
    searchResults.value = await searchSong(keyword, 20);
    settingsStore.addSearchHistory(keyword);
    if (searchResults.value.length === 0) message.info("未找到相关歌曲");
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    searching.value = false;
    hideTopLoading();
  }
}

async function onParseSong(songId: number) {
  if (!requireAuth()) return;
  if (parsing.value) return;
  parsing.value = true;
  showTopLoading("正在解析歌曲...");
  parseResult.value = null;
  destroyPlayer();
  try {
    parseResult.value = await parseMusic(String(songId), quality.value);
    if (authStore.isAuthed) {
      void loadUserQuota();
    }
    message.success("解析成功");
    await nextTick();
    initPlayer();
    scrollToParseResult();
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    parsing.value = false;
    hideTopLoading();
  }
}

async function onParseById() {
  if (!requireAuth()) return;
  if (idParsing.value) return;
  const input = idInput.value.trim();
  if (!input) {
    message.warning("请输入歌曲 ID 或链接");
    return;
  }
  idParsing.value = true;
  showTopLoading("正在解析歌曲...");
  parseResult.value = null;
  destroyPlayer();
  try {
    parseResult.value = await parseMusic(input, quality.value);
    if (authStore.isAuthed) {
      void loadUserQuota();
    }
    message.success("解析成功");
    await nextTick();
    initPlayer();
    scrollToParseResult();
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    idParsing.value = false;
    hideTopLoading();
  }
}

async function onLoadPlaylist() {
  if (!requireAuth()) return;
  if (loadingPlaylist.value) return;
  const input = playlistId.value.trim();
  if (!input) {
    message.warning("请输入歌单 ID 或链接");
    return;
  }
  loadingPlaylist.value = true;
  showTopLoading("正在加载歌单...");
  playlistInfo.value = null;
  playlistResults.value = {};
  destroyPlayer();
  try {
    playlistInfo.value = await fetchPlaylist(input);
    message.success(`歌单已加载：${playlistInfo.value.name}（${playlistInfo.value.tracks.length} 首）`);
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    loadingPlaylist.value = false;
    hideTopLoading();
  }
}

async function onParseTrack(trackId: number) {
  if (!requireAuth()) return;
  if (playlistParsing.value[trackId]) return;
  playlistParsing.value[trackId] = true;
  showTopLoading("正在解析歌曲...");
  parseResult.value = null;
  destroyPlayer();
  try {
    const result = await parseMusic(String(trackId), quality.value);
    if (authStore.isAuthed) {
      void loadUserQuota();
    }
    playlistResults.value[trackId] = result;
    parseResult.value = result;
    message.success("解析成功");
    await nextTick();
    initPlayer();
    scrollToParseResult();
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    playlistParsing.value[trackId] = false;
    hideTopLoading();
  }
}

function playTrack(result: ParseResult) {
  destroyPlayer();
  parseResult.value = result;
  nextTick(() => {
    initPlayer();
    scrollToParseResult();
  });
}

function initPlayer() {
  if (!aplayerRef.value || !parseResult.value) return;
  const current = parseResult.value;
  aplayerInstance = new APlayer({
    container: aplayerRef.value,
    audio: [{
      name: current.song_name || `歌曲 ${current.song_id}`,
      artist: current.artist_name || "未知歌手",
      url: current.stream_url,
      cover: current.cover_url || "",
    }],
    theme: "#0f6fff",
    autoplay: false,
    volume: 0.8,
  });
}

function destroyPlayer() {
  if (aplayerInstance) {
    aplayerInstance.destroy();
    aplayerInstance = null;
  }
}

function detectAudioFormat(buffer: ArrayBuffer, url: string, contentType: string): "flac" | "mp3" {
  const bytes = new Uint8Array(buffer.slice(0, 4));
  if (bytes[0] === 0x66 && bytes[1] === 0x4c && bytes[2] === 0x61 && bytes[3] === 0x43) return "flac";
  const type = contentType.toLowerCase();
  if (type.includes("flac")) return "flac";
  if (url.toLowerCase().includes(".flac")) return "flac";
  return "mp3";
}

function mergeLyrics(lyric: string, tlyric: string): string {
  const main = normalizeNeteaseLyric(lyric);
  const trans = normalizeNeteaseLyric(tlyric);
  if (!main && !trans) return "";
  if (!main) return trans;
  if (!trans) return main;
  return `${main}\n\n[Translation]\n${trans}`;
}

function formatLrcTimestamp(ms: number): string {
  const safe = Number.isFinite(ms) && ms > 0 ? Math.floor(ms) : 0;
  const minutes = Math.floor(safe / 60000);
  const seconds = Math.floor((safe % 60000) / 1000);
  const milli = safe % 1000;
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}.${String(milli).padStart(3, "0")}`;
}

function normalizeNeteaseLyric(raw: string): string {
  const source = (raw || "").replace(/\r/g, "");
  if (!source.trim()) return "";

  const lines = source.split("\n");
  const output: string[] = [];
  for (const rawLine of lines) {
    const line = rawLine.trim();
    if (!line) {
      output.push("");
      continue;
    }
    if (line.startsWith("{") && line.endsWith("}")) {
      try {
        const obj = JSON.parse(line) as { t?: number; c?: Array<{ tx?: string }> };
        if (typeof obj?.t === "number" && Array.isArray(obj.c)) {
          const text = obj.c.map((p) => (typeof p?.tx === "string" ? p.tx : "")).join("").trim();
          output.push(`[${formatLrcTimestamp(obj.t)}]${text}`);
          continue;
        }
      } catch {
        // Keep original line when parsing fails.
      }
    }
    output.push(rawLine);
  }
  return output.join("\n").replace(/\n{3,}/g, "\n\n").trim();
}

function buildLyricText(lyricResult: LyricResult | null): string {
  if (!lyricResult) return "";
  return mergeLyrics(lyricResult.lyric || "", lyricResult.tlyric || "");
}

async function safeFetchCover(coverUrl: string): Promise<{ buffer: ArrayBuffer; mime: string } | null> {
  const url = (coverUrl || "").trim();
  if (!url) return null;
  try {
    const resp = await fetch(url);
    if (!resp.ok) return null;
    return { buffer: await resp.arrayBuffer(), mime: resp.headers.get("content-type") || "image/jpeg" };
  } catch {
    return null;
  }
}

async function safeFetchLyric(songId: string): Promise<LyricResult | null> {
  try {
    return await fetchLyric(songId);
  } catch {
    return null;
  }
}

function buildMp3WithMetadata(
  audioBuffer: ArrayBuffer,
  metadata: {
    title: string;
    artist: string;
    album: string;
    albumArtist?: string;
    year?: number;
    trackNumber?: number;
    trackTotal?: number;
    discNumber?: number;
    lyric: string;
    cover?: { buffer: ArrayBuffer; mime: string } | null;
  },
): Blob {
  const writer = new ID3Writer(audioBuffer);
  const writerAny = writer as any;
  const safeSetFrame = (frame: string, value: unknown) => {
    try {
      writerAny.setFrame(frame, value);
    } catch {
      // Ignore unsupported optional frames for compatibility.
    }
  };
  if (metadata.title) writer.setFrame("TIT2", metadata.title);
  if (metadata.artist) writer.setFrame("TPE1", [metadata.artist]);
  if (metadata.album) writer.setFrame("TALB", metadata.album);
  if (metadata.albumArtist) safeSetFrame("TPE2", [metadata.albumArtist]);
  if (metadata.year && metadata.year > 0) {
    const yearText = String(metadata.year);
    safeSetFrame("TDRC", yearText);
    safeSetFrame("TYER", yearText);
  }
  if (metadata.trackNumber && metadata.trackNumber > 0) {
    let trackText = String(metadata.trackNumber);
    if (metadata.trackTotal && metadata.trackTotal > 0) {
      trackText = `${metadata.trackNumber}/${metadata.trackTotal}`;
    }
    safeSetFrame("TRCK", trackText);
  }
  if (metadata.discNumber && metadata.discNumber > 0) {
    safeSetFrame("TPOS", String(metadata.discNumber));
  }
  if (metadata.lyric) writer.setFrame("USLT", { language: "chi", description: "", lyrics: metadata.lyric });
  if (metadata.cover?.buffer) {
    writer.setFrame("APIC", { type: 3, data: metadata.cover.buffer, description: "Cover" });
  }
  writer.addTag();
  return writer.getBlob();
}

function triggerDownload(blob: Blob, fileName: string) {
  const objectUrl = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = objectUrl;
  anchor.download = fileName;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  setTimeout(() => URL.revokeObjectURL(objectUrl), 2000);
}

function sanitizeFileName(rawName: string): string {
  return rawName.replace(/[\\/:*?"<>|]/g, "_").trim();
}

async function onDownloadCurrent() {
  if (!requireAuth()) return;
  if (downloading.value) return;
  if (!parseResult.value) {
    message.warning("请先解析歌曲");
    return;
  }
  downloading.value = true;
  try {
    const current = parseResult.value;
    const songName = current.song_name || `歌曲 ${current.song_id}`;
    const artistName = current.artist_name || "未知歌手";
    const albumName = current.album_name || "未知专辑";
    const albumArtist = (current.album_artist || artistName || "").trim();
    const year = Number.isFinite(current.year) ? Number(current.year) : 0;
    const trackNumber = Number.isFinite(current.track_number) ? Number(current.track_number) : 0;
    const trackTotal = Number.isFinite(current.track_total) ? Number(current.track_total) : 0;
    const discNumber = Number.isFinite(current.disc_number) ? Number(current.disc_number) : 0;

    const audioResp = await fetch(current.stream_url);
    if (!audioResp.ok) throw new Error(`下载音频失败（HTTP ${audioResp.status}）`);

    const originalBuffer = await audioResp.arrayBuffer();
    const format = detectAudioFormat(originalBuffer, current.stream_url, audioResp.headers.get("content-type") || "");

    let finalBlob = new Blob([originalBuffer], { type: format === "flac" ? "audio/flac" : "audio/mpeg" });
    let lyricResult: LyricResult | null = null;
    let coverResult: { buffer: ArrayBuffer; mime: string } | null = null;

    if (settingsStore.writeMetadata || settingsStore.zipPackageDownload) {
      [lyricResult, coverResult] = await Promise.all([
        safeFetchLyric(current.song_id),
        safeFetchCover(current.cover_url),
      ]);
    }

    const lyrics = buildLyricText(lyricResult);

    if (settingsStore.writeMetadata) {
      try {
        if (format === "flac") {
          const tagged = writeFlacMetadata(originalBuffer, {
            title: songName,
            artist: artistName,
            album: albumName,
            albumArtist,
            year,
            trackNumber,
            trackTotal,
            discNumber,
            lyrics,
            coverData: coverResult?.buffer,
            coverMime: coverResult?.mime,
          });
          finalBlob = new Blob([tagged], { type: "audio/flac" });
        } else {
          finalBlob = buildMp3WithMetadata(originalBuffer, {
            title: songName,
            artist: artistName,
            album: albumName,
            albumArtist,
            year,
            trackNumber,
            trackTotal,
            discNumber,
            lyric: lyrics,
            cover: coverResult,
          });
        }
      } catch {
        message.warning("元数据写入失败，已回退为原始音频下载");
      }
    }

    const ext = format === "flac" ? "flac" : "mp3";
    const audioFileName = sanitizeFileName(settingsStore.buildFileName(songName, artistName, ext)) || `track.${ext}`;

    if (settingsStore.zipPackageDownload) {
      const { default: JSZip } = await import("jszip");
      const zip = new JSZip();
      const baseName = audioFileName.replace(/\.[^.]+$/, "");
      zip.file(audioFileName, finalBlob);
      if (coverResult?.buffer && coverResult.buffer.byteLength > 0) {
        zip.file(`${baseName}.jpg`, coverResult.buffer);
      }
      if (lyrics.trim()) {
        zip.file(`${baseName}.lrc`, lyrics);
      }
      const zipBlob = await zip.generateAsync({
        type: "blob",
        compression: "DEFLATE",
        compressionOptions: { level: 6 },
      });
      triggerDownload(zipBlob, sanitizeFileName(`${baseName}.zip`) || "track.zip");
      message.success("ZIP 下载任务已触发");
      return;
    }

    triggerDownload(finalBlob, audioFileName);
    message.success("下载任务已触发");
  } catch (error) {
    message.error((error as Error).message || "下载失败");
  } finally {
    downloading.value = false;
  }
}

async function onDownloadLyric() {
  if (!requireAuth()) return;
  if (lyricDownloading.value) return;
  if (!parseResult.value) {
    message.warning("请先解析歌曲");
    return;
  }

  lyricDownloading.value = true;
  try {
    const current = parseResult.value;
    const { blob, fileName } = await downloadLyricAsset(current.song_id);
    triggerDownload(blob, sanitizeFileName(fileName) || "track.lrc");
    message.success("歌词下载已触发");
  } catch (error) {
    message.error((error as Error).message || "下载歌词失败");
  } finally {
    lyricDownloading.value = false;
  }
}

async function onDownloadCover() {
  if (!requireAuth()) return;
  if (coverDownloading.value) return;
  if (!parseResult.value) {
    message.warning("请先解析歌曲");
    return;
  }

  coverDownloading.value = true;
  try {
    const current = parseResult.value;
    const { blob, fileName } = await downloadCoverAsset(current.song_id);
    triggerDownload(blob, sanitizeFileName(fileName) || "cover.jpg");
    message.success("封面下载已触发");
  } catch (error) {
    message.error((error as Error).message || "下载封面失败");
  } finally {
    coverDownloading.value = false;
  }
}

onMounted(async () => {
  settingsStore.initThemeListener();
  mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
  systemPrefersDark.value = mediaQuery.matches;
  mediaQueryListener = (event: MediaQueryListEvent) => {
    systemPrefersDark.value = event.matches;
  };
  mediaQuery.addEventListener("change", mediaQueryListener);

  try {
    const site = await getPublicSiteSettings();
    settingsStore.syncSiteName(site?.name);
    footerRecord.value.icpNo = (site?.icp_no || "").trim();
    footerRecord.value.policeNo = (site?.police_no || "").trim();
    parseRequireLogin.value = site?.parse_require_login !== false;
    settingsStore.applyDocumentTitle();
  } catch {
    // ignore
  }
  if (authStore.isAuthed) {
    void loadUserQuota();
  }
});

onUnmounted(() => {
  destroyPlayer();
  hideTopLoading();
  if (mediaQuery && mediaQueryListener) {
    mediaQuery.removeEventListener("change", mediaQueryListener);
  }
});
</script>

<template>
  <n-config-provider :theme="naiveTheme" :theme-overrides="naiveThemeOverrides">
    <main class="page-shell home-shell">
      <section class="home-center">
        <header class="home-header glass-card">
          <div class="header-text">
            <p class="eyebrow">MUSIC PARSER</p>
            <h1>音乐聚合解析</h1>
            <p class="header-desc">支持网易云音乐全品质解析，浏览器端完成下载与元数据写入。</p>
          </div>
          <button class="settings-btn" title="网站设置" @click="showSettings = true">
            <n-icon size="20"><Settings /></n-icon>
          </button>
        </header>

        <div class="method-card glass-card">
          <div class="card-title"><n-icon size="16" color="var(--brand)"><Headphones /></n-icon><span>解析方式</span></div>
          <div class="method-options">
            <label class="method-option" :class="{ active: activeTab === 'song' }"><input v-model="activeTab" type="radio" value="song" /><span>搜索解析</span></label>
            <label class="method-option" :class="{ active: activeTab === 'id' }"><input v-model="activeTab" type="radio" value="id" /><span>ID/链接解析</span></label>
            <label class="method-option" :class="{ active: activeTab === 'playlist' }"><input v-model="activeTab" type="radio" value="playlist" /><span>歌单解析</span></label>
          </div>
        </div>

        <div class="quality-card glass-card">
          <div class="card-title"><n-icon size="16" color="var(--brand)"><Headphones /></n-icon><span>全局音质选择</span></div>
          <div class="quality-tags">
            <button v-for="opt in qualityOptions" :key="opt.value" :class="['quality-tag', { active: quality === opt.value }]" @click="quality = opt.value">{{ opt.short }}</button>
          </div>
        </div>

        <div v-if="authStore.isAuthed" class="quota-card glass-card">
          <div class="card-title"><n-icon size="16" color="var(--brand)"><Headphones /></n-icon><span>今日额度</span></div>
          <n-spin :show="quotaLoading">
            <div v-if="quotaToday" class="quota-grid">
              <div class="quota-item">
                <span class="quota-label">已用次数</span>
                <strong>{{ quotaToday.used }}</strong>
              </div>
              <div class="quota-item">
                <span class="quota-label">每日上限</span>
                <strong>{{ quotaToday.daily_limit <= 0 ? "无限制" : quotaToday.daily_limit }}</strong>
              </div>
              <div class="quota-item">
                <span class="quota-label">剩余次数</span>
                <strong>{{ quotaToday.remaining < 0 ? "无限制" : quotaToday.remaining }}</strong>
              </div>
              <div class="quota-item">
                <span class="quota-label">并发占用</span>
                <strong>{{ quotaToday.in_flight }} / {{ quotaToday.concurrency_limit <= 0 ? "无限制" : quotaToday.concurrency_limit }}</strong>
              </div>
            </div>
            <div v-if="quotaTrend.length" class="trend-wrap">
              <div class="trend-title">近 7 天解析次数（北京时间）</div>
              <div class="trend-list">
                <div v-for="row in quotaTrend" :key="row.day" class="trend-row">
                  <span class="trend-day">{{ row.day.slice(5) }}</span>
                  <div class="trend-bar"><i :style="{ width: `${Math.max(6, row.count * 12)}px` }"></i></div>
                  <span class="trend-count">{{ row.count }}</span>
                </div>
              </div>
            </div>
          </n-spin>
        </div>

        <div class="main-card glass-card">
          <template v-if="activeTab === 'song'">
            <div class="form-zone">
              <div class="query-row">
                <n-input class="query-input" v-model:value="songKeyword" placeholder="输入歌曲名称搜索" size="large" clearable @keydown.enter="onSearch" />
                <n-button class="query-submit-btn" type="primary" size="large" :disabled="searching" @click="onSearch" style="width: 100px">搜索</n-button>
              </div>
            </div>
            <div v-if="hasSearched" class="song-list-wrap">
              <n-empty v-if="!searching && searchResults.length === 0" description="暂无搜索结果" />
              <div v-else-if="searchResults.length > 0" class="song-list">
                <div v-for="song in searchResults" :key="song.id" class="song-item" @click="onParseSong(song.id)">
                  <img v-if="song.cover_url" :src="song.cover_url" class="cover" alt="cover" referrerpolicy="no-referrer" />
                  <div v-else class="cover cover-empty"><n-icon size="18"><Music /></n-icon></div>
                  <div class="song-info"><span class="song-name">{{ song.name }}</span><span class="song-meta">{{ song.artists.join(" / ") }} · {{ song.album }}</span></div>
                  <n-button size="small" type="primary" :disabled="parsing" @click.stop="onParseSong(song.id)">解析</n-button>
                </div>
              </div>
            </div>
          </template>

          <template v-if="activeTab === 'id'">
            <div class="form-zone">
              <div class="query-row">
                <n-input class="query-input" v-model:value="idInput" placeholder="输入歌曲 ID 或分享链接" size="large" clearable @keydown.enter="onParseById" />
                <n-button class="query-submit-btn" type="primary" size="large" :disabled="idParsing" @click="onParseById" style="width: 120px">立即解析</n-button>
              </div>
            </div>
          </template>

          <template v-if="activeTab === 'playlist'">
            <div class="form-zone">
              <div class="query-row">
                <n-input class="query-input" v-model:value="playlistId" placeholder="输入歌单 ID 或分享链接" size="large" clearable @keydown.enter="onLoadPlaylist" />
                <n-button class="query-submit-btn" type="primary" size="large" :disabled="loadingPlaylist" @click="onLoadPlaylist" style="width: 120px">加载歌单</n-button>
              </div>
            </div>
            <div v-if="playlistInfo" class="song-list-wrap">
              <div class="playlist-header"><strong>{{ playlistInfo.name }}</strong><n-tag size="small">共 {{ playlistInfo.tracks.length }} 首</n-tag></div>
              <div class="song-list">
                <div v-for="(track, idx) in playlistInfo.tracks" :key="track.id" class="song-item">
                  <span class="song-index">{{ idx + 1 }}</span>
                  <img v-if="track.cover_url" :src="track.cover_url" class="cover cover-sm" alt="cover" referrerpolicy="no-referrer" />
                  <div v-else class="cover cover-empty cover-sm"><n-icon size="14"><Music /></n-icon></div>
                  <div class="song-info"><span class="song-name">{{ track.name }}</span><span class="song-meta">{{ track.artists.join(" / ") }} · {{ track.album }}</span></div>
                  <n-button v-if="playlistResults[track.id]" size="small" type="success" @click="playTrack(playlistResults[track.id])">播放</n-button>
                  <n-button v-else size="small" type="primary" :disabled="!!playlistParsing[track.id]" @click="onParseTrack(track.id)">解析</n-button>
                </div>
              </div>
            </div>
          </template>

        </div>

        <transition name="fade-up">
          <div v-if="parseResult" ref="parseResultRef" class="result-card glass-card">
            <div class="result-header"><n-icon size="20" color="var(--ok)"><Music /></n-icon><strong>解析结果</strong><n-tag :type="parseResult.cache_hit ? 'warning' : 'success'" size="small">{{ parseResult.cache_hit ? "缓存" : "实时" }}</n-tag></div>
            <div class="result-grid">
              <span>歌曲：{{ parseResult.song_name || `歌曲 ${parseResult.song_id}` }}</span>
              <span>歌手：{{ parseResult.artist_name || "未知歌手" }}</span>
              <span>专辑：{{ parseResult.album_name || "未知专辑" }}</span>
              <span>音质：{{ qualityOptions.find((q) => q.value === parseResult?.quality)?.label || parseResult.quality }}</span>
            </div>
            <div ref="aplayerRef" class="aplayer-box"></div>
            <div class="result-actions">
              <n-button type="primary" size="small" :loading="downloading" @click="onDownloadCurrent"><template #icon><n-icon><Download /></n-icon></template>下载音频</n-button>
              <n-button type="primary" size="small" :loading="lyricDownloading" @click="onDownloadLyric"><template #icon><n-icon><FileText /></n-icon></template>下载歌词</n-button>
              <n-button type="primary" size="small" :loading="coverDownloading" @click="onDownloadCover"><template #icon><n-icon><Photo /></n-icon></template>下载封面</n-button>
            </div>
          </div>
        </transition>

        <footer class="home-footer">
          <span class="home-footer-main">
            <span>仅供学习交流使用 · 请支持正版音乐</span>
            <span class="footer-sep">|</span>
            <span>Copyright © 2026</span>
            <a class="footer-author-link" href="https://yunzhui.top" target="_blank" rel="noopener noreferrer">云坠</a>
          </span>
          <div v-if="hasFooterRecord" class="record-row">
            <a v-if="footerRecord.icpNo" class="record-link" :href="ICP_RECORD_LINK" target="_blank" rel="noopener noreferrer">{{ footerRecord.icpNo }}</a>
            <span v-if="footerRecord.icpNo && footerRecord.policeNo" class="record-sep">|</span>
            <a v-if="footerRecord.policeNo" class="record-link" :href="policeRecordLink" target="_blank" rel="noopener noreferrer">{{ footerRecord.policeNo }}</a>
          </div>
        </footer>
      </section>

      <transition name="fade-up">
        <div v-if="showSettings" class="settings-overlay" @click.self="showSettings = false">
          <section class="settings-modal">
            <header class="settings-header"><h3>网站设置</h3><button class="close-btn" @click="showSettings = false">×</button></header>
            <div class="setting-block">
              <h4>主题设置</h4>
              <div class="row-options">
                <label :class="['opt', { active: settingsStore.theme === 'system' }]"><input type="radio" :checked="settingsStore.theme === 'system'" @change="settingsStore.setTheme('system')" />跟随系统</label>
                <label :class="['opt', { active: settingsStore.theme === 'light' }] "><input type="radio" :checked="settingsStore.theme === 'light'" @change="settingsStore.setTheme('light')" />浅色</label>
                <label :class="['opt', { active: settingsStore.theme === 'dark' }] "><input type="radio" :checked="settingsStore.theme === 'dark'" @change="settingsStore.setTheme('dark')" />深色</label>
              </div>
            </div>
            <div class="setting-block">
              <h4>文件命名格式</h4>
              <div class="row-options">
                <label :class="['opt', { active: settingsStore.fileNameFormat === 'songArtist' }]"><input type="radio" :checked="settingsStore.fileNameFormat === 'songArtist'" @change="settingsStore.setFileNameFormat('songArtist')" />歌曲名 - 歌手名</label>
                <label :class="['opt', { active: settingsStore.fileNameFormat === 'artistSong' }]"><input type="radio" :checked="settingsStore.fileNameFormat === 'artistSong'" @change="settingsStore.setFileNameFormat('artistSong')" />歌手名 - 歌曲名</label>
              </div>
            </div>
            <div class="setting-block">
              <div class="meta-title"><h4>写入歌曲元数据</h4><div class="meta-right"><n-tag size="small" type="success">推荐</n-tag><n-switch :value="settingsStore.writeMetadata" @update:value="(v: boolean) => settingsStore.setWriteMetadata(v)" /></div></div>
              <p class="meta-help">开启后，下载时将在浏览器端写入歌曲封面、歌手与专辑信息、歌词，支持 MP3 与 FLAC。</p>
            </div>
            <div class="setting-block">
              <div class="meta-title"><h4>ZIP打包下载</h4><div class="meta-right"><n-tag size="small" type="info">仅支持单曲解析</n-tag><n-switch :value="settingsStore.zipPackageDownload" @update:value="(v: boolean) => settingsStore.setZipPackageDownload(v)" /></div></div>
              <p class="meta-help">开启后将打包歌曲文件、封面图片（JPG）和歌词文件（LRC）为 ZIP 格式下载。</p>
            </div>
          </section>
        </div>
      </transition>
    </main>
  </n-config-provider>
</template>

<style scoped>
.home-shell { display: flex; justify-content: center; min-height: 100vh; padding: 32px 18px 8px; }
.home-center { width: min(920px, 96%); display: flex; flex-direction: column; gap: 14px; min-height: calc(100vh - 60px); }
.home-header { display: flex; justify-content: space-between; gap: 16px; padding: 24px; background: linear-gradient(160deg, rgba(11,83,206,.92), rgba(13,121,198,.88)); color: #fff; }
.header-text h1 { margin: 6px 0; }
.eyebrow { margin: 0; letter-spacing: .2em; font-size: 12px; }
.header-desc { margin: 0; opacity: .92; font-size: 13px; }
.settings-btn { width: 36px; height: 36px; display: grid; place-items: center; border-radius: 10px; border: 1px solid rgba(255,255,255,.2); background: rgba(255,255,255,.12); color: #fff; cursor: pointer; }
.method-card,.quality-card,.main-card,.result-card { padding: 18px 22px; }
.quota-card { padding: 18px 22px; }
.card-title { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; font-weight: 700; color: var(--text-1); font-size: 18px; }
.quota-grid { display: grid; grid-template-columns: repeat(4, minmax(0,1fr)); gap: 8px; margin-bottom: 10px; }
.quota-item { border: 1px solid var(--line-soft); border-radius: 12px; padding: 10px 12px; background: var(--song-item-bg); display: flex; flex-direction: column; gap: 4px; }
.quota-label { font-size: 12px; color: var(--text-2); }
.trend-wrap { margin-top: 8px; }
.trend-title { font-size: 12px; color: var(--text-2); margin-bottom: 6px; }
.trend-list { display: flex; flex-direction: column; gap: 6px; }
.trend-row { display: grid; grid-template-columns: 44px 1fr 40px; gap: 8px; align-items: center; }
.trend-day { font-size: 12px; color: var(--text-2); }
.trend-bar { height: 8px; border-radius: 6px; background: rgba(15,111,255,.12); overflow: hidden; }
.trend-bar i { display: block; height: 100%; background: linear-gradient(90deg, #0f6fff, #29a2ff); border-radius: 6px; }
.trend-count { font-size: 12px; color: var(--text-1); text-align: right; }
.method-options { display: flex; gap: 10px; flex-wrap: wrap; }
.method-option { flex: 1; min-width: 120px; padding: 10px 12px; border-radius: 12px; border: 1px solid var(--line-soft); background: var(--tag-bg); color: var(--text-2); text-align: center; cursor: pointer; font-weight: 600; font-size: 18px; }
.method-option input { display: none; }
.method-option.active { border-color: var(--brand); background: var(--brand-soft); color: var(--brand); }
.quality-tags { display: flex; flex-wrap: wrap; gap: 8px; }
.quality-tag { padding: 6px 14px; border-radius: 20px; border: 1px solid var(--line-soft); background: var(--tag-bg); color: var(--tag-text); font-size: 13px; font-weight: 600; cursor: pointer; font-family: inherit; }
.quality-tag.active { background: var(--tag-active-bg); color: var(--tag-active-text); border-color: var(--tag-active-bg); }
.form-zone { margin-bottom: 12px; }
.query-row { display: flex; align-items: stretch; gap: 10px; }
.query-input { flex: 1; min-width: 0; }
.query-input :deep(.n-input-wrapper) { border-radius: 18px !important; }
.query-submit-btn { border-radius: 18px !important; }
.song-list-wrap { margin-top: 12px; }
.song-list { display: flex; flex-direction: column; gap: 6px; max-height: 360px; overflow-y: auto; }
.song-item { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-radius: 10px; border: 1px solid var(--song-item-border); background: var(--song-item-bg); }
.song-index { width: 18px; text-align: center; font-size: 12px; color: var(--text-2); }
.cover { width: 48px; height: 48px; border-radius: 10px; object-fit: cover; flex-shrink: 0; }
.cover-sm { width: 40px; height: 40px; }
.cover-empty { display: grid; place-items: center; background: var(--brand-soft); color: var(--brand); }
.song-info { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.song-name,.song-meta { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.song-name { font-size: 14px; font-weight: 600; }
.song-meta { font-size: 12px; color: var(--text-2); }
.playlist-header { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.result-header { display: flex; align-items: center; gap: 8px; margin-bottom: 8px; }
.result-grid { display: grid; grid-template-columns: repeat(2, minmax(0,1fr)); gap: 6px 14px; margin-bottom: 12px; color: var(--text-2); font-size: 13px; }
.aplayer-box { border-radius: 12px; overflow: hidden; }
.result-actions { display: flex; justify-content: center; flex-wrap: wrap; gap: 8px; margin-top: 10px; }
.home-footer { margin-top: auto; text-align: center; padding: 6px 0 0; color: var(--text-2); font-size: 13px; }
.home-footer-main { display: inline-flex; align-items: center; gap: 8px; }
.footer-sep { margin: 0; opacity: .76; }
.footer-author-link { color: var(--text-2); text-decoration: none; transition: color .2s ease; }
.footer-author-link:hover { color: var(--brand); text-decoration: underline; }
.record-row { margin-top: 6px; display: flex; align-items: center; justify-content: center; gap: 8px; flex-wrap: wrap; font-size: 14px; line-height: 1.4; }
.record-link { color: var(--text-2); text-decoration: none; transition: color .2s ease; font-weight: 500; }
.record-link:hover { color: var(--brand); text-decoration: underline; }
.record-sep { color: var(--text-2); opacity: .72; }
.settings-overlay { position: fixed; inset: 0; display: grid; place-items: center; background: var(--settings-overlay); backdrop-filter: blur(6px); z-index: 50; padding: 16px; }
.settings-modal { width: min(520px, 100%); border-radius: 16px; border: 1px solid var(--settings-border); background: var(--settings-bg); backdrop-filter: blur(14px); padding: 16px; box-shadow: 0 20px 46px rgba(0,0,0,.28); }
.settings-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
.settings-header h3 { margin: 0; color: var(--text-1); }
.close-btn { width: 32px; height: 32px; border-radius: 8px; border: 1px solid var(--line-soft); background: transparent; color: var(--text-2); font-size: 20px; cursor: pointer; }
.setting-block { border-top: 1px solid var(--line-soft); padding: 10px 2px; }
.setting-block:first-of-type { border-top: none; }
.setting-block h4 { margin: 0 0 8px; color: var(--text-1); font-size: 14px; }
.row-options { display: flex; flex-wrap: wrap; gap: 8px; }
.opt { flex: 1; min-width: 140px; border: 1px solid var(--line-soft); border-radius: 10px; background: var(--tag-bg); padding: 9px 10px; text-align: center; color: var(--text-1); font-size: 13px; font-weight: 600; cursor: pointer; }
.opt input { display: none; }
.opt.active { border-color: var(--brand); background: var(--brand-soft); color: var(--brand); }
.meta-title { display: flex; justify-content: space-between; align-items: center; gap: 10px; }
.meta-title h4 { margin: 0; }
.meta-right { display: flex; align-items: center; gap: 8px; }
.meta-help { margin: 10px 0 0; color: var(--text-2); font-size: 12px; line-height: 1.5; }
.fade-up-enter-active,.fade-up-leave-active { transition: all .2s ease; }
.fade-up-enter-from,.fade-up-leave-to { opacity: 0; transform: translateY(8px); }
@media (max-width: 720px) {
  .home-shell { padding: 18px 14px 4px; }
  .home-center { min-height: calc(100vh - 46px); }
  .home-header { padding: 18px; }
  .main-card,.method-card,.quality-card,.result-card { padding: 16px; }
  .quota-card { padding: 16px; }
  .card-title { font-size: 18px; }
  .method-option { font-size: 16px; }
  .quality-tag { font-size: 13px; }
  .result-grid { grid-template-columns: 1fr; }
  .quota-grid { grid-template-columns: repeat(2, minmax(0,1fr)); }
}
</style>
