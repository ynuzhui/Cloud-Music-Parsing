<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { createDiscreteApi } from "naive-ui";
import { DeviceFloppy } from "@vicons/tabler";
import { getSettings, saveSettings, type SystemSettings } from "@/api/modules/admin";

const { message } = createDiscreteApi(["message"]);
const loading = ref(false);
const saving = ref(false);
const initialized = ref(false);

const fullSettings = ref<SystemSettings | null>(null);
const snapshot = ref("");
const form = reactive({
  enabled: false,
  host: "",
  port: 0,
  username: "",
  password: "",
  protocol: "http"
});

const portDraft = reactive({
  value: "0",
  error: "",
});

const protocolOptions = [
  { label: "HTTP", value: "http" },
  { label: "HTTPS", value: "https" },
  { label: "SOCKS4", value: "socks4" },
  { label: "SOCKS5", value: "socks5" },
  { label: "SOCKS5H", value: "socks5h" }
];

const currentPayload = computed(() => ({ ...form }));
const currentSnapshot = computed(() => JSON.stringify(currentPayload.value));
const hasInputError = computed(() => form.enabled && !!portDraft.error);
const hasPendingChanges = computed(() => initialized.value && !loading.value && !hasInputError.value && currentSnapshot.value !== snapshot.value);

function validatePort(value: string): string {
  const trimmed = value.trim();
  if (!/^\d+$/.test(trimmed)) {
    return "请输入 0-65535 之间的数字端口";
  }
  const port = Number.parseInt(trimmed, 10);
  if (Number.isNaN(port) || port < 0 || port > 65535) {
    return "端口范围必须在 0-65535";
  }
  return "";
}

function syncPortDraft() {
  portDraft.value = String(form.port);
  portDraft.error = "";
}

function onPortInput(value: string) {
  portDraft.value = value;
  const error = validatePort(value);
  portDraft.error = error;
  if (error) return;
  form.port = Number.parseInt(value.trim(), 10);
}

function onPortBlur() {
  if (portDraft.error) {
    message.warning(portDraft.error);
    return;
  }
  portDraft.value = String(form.port);
}

async function loadSettings() {
  loading.value = true;
  try {
    const data = await getSettings();
    fullSettings.value = data;
    Object.assign(form, data.proxy);
    syncPortDraft();
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
  if (hasInputError.value) {
    message.warning("请先修正端口输入");
    return;
  }
  saving.value = true;
  try {
    fullSettings.value.proxy = { ...currentPayload.value };
    await saveSettings(fullSettings.value);
    snapshot.value = currentSnapshot.value;
    message.success("代理配置保存成功");
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
        <h2 class="section-title">代理配置</h2>
        <p class="section-subtitle">配置网络代理以加速第三方平台请求。</p>
      </div>
    </header>

    <n-spin :show="loading">
      <div class="settings-body" :class="{ 'settings-body--dock': hasPendingChanges }">
        <n-card class="setting-card">
          <n-form label-placement="top" :show-feedback="false">
            <n-grid :cols="24" :x-gap="14" :y-gap="10">
              <n-form-item-gi :span="24" label="启用代理">
                <n-switch v-model:value="form.enabled" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" label="代理协议">
                <n-select v-model:value="form.protocol" :options="protocolOptions" :disabled="!form.enabled" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="10" label="代理地址">
                <n-input v-model:value="form.host" placeholder="例如：127.0.0.1" :disabled="!form.enabled" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="6" label="端口">
                <n-input
                  :value="portDraft.value"
                  :disabled="!form.enabled"
                  inputmode="numeric"
                  placeholder="请输入端口（0-65535）"
                  size="large"
                  style="width:100%"
                  @update:value="onPortInput"
                  @blur="onPortBlur"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="12" label="用户名（可选）">
                <n-input v-model:value="form.username" :disabled="!form.enabled" placeholder="留空表示无认证" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="12" label="密码（可选）">
                <n-input v-model:value="form.password" type="password" show-password-on="click" :disabled="!form.enabled" placeholder="留空表示无认证" size="large" />
              </n-form-item-gi>
            </n-grid>
          </n-form>
        </n-card>
      </div>
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

.settings-body {
  transition: padding-bottom 0.22s ease;
}

.settings-body--dock {
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

  .settings-body--dock {
    padding-bottom: 44px;
  }
}
</style>
