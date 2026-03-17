<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { createDiscreteApi, type DataTableColumns, NButton, NPopconfirm, NSpace, NSwitch, NTag } from "naive-ui";
import {
  checkCookieQrStatus,
  createCookie,
  deleteCookie,
  getCookieQrKey,
  listCookies,
  updateCookie,
  verifyAllCookies,
  verifyCookie,
  type CookieItem,
} from "@/api/modules/admin";

const { message } = createDiscreteApi(["message"]);

const loading = ref(false);
const verifyingAll = ref(false);

const showCreate = ref(false);
const showEdit = ref(false);
const showView = ref(false);
const showQrCreate = ref(false);
const rows = ref<CookieItem[]>([]);
const viewingCookie = ref("");
const qrUnikey = ref("");
const qrLoginUrl = ref("");
const qrStatusCode = ref(801);
const qrNickname = ref("");
const qrAvatar = ref("");
const qrLoading = ref(false);
const qrChecking = ref(false);
const qrSaving = ref(false);
let qrTimer: ReturnType<typeof setInterval> | null = null;

const form = reactive({
  provider: "netease",
  value: "",
  active: true,
});

const editForm = reactive({
  id: 0,
  value: "",
});

const statusLabelMap: Record<string, string> = {
  unknown: "未校验",
  valid: "有效",
  invalid: "无效",
};
const qrStatusLabelMap: Record<number, string> = {
  800: "二维码已过期，正在刷新...",
  801: "请使用网易云音乐 App 扫码登录",
  802: "扫码成功，请在手机端确认登录",
  803: "登录成功，正在保存 Cookie...",
};
const qrStatusText = computed(() => qrStatusLabelMap[qrStatusCode.value] || "等待扫码...");

function formatTime(raw: string | null): string {
  if (!raw) return "-";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(
    d.getSeconds(),
  )}`;
}

function statusTagType(status: CookieItem["status"]): "default" | "success" | "error" {
  if (status === "valid") return "success";
  if (status === "invalid") return "error";
  return "default";
}

function statusText(status: CookieItem["status"]): string {
  return statusLabelMap[status] || status;
}

function membershipText(row: CookieItem): string {
  if (row.red_vip_level > 0) return `SVIP${row.red_vip_level}`;
  // 部分账号会返回 100+ 的 VIP 类型编码，这里按 SVIP 兜底展示。
  if (row.vip_type >= 100) return "SVIP";
  if (row.vip_type > 0) return "VIP";
  return "普通用户";
}

function extractCookieCore(raw: string): string {
  const text = (raw || "").trim();
  if (!text) return "";
  const compact = text.replace(/[\r\n\t]/g, "");
  const matched = compact.match(/(?:^|;)\s*MUSIC_U\s*=\s*([^;]+)/i);
  if (matched?.[1]) {
    return matched[1].trim().replace(/^"+|"+$/g, "");
  }
  if (/^music_u\s*=/i.test(compact)) {
    return compact.replace(/^music_u\s*=/i, "").trim().replace(/^"+|"+$/g, "");
  }
  return compact.replace(/\s+/g, "").replace(/^"+|"+$/g, "");
}

function summarizeCookie(raw: string): string {
  const text = extractCookieCore(raw);
  if (!text) return "-";
  if (text.length <= 26) return text;
  return `${text.slice(0, 12)}...${text.slice(-12)}`;
}

function clearQrTimer() {
  if (!qrTimer) return;
  clearInterval(qrTimer);
  qrTimer = null;
}

function resetQrState() {
  qrUnikey.value = "";
  qrLoginUrl.value = "";
  qrStatusCode.value = 801;
  qrNickname.value = "";
  qrAvatar.value = "";
  qrLoading.value = false;
  qrChecking.value = false;
  qrSaving.value = false;
}

async function loadQrKey() {
  qrLoading.value = true;
  try {
    const data = await getCookieQrKey();
    const key = (data.unikey || "").trim();
    if (!key) throw new Error("二维码 key 获取失败");
    qrUnikey.value = key;
    qrLoginUrl.value = data.qr_url || `https://music.163.com/login?codekey=${key}`;
    qrStatusCode.value = 801;
    qrNickname.value = "";
    qrAvatar.value = "";
  } finally {
    qrLoading.value = false;
  }
}

