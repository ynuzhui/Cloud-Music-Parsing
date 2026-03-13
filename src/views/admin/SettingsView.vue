<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { createDiscreteApi } from "naive-ui";
import { DeviceFloppy } from "@vicons/tabler";
import { getSettings, saveSettings, type SystemSettings } from "@/api/modules/admin";
import { clearPublicSiteSettingsCache } from "@/api/modules/site";
import { useSettingsStore } from "@/stores/settings";

const { message } = createDiscreteApi(["message"]);
const settingsStore = useSettingsStore();
const loading = ref(false);
const saving = ref(false);
const initialized = ref(false);

const fullSettings = ref<SystemSettings | null>(null);
const snapshot = ref("");

const siteForm = reactive({
  name: "",
  keywords: "",
  description: "",
  icp_no: "",
  police_no: "",
});

const featureForm = reactive({
  allow_register: false,
  register_email_verify: false,
  default_parse_quality: "standard" as "standard" | "exhigh" | "lossless" | "hires" | "sky" | "jyeffect" | "jymaster",
  parse_require_login: true,
  default_daily_parse_limit: 100,
  default_concurrency_limit: 2,
  cookie_auto_verify: false,
});

const numericDraft = reactive({
  dailyLimit: "100",
  concurrencyLimit: "2",
  dailyLimitInvalid: false,
  concurrencyLimitInvalid: false,
});

const captchaForm = reactive({
  enabled: false,
  provider: "geetest" as "geetest" | "cloudflare",
  geetest_captcha_id: "",
  geetest_captcha_key: "",
  cloudflare_site_key: "",
  cloudflare_secret_key: "",
});

const parseQualityOptions = [
  { label: "标准", value: "standard" },
  { label: "极高", value: "exhigh" },
  { label: "无损", value: "lossless" },
  { label: "Hi-Res", value: "hires" },
  { label: "沉浸环绕声", value: "sky" },
  { label: "高清环绕声", value: "jyeffect" },
  { label: "超清母带", value: "jymaster" },
];

const captchaProviderOptions = [
  { label: "极验 4.0（推荐）", value: "geetest" },
  { label: "Cloudflare Turnstile", value: "cloudflare" },
];

const currentPayload = computed(() => ({
  site: { ...siteForm },
  feature: { ...featureForm },
  captcha: { ...captchaForm },
}));

const currentSnapshot = computed(() => JSON.stringify(currentPayload.value));
const hasNumericInputError = computed(() => numericDraft.dailyLimitInvalid || numericDraft.concurrencyLimitInvalid);
const hasPendingChanges = computed(() => initialized.value && !loading.value && !hasNumericInputError.value && currentSnapshot.value !== snapshot.value);

function isNonNegativeInteger(value: string): boolean {
  return /^\d+$/.test(value.trim());
}

function syncNumericDraft() {
  numericDraft.dailyLimit = String(featureForm.default_daily_parse_limit);
  numericDraft.concurrencyLimit = String(featureForm.default_concurrency_limit);
  numericDraft.dailyLimitInvalid = false;
  numericDraft.concurrencyLimitInvalid = false;
}

function onDailyLimitInput(value: string) {
  numericDraft.dailyLimit = value;
  if (!isNonNegativeInteger(value)) {
    numericDraft.dailyLimitInvalid = true;
    return;
  }
  numericDraft.dailyLimitInvalid = false;
  featureForm.default_daily_parse_limit = Number.parseInt(value.trim(), 10);
}

function onConcurrencyLimitInput(value: string) {
  numericDraft.concurrencyLimit = value;
  if (!isNonNegativeInteger(value)) {
    numericDraft.concurrencyLimitInvalid = true;
    return;
  }
  numericDraft.concurrencyLimitInvalid = false;
  featureForm.default_concurrency_limit = Number.parseInt(value.trim(), 10);
}

function onDailyLimitBlur() {
  if (numericDraft.dailyLimitInvalid) {
    message.warning("默认每日解析次数仅支持非负整数");
    return;
  }
  numericDraft.dailyLimit = String(featureForm.default_daily_parse_limit);
}

function onConcurrencyLimitBlur() {
  if (numericDraft.concurrencyLimitInvalid) {
    message.warning("默认并发上限仅支持非负整数");
    return;
  }
  numericDraft.concurrencyLimit = String(featureForm.default_concurrency_limit);
}

function isSmtpConfigured(): boolean {
  const smtp = fullSettings.value?.smtp;
  if (!smtp) return false;
  return !!(smtp.host?.trim()) && Number(smtp.port || 0) > 0 && !!(smtp.user?.trim());
}

function onCookieAutoVerifyChange(value: boolean) {
  if (value && !isSmtpConfigured()) {
    message.warning("请先前往 SMTP 配置页完成发件服务设置后再开启此功能");
    return;
  }
  featureForm.cookie_auto_verify = value;
}

