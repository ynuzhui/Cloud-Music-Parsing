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
  host: "",
  port: 0,
  username: "",
  password: "",
  protocol: "http"
});

const protocolOptions = [
  { label: "HTTP", value: "http" },
  { label: "HTTPS", value: "https" },
  { label: "SOCKS4", value: "socks4" },
  { label: "SOCKS5", value: "socks5" },
  { label: "SOCKS5H", value: "socks5h" }
];

async function loadSettings() {
  loading.value = true;
  try {
    const data = await getSettings();
    fullSettings.value = data;
    Object.assign(form, data.proxy);
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
    fullSettings.value.proxy = { ...form };
    await saveSettings(fullSettings.value);
    message.success("代理配置保存成功");
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
        <h2 class="section-title">代理配置</h2>
        <p class="section-subtitle">配置网络代理以加速第三方平台请求。</p>
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
            <n-form-item-gi :span="24" label="启用代理">
              <n-switch v-model:value="form.enabled" />
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="代理协议">
              <n-select v-model:value="form.protocol" :options="protocolOptions" :disabled="!form.enabled" size="large" />
            </n-form-item-gi>
            <n-form-item-gi :span="10" label="代理地址">
              <n-input v-model:value="form.host" placeholder="127.0.0.1" :disabled="!form.enabled" size="large" />
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="端口">
              <n-input-number v-model:value="form.port" :min="0" :max="65535" :disabled="!form.enabled" size="large" style="width:100%" />
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


