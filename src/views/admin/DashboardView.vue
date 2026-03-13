<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { NButton, NIcon } from "naive-ui";
import { createDiscreteApi } from "naive-ui";
import { Refresh, Users, FileMusic, ChartPie, Gauge } from "@vicons/tabler";
import { getDashboardStats, type DashboardStats } from "@/api/modules/admin";
import { useAuthStore } from "@/stores/auth";

type TrendKey = "pv" | "uv" | "parse" | "latency";
type ChangeDirection = "up" | "down" | "flat";
type ChangeBadge = {
  direction: ChangeDirection;
  symbol: string;
  text: string;
};
type MetricCard = {
  title: string;
  icon: any;
  iconClass: string;
  leftLabel: string;
  leftValue: string;
  leftChange?: ChangeBadge;
  rightLabel: string;
  rightValue: string;
  rightChange?: ChangeBadge;
};

const { message } = createDiscreteApi(["message"]);
const authStore = useAuthStore();
const loading = ref(false);
const activeTrend = ref<TrendKey>("pv");

const stats = ref<DashboardStats>({
  user_count: 0,
  user_new_today: 0,
  user_new_prev_day: 0,
  parse_count: 0,
  parse_today: 0,
  cookie_count: 0,
  cookie_new_today: 0,
  cookie_new_prev_day: 0,
  pv_total: 0,
  uv_total: 0,
  pv_today: 0,
  uv_today: 0,
  avg_latency_ms: 0,
  trend_7days: [],
  pv_trend_7days: [],
  uv_trend_7days: [],
  latency_trend_7days: [],
});

const greeting = computed(() => {
  const hour = new Date().getHours();
  if (hour < 6) return "夜深了";
  if (hour < 12) return "上午好";
  if (hour < 18) return "下午好";
  return "晚上好";
});

const trendOptions: Array<{ key: TrendKey; label: string }> = [
  { key: "pv", label: "访问趋势" },
  { key: "uv", label: "访客分布" },
  { key: "parse", label: "解析趋势" },
  { key: "latency", label: "延迟趋势" },
];

const trendSeries = computed(() => {
  switch (activeTrend.value) {
    case "uv":
      return stats.value.uv_trend_7days || [];
    case "parse":
      return stats.value.trend_7days || [];
    case "latency":
      return stats.value.latency_trend_7days || [];
    case "pv":
    default:
      return stats.value.pv_trend_7days || [];
  }
});

const trendMax = computed(() => {
  const values = trendSeries.value.map((item) => Number(item.count) || 0);
  return Math.max(1, ...values);
});

const trendTotal = computed(() => trendSeries.value.reduce((sum, item) => sum + (Number(item.count) || 0), 0));

const trendAverage = computed(() => {
  if (!trendSeries.value.length) return 0;
  return trendTotal.value / trendSeries.value.length;
});

function formatNumber(value: number): string {
  return Number(value || 0).toLocaleString("zh-CN");
}

function round1(value: number): number {
  return Number(value.toFixed(1));
}

function calcChange(current: number, previous: number): ChangeBadge {
  const safeCurrent = Number(current || 0);
  const safePrevious = Number(previous || 0);

  if (safeCurrent === safePrevious) {
    return { direction: "flat", symbol: "●", text: "0.0%" };
  }

  if (safePrevious <= 0) {
    if (safeCurrent <= 0) {
      return { direction: "flat", symbol: "●", text: "0.0%" };
    }
    return { direction: "up", symbol: "▲", text: "+100%" };
  }

  const ratio = ((safeCurrent - safePrevious) / safePrevious) * 100;
  const abs = Math.abs(ratio);

  if (ratio > 0) {
    return { direction: "up", symbol: "▲", text: `+${round1(abs)}%` };
  }

  return { direction: "down", symbol: "▼", text: `-${round1(abs)}%` };
}

