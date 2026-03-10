<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { createDiscreteApi } from "naive-ui";
import { getSettings, saveSettings, type SystemSettings } from "@/api/modules/admin";

const { message } = createDiscreteApi(["message"]);
const loading = ref(false);
const saving = ref(false);

const fullSettings = ref<SystemSettings | null>(null);
const form = reactive({
  enabled: false,
  host: "127.0.0.1",
  port: 6379,
  pass: "",
  db: 0
});

async function loadSettings() {
  loading.value = true;
  try {
    const data = await getSettings();
    fullSettings.value = data;
    Object.assign(form, data.redis);
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

async function onSave() {
  if (!fullSettings.value) return;
  saving.value = true;
  try {
    fullSettings.value.redis = { ...form };
    await saveSettings(fullSettings.value);
    message.success("Redis 配置保存成功");
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    saving.value = false;
  }
}

onMounted(loadSettings);
</script>

<template>
  <section>
    <header class="title-row">
      <div>
        <h2 class="section-title">Redis 配置</h2>
        <p class="section-subtitle">配置缓存服务器以提高解析链接的响应速度（缓存 12 小时）。</p>
      </div>
      <n-space>
        <n-button secondary :loading="loading" @click="loadSettings">重新加载</n-button>
        <n-button type="primary" :loading="saving" @click="onSave">保存设置</n-button>
      </n-space>
    </header>

    <n-spin :show="loading">
      <n-card class="setting-card">
        <n-form label-placement="top" :show-feedback="false">
          <n-grid :cols="24" :x-gap="14" :y-gap="10">
            <n-form-item-gi :span="24" label="启用 Redis">
              <n-switch v-model:value="form.enabled" />
            </n-form-item-gi>
            <n-form-item-gi :span="14" label="主机地址">
              <n-input v-model:value="form.host" placeholder="127.0.0.1" :disabled="!form.enabled" size="large" />
            </n-form-item-gi>
            <n-form-item-gi :span="10" label="端口">
              <n-input-number v-model:value="form.port" :min="1" :max="65535" :disabled="!form.enabled" size="large" style="width:100%" />
            </n-form-item-gi>
            <n-form-item-gi :span="24" label="密码">
              <n-input v-model:value="form.pass" type="password" show-password-on="click" :disabled="!form.enabled" placeholder="留空表示不使用密码" size="large" />
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="数据库编号 (DB)">
              <n-input-number v-model:value="form.db" :min="0" :max="15" :disabled="!form.enabled" size="large" style="width:100%" />
            </n-form-item-gi>
          </n-grid>
        </n-form>
      </n-card>
    </n-spin>
  </section>
</template>

<style scoped>
.title-row {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  align-items: center;
  margin-bottom: 14px;
}

.setting-card {
  border-radius: 16px;
  background: rgba(255, 255, 255, 0.9);
  border: 1px solid rgba(20, 41, 78, 0.08);
}

@media (max-width: 760px) {
  .title-row {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>


