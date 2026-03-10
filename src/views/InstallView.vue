<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { createDiscreteApi } from "naive-ui";
import { completeInstall, getHealthStatus, testDatabase, type InstallDBConfig } from "@/api/modules/install";
import { useAuthStore } from "@/stores/auth";

const router = useRouter();
const authStore = useAuthStore();
const { message } = createDiscreteApi(["message"]);
const fixedSQLitePath = "app.db";
const englishUsernamePattern = /^[A-Za-z]{4,}$/;

const testing = ref(false);
const installing = ref(false);
const waitingRestart = ref(false);
const dbChecked = ref(false);
const pageStep = ref<1 | 2>(1);

const dbForm = reactive<InstallDBConfig>({
  driver: "sqlite",
  sqlite_path: fixedSQLitePath,
  mysql_host: "127.0.0.1",
  mysql_port: "3306",
  mysql_user: "root",
  mysql_pass: "",
  mysql_db: "music_parser",
  mysql_param: "charset=utf8mb4&parseTime=True&loc=Local"
});

const adminForm = reactive({
  admin_username: "",
  admin_email: "",
  admin_password: "",
  site_name: "云音解析"
});

const dbDriverOptions = [
  { label: "SQLite（内置数据库 · 默认）", value: "sqlite" },
  { label: "MySQL 8.0+", value: "mysql" }
];

const showMySQL = computed(() => dbForm.driver === "mysql");
const currentStep = computed(() => pageStep.value);
const adminReady = computed(
  () =>
    isValidUsername(adminForm.admin_username) &&
    isValidEmail(adminForm.admin_email) &&
    adminForm.admin_password.length >= 8
);
const busy = computed(() => testing.value || installing.value || waitingRestart.value);
const installButtonDisabled = computed(() => installing.value || waitingRestart.value);

watch(
  () => [dbForm.driver, dbForm.mysql_host, dbForm.mysql_port, dbForm.mysql_user, dbForm.mysql_pass, dbForm.mysql_db, dbForm.mysql_param],
  () => {
    dbChecked.value = false;
    if (pageStep.value === 2) {
      pageStep.value = 1;
    }
  }
);

function validateDatabaseForm() {
  if (dbForm.driver === "sqlite") {
    return true;
  }
  if (!dbForm.mysql_host.trim() || !dbForm.mysql_port.trim() || !dbForm.mysql_user.trim() || !dbForm.mysql_db.trim()) {
    message.warning("请完整填写 MySQL 连接信息");
    return false;
  }
  return true;
}

function isValidEmail(raw: string) {
  const value = raw.trim();
  return value.length > 3 && value.includes("@");
}

function isValidUsername(raw: string) {
  const value = raw.trim();
  if (!value) return false;
  if (englishUsernamePattern.test(value)) return true;
  const chars = [...value];
  return chars.length >= 2 && chars.every((ch) => /^[\u4E00-\u9FFF]$/.test(ch));
}

async function onTestDB() {
  if (!validateDatabaseForm()) return;

  testing.value = true;
  try {
    await testDatabase(dbForm);
    dbChecked.value = true;
    message.success("数据库连接测试通过");
  } catch (error) {
    dbChecked.value = false;
    message.error((error as Error).message);
  } finally {
    testing.value = false;
  }
}

function onNextStep() {
  if (!dbChecked.value) {
    message.warning("请先完成数据库连接测试，测试通过后再进入下一步。");
    return;
  }
  pageStep.value = 2;
}

function onPrevStep() {
  if (installing.value || waitingRestart.value) return;
  pageStep.value = 1;
}

async function onCompleteInstall() {
  if (!dbChecked.value) {
    message.warning("请先测试数据库连接");
    return;
  }
  if (!adminReady.value) {
    message.warning("请完善管理员信息（用户名需>=4个英文字符或>=2个中文字符，邮箱有效，密码至少8位）");
    return;
  }

  installing.value = true;
  try {
    await completeInstall({
      database: dbForm,
      ...adminForm
    });
    authStore.logout();
    message.success("安装完成，正在等待服务自动重启，重启后请重新登录");
    await waitForRestart();
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    installing.value = false;
  }
}

async function waitForRestart() {
  waitingRestart.value = true;
  for (let i = 0; i < 80; i += 1) {
    await sleep(1500);
    try {
      const health = await getHealthStatus();
      if (health.installed) {
        authStore.logout();
        waitingRestart.value = false;
        router.replace("/login");
        return;
      }
    } catch {
      // restart window
    }
  }
  waitingRestart.value = false;
  message.warning("等待重启超时，请手动刷新页面");
}

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
</script>