function latestAndPrev(rows: Array<{ count: number }>) {
  const len = rows.length;
  if (len === 0) return { latest: 0, prev: 0 };
  if (len === 1) return { latest: Number(rows[0]?.count || 0), prev: 0 };
  return {
    latest: Number(rows[len - 1]?.count || 0),
    prev: Number(rows[len - 2]?.count || 0),
  };
}

const parseDayPair = computed(() => latestAndPrev(stats.value.trend_7days || []));
const pvDayPair = computed(() => latestAndPrev(stats.value.pv_trend_7days || []));
const uvDayPair = computed(() => latestAndPrev(stats.value.uv_trend_7days || []));
const latencyDayPair = computed(() => latestAndPrev(stats.value.latency_trend_7days || []));

const topCards = computed<MetricCard[]>(() => [
  {
    title: "账户数据",
    icon: Users,
    iconClass: "card-icon--blue",
    leftLabel: "用户总数",
    leftValue: formatNumber(stats.value.user_count),
    leftChange: calcChange(stats.value.user_new_today, stats.value.user_new_prev_day),
    rightLabel: "Cookie 总量",
    rightValue: formatNumber(stats.value.cookie_count),
    rightChange: calcChange(stats.value.cookie_new_today, stats.value.cookie_new_prev_day),
  },
  {
    title: "使用统计",
    icon: FileMusic,
    iconClass: "card-icon--green",
    leftLabel: "解析总量",
    leftValue: formatNumber(stats.value.parse_count),
    leftChange: calcChange(parseDayPair.value.latest, parseDayPair.value.prev),
    rightLabel: "今日解析",
    rightValue: formatNumber(stats.value.parse_today),
    rightChange: calcChange(parseDayPair.value.latest, parseDayPair.value.prev),
  },
  {
    title: "访问总览",
    icon: ChartPie,
    iconClass: "card-icon--pink",
    leftLabel: "总 PV",
    leftValue: formatNumber(stats.value.pv_total),
    leftChange: calcChange(pvDayPair.value.latest, pvDayPair.value.prev),
    rightLabel: "总 UV",
    rightValue: formatNumber(stats.value.uv_total),
    rightChange: calcChange(uvDayPair.value.latest, uvDayPair.value.prev),
  },
  {
    title: "性能指标",
    icon: Gauge,
    iconClass: "card-icon--amber",
    leftLabel: "平均延迟",
    leftValue: `${Math.round(stats.value.avg_latency_ms)} ms`,
    rightLabel: "今日延迟",
    rightValue: `${Math.round(latencyDayPair.value.latest)} ms`,
  },
]);

function formatMetric(value: number, key: TrendKey): string {
  if (key === "latency") return `${Math.round(value)} ms`;
  return formatNumber(value);
}