async function pollQrStatus() {
  if (!showQrCreate.value || !qrUnikey.value || qrChecking.value || qrSaving.value) return;
  qrChecking.value = true;
  try {
    const data = await checkCookieQrStatus(qrUnikey.value);
    const code = Number(data.code) || 801;
    qrStatusCode.value = code;
    qrNickname.value = (data.nickname || "").trim();
    qrAvatar.value = (data.avatar_url || "").trim();

    if (code === 800) {
      await loadQrKey();
      return;
    }
    if (code !== 803) return;

    qrSaving.value = true;
    const musicU = extractCookieCore(data.cookie || data.music_u || "");
    if (!musicU) {
      message.error("扫码成功，但未获取到 MUSIC_U，请重试");
      qrSaving.value = false;
      await loadQrKey();
      return;
    }

    const autoLabel = `扫码-${new Date().toISOString().replace(/[:.]/g, "-")}`;
    await createCookie({ provider: "netease", label: autoLabel, value: musicU, active: true });
    message.success("扫码添加 Cookie 成功");
    showQrCreate.value = false;
    clearQrTimer();
    resetQrState();
    await loadRows();
  } catch (error) {
    message.error((error as Error).message);
    qrSaving.value = false;
  } finally {
    qrChecking.value = false;
  }
}

async function openQrCreateModal() {
  showQrCreate.value = true;
  clearQrTimer();
  resetQrState();
  try {
    await loadQrKey();
  } catch (error) {
    message.error((error as Error).message);
    return;
  }
  qrTimer = setInterval(() => {
    void pollQrStatus();
  }, 1000);
}

function closeQrCreateModal() {
  showQrCreate.value = false;
  clearQrTimer();
  resetQrState();
}

const columns: DataTableColumns<CookieItem> = [
  { title: "ID", key: "id", width: 64, align: "center" },
  {
    title: "账号",
    key: "nickname",
    width: 220,
    align: "center",
    render: (row) =>
      h("div", { class: "account-inline" }, [
        h("span", { class: "nickname" }, row.nickname || "-"),
        h("span", { class: "account-sep" }, " | "),
        h("span", { class: "membership" }, membershipText(row)),
      ]),
  },
  {
    title: "状态",
    key: "status",
    width: 88,
    align: "center",
    render: (row) => h(NTag, { type: statusTagType(row.status), size: "small" }, { default: () => statusText(row.status) }),
  },
  {
    title: "Cookie",
    key: "value",
    align: "center",
    render: (row) =>
      h(
        "span",
        {
          class: "cookie-summary",
          title: extractCookieCore(row.value) || "-",
        },
        summarizeCookie(row.value),
      ),
  },
  {
    title: "使用/失败",
    key: "call_count",
    width: 96,
    align: "center",
    render: (row) => `${row.call_count} / ${row.fail_count}`,
  },
  {
    title: "最近校验",
    key: "last_checked",
    width: 186,
    align: "center",
    render: (row) =>
      h(
        "span",
        {
          class: "checked-time",
          title: formatTime(row.last_checked),
        },
        formatTime(row.last_checked),
      ),
  },
  {
    title: "启用",
    key: "active",
    width: 78,
    align: "center",
    render: (row) =>
      h(NSwitch, {
        value: row.active,
        onUpdateValue: async (value: boolean) => {
          await onToggleActive(row, value);
        },
      }),
  },
  {
    title: "操作",
    key: "actions",
    width: 230,
    align: "center",
    render: (row) =>
      h(
        NSpace,
        { size: 6, wrapItem: false, justify: "center" },
        {
          default: () => [
            h(
              NButton,
              {
                type: "primary",
                tertiary: true,
                size: "small",
                onClick: () => onVerify(row),
              },
              { default: () => "校验" },
            ),
            h(
              NButton,
              { type: "info", tertiary: true, size: "small", onClick: () => openEdit(row) },
              { default: () => "编辑" },
            ),
            h(
              NButton,
              { tertiary: true, size: "small", onClick: () => onView(row) },
              { default: () => "查看" },
            ),
            h(
              NPopconfirm,
              { onPositiveClick: () => onDelete(row.id) },
              {
                default: () => "确定删除这条 Cookie 吗？",
                trigger: () => h(NButton, { type: "error", tertiary: true, size: "small" }, { default: () => "删除" }),
              },
            ),
          ],
        },
      ),
  },
];

