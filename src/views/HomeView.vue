<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { createDiscreteApi, type MessageReactive } from "naive-ui";
import {
  Music,
  Headphones,
  Settings,
  Download,
  Login,
  UserCircle,
  PlayerPause,
  PlayerPlay,
  PlayerTrackNext,
  PlayerTrackPrev,
  Volume3,
  Vinyl,
  Repeat,
  RepeatOnce,
  ArrowsShuffle2,
  Maximize,
  Minimize,
} from "@vicons/tabler";
import {
  parseMusic,
  searchSong,
  fetchPlaylist,
  fetchLyric,
  type ParseResult,
  type SearchSongItem,
  type PlaylistInfo,
  type LyricResult,
} from "@/api/modules/music";
import { getPublicSiteSettings } from "@/api/modules/site";
import { useAuthStore } from "@/stores/auth";
import { useSettingsStore } from "@/stores/settings";
import { useRouter } from "vue-router";
import { ID3Writer } from "browser-id3-writer";
import { writeFlacMetadata } from "@/utils/flacMetadata";

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
type DownloadKind = "audio" | "lyric" | "cover";
type DownloadStage = "idle" | "downloading" | "processing" | "done" | "error";
type DownloadProgressState = {
  stage: DownloadStage;
  kind: DownloadKind;
  loadedBytes: number;
  totalBytes: number | null;
  fileName: string;
  detail: string;
};
const downloadProgress = ref<DownloadProgressState>({
  stage: "idle",
  kind: "audio",
  loadedBytes: 0,
  totalBytes: null,
  fileName: "",
  detail: "",
});
let downloadProgressResetTimer: ReturnType<typeof setTimeout> | null = null;
const showSettings = ref(false);
const parseResultRef = ref<HTMLElement | null>(null);
const footerRecord = ref<{ icpNo: string; policeNo: string }>({
  icpNo: "",
  policeNo: "",
});
const hasFooterRecord = computed(() => !!(footerRecord.value.icpNo || footerRecord.value.policeNo));
const parseRequireLogin = ref(true);
const policeRecordLink = computed(() => {
  const no = (footerRecord.value.policeNo || "").trim();
  if (!no) return POLICE_RECORD_BASE_LINK;
  return `${POLICE_RECORD_BASE_LINK}?police=${encodeURIComponent(no)}`;
});

const userMenuOptions = computed(() => {
  const items: Array<{ label: string; key: string }> = [];
  if (authStore.isAdmin) {
    items.push({ label: "管理后台", key: "dashboard" });
  }
  items.push({ label: "退出登录", key: "logout" });
  return items;
});

function onUserMenuSelect(key: string) {
  if (key === "dashboard") {
    router.push("/dashboard");
  } else if (key === "logout") {
    authStore.logout();
    message.success("已退出登录");
  }
}

const quality = computed({
  get: () => settingsStore.quality,
  set: (v: string) => settingsStore.setQuality(v),
});

const qualityOptions = [
  { label: "超清母带", value: "jymaster", short: "超清母带" },
  { label: "高清环绕声", value: "jyeffect", short: "高清环绕" },
  { label: "沉浸环绕声", value: "sky", short: "沉浸环绕" },
  { label: "高解析度无损", value: "hires", short: "Hi-Res" },
  { label: "无损", value: "lossless", short: "无损" },
  { label: "极高", value: "exhigh", short: "极高" },
  { label: "标准", value: "standard", short: "标准" },
];

type PlayContextType = "search" | "playlist" | "id";
type PlayContext = {
  type: PlayContextType;
  ids: string[];
  currentIndex: number;
};

type TimedLyricLine = {
  time: number;
  main: string;
  trans: string;
};

type LyricCacheEntry = {
  loading: boolean;
  loaded: boolean;
  raw: LyricResult | null;
  lines: TimedLyricLine[];
  merged: string;
};

type PlayMode = "single" | "list" | "shuffle";
type DownloadMenuKey = "audio" | "lyric" | "cover";
type PlayerTransitionPhase =
  | "opening-hide-card"
  | "opening-panel"
  | "opening-shift"
  | "closing-shift"
  | "closing-panel"
  | null;
type PlayerSettingMenuKey =
  | "mode_single"
  | "mode_list"
  | "mode_shuffle"
  | "speed_075"
  | "speed_100"
  | "speed_125"
  | "speed_150";

const audioRef = ref<HTMLAudioElement | null>(null);
const lyricPanelRef = ref<HTMLElement | null>(null);
const lyricLineRefs = ref<Array<HTMLElement | null>>([]);
const playerPanelRef = ref<HTMLElement | null>(null);
const homeHeaderRef = ref<HTMLElement | null>(null);
const resultContainerRef = ref<HTMLElement | null>(null);
const showFullPlayer = ref(false);
const fullPlayerMode = ref<"lyric" | "disc">("lyric");
const isPlayerFullscreen = ref(false);
const playMode = ref<PlayMode>("single");
const isPlaying = ref(false);
const currentTime = ref(0);
const duration = ref(0);
const volume = ref(0.8);
const switchingTrack = ref(false);
const playContext = ref<PlayContext>({ type: "id", ids: [], currentIndex: -1 });
const parseCache = ref<Record<string, ParseResult>>({});
const lyricCache = ref<Record<string, LyricCacheEntry>>({});
const lyricPendingMap = new Map<string, Promise<LyricCacheEntry | null>>();
const lyricPanelUserScrolling = ref(false);
const playbackRate = ref(1);
const compactCardHidden = ref(false);
const compactCardPlaceholderHeight = ref(0);
const playerInlineHeight = ref(0);
const playerTransitioning = ref(false);
const playerTransitionPhase = ref<PlayerTransitionPhase>(null);
let topLoadingMessage: MessageReactive | null = null;
let lyricPanelScrollTimer: ReturnType<typeof setTimeout> | null = null;
let lyricPanelLastTouchY: number | null = null;
let playerLayoutObserver: ResizeObserver | null = null;
const playerPanelMotionMs = 280;
const playerShellShiftMs = 240;

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

function clampWindowScroll(top: number): number {
  const max = Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
  return Math.max(0, Math.min(top, max));
}

function wait(ms: number): Promise<void> {
  if (ms <= 0) return Promise.resolve();
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}

function getViewportBottomGap(): number {
  const scrollBottom = window.scrollY + window.innerHeight;
  return Math.max(0, document.documentElement.scrollHeight - scrollBottom);
}

function restoreViewportBottomGap(gap: number) {
  const target = document.documentElement.scrollHeight - window.innerHeight - Math.max(0, gap);
  window.scrollTo({ top: clampWindowScroll(target), behavior: "auto" });
}

function captureCompactCardHeight() {
  const card = parseResultRef.value;
  if (!card) return;
  compactCardPlaceholderHeight.value = Math.max(0, Math.ceil(card.getBoundingClientRect().height));
}