async function loadStats() {
  loading.value = true;
  try {
    stats.value = await getDashboardStats();
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

onMounted(loadStats);
</script>

<template>
  <section class="dashboard-shell">
    <header class="header-row">
      <div>
        <h2 class="header-title">{{ greeting }}，{{ authStore.user?.username || "Admin" }}</h2>
        <p class="header-desc">后台统计总览与趋势分析</p>
      </div>
      <div class="header-actions">
        <n-button circle quaternary :loading="loading" title="刷新统计" @click="loadStats">
          <template #icon>
            <n-icon><Refresh /></n-icon>
          </template>
        </n-button>
      </div>
    </header>

    <div class="cards-grid">
      <article v-for="card in topCards" :key="card.title" class="stat-card">
        <div class="card-head">
          <div class="card-head-left">
            <div :class="['card-icon', card.iconClass]">
              <n-icon size="18">
                <component :is="card.icon" />
              </n-icon>
            </div>
            <strong>{{ card.title }}</strong>
          </div>
        </div>
        <div class="card-body">
          <div class="card-metrics">
            <div class="metric-row">
              <span class="metric-label">{{ card.leftLabel }}</span>
              <div class="metric-main">
                <span class="metric-value">{{ card.leftValue }}</span>
                <span v-if="card.leftChange" :class="['metric-change', `is-${card.leftChange.direction}`]">
                  <i>{{ card.leftChange.symbol }}</i>
                  <em>{{ card.leftChange.text }}</em>
                </span>
              </div>
            </div>
            <div class="metric-row">
              <span class="metric-label">{{ card.rightLabel }}</span>
              <div class="metric-main">
                <span class="metric-value">{{ card.rightValue }}</span>
                <span v-if="card.rightChange" :class="['metric-change', `is-${card.rightChange.direction}`]">
                  <i>{{ card.rightChange.symbol }}</i>
                  <em>{{ card.rightChange.text }}</em>
                </span>
              </div>
            </div>
          </div>
        </div>
      </article>
    </div>

    <section class="analysis-card">
      <header class="analysis-header">
        <div class="analysis-title-row">
          <h3>趋势分析</h3>
          <p>最近 7 天（北京时间）</p>
        </div>
        <nav class="analysis-tabs">
          <button
            v-for="tab in trendOptions"
            :key="tab.key"
            :class="['tab-btn', { active: activeTrend === tab.key }]"
            type="button"
            @click="activeTrend = tab.key"
          >
            {{ tab.label }}
          </button>
        </nav>
      </header>

      <div class="analysis-summary">
        <span>总计：{{ formatMetric(trendTotal, activeTrend) }}</span>
        <span>日均：{{ formatMetric(trendAverage, activeTrend) }}</span>
        <span v-if="activeTrend === 'pv' || activeTrend === 'uv'">
          今日：{{ activeTrend === "pv" ? formatNumber(stats.pv_today) : formatNumber(stats.uv_today) }}
        </span>
      </div>

      <div v-if="trendSeries.length" class="analysis-chart">
        <div class="bars-wrap">
          <div v-for="row in trendSeries" :key="row.day" class="bar-col">
            <span class="bar-top">{{ row.count }}</span>
            <div class="bar-track">
              <i class="bar-fill" :style="{ height: `${Math.max(8, (Number(row.count || 0) / trendMax) * 180)}px` }"></i>
            </div>
            <span class="bar-label">{{ row.day.slice(5) }}</span>
          </div>
        </div>
      </div>
      <div v-else class="empty-tip">暂无统计数据</div>
    </section>
  </section>
</template>

<style scoped>
.dashboard-shell {
  display: flex;
  flex-direction: column;
  gap: 16px;
  font-family: "Zhuque Fangsong (technical preview)", "Manrope", "Noto Sans SC", "Segoe UI", sans-serif;
}

.dashboard-shell :deep(*) {
  font-family: inherit;
}

.header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  background: linear-gradient(160deg, rgba(11, 83, 206, 0.92), rgba(13, 121, 198, 0.88));
  backdrop-filter: blur(12px);
  border: 1px solid var(--line-soft);
  border-radius: 18px;
  padding: 22px 24px;
  color: #fff;
  transition: background 0.35s ease, border-color 0.25s ease;
}

.header-title {
  margin: 0;
  font-size: 26px;
  line-height: 1.15;
  color: #fff;
}

.header-desc {
  margin: 8px 0 0;
  color: rgba(255, 255, 255, 0.85);
  font-size: 14px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.cards-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.stat-card {
  border: 1px solid var(--line-soft);
  border-radius: 18px;
  background: var(--card-bg);
  backdrop-filter: blur(12px);
  box-shadow: 0 8px 20px rgba(16, 40, 89, 0.04);
  padding: 14px 16px;
  transition: background 0.35s ease, border-color 0.25s ease;
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.card-head-left {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-1);
  font-size: 16px;
}

.card-icon {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  display: grid;
  place-items: center;
}

.card-icon--blue {
  color: #2f6bff;
  background: rgba(47, 107, 255, 0.14);
}

.card-icon--green {
  color: #1fb97a;
  background: rgba(31, 185, 122, 0.14);
}

.card-icon--pink {
  color: #e04992;
  background: rgba(224, 73, 146, 0.14);
}

.card-icon--amber {
  color: #de9a21;
  background: rgba(222, 154, 33, 0.14);
}

.card-body {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 8px;
}

.card-metrics {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.metric-row {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.metric-main {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: fit-content;
}

.metric-label {
  font-size: 13px;
  color: var(--text-2);
}

.metric-value {
  font-size: 20px;
  line-height: 1.1;
  font-weight: 700;
  color: var(--text-1);
}

.metric-change {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 700;
  width: fit-content;
  padding: 2px 8px;
  border-radius: 999px;
}

.metric-change i,
.metric-change em {
  font-style: normal;
}

.metric-change i {
  font-size: 10px;
}

.metric-change.is-up {
  color: #149046;
  background: rgba(20, 144, 70, 0.12);
}

.metric-change.is-down {
  color: #c13636;
  background: rgba(193, 54, 54, 0.12);
}

.metric-change.is-flat {
  color: #667e9e;
  background: rgba(102, 126, 158, 0.12);
}

.analysis-card {
  border: 1px solid var(--line-soft);
  border-radius: 18px;
  background: var(--card-bg);
  backdrop-filter: blur(12px);
  box-shadow: 0 8px 20px rgba(16, 40, 89, 0.04);
  padding: 18px 18px 14px;
  transition: background 0.35s ease, border-color 0.25s ease;
}

.analysis-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  border-bottom: 1px solid var(--line-soft);
  padding-bottom: 12px;
}

.analysis-title-row h3 {
  margin: 0;
  font-size: 22px;
  color: var(--text-1);
}

.analysis-title-row p {
  margin: 6px 0 0;
  font-size: 13px;
  color: var(--text-2);
}

.analysis-tabs {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.analysis-tabs .tab-btn:not(:last-child)::after {
  content: "/";
  margin-left: 12px;
  color: var(--line-soft);
}

.tab-btn {
  border: none;
  background: transparent;
  color: var(--text-2);
  font-size: 17px;
  font-weight: 600;
  cursor: pointer;
  padding: 0;
}

.tab-btn.active {
  color: var(--text-1);
}

.analysis-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  margin: 14px 0 8px;
  color: var(--text-2);
  font-size: 14px;
}

.analysis-chart {
  padding: 10px 4px 2px;
}

.bars-wrap {
  display: grid;
  grid-template-columns: repeat(7, minmax(0, 1fr));
  gap: 8px;
  align-items: end;
  min-height: 250px;
}

.bar-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.bar-top {
  font-size: 12px;
  color: var(--text-2);
}

.bar-track {
  width: 100%;
  height: 180px;
  border-radius: 10px;
  background: var(--tag-bg);
  border: 1px solid var(--line-soft);
  display: flex;
  align-items: flex-end;
  justify-content: center;
  overflow: hidden;
}

.bar-fill {
  display: block;
  width: calc(100% - 14px);
  min-width: 12px;
  border-radius: 8px 8px 2px 2px;
  background: linear-gradient(180deg, var(--brand) 0%, var(--brand-deep) 100%);
}

.bar-label {
  font-size: 12px;
  color: var(--text-2);
}

.empty-tip {
  color: var(--text-2);
  font-size: 14px;
  text-align: center;
  padding: 30px 0 26px;
}

@media (max-width: 1380px) {
  .cards-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 900px) {
  .header-row {
    flex-direction: column;
    align-items: flex-start;
  }

  .analysis-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .bars-wrap {
    gap: 6px;
    min-height: 220px;
  }
}

@media (max-width: 640px) {
  .cards-grid {
    grid-template-columns: 1fr;
  }

  .header-title {
    font-size: 22px;
  }

  .metric-value {
    font-size: 18px;
  }

  .tab-btn {
    font-size: 15px;
  }

  .analysis-summary {
    gap: 10px;
    flex-wrap: wrap;
  }
}
</style>
