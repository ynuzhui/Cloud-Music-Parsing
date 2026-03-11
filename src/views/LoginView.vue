<script setup lang="ts">
import { onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { createDiscreteApi } from "naive-ui";
import { login } from "@/api/modules/auth";
import { getPublicSiteSettings, type PublicSiteSettings } from "@/api/modules/site";
import { useAuthStore } from "@/stores/auth";
import { resolveCaptchaPayload } from "@/utils/captcha";

const router = useRouter();
const authStore = useAuthStore();
const { message } = createDiscreteApi(["message"]);

const loading = ref(false);
const captchaConfig = ref<PublicSiteSettings["captcha"]>();
const form = reactive({
  email: "",
  password: "",
  remember: true
});

async function refreshCaptchaConfig(force = false) {
  try {
    const site = await getPublicSiteSettings(force);
    captchaConfig.value = site.captcha;
  } catch {
    captchaConfig.value = undefined;
  }
}

async function onLogin() {
  if (!form.email || !form.password) {
    message.warning("请输入邮箱和密码");
    return;
  }
  if (!form.email.includes("@")) {
    message.warning("请输入有效邮箱地址");
    return;
  }

  await refreshCaptchaConfig(true);

  let captchaPayload;
  try {
    captchaPayload = await resolveCaptchaPayload(captchaConfig.value, "login");
  } catch (error) {
    message.warning((error as Error).message);
    return;
  }

  loading.value = true;
  try {
    const result = await login({
      ...form,
      captcha: captchaPayload
    });
    authStore.setSession(result);
    message.success("登录成功");
    router.replace(authStore.isAdmin ? "/dashboard" : "/");
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

function goRegister() {
  router.push("/register");
}

onMounted(async () => {
  await refreshCaptchaConfig(true);
});
</script>

<template>
  <main class="page-shell login-shell">
    <div class="orb orb-a" />
    <div class="orb orb-b" />

    <section class="login-panel glass-card" v-motion-slide-visible-once-bottom>
      <div class="left-banner">
        <p class="badge">MUSIC PARSER</p>
        <h1>欢迎回来</h1>
        <p class="desc">登录后可进入数据中心，查看访问与解析趋势，管理用户、用户组、Cookie 与系统配置。</p>
      </div>
      <div class="right-form">
        <h2>用户登录</h2>
        <n-form :show-feedback="false" label-placement="top">
          <n-form-item label="邮箱">
            <n-input v-model:value="form.email" placeholder="请输入邮箱地址" size="large" />
          </n-form-item>
          <n-form-item label="密码">
            <n-input v-model:value="form.password" type="password" show-password-on="click" placeholder="请输入密码" size="large" />
          </n-form-item>
          <n-form-item>
            <n-checkbox v-model:checked="form.remember">7 天内保持登录</n-checkbox>
          </n-form-item>
          <n-button type="primary" size="large" block :loading="loading" @click="onLogin">用户登录</n-button>
          <n-button tertiary size="large" block class="register-link" @click="goRegister">没有账号，去注册</n-button>
        </n-form>
      </div>
    </section>
  </main>
</template>

<style scoped>
.login-shell {
  position: relative;
  display: grid;
  place-items: center;
  overflow: hidden;
}

/* ── 装饰光球（与安装页统一）── */
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

.login-panel {
  width: min(980px, 100%);
  display: grid;
  grid-template-columns: 1.1fr 1fr;
  min-height: 520px;
  overflow: hidden;
  box-shadow: 0 30px 80px rgba(21, 40, 86, 0.14);
  position: relative;
  z-index: 1;
}

.left-banner {
  background:
    linear-gradient(145deg, rgba(15, 85, 203, 0.94), rgba(24, 150, 210, 0.9)),
    radial-gradient(circle at 10% 10%, rgba(255, 255, 255, 0.2), transparent 52%);
  color: #f9fbff;
  padding: 42px 36px;
}

.left-banner h1 {
  margin: 14px 0 12px;
  font-size: clamp(28px, 3vw, 42px);
  line-height: 1.05;
}

.badge {
  margin: 0;
  font-size: 12px;
  letter-spacing: 0.22em;
  opacity: 0.9;
}

.desc {
  margin: 0;
  line-height: 1.7;
  max-width: 320px;
  color: rgba(255, 255, 255, 0.86);
}

.right-form {
  padding: 42px 36px;
  background: rgba(255, 255, 255, 0.92);
}

.right-form h2 {
  margin: 0 0 20px;
  font-size: 24px;
}

.register-link {
  margin-top: 10px;
}

@media (max-width: 840px) {
  .login-panel {
    grid-template-columns: 1fr;
  }

  .left-banner {
    padding: 28px 22px;
  }

  .right-form {
    padding: 26px 22px;
  }
}
</style>