function updatePlayerInlineHeight() {
  const header = homeHeaderRef.value;
  const resultContainer = resultContainerRef.value;
  if (!header || !resultContainer) {
    playerInlineHeight.value = 0;
    return;
  }
  const headerTop = header.getBoundingClientRect().top + window.scrollY;
  const resultBottom = resultContainer.getBoundingClientRect().bottom + window.scrollY;
  playerInlineHeight.value = Math.max(0, Math.round(resultBottom - headerTop));
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

function normalizeContextIds(ids: Array<string | number>): string[] {
  const output: string[] = [];
  const seen = new Set<string>();
  for (const raw of ids) {
    const id = String(raw ?? "").trim();
    if (!id || seen.has(id)) continue;
    seen.add(id);
    output.push(id);
  }
  return output;
}

function setPlayContext(type: PlayContextType, ids: Array<string | number>, currentSongId: string) {
  const normalized = normalizeContextIds(ids);
  const current = String(currentSongId || "").trim();
  if (current && !normalized.includes(current)) {
    normalized.push(current);
  }
  playContext.value = {
    type,
    ids: normalized,
    currentIndex: current ? normalized.indexOf(current) : -1,
  };
}

const hasPrevTrack = computed(() => playContext.value.currentIndex > 0);
const hasNextTrack = computed(() => {
  const ctx = playContext.value;
  return ctx.currentIndex >= 0 && ctx.currentIndex < ctx.ids.length - 1;
});

const currentSongId = computed(() => parseResult.value?.song_id || "");
const currentLyricEntry = computed(() => {
  const id = currentSongId.value;
  if (!id) return null;
  return lyricCache.value[id] || null;
});
const currentTimedLyrics = computed(() =>
  (currentLyricEntry.value?.lines || []).filter((line) => {
    const main = (line.main || "").trim();
    const trans = (line.trans || "").trim();
    return !!(main || trans);
  }),
);
const currentLyricIndex = computed(() => {
  const lines = currentTimedLyrics.value;
  if (!lines.length) return -1;
  const target = currentTime.value * 1000 + 100;
  let left = 0;
  let right = lines.length - 1;
  let answer = -1;
  while (left <= right) {
    const mid = (left + right) >> 1;
    if (lines[mid].time <= target) {
      answer = mid;
      left = mid + 1;
    } else {
      right = mid - 1;
    }
  }
  return answer;
});
const seekMax = computed(() => (Number.isFinite(duration.value) && duration.value > 0 ? duration.value : 0));
const seekValue = computed(() => Math.min(Math.max(currentTime.value, 0), seekMax.value));
const displaySongName = computed(() => {
  if (!parseResult.value) return "";
  return parseResult.value.song_name || `歌曲 ${parseResult.value.song_id}`;
});
const displayArtistName = computed(() => parseResult.value?.artist_name || "未知歌手");
const displayAlbumName = computed(() => parseResult.value?.album_name || "未知专辑");
const displayQualityLabel = computed(() => {
  if (!parseResult.value) return "";
  return qualityOptions.find((q) => q.value === parseResult.value?.quality)?.label || parseResult.value.quality || "";
});
const displaySongArtistLine = computed(() => {
  const song = displaySongName.value;
  const artist = displayArtistName.value;
  if (!song) return "";
  return `${song} - ${artist}`;
});

const compactLyricLineHeight = 24;
const compactLyricRows = computed(() => {
  const entry = currentLyricEntry.value;
  const lines = currentTimedLyrics.value.map((line) => line.main || line.trans);
  if (!parseResult.value) {
    return ["", "点击播放查看歌词", ""];
  }
  if (!entry || entry.loading) {
    return ["", "歌词加载中...", ""];
  }
  if (!lines.length) {
    return ["", "暂无滚动歌词", ""];
  }
  return ["", ...lines, ""];
});
const compactActiveVirtualIndex = computed(() => {
  if (currentTimedLyrics.value.length === 0) return 1;
  return Math.max(0, currentLyricIndex.value) + 1;
});
const compactLyricOffset = computed(() => {
  const rows = compactLyricRows.value;
  if (rows.length <= 3) return 0;
  const maxOffset = rows.length - 3;
  const preferredOffset = compactActiveVirtualIndex.value - 1;
  return Math.max(0, Math.min(preferredOffset, maxOffset));
});
const compactLyricTrackStyle = computed(() => ({
  transform: `translateY(-${compactLyricOffset.value * compactLyricLineHeight}px)`,
}));
const showCompactCard = computed(() => !!parseResult.value && !compactCardHidden.value);
const showCompactCardPlaceholder = computed(
  () =>
    !!parseResult.value &&
    compactCardHidden.value &&
    compactCardPlaceholderHeight.value > 0 &&
    playerTransitionPhase.value === "opening-hide-card",
);
const compactCardPlaceholderStyle = computed(() => ({
  height: `${compactCardPlaceholderHeight.value}px`,
}));
const homeShellClass = computed(() => ({
  "with-player": showFullPlayer.value && !!parseResult.value,
  "with-player-opening-panel": playerTransitionPhase.value === "opening-panel",
  "with-player-opening-shift": playerTransitionPhase.value === "opening-shift",
  "with-player-closing-shift": playerTransitionPhase.value === "closing-shift",
}));
const playerInlinePanelStyle = computed(() => {
  if (isPlayerFullscreen.value || playerInlineHeight.value <= 0) return undefined;
  const value = `${playerInlineHeight.value}px`;
  return {
    minHeight: value,
    height: value,
    maxHeight: value,
  };
});

const fullModeToggleTitle = computed(() => (fullPlayerMode.value === "lyric" ? "切换唱片模式" : "切换歌词模式"));
const playModeTitle = computed(() => {
  if (playMode.value === "single") return "单曲循环";
  if (playMode.value === "shuffle") return "随机播放";
  return "列表播放";
});
const isSingleMode = computed(() => playMode.value === "single");
const isShuffleMode = computed(() => playMode.value === "shuffle");
const isDownloadBusy = computed(() => downloading.value || lyricDownloading.value || coverDownloading.value);
const downloadMenuOptions: Array<{ label: string; key: DownloadMenuKey }> = [
  { label: "下载音乐", key: "audio" },
  { label: "下载封面", key: "cover" },
  { label: "下载歌词", key: "lyric" },
];
const playerSettingsMenuOptions: Array<{ label: string; key: PlayerSettingMenuKey }> = [
  { label: "单曲循环", key: "mode_single" },
  { label: "列表播放", key: "mode_list" },
  { label: "随机播放", key: "mode_shuffle" },
  { label: "播放速度 0.75x", key: "speed_075" },
  { label: "播放速度 1.0x", key: "speed_100" },
  { label: "播放速度 1.25x", key: "speed_125" },
  { label: "播放速度 1.5x", key: "speed_150" },
];
const downloadButtonTitle = computed(() => (isDownloadBusy.value ? "下载进行中" : "下载"));
const downloadProgressVisible = computed(() => downloadProgress.value.stage !== "idle");
const downloadProgressPercent = computed(() => {
  const total = downloadProgress.value.totalBytes;
  if (!total || total <= 0) return null;
  return Math.max(0, Math.min(100, (downloadProgress.value.loadedBytes / total) * 100));
});
const downloadProgressTitle = computed(() => {
  const kindLabel = downloadProgress.value.kind === "audio" ? "音乐" : downloadProgress.value.kind === "cover" ? "封面" : "歌词";
  if (downloadProgress.value.stage === "processing") return `${kindLabel}处理中`;
  if (downloadProgress.value.stage === "done") return `${kindLabel}下载完成`;
  if (downloadProgress.value.stage === "error") return `${kindLabel}下载失败`;
  return `${kindLabel}下载中`;
});
const downloadProgressMetric = computed(() => {
  if (downloadProgress.value.stage === "processing") return "处理中";
  if (downloadProgress.value.stage === "done") return "完成";
  if (downloadProgress.value.stage === "error") return "失败";
  if (downloadProgressPercent.value !== null) return `${downloadProgressPercent.value.toFixed(1)}%`;
  return `${formatMegaBytes(downloadProgress.value.loadedBytes)} 已下载`;
});
const downloadProgressNote = computed(() => {
  if (downloadProgress.value.stage === "processing") {
    return downloadProgress.value.detail || "正在处理文件，请稍候";
  }
  if (downloadProgress.value.stage === "done") {
    return downloadProgress.value.fileName || "下载已触发";
  }
  if (downloadProgress.value.stage === "error") {
    return downloadProgress.value.detail || "下载失败";
  }
  if (downloadProgress.value.totalBytes && downloadProgress.value.totalBytes > 0) {
    return `${formatMegaBytes(downloadProgress.value.loadedBytes)} / ${formatMegaBytes(downloadProgress.value.totalBytes)}`;
  }
  return `${formatMegaBytes(downloadProgress.value.loadedBytes)} 已下载`;
});
const downloadProgressTrackClass = computed(() => ({
  indeterminate: downloadProgress.value.stage === "downloading" && downloadProgressPercent.value === null,
  processing: downloadProgress.value.stage === "processing",
}));
const downloadProgressWrapClass = computed(() => ({
  done: downloadProgress.value.stage === "done",
  error: downloadProgress.value.stage === "error",
}));
const downloadProgressFillStyle = computed(() => {
  if (downloadProgress.value.stage === "done") return { width: "100%" };
  if (downloadProgress.value.stage === "processing") return { width: "100%" };
  if (downloadProgressPercent.value !== null) return { width: `${downloadProgressPercent.value}%` };
  return { width: "32%" };
});

function formatMegaBytes(bytes: number): string {
  const safe = Number.isFinite(bytes) && bytes > 0 ? bytes : 0;
  return `${(safe / 1024 / 1024).toFixed(2)} MB`;
}

function formatPlayerTime(secondsValue: number): string {
  const safe = Number.isFinite(secondsValue) && secondsValue > 0 ? Math.floor(secondsValue) : 0;
  const hours = Math.floor(safe / 3600);
  const minutes = Math.floor((safe % 3600) / 60);
  const seconds = safe % 60;
  if (hours > 0) {
    return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  }
  return `${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

const elapsedText = computed(() => formatPlayerTime(currentTime.value));
const durationText = computed(() => formatPlayerTime(duration.value));

function resetAudioState() {
  isPlaying.value = false;
  currentTime.value = 0;
  duration.value = 0;
}

function stopAndResetAudio() {
  const audio = audioRef.value;
  if (!audio) return;
  audio.pause();
  audio.removeAttribute("src");
  audio.load();
  resetAudioState();
}

function setAudioVolume(next: number) {
  const safe = Math.min(1, Math.max(0, next));
  volume.value = safe;
  if (audioRef.value) {
    audioRef.value.volume = safe;
  }
}

function setPlaybackRate(next: number) {
  const safe = Math.max(0.5, Math.min(2, next));
  playbackRate.value = safe;
  if (audioRef.value) {
    audioRef.value.playbackRate = safe;
  }
}

function cacheParseResult(result: ParseResult) {
  const id = String(result.song_id || "").trim();
  if (!id) return;
  parseCache.value = { ...parseCache.value, [id]: result };
}

function getCachedTrack(songId: string): ParseResult | null {
  const key = String(songId || "").trim();
  if (!key) return null;
  const fromParseCache = parseCache.value[key];
  if (fromParseCache) return fromParseCache;
  const numberId = Number(key);
  if (Number.isFinite(numberId) && playlistResults.value[numberId]) {
    return playlistResults.value[numberId];
  }
  return null;
}

function loadAudioSource(url: string, autoplay: boolean) {
  const audio = audioRef.value;
  if (!audio) return;
  audio.pause();
  audio.src = url;
  audio.load();
  resetAudioState();
  audio.volume = volume.value;
  audio.playbackRate = playbackRate.value;

  if (!autoplay) return;
  const play = () => {
    void audio.play().catch(() => {
      // Ignore browser autoplay blocking.
    });
  };
  if (audio.readyState >= 2) {
    play();
    return;
  }
  audio.addEventListener("canplay", play, { once: true });
}

type ActivateTrackOptions = {
  contextType: PlayContextType;
  contextIds: Array<string | number>;
  currentSongId: string;
  autoplay: boolean;
  scroll: boolean;
};

async function activateTrack(result: ParseResult, options: ActivateTrackOptions) {
  parseResult.value = result;
  cacheParseResult(result);
  setPlayContext(options.contextType, options.contextIds, options.currentSongId || result.song_id);
  await nextTick();
  loadAudioSource(result.stream_url, options.autoplay);
  if (options.scroll) {
    scrollToParseResult();
  }
  void ensureLyricCached(result.song_id);
}

function clearCurrentTrack() {
  parseResult.value = null;
  playContext.value = { type: "id", ids: [], currentIndex: -1 };
  compactCardHidden.value = false;
  compactCardPlaceholderHeight.value = 0;
  playerTransitioning.value = false;
  playerTransitionPhase.value = null;
  showFullPlayer.value = false;
  void exitPlayerFullscreenIfNeeded();
  stopAndResetAudio();
}

async function openFullPlayer() {
  if (!parseResult.value || playerTransitioning.value) return;
  const keepBottomGap = getViewportBottomGap();
  captureCompactCardHeight();
  playerTransitioning.value = true;
  playerTransitionPhase.value = "opening-hide-card";
  compactCardHidden.value = true;
  await nextTick();
  restoreViewportBottomGap(keepBottomGap);
  updatePlayerInlineHeight();
  playerTransitionPhase.value = "opening-panel";
  showFullPlayer.value = true;
  await nextTick();
  updatePlayerInlineHeight();
  restoreViewportBottomGap(keepBottomGap);
  await wait(playerPanelMotionMs);
  playerTransitionPhase.value = "opening-shift";
  await wait(playerShellShiftMs);
  restoreViewportBottomGap(keepBottomGap);
  playerTransitioning.value = false;
  playerTransitionPhase.value = null;
  updatePlayerInlineHeight();
}

async function closeFullPlayer() {
  if (playerTransitioning.value) return;
  const keepBottomGap = getViewportBottomGap();
  playerTransitioning.value = true;
  playerTransitionPhase.value = "closing-shift";
  await wait(playerShellShiftMs);
  playerTransitionPhase.value = "closing-panel";
  await wait(playerPanelMotionMs);
  await exitPlayerFullscreenIfNeeded();
  showFullPlayer.value = false;
  compactCardHidden.value = false;
  await nextTick();
  restoreViewportBottomGap(keepBottomGap);
  playerTransitioning.value = false;
  playerTransitionPhase.value = null;
  updatePlayerInlineHeight();
}

function syncPlayerFullscreenState() {
  const panel = playerPanelRef.value;
  const fullscreenEl = document.fullscreenElement;
  isPlayerFullscreen.value = !!panel && !!fullscreenEl && (fullscreenEl === panel || panel.contains(fullscreenEl));
}

async function exitPlayerFullscreenIfNeeded() {
  const panel = playerPanelRef.value;
  if (!panel) return;
  const fullscreenEl = document.fullscreenElement;
  if (!fullscreenEl) {
    isPlayerFullscreen.value = false;
    return;
  }
  if (fullscreenEl === panel || panel.contains(fullscreenEl)) {
    try {
      await document.exitFullscreen();
    } catch {
      // Ignore fullscreen exit failures.
    }
  }
  isPlayerFullscreen.value = false;
}

async function togglePlayerFullscreen() {
  const panel = playerPanelRef.value;
  if (!panel) return;
  const fullscreenEl = document.fullscreenElement;
  try {
    if (fullscreenEl && (fullscreenEl === panel || panel.contains(fullscreenEl))) {
      await document.exitFullscreen();
    } else {
      await panel.requestFullscreen();
    }
  } catch {
    message.warning("当前环境暂不支持全屏播放");
  }
}

function clearLyricPanelScrollTimer() {
  if (!lyricPanelScrollTimer) return;
  clearTimeout(lyricPanelScrollTimer);
  lyricPanelScrollTimer = null;
}

function markLyricPanelUserScrolling() {
  lyricPanelUserScrolling.value = true;
  clearLyricPanelScrollTimer();
  lyricPanelScrollTimer = setTimeout(() => {
    lyricPanelUserScrolling.value = false;
    lyricPanelScrollTimer = null;
  }, 1800);
}

function stopLyricPanelUserScrolling() {
  lyricPanelUserScrolling.value = false;
  lyricPanelLastTouchY = null;
  clearLyricPanelScrollTimer();
}

function onLyricPanelWheel(event: WheelEvent) {
  markLyricPanelUserScrolling();
  const panel = event.currentTarget as HTMLElement | null;
  if (!panel) return;
  panel.scrollTop += event.deltaY;
  event.preventDefault();
  event.stopPropagation();
}

function onLyricPanelTouchStart(event: TouchEvent) {
  markLyricPanelUserScrolling();
  const touch = event.touches[0];
  lyricPanelLastTouchY = touch ? touch.clientY : null;
}

function onLyricPanelTouchMove(event: TouchEvent) {
  markLyricPanelUserScrolling();
  const panel = event.currentTarget as HTMLElement | null;
  const touch = event.touches[0];
  if (!panel || !touch) return;
  const lastY = lyricPanelLastTouchY ?? touch.clientY;
  const deltaY = lastY - touch.clientY;
  lyricPanelLastTouchY = touch.clientY;
  panel.scrollTop += deltaY;
  event.preventDefault();
  event.stopPropagation();
}

function onLyricPanelTouchEnd() {
  lyricPanelLastTouchY = null;
}

function toggleFullPlayerMode() {
  fullPlayerMode.value = fullPlayerMode.value === "lyric" ? "disc" : "lyric";
}

function cyclePlayMode() {
  if (playMode.value === "single") {
    playMode.value = "list";
    return;
  }
  if (playMode.value === "list") {
    playMode.value = "shuffle";
    return;
  }
  playMode.value = "single";
}

function onPlayerSettingSelect(key: string | number) {
  const normalized = String(key) as PlayerSettingMenuKey;
  if (normalized === "mode_single") {
    playMode.value = "single";
    return;
  }
  if (normalized === "mode_list") {
    playMode.value = "list";
    return;
  }
  if (normalized === "mode_shuffle") {
    playMode.value = "shuffle";
    return;
  }
  if (normalized === "speed_075") {
    setPlaybackRate(0.75);
    return;
  }
  if (normalized === "speed_125") {
    setPlaybackRate(1.25);
    return;
  }
  if (normalized === "speed_150") {
    setPlaybackRate(1.5);
    return;
  }
  setPlaybackRate(1);
}

function togglePlayPause() {
  const audio = audioRef.value;
  if (!audio || !parseResult.value) return;
  if (audio.paused) {
    void audio.play().catch(() => {
      message.warning("播放失败，请稍后重试");
    });
    return;
  }
  audio.pause();
}

function onSeekInput(event: Event) {
  const audio = audioRef.value;
  if (!audio) return;
  const next = Number((event.target as HTMLInputElement).value);
  if (!Number.isFinite(next)) return;
  audio.currentTime = next;
  currentTime.value = next;
}

function onVolumeInput(event: Event) {
  const next = Number((event.target as HTMLInputElement).value);
  if (!Number.isFinite(next)) return;
  setAudioVolume(next);
}

function onAudioTimeUpdate() {
  const audio = audioRef.value;
  if (!audio) return;
  currentTime.value = Number.isFinite(audio.currentTime) ? audio.currentTime : 0;
}

function onAudioLoadedMetadata() {
  const audio = audioRef.value;
  if (!audio) return;
  duration.value = Number.isFinite(audio.duration) && audio.duration > 0 ? audio.duration : 0;
}

function onAudioDurationChange() {
  const audio = audioRef.value;
  if (!audio) return;
  duration.value = Number.isFinite(audio.duration) && audio.duration > 0 ? audio.duration : 0;
}

function onAudioPlay() {
  isPlaying.value = true;
}

function onAudioPause() {
  isPlaying.value = false;
}

function onAudioEnded() {
  isPlaying.value = false;
  if (!parseResult.value) return;
  if (playMode.value === "single") {
    const audio = audioRef.value;
    if (!audio) return;
    audio.currentTime = 0;
    void audio.play().catch(() => {
      // ignore replay failure
    });
    return;
  }
  if (playMode.value === "shuffle") {
    void playRandomTrack();
    return;
  }
  if (hasNextTrack.value) {
    void playAdjacentTrack(1);
  }
}

function setLyricLineRef(element: unknown, index: number) {
  const candidate = (element && typeof element === "object" && "$el" in (element as Record<string, unknown>))
    ? (element as { $el?: unknown }).$el
    : element;
  lyricLineRefs.value[index] = candidate instanceof HTMLElement ? candidate : null;
}

function scrollLyricLineIntoCenter(index: number, behavior: ScrollBehavior) {
  if (!showFullPlayer.value || fullPlayerMode.value !== "lyric") return;
  if (index < 0) return;
  const lineEl = lyricLineRefs.value[index];
  lineEl?.scrollIntoView({ behavior, block: "center" });
}

function onLyricRowClick(line: TimedLyricLine, index: number) {
  const audio = audioRef.value;
  if (!audio) return;
  const targetSeconds = Math.max(0, line.time / 1000);
  audio.currentTime = targetSeconds;
  currentTime.value = targetSeconds;
  stopLyricPanelUserScrolling();
  void nextTick(() => {
    scrollLyricLineIntoCenter(index, "smooth");
  });
}

async function playContextTrackByIndex(targetIndex: number) {
  if (switchingTrack.value) return;
  const ctx = playContext.value;
  if (targetIndex < 0 || targetIndex >= ctx.ids.length) return;
  const targetSongId = ctx.ids[targetIndex];
  const cached = getCachedTrack(targetSongId);
  if (cached) {
    await activateTrack(cached, {
      contextType: ctx.type,
      contextIds: ctx.ids,
      currentSongId: targetSongId,
      autoplay: true,
      scroll: false,
    });
    return;
  }

  switchingTrack.value = true;
  showTopLoading("正在解析上一首/下一首...");
  try {
    const parsed = await parseMusic(targetSongId, quality.value);
    if (ctx.type === "playlist") {
      const numberId = Number(targetSongId);
      if (Number.isFinite(numberId)) {
        playlistResults.value[numberId] = parsed;
      }
    }
    await activateTrack(parsed, {
      contextType: ctx.type,
      contextIds: ctx.ids,
      currentSongId: targetSongId,
      autoplay: true,
      scroll: false,
    });
  } catch (error) {
    message.error((error as Error).message || "切换歌曲失败");
  } finally {
    switchingTrack.value = false;
    hideTopLoading();
  }
}

async function playAdjacentTrack(offset: -1 | 1) {
  const ctx = playContext.value;
  const targetIndex = ctx.currentIndex + offset;
  await playContextTrackByIndex(targetIndex);
}

async function playRandomTrack() {
  const ctx = playContext.value;
  if (!ctx.ids.length) return;
  if (ctx.ids.length === 1) {
    const audio = audioRef.value;
    if (!audio) return;
    audio.currentTime = 0;
    void audio.play().catch(() => {
      // ignore replay failure
    });
    return;
  }
  let targetIndex = ctx.currentIndex;
  while (targetIndex === ctx.currentIndex) {
    targetIndex = Math.floor(Math.random() * ctx.ids.length);
  }
  await playContextTrackByIndex(targetIndex);
}

function onPrevTrack() {
  if (!hasPrevTrack.value) return;
  void playAdjacentTrack(-1);
}

function onNextTrack() {
  if (!hasNextTrack.value) return;
  void playAdjacentTrack(1);
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
  clearCurrentTrack();
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
  if (parsing.value || switchingTrack.value) return;
  parsing.value = true;
  showTopLoading("正在解析歌曲...");
  try {
    const result = await parseMusic(String(songId), quality.value);
    const contextIds = searchResults.value.map((song) => song.id);
    await activateTrack(result, {
      contextType: "search",
      contextIds,
      currentSongId: String(songId),
      autoplay: false,
      scroll: true,
    });
    message.success("解析成功");
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    parsing.value = false;
    hideTopLoading();
  }
}

async function onParseById() {
  if (!requireAuth()) return;
  if (idParsing.value || switchingTrack.value) return;
  const input = idInput.value.trim();
  if (!input) {
    message.warning("请输入歌曲 ID 或链接");
    return;
  }
  idParsing.value = true;
  showTopLoading("正在解析歌曲...");
  try {
    const result = await parseMusic(input, quality.value);
    await activateTrack(result, {
      contextType: "id",
      contextIds: [result.song_id],
      currentSongId: result.song_id,
      autoplay: false,
      scroll: true,
    });
    message.success("解析成功");
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
  if (playlistParsing.value[trackId] || switchingTrack.value) return;
  playlistParsing.value[trackId] = true;
  showTopLoading("正在解析歌曲...");
  try {
    const result = await parseMusic(String(trackId), quality.value);
    playlistResults.value[trackId] = result;
    const contextIds = playlistInfo.value?.tracks?.map((track) => track.id) || [trackId];
    await activateTrack(result, {
      contextType: "playlist",
      contextIds,
      currentSongId: String(trackId),
      autoplay: false,
      scroll: true,
    });
    message.success("解析成功");
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    playlistParsing.value[trackId] = false;
    hideTopLoading();
  }
}

async function playTrack(result: ParseResult) {
  const contextIds = playlistInfo.value?.tracks?.map((track) => track.id) || [result.song_id];
  await activateTrack(result, {
    contextType: "playlist",
    contextIds,
    currentSongId: result.song_id,
    autoplay: true,
    scroll: true,
  });
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

type ParsedLrcLine = {
  time: number;
  text: string;
};

function parseLrc(raw: string): ParsedLrcLine[] {
  const normalized = normalizeNeteaseLyric(raw);
  if (!normalized) return [];
  const rows = normalized.split("\n");
  const output: ParsedLrcLine[] = [];
  const timeReg = /\[(\d{1,2}):(\d{2})(?:\.(\d{1,3}))?\]/g;

  for (const rawRow of rows) {
    const row = rawRow.trim();
    if (!row || /^\[(ti|ar|al|by|offset):/i.test(row)) continue;
    const tags = [...row.matchAll(timeReg)];
    if (!tags.length) continue;
    const text = row.replace(timeReg, "").trim();
    for (const tag of tags) {
      const minutes = Number(tag[1] || 0);
      const seconds = Number(tag[2] || 0);
      const fraction = (tag[3] || "").padEnd(3, "0").slice(0, 3);
      const millis = Number(fraction || "0");
      const time = minutes * 60000 + seconds * 1000 + millis;
      output.push({ time, text });
    }
  }

  output.sort((a, b) => a.time - b.time);
  const dedup = new Map<number, string>();
  for (const item of output) {
    if (!dedup.has(item.time) || item.text) {
      dedup.set(item.time, item.text);
    }
  }
  return Array.from(dedup.entries()).map(([time, text]) => ({ time, text }));
}

function matchTranslation(time: number, transLines: ParsedLrcLine[]): string {
  if (!transLines.length) return "";
  let left = 0;
  let right = transLines.length - 1;
  let floorIndex = -1;
  while (left <= right) {
    const mid = (left + right) >> 1;
    if (transLines[mid].time <= time) {
      floorIndex = mid;
      left = mid + 1;
    } else {
      right = mid - 1;
    }
  }

  const candidates: ParsedLrcLine[] = [];
  if (floorIndex >= 0) candidates.push(transLines[floorIndex]);
  if (floorIndex + 1 < transLines.length) candidates.push(transLines[floorIndex + 1]);
  if (!candidates.length) return "";

  let best = candidates[0];
  let bestDiff = Math.abs(best.time - time);
  for (let i = 1; i < candidates.length; i += 1) {
    const diff = Math.abs(candidates[i].time - time);
    if (diff < bestDiff) {
      best = candidates[i];
      bestDiff = diff;
    }
  }
  return bestDiff <= 600 ? best.text : "";
}

function buildTimedLyrics(lyricResult: LyricResult | null): TimedLyricLine[] {
  if (!lyricResult) return [];
  const mainLines = parseLrc(lyricResult.lyric || "");
  const transLines = parseLrc(lyricResult.tlyric || "");
  if (!mainLines.length && !transLines.length) return [];
  if (!mainLines.length) {
    return transLines.map((line) => ({
      time: line.time,
      main: line.text,
      trans: "",
    }));
  }
  return mainLines.map((line) => ({
    time: line.time,
    main: line.text,
    trans: matchTranslation(line.time, transLines),
  }));
}

function updateLyricCache(songId: string, entry: LyricCacheEntry) {
  lyricCache.value = {
    ...lyricCache.value,
    [songId]: entry,
  };
}

async function ensureLyricCached(songId: string): Promise<LyricCacheEntry | null> {
  const id = String(songId || "").trim();
  if (!id) return null;
  const cached = lyricCache.value[id];
  if (cached?.loaded) return cached;
  const pending = lyricPendingMap.get(id);
  if (pending) return pending;

  const task = (async () => {
    updateLyricCache(id, { loading: true, loaded: false, raw: null, lines: [], merged: "" });
    try {
      const raw = await fetchLyric(id);
      const entry: LyricCacheEntry = {
        loading: false,
        loaded: true,
        raw,
        lines: buildTimedLyrics(raw),
        merged: buildLyricText(raw),
      };
      updateLyricCache(id, entry);
      return entry;
    } catch {
      const emptyEntry: LyricCacheEntry = {
        loading: false,
        loaded: true,
        raw: null,
        lines: [],
        merged: "",
      };
      updateLyricCache(id, emptyEntry);
      return emptyEntry;
    } finally {
      lyricPendingMap.delete(id);
    }
  })();
  lyricPendingMap.set(id, task);
  return task;
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
  const id = String(songId || "").trim();
  if (!id) return null;
  const cached = lyricCache.value[id];
  if (cached?.loaded) return cached.raw;
  const next = await ensureLyricCached(id);
  return next?.raw || null;
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

function parseContentLength(raw: string | null): number | null {
  const value = Number(raw || "");
  if (!Number.isFinite(value) || value <= 0) return null;
  return value;
}

function parseFileNameFromDisposition(disposition: string | null): string {
  const raw = disposition || "";
  const utf8Match = raw.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8Match?.[1]) {
    try {
      return decodeURIComponent(utf8Match[1]).trim();
    } catch {
      // ignore decode error and fallback
    }
  }
  const plainMatch = raw.match(/filename="([^"]+)"/i) || raw.match(/filename=([^;]+)/i);
  return plainMatch?.[1]?.trim() || "";
}

function clearDownloadProgressResetTimer() {
  if (!downloadProgressResetTimer) return;
  clearTimeout(downloadProgressResetTimer);
  downloadProgressResetTimer = null;
}

function scheduleDownloadProgressReset(delay = 2500) {
  clearDownloadProgressResetTimer();
  downloadProgressResetTimer = setTimeout(() => {
    downloadProgress.value = {
      stage: "idle",
      kind: downloadProgress.value.kind,
      loadedBytes: 0,
      totalBytes: null,
      fileName: "",
      detail: "",
    };
    downloadProgressResetTimer = null;
  }, delay);
}

function startDownloadProgress(kind: DownloadKind) {
  clearDownloadProgressResetTimer();
  downloadProgress.value = {
    stage: "downloading",
    kind,
    loadedBytes: 0,
    totalBytes: null,
    fileName: "",
    detail: "",
  };
}

function updateDownloadProgressState(loadedBytes: number, totalBytes: number | null) {
  downloadProgress.value = {
    ...downloadProgress.value,
    stage: "downloading",
    loadedBytes: Math.max(0, loadedBytes),
    totalBytes: totalBytes && totalBytes > 0 ? totalBytes : null,
  };
}

function markDownloadProcessing(detail: string) {
  downloadProgress.value = {
    ...downloadProgress.value,
    stage: "processing",
    detail,
  };
}

function finishDownloadProgress(stage: "done" | "error", detail: string, fileName = "") {
  downloadProgress.value = {
    ...downloadProgress.value,
    stage,
    detail,
    fileName,
  };
  scheduleDownloadProgressReset(stage === "done" ? 2600 : 4200);
}

async function readResponseArrayBufferWithProgress(
  resp: Response,
  onProgress: (loadedBytes: number, totalBytes: number | null) => void,
): Promise<{ buffer: ArrayBuffer; totalBytes: number | null }> {
  const totalBytes = parseContentLength(resp.headers.get("content-length"));
  if (!resp.body) {
    const fallback = await resp.arrayBuffer();
    onProgress(fallback.byteLength, totalBytes);
    return { buffer: fallback, totalBytes };
  }
  const reader = resp.body.getReader();
  const chunks: Uint8Array[] = [];
  let loadedBytes = 0;
  onProgress(0, totalBytes);
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    if (!value) continue;
    chunks.push(value);
    loadedBytes += value.byteLength;
    onProgress(loadedBytes, totalBytes);
  }
  const output = new Uint8Array(loadedBytes);
  let offset = 0;
  for (const chunk of chunks) {
    output.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return { buffer: output.buffer, totalBytes };
}

async function downloadBackendAssetWithProgress(path: string, id: string, fallbackName: string) {
  const token = localStorage.getItem("mp_token");
  const resp = await fetch(path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify({ id }),
  });

  if (!resp.ok) {
    let errMsg = `请求失败（HTTP ${resp.status}）`;
    try {
      const payload = await resp.json();
      if (payload?.msg) errMsg = String(payload.msg);
    } catch {
      // ignore parse error
    }
    throw new Error(errMsg);
  }

  const { buffer, totalBytes } = await readResponseArrayBufferWithProgress(resp, updateDownloadProgressState);
  if (totalBytes) {
    updateDownloadProgressState(Math.max(totalBytes, buffer.byteLength), totalBytes);
  } else {
    updateDownloadProgressState(buffer.byteLength, null);
  }
  const fileName = parseFileNameFromDisposition(resp.headers.get("content-disposition")) || fallbackName;
  const contentType = resp.headers.get("content-type") || "application/octet-stream";
  return { blob: new Blob([buffer], { type: contentType }), fileName };
}

function onDownloadMenuSelect(key: string | number) {
  const selected = String(key) as DownloadMenuKey;
  if (selected === "audio") {
    void onDownloadCurrent();
    return;
  }
  if (selected === "cover") {
    void onDownloadCover();
    return;
  }
  if (selected === "lyric") {
    void onDownloadLyric();
  }
}

async function onDownloadCurrent() {
  if (!requireAuth()) return;
  if (isDownloadBusy.value) return;
  if (!parseResult.value) {
    message.warning("请先解析歌曲");
    return;
  }
  downloading.value = true;
  startDownloadProgress("audio");
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
    const { buffer: originalBuffer, totalBytes } = await readResponseArrayBufferWithProgress(audioResp, updateDownloadProgressState);
    if (totalBytes) {
      updateDownloadProgressState(Math.max(totalBytes, originalBuffer.byteLength), totalBytes);
    } else {
      updateDownloadProgressState(originalBuffer.byteLength, null);
    }
    const format = detectAudioFormat(originalBuffer, current.stream_url, audioResp.headers.get("content-type") || "");

    let finalBlob = new Blob([originalBuffer], { type: format === "flac" ? "audio/flac" : "audio/mpeg" });
    let lyricResult: LyricResult | null = null;
    let coverResult: { buffer: ArrayBuffer; mime: string } | null = null;

    if (settingsStore.writeMetadata || settingsStore.zipPackageDownload) {
      markDownloadProcessing("正在准备歌词与封面资源...");
      [lyricResult, coverResult] = await Promise.all([
        safeFetchLyric(current.song_id),
        safeFetchCover(current.cover_url),
      ]);
    }

    const lyrics = buildLyricText(lyricResult);

    if (settingsStore.writeMetadata) {
      markDownloadProcessing("正在写入音频元数据...");
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
      markDownloadProcessing("正在打包 ZIP 文件...");
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
      const zipFileName = sanitizeFileName(`${baseName}.zip`) || "track.zip";
      triggerDownload(zipBlob, zipFileName);
      finishDownloadProgress("done", "ZIP 下载任务已触发", zipFileName);
      message.success("ZIP 下载任务已触发");
      return;
    }

    triggerDownload(finalBlob, audioFileName);
    finishDownloadProgress("done", "下载任务已触发", audioFileName);
    message.success("下载任务已触发");
  } catch (error) {
    const errorMessage = (error as Error).message || "下载失败";
    finishDownloadProgress("error", errorMessage);
    message.error(errorMessage);
  } finally {
    downloading.value = false;
  }
}

async function onDownloadLyric() {
  if (!requireAuth()) return;
  if (isDownloadBusy.value) return;
  if (!parseResult.value) {
    message.warning("请先解析歌曲");
    return;
  }

  lyricDownloading.value = true;
  startDownloadProgress("lyric");
  try {
    const current = parseResult.value;
    const { blob, fileName } = await downloadBackendAssetWithProgress("/api/music/lyric/download", current.song_id, "track.lrc");
    const safeName = sanitizeFileName(fileName) || "track.lrc";
    triggerDownload(blob, safeName);
    finishDownloadProgress("done", "歌词下载已触发", safeName);
    message.success("歌词下载已触发");
  } catch (error) {
    const errorMessage = (error as Error).message || "下载歌词失败";
    finishDownloadProgress("error", errorMessage);
    message.error(errorMessage);
  } finally {
    lyricDownloading.value = false;
  }
}

async function onDownloadCover() {
  if (!requireAuth()) return;
  if (isDownloadBusy.value) return;
  if (!parseResult.value) {
    message.warning("请先解析歌曲");
    return;
  }

  coverDownloading.value = true;
  startDownloadProgress("cover");
  try {
    const current = parseResult.value;
    const { blob, fileName } = await downloadBackendAssetWithProgress("/api/music/cover/download", current.song_id, "cover.jpg");
    const safeName = sanitizeFileName(fileName) || "cover.jpg";
    triggerDownload(blob, safeName);
    finishDownloadProgress("done", "封面下载已触发", safeName);
    message.success("封面下载已触发");
  } catch (error) {
    const errorMessage = (error as Error).message || "下载封面失败";
    finishDownloadProgress("error", errorMessage);
    message.error(errorMessage);
  } finally {
    coverDownloading.value = false;
  }
}

watch(volume, (next) => {
  if (audioRef.value) {
    audioRef.value.volume = next;
  }
});

watch(playbackRate, (next) => {
  if (audioRef.value) {
    audioRef.value.playbackRate = next;
  }
});

watch(
  () => currentSongId.value,
  (songId) => {
    lyricLineRefs.value = [];
    if (!songId) {
      showFullPlayer.value = false;
      compactCardHidden.value = false;
      return;
    }
    void ensureLyricCached(songId);
  },
);

watch(
  [() => currentLyricIndex.value, () => showFullPlayer.value, () => fullPlayerMode.value, () => currentTimedLyrics.value.length],
  () => {
    if (!showFullPlayer.value || fullPlayerMode.value !== "lyric") return;
    if (lyricPanelUserScrolling.value) return;
    const index = currentLyricIndex.value;
    if (index < 0) return;
    void nextTick(() => {
      scrollLyricLineIntoCenter(index, isPlaying.value ? "smooth" : "auto");
    });
  },
);

watch(
  () => showFullPlayer.value,
  (visible) => {
    void nextTick(() => {
      updatePlayerInlineHeight();
    });
    if (!visible) {
      stopLyricPanelUserScrolling();
      void exitPlayerFullscreenIfNeeded();
    }
  },
);

watch(
  () => [activeTab.value, searchResults.value.length, playlistInfo.value?.tracks?.length || 0, showFullPlayer.value],
  () => {
    void nextTick(() => {
      updatePlayerInlineHeight();
    });
  },
);

onMounted(async () => {
  document.addEventListener("fullscreenchange", syncPlayerFullscreenState);
  window.addEventListener("resize", updatePlayerInlineHeight, { passive: true });
  setAudioVolume(0.8);
  setPlaybackRate(1);
  await nextTick();
  updatePlayerInlineHeight();
  if (typeof ResizeObserver !== "undefined") {
    playerLayoutObserver = new ResizeObserver(() => {
      updatePlayerInlineHeight();
    });
    if (homeHeaderRef.value) {
      playerLayoutObserver.observe(homeHeaderRef.value);
    }
    if (resultContainerRef.value) {
      playerLayoutObserver.observe(resultContainerRef.value);
    }
  }
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
});

onUnmounted(() => {
  document.removeEventListener("fullscreenchange", syncPlayerFullscreenState);
  window.removeEventListener("resize", updatePlayerInlineHeight);
  playerLayoutObserver?.disconnect();
  playerLayoutObserver = null;
  clearLyricPanelScrollTimer();
  clearDownloadProgressResetTimer();
  void exitPlayerFullscreenIfNeeded();
  stopAndResetAudio();
  hideTopLoading();
});
</script>

<template>
    <main class="page-shell home-shell" :class="homeShellClass">
      <section class="home-center">
        <header ref="homeHeaderRef" class="home-header glass-card">
          <div class="header-text">
            <p class="eyebrow">MUSIC PARSER</p>
            <h1>{{ settingsStore.siteName }}</h1>
            <p class="header-desc">支持网易云音乐全品质解析，浏览器端完成下载与元数据写入。</p>
          </div>
          <div class="header-actions">
            <button v-if="!authStore.isAuthed" class="settings-btn" title="登录" @click="router.push('/login')">
              <n-icon size="20"><Login /></n-icon>
            </button>
            <n-dropdown v-else :options="userMenuOptions" trigger="click" @select="onUserMenuSelect">
              <button class="settings-btn user-btn" :title="authStore.user?.username || '用户'">
                <n-icon size="20"><UserCircle /></n-icon>
              </button>
            </n-dropdown>
            <button class="settings-btn" title="网站设置" @click="showSettings = true">
              <n-icon size="20"><Settings /></n-icon>
            </button>
          </div>
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
          <div class="card-title"><n-icon size="16" color="var(--brand)"><Headphones /></n-icon><span>音质选择</span></div>
          <div class="quality-tags">
            <button v-for="opt in qualityOptions" :key="opt.value" :class="['quality-tag', { active: quality === opt.value }]" @click="quality = opt.value">{{ opt.short }}</button>
          </div>
        </div>

        <div ref="resultContainerRef" class="main-card glass-card">
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
          <div
            v-if="showCompactCard"
            ref="parseResultRef"
            :class="[
              'result-card',
              'glass-card',
              {
                'result-card-hidden': compactCardHidden,
                'result-card-closing': playerTransitionPhase === 'closing-panel',
              },
            ]"
          >
            <div class="compact-player">
              <div class="compact-cover-stack">
                <button class="compact-cover-btn" type="button" @click="openFullPlayer">
                  <img v-if="parseResult?.cover_url" :src="parseResult?.cover_url" alt="cover" referrerpolicy="no-referrer" />
                  <div v-else class="compact-cover-empty"><n-icon size="24"><Music /></n-icon></div>
                </button>
                <span class="compact-quality-tag">{{ displayQualityLabel }}</span>
              </div>
              <div class="compact-main">
                <button class="compact-meta-btn" type="button" @click="openFullPlayer">
                  <p class="compact-song-line">{{ displaySongArtistLine }}</p>
                  <div class="compact-lyrics-window">
                    <div class="compact-lyrics-track" :style="compactLyricTrackStyle">
                      <p
                        v-for="(line, index) in compactLyricRows"
                        :key="`${line}-${index}`"
                        class="compact-lyric-line"
                        :class="{ active: index === compactActiveVirtualIndex }"
                      >
                        {{ line || " " }}
                      </p>
                    </div>
                  </div>
                </button>
                <div class="compact-controls-wrap">
                  <div class="compact-controls">
                    <button class="icon-btn play-btn" type="button" @click="togglePlayPause">
                      <n-icon size="20"><PlayerPause v-if="isPlaying" /><PlayerPlay v-else /></n-icon>
                    </button>
                    <div class="progress-wrap">
                      <span class="time-text">{{ elapsedText }}</span>
                      <input class="range-input range-progress" type="range" min="0" :max="seekMax" step="0.1" :value="seekValue" @input="onSeekInput" />
                      <span class="time-text">{{ durationText }}</span>
                    </div>
                    <div class="volume-wrap">
                      <n-icon size="21"><Volume3 /></n-icon>
                      <input class="range-input range-volume" type="range" min="0" max="1" step="0.01" :value="volume" @input="onVolumeInput" />
                    </div>
                    <n-dropdown :options="downloadMenuOptions" trigger="click" :disabled="isDownloadBusy" @select="onDownloadMenuSelect">
                      <button class="icon-btn download-btn" type="button" :disabled="isDownloadBusy" :title="downloadButtonTitle">
                        <n-icon size="19"><Download /></n-icon>
                      </button>
                    </n-dropdown>
                  </div>
                </div>
              </div>
            </div>
            <div v-if="downloadProgressVisible" class="download-progress-wrap" :class="downloadProgressWrapClass">
              <div class="download-progress-head">
                <span class="download-progress-title">{{ downloadProgressTitle }}</span>
                <span class="download-progress-metric">{{ downloadProgressMetric }}</span>
              </div>
              <div class="download-progress-track" :class="downloadProgressTrackClass">
                <div class="download-progress-fill" :style="downloadProgressFillStyle"></div>
              </div>
              <p class="download-progress-note">{{ downloadProgressNote }}</p>
            </div>
          </div>
        </transition>
        <div
          v-if="showCompactCardPlaceholder"
          class="result-card-placeholder"
          :style="compactCardPlaceholderStyle"
        ></div>
        <audio
          ref="audioRef"
          class="native-audio"
          preload="metadata"
          @timeupdate="onAudioTimeUpdate"
          @loadedmetadata="onAudioLoadedMetadata"
          @durationchange="onAudioDurationChange"
          @play="onAudioPlay"
          @pause="onAudioPause"
          @ended="onAudioEnded"
        />
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

      <transition name="player-inline">
        <aside
          v-if="showFullPlayer && parseResult"
          ref="playerPanelRef"
          class="player-panel player-panel-inline glass-card"
          :class="{
            'is-player-fullscreen': isPlayerFullscreen,
            'player-panel-opening': playerTransitionPhase === 'opening-panel',
            'player-panel-closing': playerTransitionPhase === 'closing-panel',
          }"
          :style="playerInlinePanelStyle"
        >
          <header class="player-panel-header">
            <button class="panel-top-btn" type="button" title="全屏播放器" @click="togglePlayerFullscreen">
              <n-icon size="18"><Minimize v-if="isPlayerFullscreen" /><Maximize v-else /></n-icon>
            </button>
            <button class="panel-close-arrow" type="button" title="关闭播放器" @click="closeFullPlayer">›</button>
          </header>
          <template v-if="isPlayerFullscreen">
            <div class="player-fullscreen-body">
              <section class="player-fullscreen-left">
                <div v-if="fullPlayerMode === 'disc'" class="disc-stage fullscreen-disc-stage">
                  <div class="disc-needle" :class="{ engaged: isPlaying }">
                    <span class="needle-arm"></span>
                    <span class="needle-head"></span>
                  </div>
                  <div class="disc-record" :class="{ spinning: isPlaying }">
                    <div class="disc-record-rings"></div>
                    <img v-if="parseResult.cover_url" :src="parseResult.cover_url" alt="cover" referrerpolicy="no-referrer" class="disc-record-cover" />
                    <div v-else class="disc-record-cover disc-record-fallback"><n-icon size="38"><Music /></n-icon></div>
                  </div>
                </div>
                <div v-else class="fullscreen-cover-wrap">
                  <img v-if="parseResult.cover_url" :src="parseResult.cover_url" alt="cover" referrerpolicy="no-referrer" class="fullscreen-cover" />
                  <div v-else class="fullscreen-cover fullscreen-cover-fallback"><n-icon size="58"><Music /></n-icon></div>
                </div>
                <div class="fullscreen-meta">
                  <p class="fullscreen-meta-song">{{ displaySongName }}</p>
                  <p class="fullscreen-meta-artist">{{ displayArtistName }}</p>
                  <p class="fullscreen-meta-album">{{ displayAlbumName }}</p>
                </div>
              </section>
              <section
                ref="lyricPanelRef"
                class="panel-lyrics fullscreen-lyrics"
                @wheel.prevent.stop="onLyricPanelWheel"
                @touchstart.stop="onLyricPanelTouchStart"
                @touchmove.prevent.stop="onLyricPanelTouchMove"
                @touchend.stop="onLyricPanelTouchEnd"
                @touchcancel.stop="onLyricPanelTouchEnd"
              >
                <p v-if="currentLyricEntry?.loading" class="lyric-placeholder">歌词加载中...</p>
                <p v-else-if="currentTimedLyrics.length === 0" class="lyric-placeholder">暂无滚动歌词</p>
                <div v-else class="lyric-line-list fullscreen-lyric-list">
                  <div
                    v-for="(line, index) in currentTimedLyrics"
                    :key="`${line.time}-${index}`"
                    :ref="(el) => setLyricLineRef(el, index)"
                    class="lyric-row fullscreen-lyric-row"
                    :class="{ active: index === currentLyricIndex }"
                    @click="onLyricRowClick(line, index)"
                  >
                    <p class="lyric-main-text">{{ line.main || line.trans }}</p>
                    <p v-if="line.trans" class="lyric-translation">{{ line.trans }}</p>
                  </div>
                </div>
              </section>
            </div>
          </template>
          <template v-else>
            <div class="player-headline">
              <p class="panel-song">{{ displaySongName }}</p>
              <p class="panel-artist">{{ displayArtistName }}</p>
            </div>
            <transition name="player-mode-fade" mode="out-in">
              <div
                v-if="fullPlayerMode === 'lyric'"
                key="lyric"
                ref="lyricPanelRef"
                class="panel-lyrics"
                @wheel.prevent.stop="onLyricPanelWheel"
                @touchstart.stop="onLyricPanelTouchStart"
                @touchmove.prevent.stop="onLyricPanelTouchMove"
                @touchend.stop="onLyricPanelTouchEnd"
                @touchcancel.stop="onLyricPanelTouchEnd"
              >
                <p v-if="currentLyricEntry?.loading" class="lyric-placeholder">歌词加载中...</p>
                <p v-else-if="currentTimedLyrics.length === 0" class="lyric-placeholder">暂无滚动歌词</p>
                <div v-else class="lyric-line-list">
                  <div
                    v-for="(line, index) in currentTimedLyrics"
                    :key="`${line.time}-${index}`"
                    :ref="(el) => setLyricLineRef(el, index)"
                    class="lyric-row"
                    :class="{ active: index === currentLyricIndex }"
                    @click="onLyricRowClick(line, index)"
                  >
                    <p class="lyric-main-text">{{ line.main || line.trans }}</p>
                    <p v-if="line.trans" class="lyric-translation">{{ line.trans }}</p>
                  </div>
                </div>
              </div>
              <div v-else key="disc" class="disc-stage">
                <div class="disc-needle" :class="{ engaged: isPlaying }">
                  <span class="needle-arm"></span>
                  <span class="needle-head"></span>
                </div>
                <div class="disc-record" :class="{ spinning: isPlaying }">
                  <div class="disc-record-rings"></div>
                  <img v-if="parseResult.cover_url" :src="parseResult.cover_url" alt="cover" referrerpolicy="no-referrer" class="disc-record-cover" />
                  <div v-else class="disc-record-cover disc-record-fallback"><n-icon size="38"><Music /></n-icon></div>
                </div>
              </div>
            </transition>
          </template>
          <div class="panel-progress" :class="{ 'fullscreen-progress': isPlayerFullscreen }">
            <span class="time-text">{{ elapsedText }}</span>
            <input class="range-input range-progress" type="range" min="0" :max="seekMax" step="0.1" :value="seekValue" @input="onSeekInput" />
            <span class="time-text">{{ durationText }}</span>
          </div>
          <div v-if="isPlayerFullscreen" class="panel-controls-fullscreen">
            <div class="panel-controls-center">
              <button class="icon-btn panel-mode-btn" type="button" :title="fullModeToggleTitle" @click="toggleFullPlayerMode">
                <n-icon size="19"><Vinyl v-if="fullPlayerMode === 'disc'" /><Music v-else /></n-icon>
              </button>
              <button class="icon-btn" type="button" :disabled="!hasPrevTrack || switchingTrack" @click="onPrevTrack"><n-icon size="20"><PlayerTrackPrev /></n-icon></button>
              <button class="icon-btn panel-play-main" type="button" @click="togglePlayPause"><n-icon size="24"><PlayerPause v-if="isPlaying" /><PlayerPlay v-else /></n-icon></button>
              <button class="icon-btn" type="button" :disabled="!hasNextTrack || switchingTrack" @click="onNextTrack"><n-icon size="20"><PlayerTrackNext /></n-icon></button>
              <button class="icon-btn panel-mode-btn" type="button" :title="playModeTitle" @click="cyclePlayMode">
                <n-icon size="19"><RepeatOnce v-if="isSingleMode" /><ArrowsShuffle2 v-else-if="isShuffleMode" /><Repeat v-else /></n-icon>
              </button>
            </div>
            <div class="panel-tools-right">
              <label class="quality-select-wrap">
                <span class="quality-select-label">音质</span>
                <select v-model="quality" class="quality-select-inline">
                  <option v-for="opt in qualityOptions" :key="opt.value" :value="opt.value">{{ opt.short }}</option>
                </select>
              </label>
              <n-dropdown :options="playerSettingsMenuOptions" trigger="click" @select="onPlayerSettingSelect">
                <button class="icon-btn panel-mode-btn" type="button" title="播放设置">
                  <n-icon size="19"><Settings /></n-icon>
                </button>
              </n-dropdown>
              <div class="panel-volume panel-volume-inline">
                <n-icon size="21"><Volume3 /></n-icon>
                <input class="range-input range-volume" type="range" min="0" max="1" step="0.01" :value="volume" @input="onVolumeInput" />
              </div>
            </div>
          </div>
          <div v-else class="panel-controls">
            <button class="icon-btn panel-mode-btn" type="button" :title="fullModeToggleTitle" @click="toggleFullPlayerMode">
              <n-icon size="19"><Vinyl v-if="fullPlayerMode === 'disc'" /><Music v-else /></n-icon>
            </button>
            <button class="icon-btn" type="button" :disabled="!hasPrevTrack || switchingTrack" @click="onPrevTrack"><n-icon size="20"><PlayerTrackPrev /></n-icon></button>
            <button class="icon-btn panel-play-main" type="button" @click="togglePlayPause"><n-icon size="24"><PlayerPause v-if="isPlaying" /><PlayerPlay v-else /></n-icon></button>
            <button class="icon-btn" type="button" :disabled="!hasNextTrack || switchingTrack" @click="onNextTrack"><n-icon size="20"><PlayerTrackNext /></n-icon></button>
            <button class="icon-btn panel-mode-btn" type="button" :title="playModeTitle" @click="cyclePlayMode">
              <n-icon size="19"><RepeatOnce v-if="isSingleMode" /><ArrowsShuffle2 v-else-if="isShuffleMode" /><Repeat v-else /></n-icon>
            </button>
          </div>
          <div v-if="!isPlayerFullscreen" class="panel-volume">
            <n-icon size="21"><Volume3 /></n-icon>
            <input class="range-input range-volume" type="range" min="0" max="1" step="0.01" :value="volume" @input="onVolumeInput" />
          </div>
        </aside>
      </transition>

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
</template>

<style scoped>
.home-shell {
  --player-panel-width: 438px;
  display: grid;
  grid-template-columns: minmax(0, min(920px, 96vw));
  justify-content: center;
  align-items: stretch;
  gap: 14px;
  min-height: 100vh;
  padding: 32px 18px 8px;
  overflow-x: hidden;
  transition: grid-template-columns .32s cubic-bezier(.22,.81,.24,1), padding .3s ease, transform .26s ease;
}
.home-center { width: 100%; display: flex; flex-direction: column; gap: 14px; min-height: calc(100vh - 60px); transition: transform .32s cubic-bezier(.22,.81,.24,1), opacity .24s ease; }
.home-shell.with-player {
  grid-template-columns:
    minmax(0, min(920px, max(320px, calc(100vw - var(--player-panel-width) - 56px))))
    var(--player-panel-width);
}
.home-shell.with-player.with-player-opening-panel { transform: translateX(12px); }
.home-shell.with-player.with-player-opening-shift { transform: translateX(0); }
.home-shell.with-player.with-player-closing-shift { transform: translateX(12px); }
.home-header { display: flex; justify-content: space-between; gap: 16px; padding: 24px; background: linear-gradient(160deg, rgba(11,83,206,.92), rgba(13,121,198,.88)); color: #fff; }
.header-text h1 { margin: 6px 0; }
.eyebrow { margin: 0; letter-spacing: .2em; font-size: 12px; }
.header-desc { margin: 0; opacity: .92; font-size: 13px; }
.header-actions { display: flex; gap: 8px; align-items: flex-start; }
.settings-btn { width: 36px; height: 36px; display: grid; place-items: center; border-radius: 10px; border: 1px solid rgba(255,255,255,.2); background: rgba(255,255,255,.12); color: #fff; cursor: pointer; transition: background .2s; }
.settings-btn:hover { background: rgba(255,255,255,.22); }
.method-card,.quality-card,.main-card,.result-card { padding: 18px 22px; }
.result-card { transition: opacity .24s ease, transform .28s ease; }
.result-card-hidden { opacity: 0; pointer-events: none; }
.result-card-closing { animation: card-reveal .32s cubic-bezier(.2,.82,.25,1); }
.card-title { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; font-weight: 700; color: var(--text-1); font-size: 18px; }
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
.song-list {
  --song-item-gap: 6px;
  --song-item-visual-height: 70px;
  display: flex;
  flex-direction: column;
  gap: var(--song-item-gap);
  max-height: calc(var(--song-item-visual-height) * 4 + var(--song-item-gap) * 3);
  overflow-y: auto;
}
.song-item { display: flex; align-items: center; gap: 10px; padding: 10px 12px; border-radius: 10px; border: 1px solid var(--song-item-border); background: var(--song-item-bg); min-height: 70px; }
.song-index { width: 18px; text-align: center; font-size: 12px; color: var(--text-2); }
.cover { width: 48px; height: 48px; border-radius: 10px; object-fit: cover; flex-shrink: 0; }
.cover-sm { width: 40px; height: 40px; }
.cover-empty { display: grid; place-items: center; background: var(--brand-soft); color: var(--brand); }
.song-info { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.song-name,.song-meta { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.song-name { font-size: 14px; font-weight: 600; }
.song-meta { font-size: 12px; color: var(--text-2); }
.playlist-header { display: flex; align-items: center; gap: 10px; margin-bottom: 10px; }
.compact-player { display: grid; grid-template-columns: 92px 1fr; gap: 12px; padding: 10px 12px; border-radius: 14px; border: 1px solid var(--line-soft); background: rgba(255, 255, 255, 0.46); align-items: center; }
[data-theme="dark"] .compact-player { background: rgba(25, 36, 57, 0.5); }
.compact-player, .compact-player * { font-family: inherit; }
.compact-cover-stack { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; align-self: center; }
.compact-cover-btn { width: 92px; height: 92px; border: none; padding: 0; border-radius: 10px; overflow: hidden; background: transparent; cursor: pointer; }
.compact-cover-btn img { width: 100%; height: 100%; object-fit: cover; display: block; }
.compact-cover-empty { width: 100%; height: 100%; display: grid; place-items: center; border-radius: 12px; background: var(--brand-soft); color: var(--brand); }
.compact-quality-tag { max-width: 92px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; text-align: center; font-size: 12px; line-height: 1.2; padding: 3px 8px; border-radius: 999px; background: var(--brand-soft); color: var(--brand-deep); font-weight: 700; }
.compact-main { min-width: 0; display: flex; flex-direction: column; gap: 10px; justify-content: center; }
.compact-meta-btn { border: none; background: transparent; padding: 0; text-align: center; cursor: pointer; min-width: 0; width: 100%; }
.compact-song-line { margin: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-size: 17px; line-height: 1.15; color: var(--text-1); font-weight: 700; }
.compact-lyrics-window { margin-top: 7px; height: 72px; overflow: hidden; display: flex; justify-content: center; }
.compact-lyrics-track { width: 100%; transition: transform .35s cubic-bezier(.2,.82,.25,1); will-change: transform; }
.compact-lyric-line { margin: 0; height: 24px; line-height: 24px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-size: 16px; color: var(--text-2); font-weight: 600; }
.compact-lyric-line.active { color: #1d86ff; font-weight: 700; }
.compact-controls-wrap { display: flex; justify-content: center; width: 100%; overflow-x: auto; scrollbar-width: none; }
.compact-controls-wrap::-webkit-scrollbar { display: none; }
.compact-controls { display: flex; align-items: center; justify-content: center; gap: 8px; flex-wrap: nowrap; width: fit-content; max-width: 100%; margin: 0 auto; white-space: nowrap; }
.icon-btn { width: 42px; height: 42px; border-radius: 999px; border: 1px solid var(--line-soft); background: rgba(255,255,255,.72); color: var(--text-1); display: grid; place-items: center; cursor: pointer; transition: transform .18s ease, background .2s ease, opacity .2s ease; }
[data-theme="dark"] .icon-btn { background: rgba(20, 30, 48, 0.72); }
.icon-btn:hover { transform: translateY(-1px); }
.icon-btn:disabled { opacity: .5; cursor: not-allowed; transform: none; }
.play-btn { width: 46px; height: 46px; }
.progress-wrap { flex: 0 1 276px; width: 276px; min-width: 176px; display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 6px; }
.time-text { font-size: 15px; color: var(--text-2); font-variant-numeric: tabular-nums; min-width: 40px; text-align: center; font-weight: 600; }
.range-input { appearance: none; -webkit-appearance: none; background: transparent; }
.range-input::-webkit-slider-runnable-track { height: 4px; border-radius: 999px; background: rgba(15, 111, 255, .2); }
.range-input::-webkit-slider-thumb { appearance: none; -webkit-appearance: none; width: 14px; height: 14px; border-radius: 50%; margin-top: -5px; background: var(--brand); border: 2px solid #fff; box-shadow: 0 2px 6px rgba(15,111,255,.25); }
.range-input::-moz-range-track { height: 4px; border-radius: 999px; background: rgba(15, 111, 255, .2); }
.range-input::-moz-range-thumb { width: 14px; height: 14px; border-radius: 50%; background: var(--brand); border: none; }
.range-progress { width: 100%; min-width: 140px; }
.volume-wrap { display: flex; align-items: center; gap: 6px; color: var(--text-2); min-width: 102px; flex: 0 0 auto; }
.range-volume { width: 74px; }
.download-btn { width: 42px; height: 42px; }
.native-audio { display: none; }
.download-progress-wrap { margin-top: 10px; padding: 9px 11px 8px; border-radius: 12px; border: 1px solid var(--line-soft); background: color-mix(in oklab, var(--card-bg) 88%, #d9e9ff 12%); }
[data-theme="dark"] .download-progress-wrap { background: color-mix(in oklab, var(--card-bg) 92%, #1f3250 8%); }
.download-progress-wrap.done { border-color: color-mix(in oklab, #22c55e 42%, var(--line-soft) 58%); }
.download-progress-wrap.error { border-color: color-mix(in oklab, #ef4444 44%, var(--line-soft) 56%); }
.download-progress-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; }
.download-progress-title { font-size: 13px; color: var(--text-1); font-weight: 700; }
.download-progress-metric { font-size: 12px; color: var(--text-2); font-variant-numeric: tabular-nums; }
.download-progress-track { position: relative; margin-top: 7px; height: 6px; border-radius: 999px; background: rgba(15, 111, 255, .17); overflow: hidden; }
.download-progress-fill { height: 100%; border-radius: inherit; background: linear-gradient(90deg, #1f8dff 0%, #56b1ff 100%); transition: width .2s linear; will-change: transform, width; }
.download-progress-wrap.done .download-progress-fill { background: linear-gradient(90deg, #22c55e 0%, #5dd390 100%); }
.download-progress-wrap.error .download-progress-fill { background: linear-gradient(90deg, #ef4444 0%, #f98080 100%); }
.download-progress-track.indeterminate .download-progress-fill { animation: download-indeterminate 1.15s linear infinite; }
.download-progress-track.processing .download-progress-fill { animation: download-processing 1.2s ease-in-out infinite; }
.download-progress-note { margin: 7px 0 0; font-size: 12px; line-height: 1.3; color: var(--text-2); word-break: break-word; }
.result-card-placeholder {
  width: 100%;
  border-radius: 16px;
  background: transparent;
  pointer-events: none;
}
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
.player-panel { width: auto; border-radius: 20px; padding: 12px 16px 14px; display: flex; flex-direction: column; gap: 10px; background: color-mix(in oklab, var(--card-bg) 88%, #d5e8ff 12%); box-shadow: 0 14px 34px rgba(6, 20, 44, .22); overflow: hidden; }
[data-theme="dark"] .player-panel { background: color-mix(in oklab, var(--card-bg) 88%, #213451 12%); }
.player-panel-inline {
  width: var(--player-panel-width);
  min-height: 0;
  height: auto;
  max-height: none;
  margin: 0;
  position: relative;
  top: 0;
  align-self: start;
  overscroll-behavior: contain;
  transition: transform .26s ease, opacity .26s ease, height .24s ease, max-height .24s ease;
}
.player-panel-inline.player-panel-opening { transform: translateX(18px); opacity: .88; }
.player-panel-inline.player-panel-closing { transform: translateX(28px); opacity: 0; }
.player-panel-header { display: flex; justify-content: space-between; align-items: center; gap: 10px; }
.panel-top-btn,.panel-close-arrow {
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 999px;
  background: rgba(255,255,255,.62);
  color: var(--brand-deep);
  padding: 0;
  cursor: pointer;
  display: grid;
  place-items: center;
  transition: transform .2s ease, background .2s ease, color .2s ease;
}
.panel-top-btn:hover,.panel-close-arrow:hover { transform: translateY(-1px); background: rgba(255,255,255,.8); }
.panel-close-arrow { font-size: 30px; line-height: 1; transform: translateX(1px); }
[data-theme="dark"] .panel-close-arrow { background: rgba(20, 30, 48, 0.72); color: #8ebaff; }
[data-theme="dark"] .panel-top-btn { background: rgba(20, 30, 48, 0.72); color: #8ebaff; }
.player-headline { text-align: center; padding-top: 2px; }
.panel-song,.panel-artist { margin: 0; text-align: center; }
.panel-song { font-size: 27px; color: var(--text-1); font-weight: 800; line-height: 1.08; }
.panel-artist { margin-top: 5px; color: var(--text-2); font-size: 16px; font-weight: 700; }
.panel-lyrics {
  flex: 1;
  min-height: 0;
  padding: 6px 8px;
  overflow-y: auto;
  display: flex;
  justify-content: center;
  scroll-padding-block: 40%;
  overscroll-behavior: contain;
  touch-action: pan-y;
}
.lyric-placeholder { margin: 4px 0; color: var(--text-2); text-align: center; }
.lyric-line-list { width: min(100%, 520px); display: flex; flex-direction: column; gap: 12px; padding: 26px 6px 34px; margin: 0 auto; }
.lyric-row p { margin: 0; }
.lyric-row {
  color: var(--text-2);
  text-align: center;
  cursor: pointer;
  opacity: .62;
  transition: color .2s ease, transform .2s ease, opacity .2s ease;
}
.lyric-row.active { color: #1d86ff; transform: scale(1.03); opacity: 1; }
.lyric-main-text { font-size: clamp(13px, 1.32vw, 19px); line-height: 1.35; font-weight: 700; }
.lyric-translation { margin-top: 4px; font-size: clamp(9px, .9vw, 12px); opacity: .85; }
.disc-stage { flex: 1; min-height: 0; display: grid; place-items: center; position: relative; overflow: hidden; }
.disc-record { width: min(74vw, 332px); aspect-ratio: 1; border-radius: 50%; position: relative; display: grid; place-items: center; box-shadow: 0 16px 34px rgba(6, 16, 34, .34), inset 0 0 0 3px rgba(255,255,255,.07); background: radial-gradient(circle at center, rgba(26, 29, 37, 1) 0%, rgba(6, 8, 12, 1) 48%, rgba(38, 41, 53, 1) 100%); }
.disc-record-rings { position: absolute; inset: 0; border-radius: 50%; background: repeating-radial-gradient(circle, rgba(255,255,255,.035) 0 3px, rgba(255,255,255,.008) 3px 7px); pointer-events: none; }
.disc-record-cover { width: 58%; aspect-ratio: 1; object-fit: cover; border-radius: 50%; border: 5px solid rgba(255,255,255,.12); z-index: 1; }
.disc-record-fallback { display: grid; place-items: center; color: #89b9ff; background: radial-gradient(circle, rgba(34, 58, 96, 1), rgba(12, 20, 34, 1)); }
.disc-record.spinning { animation: disc-spin 7.5s linear infinite; }
.disc-needle { position: absolute; top: 6%; right: 7%; width: min(40vw, 170px); height: min(40vw, 170px); transform-origin: 15% 12%; transform: rotate(-28deg); transition: transform .38s cubic-bezier(.35,.78,.2,1); z-index: 4; pointer-events: none; }
.disc-needle.engaged { transform: rotate(-3deg); }
.needle-arm { position: absolute; left: 16%; top: 13%; width: 10px; height: 72%; border-radius: 999px; background: linear-gradient(180deg, #f4f7ff, #cad4ea 40%, #eff3ff); box-shadow: 0 0 0 1px rgba(0,0,0,.08); }
.needle-head { position: absolute; right: 5%; top: 60%; width: 34px; height: 14px; border-radius: 4px; background: linear-gradient(180deg, #f6f8ff, #d8e0f2); box-shadow: 0 3px 10px rgba(0,0,0,.2); transform: rotate(18deg); }
.disc-needle::before { content: ""; position: absolute; width: 24px; height: 24px; border-radius: 50%; left: 10%; top: 4%; background: radial-gradient(circle, #f8f8f8 0 32%, #d0d4dc 33% 68%, #f0f2f7 69% 100%); box-shadow: 0 0 0 8px rgba(255,255,255,.05); }
.panel-progress { display: grid; grid-template-columns: auto 1fr auto; align-items: center; gap: 8px; }
.panel-controls { display: flex; justify-content: center; align-items: center; gap: 14px; }
.panel-play-main { width: 56px; height: 56px; }
.panel-mode-btn { width: 40px; height: 40px; }
.panel-volume { display: flex; align-items: center; justify-content: center; gap: 8px; color: var(--text-2); }
.player-fullscreen-body {
  flex: 1;
  min-height: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: clamp(20px, 2.8vw, 48px);
  align-items: stretch;
  padding: 4px 2px 2px;
}
.player-fullscreen-left {
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
}
.fullscreen-cover-wrap {
  width: 70%;
  max-width: 50vh;
  aspect-ratio: 1;
  border-radius: 24px;
  overflow: hidden;
  box-shadow: 0 24px 56px rgba(6, 20, 44, .28);
  border: 1px solid rgba(255,255,255,.22);
}
.fullscreen-cover { width: 100%; height: 100%; object-fit: cover; display: block; }
.fullscreen-cover-fallback {
  display: grid;
  place-items: center;
  background: color-mix(in oklab, var(--brand-soft) 68%, #f4f8ff 32%);
  color: var(--brand);
}
.fullscreen-meta { width: 70%; max-width: 50vh; text-align: center; }
.fullscreen-meta p { margin: 0; }
.fullscreen-meta-song { font-size: clamp(24px, 2.05vw, 34px); line-height: 1.2; font-weight: 800; color: var(--text-1); }
.fullscreen-meta-artist { margin-top: 8px; font-size: clamp(16px, 1.3vw, 20px); color: var(--text-2); font-weight: 700; }
.fullscreen-meta-album { margin-top: 7px; font-size: clamp(13px, .95vw, 15px); color: var(--text-2); opacity: .9; }
.fullscreen-lyrics {
  padding: 8px 6px 6px;
  scroll-padding-block: 45%;
  mask: linear-gradient(180deg, transparent 0%, rgba(255,255,255,.72) 7%, #fff 14%, #fff 86%, transparent 100%);
  -webkit-mask: linear-gradient(180deg, transparent 0%, rgba(255,255,255,.72) 7%, #fff 14%, #fff 86%, transparent 100%);
}
.fullscreen-lyric-list {
  width: min(100%, 920px);
  gap: 20px;
  padding: 16vh 18px 20vh;
}
.fullscreen-lyric-row { opacity: .26; }
.fullscreen-lyric-row.active { opacity: 1; transform: scale(1.02); }
.fullscreen-lyric-row .lyric-main-text { font-size: clamp(32px, 2.85vw, 54px); line-height: 1.28; }
.fullscreen-lyric-row .lyric-translation { margin-top: 8px; font-size: clamp(16px, 1.35vw, 24px); }
.fullscreen-disc-stage { width: 100%; min-height: min(56vh, 520px); }
.fullscreen-disc-stage .disc-record { width: min(33vw, 430px); }
.fullscreen-disc-stage .disc-needle { width: min(23vw, 228px); height: min(23vw, 228px); top: 1%; right: 10%; }
.panel-controls-fullscreen {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 80px;
}
.panel-controls-center { display: flex; align-items: center; gap: 14px; }
.panel-tools-right {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 10px;
}
.quality-select-wrap {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  border-radius: 999px;
  border: 1px solid var(--line-soft);
  background: rgba(255,255,255,.64);
  padding: 6px 10px;
}
[data-theme="dark"] .quality-select-wrap { background: rgba(20, 30, 48, 0.72); }
.quality-select-label { font-size: 12px; color: var(--text-2); font-weight: 600; }
.quality-select-inline {
  border: none;
  background: transparent;
  color: var(--text-1);
  font-size: 13px;
  font-weight: 700;
  line-height: 1;
  padding: 0 10px 0 0;
  outline: none;
  cursor: pointer;
  font-family: inherit;
}
.quality-select-inline option { color: #111a2c; }
.panel-volume-inline { min-width: 130px; justify-content: flex-end; }
.fullscreen-progress { margin-top: 2px; }
.player-inline-enter-active,.player-inline-leave-active { transition: opacity .22s ease; }
.player-inline-enter-active .player-panel-inline,.player-inline-leave-active .player-panel-inline { transition: transform .26s ease, opacity .22s ease; }
.player-inline-enter-from,.player-inline-leave-to { opacity: 0; }
.player-inline-enter-from .player-panel-inline,.player-inline-leave-to .player-panel-inline { transform: translateX(28px); opacity: 0; }
.player-mode-fade-enter-active,.player-mode-fade-leave-active { transition: opacity .2s ease, transform .2s ease; }
.player-mode-fade-enter-from,.player-mode-fade-leave-to { opacity: 0; transform: translateY(8px); }
.player-panel-inline.is-player-fullscreen,
.player-panel-inline:fullscreen {
  width: 100%;
  height: 100%;
  max-height: 100%;
  margin: 0;
  border-radius: 0;
  border: none;
  box-shadow: none;
  padding: 18px 30px 22px;
  position: relative;
  top: 0;
  background:
    radial-gradient(80rem 52rem at 16% 14%, rgba(255, 255, 255, .42), transparent 68%),
    linear-gradient(180deg, rgba(234, 242, 252, 0.98), rgba(214, 227, 246, 0.98));
}
[data-theme="dark"] .player-panel-inline.is-player-fullscreen,
[data-theme="dark"] .player-panel-inline:fullscreen {
  background:
    radial-gradient(76rem 52rem at 30% 8%, rgba(53, 84, 138, 0.3), transparent 60%),
    linear-gradient(180deg, rgba(20, 29, 44, 0.98), rgba(12, 18, 30, 0.99));
}
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .panel-song,
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .panel-artist,
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .fullscreen-meta-song,
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .fullscreen-meta-artist,
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .fullscreen-meta-album,
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .lyric-row,
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .time-text,
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .panel-volume,
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .quality-select-label,
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .quality-select-inline {
  color: #dbe9ff;
}
[data-theme="dark"] .player-panel-inline.is-player-fullscreen .lyric-row.active {
  color: #4da6ff;
}
.fade-up-enter-active,.fade-up-leave-active { transition: all .2s ease; }
.fade-up-enter-from,.fade-up-leave-to { opacity: 0; transform: translateY(8px); }
@keyframes card-reveal {
  from { opacity: .62; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}
@keyframes disc-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
@keyframes download-indeterminate {
  from { transform: translateX(-140%); }
  to { transform: translateX(320%); }
}
@keyframes download-processing {
  0%, 100% { opacity: .55; }
  50% { opacity: 1; }
}
@media (max-width: 1240px) {
  .home-shell { --player-panel-width: 400px; }
}
@media (max-width: 980px) {
  .home-shell,
  .home-shell.with-player {
    grid-template-columns: minmax(0, 1fr);
    --player-panel-width: min(100vw - 28px, 720px);
    padding: 18px 14px 4px;
  }
  .player-panel-inline {
    position: relative;
    top: 0;
    width: var(--player-panel-width);
    min-height: 0;
    height: auto;
    max-height: none;
    margin: 0 auto;
  }
  .player-fullscreen-body {
    grid-template-columns: minmax(0, 1fr);
    gap: 14px;
  }
  .player-fullscreen-left { gap: 12px; }
  .fullscreen-cover-wrap { width: min(58vw, 340px); }
  .fullscreen-disc-stage { min-height: min(42vh, 420px); }
  .fullscreen-disc-stage .disc-record { width: min(52vw, 320px); }
  .fullscreen-disc-stage .disc-needle { width: min(34vw, 200px); height: min(34vw, 200px); right: 4%; }
  .fullscreen-lyric-list { width: 100%; padding: 7vh 8px 14vh; }
  .fullscreen-lyric-row .lyric-main-text { font-size: clamp(24px, 4.8vw, 38px); }
  .fullscreen-lyric-row .lyric-translation { font-size: clamp(13px, 2.7vw, 19px); }
  .panel-controls-fullscreen {
    flex-wrap: wrap;
    justify-content: center;
    row-gap: 10px;
  }
  .panel-tools-right {
    margin-left: 0;
    width: 100%;
    justify-content: center;
    flex-wrap: wrap;
  }
}
@media (max-width: 720px) {
  .home-shell,
  .home-shell.with-player { --player-panel-width: 100vw; }
  .home-center { min-height: calc(100vh - 46px); }
  .home-header { padding: 18px; }
  .main-card,.method-card,.quality-card,.result-card { padding: 16px; }
  .card-title { font-size: 18px; }
  .method-option { font-size: 16px; }
  .quality-tag { font-size: 13px; }
  .result-grid { grid-template-columns: 1fr; }
  .compact-player { grid-template-columns: 78px 1fr; gap: 9px; padding: 8px 10px; }
  .compact-cover-stack { gap: 5px; }
  .compact-cover-btn { width: 78px; height: 78px; }
  .compact-quality-tag { max-width: 78px; font-size: 11px; padding: 2px 6px; }
  .compact-song-line { font-size: 14px; }
  .compact-lyrics-window { height: 72px; }
  .compact-lyric-line { height: 24px; line-height: 24px; font-size: 13px; }
  .compact-controls { width: fit-content; max-width: none; gap: 6px; }
  .progress-wrap { width: 220px; min-width: 148px; order: 0; gap: 5px; }
  .time-text { min-width: 34px; font-size: 13px; }
  .volume-wrap { width: auto; min-width: 92px; gap: 5px; }
  .range-volume { width: 62px; }
  .download-progress-wrap { margin-top: 9px; padding: 8px 9px 7px; }
  .download-progress-title { font-size: 12px; }
  .download-progress-note { font-size: 11px; }
  .player-panel-inline { margin: 0; width: 100%; height: 100vh; max-height: 100vh; border-radius: 0; }
  .panel-top-btn,.panel-close-arrow { width: 30px; height: 30px; }
  .lyric-main-text { font-size: clamp(13px, 3.8vw, 19px); }
  .lyric-translation { font-size: clamp(8px, 2.8vw, 11px); }
  .panel-song { font-size: 25px; }
  .panel-artist { font-size: 15px; }
  .disc-record { width: min(82vw, 300px); }
  .disc-needle { width: min(46vw, 160px); height: min(46vw, 160px); right: 2%; top: 7%; }
  .player-fullscreen-body { gap: 8px; }
  .fullscreen-cover-wrap { width: min(66vw, 296px); border-radius: 16px; }
  .fullscreen-meta-song { font-size: clamp(20px, 6.4vw, 28px); }
  .fullscreen-meta-artist { font-size: clamp(14px, 4.1vw, 17px); }
  .fullscreen-meta-album { font-size: clamp(12px, 3.6vw, 14px); }
  .fullscreen-lyric-list { padding: 4vh 4px 10vh; gap: 14px; }
  .fullscreen-lyric-row .lyric-main-text { font-size: clamp(22px, 6.2vw, 32px); }
  .fullscreen-lyric-row .lyric-translation { font-size: clamp(13px, 3.6vw, 17px); }
  .panel-controls-center { gap: 10px; }
  .quality-select-wrap { padding: 5px 9px; }
  .quality-select-inline { font-size: 12px; }
  .panel-volume-inline { min-width: 116px; }
}
</style>
