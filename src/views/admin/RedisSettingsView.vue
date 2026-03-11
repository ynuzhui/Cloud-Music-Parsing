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
  host: "127.0.0.1",
  port: 6379,
  pass: "",
  db: 0
});

const redisDraft = reactive({
  port: "6379",
  db: "0",
  portError: "",
  dbError: "",
});

const currentPayload = computed(() => ({ ...form }));
const currentSnapshot = computed(() => JSON.stringify(currentPayload.value));
const hasInputError = computed(() => form.enabled && (!!redisDraft.portError || !!redisDraft.dbError));
const hasPendingChanges = computed(() => initialized.value && !loading.value && !hasInputError.value && currentSnapshot.value !== snapshot.value);

function validateIntegerRange(value: string, min: number, max: number, label: string): string {
  const trimmed = value.trim();
  if (!/^\d+$/.test(trimmed)) {
    return `请输入${label}（仅数字）`;
  }
  const parsed = Number.parseInt(trimmed, 10);
  if (Number.isNaN(parsed) || parsed < min || parsed > max) {
    return `${label}范围必须在 ${min}-${max}`;
  }
  return "";
}

function syncRedisDraft() {
  redisDraft.port = String(form.port);
  redisDraft.db = String(form.db);
  redisDraft.portError = "";
  redisDraft.dbError = "";
}

function onPortInput(value: string) {
  redisDraft.port = value;
  const error = validateIntegerRange(value, 1, 65535, "端口");
  redisDraft.portError = error;
  if (error) return;
  form.port = Number.parseInt(value.trim(), 10);
}

function onDbInput(value: string) {
  redisDraft.db = value;
  const error = validateIntegerRange(value, 0, 15, "数据库编号");
  redisDraft.dbError = error;
  if (error) return;
  form.db = Number.parseInt(value.trim(), 10);
}

function onPortBlur() {
  if (redisDraft.portError) {
    message.warning(redisDraft.portError);
    return;
  }
  redisDraft.port = String(form.port);
}

function onDbBlur() {
  if (redisDraft.dbError) {
    message.warning(redisDraft.dbError);
    return;
  }
  redisDraft.db = String(form.db);
}

async function loadSettings() {
  loading.value = true;
  try {
    const data = await getSettings();
    fullSettings.value = data;
    Object.assign(form, data.redis);
    syncRedisDraft();
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
    message.warning("请先修正 Redis 数字输入");
    return;
  }
  saving.value = true;
  try {
    fullSettings.value.redis = { ...currentPayload.value };
    await saveSettings(fullSettings.value);
    snapshot.value = currentSnapshot.value;
    message.success("Redis 配置保存成功");
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
        <h2 class="section-title">Redis 配置</h2>
        <p class="section-subtitle">配置缓存服务器以提高解析链接的响应速度（缓存 12 小时）。</p>
      </div>
    </header>

    <n-spin :show="loading">
      <div class="settings-body" :class="{ 'settings-body--dock': hasPendingChanges }">
        <n-card class="setting-card">
          <n-form label-placement="top" :show-feedback="false">
            <n-grid :cols="24" :x-gap="14" :y-gap="10">
              <n-form-item-gi :span="24" label="启用 Redis">
                <n-switch v-model:value="form.enabled" />
              </n-form-item-gi>
              <n-form-item-gi :span="14" label="主机地址">
                <n-input v-model:value="form.host" placeholder="例如：127.0.0.1" :disabled="!form.enabled" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="10" label="端口">
                <n-input
                  :value="redisDraft.port"
                  :disabled="!form.enabled"
                  inputmode="numeric"
                  placeholder="请输入端口（1-65535）"
                  size="large"
                  style="width:100%"
                  @update:value="onPortInput"
                  @blur="onPortBlur"
                />
              </n-form-item-gi>
              <n-form-item-gi :span="24" label="密码">
                <n-input v-model:value="form.pass" type="password" show-password-on="click" :disabled="!form.enabled" placeholder="留空表示不使用密码" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" label="数据库编号 (DB)">
                <n-input
                  :value="redisDraft.db"
                  :disabled="!form.enabled"
                  inputmode="numeric"
                  placeholder="请输入数据库编号（0-15）"
                  size="large"
                  style="width:100%"
                  @update:value="onDbInput"
                  @blur="onDbBlur"
                />
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