function openEdit(row: CookieItem) {
  editForm.id = row.id;
  editForm.value = "";
  showEdit.value = true;
}

function onView(row: CookieItem) {
  viewingCookie.value = extractCookieCore(row.value) || "-";
  showView.value = true;
}

async function loadRows() {
  loading.value = true;
  try {
    const data = await listCookies();
    rows.value = data.map((row) => ({
      ...row,
      value: extractCookieCore(row.value),
    }));
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

async function onCreate() {
  const value = extractCookieCore(form.value);
  if (!value) {
    message.warning("请填写 Cookie 内容");
    return;
  }
  try {
    const autoLabel = `Cookie-${new Date().toISOString().replace(/[:.]/g, "-")}`;
    await createCookie({ ...form, label: autoLabel, value });
    message.success("新增成功");
    showCreate.value = false;
    form.value = "";
    form.active = true;
    await loadRows();
  } catch (error) {
    message.error((error as Error).message);
  }
}

async function onEdit() {
  const value = extractCookieCore(editForm.value);
  if (!value) {
    message.warning("请填写新 Cookie 值");
    return;
  }
  try {
    await updateCookie(editForm.id, { value });
    message.success("更新成功");
    showEdit.value = false;
    await loadRows();
  } catch (error) {
    message.error((error as Error).message);
  }
}

async function onToggleActive(row: CookieItem, active: boolean) {
  try {
    await updateCookie(row.id, { active });
    row.active = active;
    message.success("状态已更新");
  } catch (error) {
    message.error((error as Error).message);
  }
}

async function onDelete(id: number) {
  try {
    await deleteCookie(id);
    message.success("删除成功");
    await loadRows();
  } catch (error) {
    message.error((error as Error).message);
  }
}

async function onVerify(row: CookieItem) {
  const loadingMessage = message.loading("正在校验 Cookie，请稍候...", { duration: 0 });
  try {
    const data = await verifyCookie(row.id);
    const stateText = data.status === "valid" ? "有效" : data.status === "invalid" ? "无效" : "未校验";
    message.success(`校验完成：${stateText}`);
    await loadRows();
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    loadingMessage.destroy();
  }
}

async function onVerifyAll() {
  verifyingAll.value = true;
  try {
    const data = await verifyAllCookies();
    message.success(`批量校验完成：总计 ${data.total}，有效 ${data.valid}，无效 ${data.invalid}`);
    await loadRows();
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    verifyingAll.value = false;
  }
}

onMounted(loadRows);
onBeforeUnmount(() => {
  clearQrTimer();
});
</script>

<template>
  <section>
    <header class="title-row">
      <div>
        <h2 class="section-title">Cookie 池管理</h2>
        <p class="section-subtitle">支持手动维护、实时校验与状态管理。</p>
      </div>
      <n-space>
        <n-button secondary @click="loadRows">刷新</n-button>
        <n-button secondary :loading="verifyingAll" @click="onVerifyAll">校验全部</n-button>
        <n-button type="primary" secondary @click="openQrCreateModal">扫码添加</n-button>
        <n-button type="primary" @click="showCreate = true">新增 Cookie</n-button>
      </n-space>
    </header>

    <n-card class="table-card">
      <n-data-table
        :columns="columns"
        :data="rows"
        :loading="loading"
        :bordered="false"
        :single-line="false"
        table-layout="fixed"
        size="small"
      />
    </n-card>

    <n-modal v-model:show="showCreate" preset="card" title="新增 Cookie" style="max-width: 560px">
      <n-form label-placement="top">
        <n-form-item label="Cookie 内容">
          <n-input
            v-model:value="form.value"
            type="textarea"
            :autosize="{ minRows: 4, maxRows: 10 }"
            placeholder="请输入 MUSIC_U 的值"
          />
        </n-form-item>
        <n-form-item>
          <n-switch v-model:value="form.active">
            <template #checked>启用</template>
            <template #unchecked>禁用</template>
          </n-switch>
        </n-form-item>
        <n-space justify="end">
          <n-button @click="showCreate = false">取消</n-button>
          <n-button type="primary" @click="onCreate">保存</n-button>
        </n-space>
      </n-form>
    </n-modal>

    <n-modal v-model:show="showEdit" preset="card" title="编辑 Cookie" style="max-width: 560px">
      <n-form label-placement="top">
        <n-form-item label="新 Cookie 内容">
          <n-input
            v-model:value="editForm.value"
            type="textarea"
            :autosize="{ minRows: 4, maxRows: 10 }"
            placeholder="请输入新的 MUSIC_U 值"
          />
        </n-form-item>
        <n-space justify="end">
          <n-button @click="showEdit = false">取消</n-button>
          <n-button type="primary" @click="onEdit">保存修改</n-button>
        </n-space>
      </n-form>
    </n-modal>

    <n-modal v-model:show="showView" preset="card" title="查看 Cookie" style="max-width: 720px">
      <div class="cookie-view-text">{{ viewingCookie }}</div>
    </n-modal>

    <n-modal v-model:show="showQrCreate" preset="card" title="扫码添加 Cookie" style="max-width: 520px" @update:show="(show: boolean) => !show && closeQrCreateModal()">
      <div class="qr-create-wrap">
        <div class="qr-code-box">
          <n-qr-code v-if="qrLoginUrl" :value="qrLoginUrl" :size="220" />
          <div v-else class="qr-code-loading">二维码加载中...</div>
        </div>
        <p class="qr-status">{{ qrStatusText }}</p>
        <div v-if="qrNickname" class="qr-user">
          <n-avatar v-if="qrAvatar" round :src="qrAvatar" :size="28" />
          <span>{{ qrNickname }}</span>
        </div>
        <n-space justify="center">
          <n-button @click="closeQrCreateModal">关闭</n-button>
          <n-button secondary :loading="qrLoading" @click="loadQrKey">刷新二维码</n-button>
        </n-space>
      </div>
    </n-modal>
  </section>
</template>

<style scoped>
.title-row {
  display: flex;
  justify-content: center;
  gap: 14px;
  align-items: center;
  margin-bottom: 14px;
  text-align: center;
}

.table-card {
  width: 100%;
  max-width: 100%;
  overflow: hidden;
  border-radius: 16px;
  background: var(--card-bg);
  border: 1px solid rgba(20, 41, 78, 0.08);
}

.table-card :deep(.n-data-table-wrapper) {
  overflow-x: hidden !important;
}

.table-card :deep(.n-data-table-th) {
  text-align: center;
  white-space: nowrap;
}

.table-card :deep(.n-data-table-th__title) {
  width: 100%;
  display: flex;
  justify-content: center;
  align-items: center;
}

.table-card :deep(.n-data-table-td) {
  vertical-align: middle;
  white-space: nowrap;
}

.table-card :deep(.account-inline) {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-width: 0;
  width: 100%;
}

.table-card :deep(.nickname) {
  font-weight: 600;
  color: var(--text-1);
  white-space: nowrap;
}

.table-card :deep(.membership) {
  font-weight: 600;
  font-size: 12px;
  color: var(--text-2);
  flex-shrink: 0;
}

.table-card :deep(.account-sep) {
  color: var(--text-2);
  opacity: 0.7;
  flex-shrink: 0;
}

.table-card :deep(.cookie-summary) {
  display: inline-block;
  width: 100%;
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: inherit;
  font-weight: 400;
  color: inherit;
  text-align: center;
}

.table-card :deep(.checked-time) {
  display: inline-block;
  white-space: nowrap;
  text-align: center;
  font-variant-numeric: tabular-nums;
}

.cookie-view-text {
  border: 1px solid rgba(20, 41, 78, 0.12);
  border-radius: 10px;
  background: rgba(248, 250, 253, 0.85);
  padding: 12px 14px;
  white-space: normal;
  word-break: break-all;
  overflow-wrap: anywhere;
  line-height: 1.6;
  color: var(--text-1);
}

.qr-create-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 8px 4px 2px;
}

.qr-code-box {
  width: 236px;
  height: 236px;
  border-radius: 14px;
  border: 1px solid rgba(20, 41, 78, 0.1);
  background: rgba(248, 250, 253, 0.86);
  display: grid;
  place-items: center;
}

.qr-code-loading {
  font-size: 13px;
  color: var(--text-2);
}

.qr-status {
  margin: 0;
  font-size: 13px;
  color: var(--text-2);
  text-align: center;
}

.qr-user {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: var(--text-1);
  font-weight: 600;
}

@media (max-width: 760px) {
  .title-row {
    flex-direction: column;
    align-items: center;
  }
}
</style>