<template>
  <main class="page-shell install-shell">
    <div class="orb orb-a" />
    <div class="orb orb-b" />

    <section class="install-wrap">
      <aside class="install-side glass-card">
        <p class="eyebrow">FIRST RUN SETUP</p>
        <h1>初始化安装向导</h1>

        <n-steps vertical size="small" :current="currentStep">
          <n-step title="步骤 1：数据库连接测试" description="支持 SQLite / MySQL 8.0+" />
          <n-step title="步骤 2：管理员配置" description="填写管理员用户名、邮箱和密码" />
        </n-steps>

        <div class="state-box">
          <p class="state-copy">请先完成数据库配置与连接测试，通过后继续填写管理员信息。</p>
        </div>
      </aside>

      <section class="install-main glass-card">
        <header class="head">
          <h2>安装配置</h2>
          <p v-if="pageStep === 1">请选择数据库并完成连接测试。</p>
          <p v-else>请填写管理员账号信息并完成安装。</p>
        </header>

        <div class="form-zone">
          <transition name="step-fade" mode="out-in">
            <!-- ========== 步骤 1：数据库配置 ========== -->
            <n-card v-if="pageStep === 1" key="db-step" title="数据库配置" size="small" class="form-card">
              <n-form label-placement="top" :show-feedback="false">
                <n-grid :x-gap="18" :y-gap="14" :cols="24">
                  <!-- 数据库类型下拉选择 -->
                  <n-form-item-gi :span="24" label="数据库类型">
                    <n-select
                      v-model:value="dbForm.driver"
                      :options="dbDriverOptions"
                      placeholder="请选择数据库类型"
                      size="large"
                    />
                  </n-form-item-gi>

                  <!-- MySQL 配置区域（展开动画）-->
                  <n-form-item-gi v-if="showMySQL" :span="24" :show-label="false">
                    <n-collapse-transition :show="showMySQL">
                      <div class="mysql-config-zone">
                        <p class="mysql-config-title">MySQL 连接配置</p>
                        <n-grid :x-gap="18" :y-gap="14" :cols="24">
                          <n-form-item-gi :span="12" label="连接地址">
                            <n-input v-model:value="dbForm.mysql_host" placeholder="127.0.0.1" size="large" />
                          </n-form-item-gi>
                          <n-form-item-gi :span="12" label="端口">
                            <n-input v-model:value="dbForm.mysql_port" placeholder="3306" size="large" />
                          </n-form-item-gi>
                          <n-form-item-gi :span="12" label="数据库名">
                            <n-input v-model:value="dbForm.mysql_db" placeholder="music_parser" size="large" />
                          </n-form-item-gi>
                          <n-form-item-gi :span="12" label="用户名">
                            <n-input v-model:value="dbForm.mysql_user" placeholder="root" size="large" />
                          </n-form-item-gi>
                          <n-form-item-gi :span="24" label="密码">
                            <n-input v-model:value="dbForm.mysql_pass" type="password" show-password-on="click" placeholder="请输入数据库密码" size="large" />
                          </n-form-item-gi>
                        </n-grid>
                      </div>
                    </n-collapse-transition>
                  </n-form-item-gi>
                </n-grid>
              </n-form>
            </n-card>

            <!-- ========== 步骤 2：管理员配置 ========== -->
            <n-card v-else key="admin-step" title="管理员配置" size="small" class="form-card">
              <n-form label-placement="top" :show-feedback="false">
                <n-grid :x-gap="18" :y-gap="14" :cols="24">
                  <n-form-item-gi :span="24" label="管理员用户名">
                    <n-input v-model:value="adminForm.admin_username" placeholder="至少4个英文字符，或至少2个中文字符" size="large" />
                  </n-form-item-gi>
                  <n-form-item-gi :span="24" label="管理员邮箱">
                    <n-input v-model:value="adminForm.admin_email" placeholder="admin@example.com" size="large" />
                  </n-form-item-gi>
                  <n-form-item-gi :span="24" label="管理员密码">
                    <n-input v-model:value="adminForm.admin_password" type="password" show-password-on="click" placeholder="请输入密码（至少 8 位）" size="large" />
                  </n-form-item-gi>
                </n-grid>
              </n-form>
            </n-card>
          </transition>
        </div>

        <footer class="action-bar">
          <template v-if="pageStep === 1">
            <n-button class="action-btn" size="large" :loading="testing" :disabled="busy" @click="onTestDB">测试数据库连接</n-button>
            <n-button class="action-btn" size="large" :disabled="busy" @click="onNextStep">下一步</n-button>
          </template>
          <template v-else>
            <n-button class="action-btn" size="large" :disabled="installButtonDisabled" @click="onPrevStep">上一步</n-button>
            <n-button class="action-btn" size="large" :loading="installing || waitingRestart" :disabled="installButtonDisabled" @click="onCompleteInstall">完成安装</n-button>
          </template>
        </footer>
      </section>
    </section>
  </main>