async function loadSettings() {
  loading.value = true;
  try {
    const data = await getSettings();
    fullSettings.value = data;
    Object.assign(siteForm, data.site);
    Object.assign(featureForm, data.feature);
    featureForm.default_parse_quality = data.feature?.default_parse_quality || "standard";
    featureForm.parse_require_login = data.feature?.parse_require_login ?? true;
    featureForm.default_daily_parse_limit = Number(data.feature?.default_daily_parse_limit ?? 100);
    featureForm.default_concurrency_limit = Number(data.feature?.default_concurrency_limit ?? 2);
    Object.assign(captchaForm, {
      enabled: data.captcha?.enabled ?? false,
      provider: data.captcha?.provider ?? "geetest",
      geetest_captcha_id: data.captcha?.geetest_captcha_id ?? "",
      geetest_captcha_key: data.captcha?.geetest_captcha_key ?? "",
      cloudflare_site_key: data.captcha?.cloudflare_site_key ?? "",
      cloudflare_secret_key: data.captcha?.cloudflare_secret_key ?? "",
    });
    syncNumericDraft();
    settingsStore.syncSiteName(data.site?.name);
    settingsStore.applyDocumentTitle();

    snapshot.value = currentSnapshot.value;
    initialized.value = true;
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

async function onSave() {
  if (!fullSettings.value) return;
  if (hasNumericInputError.value) {
    message.warning("请先修正数字输入项后再保存");
    return;
  }
  saving.value = true;
  try {
    fullSettings.value.site = { ...currentPayload.value.site };
    fullSettings.value.feature = { ...currentPayload.value.feature };
    fullSettings.value.captcha = { ...currentPayload.value.captcha };
    await saveSettings(fullSettings.value);
    syncNumericDraft();
    snapshot.value = currentSnapshot.value;
    clearPublicSiteSettingsCache();
    settingsStore.setSiteName(siteForm.name);
    settingsStore.applyDocumentTitle();
    message.success("设置保存成功");
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    saving.value = false;
  }
}

function onKeydown(event: KeyboardEvent) {
  const key = (event.key || "").toLowerCase();
  if ((event.ctrlKey || event.metaKey) && key === "s") {
    event.preventDefault();
    if (hasPendingChanges.value && !saving.value) {
      void onSave();
    }
  }
}

onMounted(() => {
  void loadSettings();
  window.addEventListener("keydown", onKeydown);
});

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <section class="settings-page">
    <header class="title-row">
      <div>
        <h2 class="section-title">站点配置</h2>
        <p class="section-subtitle">基本站点信息与功能设置。</p>
      </div>
    </header>

    <n-spin :show="loading">
      <n-space vertical :size="16" class="settings-stack" :class="{ 'settings-stack--dock': hasPendingChanges }">
        <n-card title="站点信息" class="setting-card">
          <n-form label-placement="top" :show-feedback="false">
            <n-grid :cols="24" :x-gap="14" :y-gap="8">
              <n-form-item-gi :span="12" label="站点名称">
                <n-input v-model:value="siteForm.name" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="12" label="关键词">
                <n-input v-model:value="siteForm.keywords" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="24" label="站点介绍">
                <n-input v-model:value="siteForm.description" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="12" label="ICP备案号">
                <n-input v-model:value="siteForm.icp_no" placeholder="例如：粤ICP备12345678号" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="12" label="公安联网备案号">
                <n-input v-model:value="siteForm.police_no" placeholder="例如：粤公网安备12345678901234号" size="large" />
              </n-form-item-gi>
            </n-grid>
          </n-form>
        </n-card>

        <n-card title="功能设置" class="setting-card">
          <n-form label-placement="top" :show-feedback="false">
            <n-form-item label="是否允许用户注册">
              <n-switch v-model:value="featureForm.allow_register" />
            </n-form-item>
            <n-form-item label="注册时启用邮箱验证码">
              <n-switch v-model:value="featureForm.register_email_verify" />
            </n-form-item>
            <n-form-item label="是否必须登录后解析">
              <n-switch v-model:value="featureForm.parse_require_login" />
            </n-form-item>
            <n-form-item label="Cookie 自动校验">
              <n-switch :value="featureForm.cookie_auto_verify" @update:value="onCookieAutoVerifyChange" />
            </n-form-item>
            <n-form-item label="默认解析音质">
              <n-select
                v-model:value="featureForm.default_parse_quality"
                :options="parseQualityOptions"
                placeholder="请选择默认解析音质"
                size="large"
              />
            </n-form-item>
            <n-grid :cols="24" :x-gap="14" :y-gap="8">
              <n-form-item-gi :span="12" label="默认每日解析次数">
                <n-input
                  :value="numericDraft.dailyLimit"
                  inputmode="numeric"
                  placeholder="请输入非负整数"
                  size="large"
                  style="width: 100%"
                  @update:value="onDailyLimitInput"
                  @blur="onDailyLimitBlur"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" label="默认并发上限">
                <n-input
                  :value="numericDraft.concurrencyLimit"
                  inputmode="numeric"
                  placeholder="请输入非负整数"
                  size="large"
                  style="width: 100%"
                  @update:value="onConcurrencyLimitInput"
                  @blur="onConcurrencyLimitBlur"
                />
              </n-form-item-gi>
            </n-grid>
          </n-form>
        </n-card>

        <n-card title="验证码设置" class="setting-card">
          <n-form label-placement="top" :show-feedback="false">
            <n-form-item label="启用验证码">
              <n-switch v-model:value="captchaForm.enabled" />
            </n-form-item>
            <n-grid :cols="24" :x-gap="14" :y-gap="8">
              <n-form-item-gi :span="24" label="验证码提供方">
                <n-select
                  v-model:value="captchaForm.provider"
                  :options="captchaProviderOptions"
                  placeholder="请选择验证码提供方"
                  size="large"
                />
              </n-form-item-gi>
            </n-grid>

            <template v-if="captchaForm.provider === 'geetest'">
              <n-grid :cols="24" :x-gap="14" :y-gap="8">
                <n-form-item-gi :span="12" label="极验 Captcha ID">
                  <n-input v-model:value="captchaForm.geetest_captcha_id" placeholder="请输入极验验证码 ID" size="large" />
                </n-form-item-gi>
                <n-form-item-gi :span="12" label="极验 Private Key">
                  <n-input v-model:value="captchaForm.geetest_captcha_key" placeholder="请输入极验私钥" size="large" type="password" show-password-on="click" />
                </n-form-item-gi>
              </n-grid>
            </template>

            <template v-else>
              <n-grid :cols="24" :x-gap="14" :y-gap="8">
                <n-form-item-gi :span="12" label="Cloudflare Site Key">
                  <n-input v-model:value="captchaForm.cloudflare_site_key" placeholder="请输入 Cloudflare 站点密钥" size="large" />
                </n-form-item-gi>
                <n-form-item-gi :span="12" label="Cloudflare Secret Key">
                  <n-input v-model:value="captchaForm.cloudflare_secret_key" placeholder="请输入 Cloudflare 私密密钥" size="large" type="password" show-password-on="click" />
                </n-form-item-gi>
              </n-grid>
            </template>
          </n-form>
        </n-card>
      </n-space>
    </n-spin>

    <transition name="save-dock">
      <div v-if="hasPendingChanges" class="save-dock">
        <span class="save-dock-label">有未保存的更改</span>
        <n-button type="primary" :loading="saving" @click="onSave">
          <template #icon>
            <n-icon><DeviceFloppy /></n-icon>
          </template>
          保存设置
        </n-button>
        <span class="save-dock-shortcut">Ctrl + S</span>
      </div>
    </transition>
  </section>
</template>

<style scoped>
.title-row {
  display: flex;
  justify-content: flex-start;
  gap: 14px;
  align-items: flex-start;
  margin-bottom: 14px;
}

.setting-card {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(20, 41, 78, 0.08);
}

.field-tip {
  margin-top: 6px;
  color: var(--text-2);
  font-size: 12px;
  line-height: 1.55;
}

.settings-stack {
  transition: padding-bottom 0.22s ease;
}

.settings-stack--dock {
  padding-bottom: 30px;
}

.save-dock {
  position: fixed;
  right: 20px;
  bottom: 16px;
  z-index: 1200;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 26px;
  background: rgba(250, 252, 255, 0.96);
  border: 1px solid rgba(20, 41, 78, 0.1);
  box-shadow: 0 10px 22px rgba(20, 41, 78, 0.09);
  backdrop-filter: blur(8px);
}

.save-dock-label {
  color: var(--text-1);
  font-weight: 700;
  font-size: 13px;
}

.save-dock :deep(.n-button) {
  height: 36px;
  padding: 0 16px;
  border-radius: 12px;
  font-weight: 700;
}

.save-dock-shortcut {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 70px;
  height: 32px;
  padding: 0 8px;
  border-radius: 10px;
  background: rgba(18, 43, 89, 0.06);
  border: 1px solid rgba(18, 43, 89, 0.12);
  color: var(--text-2);
  font-size: 12px;
  letter-spacing: 0.03em;
}

.save-dock-enter-active,
.save-dock-leave-active {
  transition: all 0.22s ease;
}

.save-dock-enter-from,
.save-dock-leave-to {
  opacity: 0;
  transform: translateY(16px);
}

@media (max-width: 760px) {
  .title-row {
    flex-direction: column;
    align-items: flex-start;
  }

  .save-dock {
    right: 10px;
    left: 10px;
    bottom: 10px;
    gap: 5px;
    padding: 7px 8px;
  }

  .save-dock-label {
    font-size: 11px;
  }

  .save-dock :deep(.n-button) {
    height: 32px;
    padding: 0 12px;
    border-radius: 10px;
  }

  .save-dock-shortcut {
    min-width: 58px;
    height: 28px;
    border-radius: 9px;
    font-size: 10px;
  }

  .settings-stack--dock {
    padding-bottom: 44px;
  }
}
</style>
