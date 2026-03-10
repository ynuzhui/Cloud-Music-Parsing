<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { NCard, NGrid, NGridItem, NStatistic, NButton, NIcon } from "naive-ui";
import { createDiscreteApi } from "naive-ui";
import { Refresh, Users, FileMusic, Cookie } from "@vicons/tabler";
import { getDashboardStats, type DashboardStats } from "@/api/modules/admin";
import { useAuthStore } from "@/stores/auth";

const { message } = createDiscreteApi(["message"]);
const authStore = useAuthStore();
const loading = ref(false);
const stats = ref<DashboardStats>({
  user_count: 0,
  parse_count: 0,
  cookie_count: 0,
  trend_7days: []
});

const greeting = computed(() => {
  const hour = new Date().getHours();
  if (hour < 6) return "夜深了";
  if (hour < 12) return "上午好";
  if (hour < 18) return "下午好";
  return "晚上好";
});

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
  <section>
    <!-- 问候区域 -->
    <header class="greeting-row">
      <div>
        <h2 class="greeting-title">{{ greeting }}，{{ authStore.user?.username || "Admin" }}</h2>
        <p class="greeting-desc">这是您的网站数据概览。</p>
      </div>
      <n-button secondary :loading="loading" @click="loadStats">
        <template #icon>
          <n-icon><Refresh /></n-icon>
        </template>
        刷新统计
      </n-button>
    </header>

    <!-- 统计卡片 -->
    <n-grid :x-gap="16" :y-gap="16" :cols="24">
      <n-grid-item :span="8">
        <n-card class="stat-card">
          <div class="stat-inner">
            <div class="stat-icon-wrap stat-icon--blue">
              <n-icon size="26"><Users /></n-icon>
            </div>
            <n-statistic label="用户总数" :value="stats.user_count" />
          </div>
        </n-card>
      </n-grid-item>
      <n-grid-item :span="8">
        <n-card class="stat-card">
          <div class="stat-inner">
            <div class="stat-icon-wrap stat-icon--green">
              <n-icon size="26"><FileMusic /></n-icon>
            </div>
            <n-statistic label="解析总量" :value="stats.parse_count" />
          </div>
        </n-card>
      </n-grid-item>
      <n-grid-item :span="8">
        <n-card class="stat-card">
          <div class="stat-inner">
            <div class="stat-icon-wrap stat-icon--orange">
              <n-icon size="26"><Cookie /></n-icon>
            </div>
            <n-statistic label="Cookie 总量" :value="stats.cookie_count" />
          </div>
        </n-card>
      </n-grid-item>
    </n-grid>
  </section>
</template>

<style scoped>
.greeting-row {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  margin-bottom: 20px;
}

.greeting-title {
  margin: 0;
  font-size: 26px;
  font-weight: 700;
}

.greeting-desc {
  margin: 4px 0 0;
  color: var(--text-2);
  font-size: 14px;
}

/* ── 统计卡片 ── */
.stat-card {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.88);
  border: 1px solid rgba(20, 41, 78, 0.08);
}

.stat-inner {
  display: flex;
  align-items: center;
  gap: 16px;
}

.stat-icon-wrap {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  display: grid;
  place-items: center;
  flex-shrink: 0;
}

.stat-icon--blue {
  background: rgba(15, 111, 255, 0.12);
  color: #0f6fff;
}

.stat-icon--green {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
}

.stat-icon--orange {
  background: rgba(251, 146, 60, 0.12);
  color: #fb923c;
}

@media (max-width: 740px) {
  .greeting-row {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>