</template>

<style scoped>
/* ── 页面外壳 ── */
.install-shell {
  position: relative;
  display: grid;
  place-items: center;
  overflow: hidden;
}

/* ── 装饰光球 ── */
.orb {
  position: absolute;
  border-radius: 999px;
  filter: blur(8px);
  pointer-events: none;
}

.orb-a {
  width: 34rem;
  height: 34rem;
  top: -12rem;
  left: -10rem;
  background: radial-gradient(circle at center, rgba(44, 125, 255, 0.24), transparent 62%);
}

.orb-b {
  width: 28rem;
  height: 28rem;
  right: -8rem;
  bottom: -10rem;
  background: radial-gradient(circle at center, rgba(255, 146, 56, 0.2), transparent 62%);
}

/* ── 整体容器（适度放大）── */
.install-wrap {
  width: min(1440px, 96%);
  display: grid;
  grid-template-columns: 400px minmax(0, 1fr);
  gap: 24px;
  position: relative;
  z-index: 1;
}

/* ── 左侧面板 ── */
.install-side {
  padding: 36px 28px;
  background:
    linear-gradient(165deg, rgba(11, 83, 206, 0.92), rgba(13, 121, 198, 0.88)),
    radial-gradient(circle at 10% 12%, rgba(255, 255, 255, 0.2), transparent 54%);
  color: #f8fbff;
  border-color: rgba(255, 255, 255, 0.18);
}

.eyebrow {
  margin: 0;
  letter-spacing: 0.2em;
  font-size: 12px;
  opacity: 0.86;
}

.install-side h1 {
  margin: 10px 0 14px;
  font-size: 30px;
  line-height: 1.05;
}

.state-box {
  margin-top: 20px;
}

.state-copy {
  margin: 0;
  line-height: 1.6;
  color: rgba(250, 253, 255, 0.95);
  font-size: 14px;
}

/* ── 右侧主表单 ── */
.install-main {
  padding: 34px 36px;
  box-shadow: 0 28px 70px rgba(17, 38, 82, 0.12);
}

.head h2 {
  margin: 0;
  font-size: 26px;
}

.head p {
  margin: 6px 0 0;
  color: var(--text-2);
}

/* ── 表单区域 ── */
.form-zone {
  margin-top: 20px;
  display: grid;
  gap: 12px;
}

.form-card {
  border-radius: 16px;
  border: 1px solid rgba(20, 41, 78, 0.08);
  background: rgba(255, 255, 255, 0.92);
}

/* ── MySQL 配置弹出区域 ── */
.mysql-config-zone {
  width: 100%;
  padding: 18px 20px 12px;
  border-radius: 14px;
  background: rgba(220, 233, 255, 0.35);
  border: 1px solid rgba(15, 111, 255, 0.15);
}

.mysql-config-title {
  margin: 0 0 14px;
  font-size: 15px;
  font-weight: 700;
  color: var(--brand-deep);
  letter-spacing: 0.02em;
}

/* ── 操作按钮区域（居中） ── */
.action-bar {
  margin-top: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 18px;
}

.action-btn {
  width: 180px;
  height: 46px;
  border-radius: 12px;
  border: 1px solid rgba(15, 111, 255, 0.36);
  background: rgba(220, 233, 255, 0.7);
  color: var(--brand-deep);
  font-weight: 700;
  text-align: center;
}

:deep(.action-btn .n-button__content) {
  width: 100%;
  justify-content: center;
}

/* ── 步骤切换过渡动画 ── */
.step-fade-enter-active,
.step-fade-leave-active {
  transition: all 0.24s ease;
}

.step-fade-enter-from {
  opacity: 0;
  transform: translateX(18px);
}

.step-fade-leave-to {
  opacity: 0;
  transform: translateX(-12px);
}

/* ── 响应式 ── */
@media (max-width: 980px) {
  .install-wrap {
    grid-template-columns: 1fr;
  }

  .install-side h1 {
    font-size: 25px;
  }
}

@media (max-width: 640px) {
  .install-main {
    padding: 20px;
  }

  .action-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .action-btn {
    width: 100%;
  }
}
</style>



