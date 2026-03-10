<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { createDiscreteApi } from "naive-ui";
import { getSettings, saveSettings, testSmtp, type SystemSettings } from "@/api/modules/admin";

const { message } = createDiscreteApi(["message"]);
const loading = ref(false);
const saving = ref(false);
const testing = ref(false);
const testEmail = ref("");

const fullSettings = ref<SystemSettings | null>(null);
const form = reactive({
  host: "",
  port: 465,
  user: "",
  pass: "",
  ssl: true
});

async function loadSettings() {
  loading.value = true;
  try {
    const data = await getSettings();
    fullSettings.value = data;
    Object.assign(form, data.smtp);
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
    fullSettings.value.smtp = { ...form };
    await saveSettings(fullSettings.value);
    message.success("SMTP 配置保存成功");
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    saving.value = false;
  }
}

async function onTestSmtp() {
  if (!testEmail.value || !testEmail.value.includes("@")) {
    message.warning("请输入有效的收件人邮箱地址");
    return;
  }
  testing.value = true;
  try {
    await testSmtp(testEmail.value);
    message.success("测试邮件发送成功，请检查收件箱");
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    testing.value = false;
  }
}

onMounted(loadSettings);
</script>

<template>
  <section>
    <header class="title-row">
      <div>
        <h2 class="section-title">SMTP 配置</h2>
        <p class="section-subtitle">配置 SMTP 服务器以启用系统邮件功能。</p>
      </div>
      <n-space>
        <n-button secondary :loading="loading" @click="loadSettings">重新加载</n-button>
        <n-button type="primary" :loading="saving" @click="onSave">保存设置</n-button>
      </n-space>
    </header>

    <n-spin :show="loading">
      <n-space vertical :size="16">
        <n-card class="setting-card">
          <n-form label-placement="top" :show-feedback="false">
            <n-grid :cols="24" :x-gap="14" :y-gap="10">
              <n-form-item-gi :span="14" label="SMTP 主机">
                <n-input v-model:value="form.host" placeholder="smtp.example.com" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="5" label="端口">
                <n-input-number v-model:value="form.port" :min="1" :max="65535" size="large" style="width:100%" />
              </n-form-item-gi>
              <n-form-item-gi :span="5" label="SSL / TLS">
                <n-switch v-model:value="form.ssl" />
              </n-form-item-gi>
              <n-form-item-gi :span="12" label="用户名">
                <n-input v-model:value="form.user" placeholder="user@example.com" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="12" label="密码">
                <n-input v-model:value="form.pass" type="password" show-password-on="click" placeholder="SMTP 授权码 / 密码" size="large" />
              </n-form-item-gi>
            </n-grid>
          </n-form>
        </n-card>

        <n-card title="发送测试邮件" class="setting-card">
          <n-form label-placement="top" :show-feedback="false">
            <n-grid :cols="24" :x-gap="14" :y-gap="10">
              <n-form-item-gi :span="16" label="收件人地址">
                <n-input v-model:value="testEmail" placeholder="test@example.com" size="large" />
              </n-form-item-gi>
              <n-form-item-gi :span="8" label=" ">
                <n-button type="warning" block :loading="testing" @click="onTestSmtp" size="large">发送测试邮件</n-button>
              </n-form-item-gi>
            </n-grid>
          </n-form>
          <template #header-extra>
            <n-tag type="info" size="small">请先保存配置</n-tag>
          </template>
        </n-card>
      </n-space>
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


