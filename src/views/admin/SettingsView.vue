<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { createDiscreteApi } from "naive-ui";
import { getSettings, saveSettings, type SystemSettings } from "@/api/modules/admin";
import { clearPublicSiteSettingsCache } from "@/api/modules/site";
import { useSettingsStore } from "@/stores/settings";

const { message } = createDiscreteApi(["message"]);
const settingsStore = useSettingsStore();
const loading = ref(false);
const saving = ref(false);

const fullSettings = ref<SystemSettings | null>(null);

const siteForm = reactive({
  name: "",
  keywords: "",
  description: "",
  icp_no: "",
  police_no: "",
});

const featureForm = reactive({
  allow_register: false,
  default_parse_quality: "standard" as "standard" | "exhigh" | "lossless" | "hires" | "jymaster",
});

const parseQualityOptions = [
  { label: "标准", value: "standard" },
  { label: "极高", value: "exhigh" },
  { label: "无损", value: "lossless" },
  { label: "Hi-Res", value: "hires" },
  { label: "超清母带", value: "jymaster" },
];

async function loadSettings() {
  loading.value = true;
  try {
    const data = await getSettings();
    fullSettings.value = data;
    Object.assign(siteForm, data.site);
    Object.assign(featureForm, data.feature);
    featureForm.default_parse_quality = data.feature?.default_parse_quality || "standard";
    settingsStore.syncSiteName(data.site?.name);
    settingsStore.applyDocumentTitle();
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
    fullSettings.value.site = { ...siteForm };
    fullSettings.value.feature = { ...featureForm };
    await saveSettings(fullSettings.value);
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

onMounted(loadSettings);
</script>

<template>
  <section>
    <header class="title-row">
      <div>
        <h2 class="section-title">站点配置</h2>
        <p class="section-subtitle">基本站点信息与功能设置。</p>
      </div>
      <n-space>
        <n-button secondary :loading="loading" @click="loadSettings">重新加载</n-button>
        <n-button type="primary" :loading="saving" @click="onSave">保存设置</n-button>
      </n-space>
    </header>

    <n-spin :show="loading">
      <n-space vertical :size="16">
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
            <n-form-item label="默认解析音质">
              <n-select
                v-model:value="featureForm.default_parse_quality"
                :options="parseQualityOptions"
                placeholder="请选择默认解析音质"
                size="large"
              />
            </n-form-item>
          </n-form>
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


