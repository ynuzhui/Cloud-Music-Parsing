<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { createDiscreteApi } from "naive-ui";
import { register, sendRegisterEmailCode } from "@/api/modules/auth";
import { getPublicSiteSettings, type PublicSiteSettings } from "@/api/modules/site";
import { resolveCaptchaPayload } from "@/utils/captcha";

const router = useRouter();
const { message } = createDiscreteApi(["message"]);
const usernamePattern = /^[A-Za-z\u4E00-\u9FFF][A-Za-z0-9_\-\u4E00-\u9FFF]{1,31}$/;

const loading = ref(false);
const sendingEmailCode = ref(false);
const registerEmailVerify = ref(false);
const emailCodeCountdown = ref(0);
const captchaConfig = ref<PublicSiteSettings["captcha"]>();
let countdownTimer: number | null = null;

const form = reactive({
  username: "",
  email: "",
  password: "",
  confirm_password: "",
  email_code: "",
});

const sendCodeButtonText = computed(() => (emailCodeCountdown.value > 0 ? `${emailCodeCountdown.value}s 后重试` : "发送验证码"));

async function refreshCaptchaConfig(force = false) {
  try {
    const site = await getPublicSiteSettings(force);
    captchaConfig.value = site.captcha;
    registerEmailVerify.value = !!site.register_email_verify;
  } catch {
    captchaConfig.value = undefined;
    registerEmailVerify.value = false;
  }
}

function isValidUsername(raw: string) {
  return usernamePattern.test(raw.trim());
}

function isValidEmail(raw: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(raw.trim());
}

function startEmailCodeCountdown() {
  emailCodeCountdown.value = 60;
  if (countdownTimer) {
    window.clearInterval(countdownTimer);
  }
  countdownTimer = window.setInterval(() => {
    if (emailCodeCountdown.value <= 1) {
      emailCodeCountdown.value = 0;
      if (countdownTimer) {
        window.clearInterval(countdownTimer);
        countdownTimer = null;
      }
      return;
    }
    emailCodeCountdown.value -= 1;
  }, 1000);
}

async function onSendEmailCode() {
  if (!registerEmailVerify.value || sendingEmailCode.value || emailCodeCountdown.value > 0) return;
  const email = form.email.trim();
  if (!isValidEmail(email)) {
    message.warning("请先输入有效邮箱地址");
    return;
  }
  sendingEmailCode.value = true;
  try {
    await sendRegisterEmailCode(email);
    message.success("验证码已发送，请查收邮箱");
    startEmailCodeCountdown();
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    sendingEmailCode.value = false;
  }
}

async function onRegister() {
  const username = form.username.trim();
  const email = form.email.trim();
  if (!isValidUsername(username)) {
    message.warning("用户名需以中文或英文开头，长度 2-32，可包含数字、下划线和短横线");
    return;
  }
  if (!isValidEmail(email)) {
    message.warning("请输入有效邮箱地址");
    return;
  }
  if (form.password.length < 8) {
    message.warning("密码至少 8 位");
    return;
  }
  if (!form.confirm_password) {
    message.warning("请再次输入密码");
    return;
  }
  if (form.password !== form.confirm_password) {
    message.warning("两次输入的密码不一致");
    return;
  }
  if (registerEmailVerify.value && !form.email_code.trim()) {
    message.warning("请输入邮箱验证码");
    return;
  }

  await refreshCaptchaConfig(true);

  let captchaPayload;
  try {
    captchaPayload = await resolveCaptchaPayload(captchaConfig.value, "register");
  } catch (error) {
    message.warning((error as Error).message);
    return;
  }

  loading.value = true;
  try {
    await register({
      username,
      email,
      password: form.password,
      confirm_password: form.confirm_password,
      email_code: registerEmailVerify.value ? form.email_code.trim() : undefined,
      captcha: captchaPayload,
    });
    message.success("注册成功，请登录");
    router.replace("/login");
  } catch (error) {
    message.error((error as Error).message);
  } finally {
    loading.value = false;
  }
}

function goLogin() {
  router.push("/login");
}

onMounted(async () => {
  await refreshCaptchaConfig(true);
});

onBeforeUnmount(() => {
  if (countdownTimer) {
    window.clearInterval(countdownTimer);
    countdownTimer = null;
  }
});
</script>

<template>
  <main class="page-shell register-shell">
    <div class="orb orb-a" />
    <div class="orb orb-b" />

    <section class="register-panel glass-card" v-motion-slide-visible-once-bottom>
      <div class="left-banner">
        <p class="badge">MUSIC PARSER</p>
        <h1>创建新账号</h1>
        <p class="desc">注册后即可使用解析与下载等功能，并拥有个人额度与记录。若当前关闭注册，请联系管理员在站点配置中开启。</p>
      </div>
      <div class="right-form">
        <h2>用户注册</h2>
        <n-form :show-feedback="false" label-placement="top">
          <n-form-item label="用户名">
            <n-input v-model:value="form.username" placeholder="请输入用户名（中文或英文开头）" size="large" />
          </n-form-item>
          <n-form-item label="邮箱">
            <div class="email-row">
              <n-input v-model:value="form.email" placeholder="请输入邮箱地址" size="large" />
              <n-button
                v-if="registerEmailVerify"
                class="email-send-btn"
                size="large"
                :loading="sendingEmailCode"
                :disabled="emailCodeCountdown > 0"
                @click="onSendEmailCode"
              >
                {{ sendCodeButtonText }}
              </n-button>
            </div>
          </n-form-item>
          <n-form-item v-if="registerEmailVerify" label="邮箱验证码">
            <n-input v-model:value="form.email_code" placeholder="请输入邮箱验证码" size="large" />
          </n-form-item>
          <n-form-item label="密码">
            <n-input v-model:value="form.password" type="password" show-password-on="click" placeholder="请输入密码（至少 8 位）" size="large" />
          </n-form-item>
          <n-form-item label="确认密码">
            <n-input v-model:value="form.confirm_password" type="password" show-password-on="click" placeholder="请再次输入密码" size="large" />
          </n-form-item>
          <div class="action-row">
            <n-button type="primary" size="large" block :loading="loading" @click="onRegister">立即注册</n-button>
            <n-button tertiary size="large" block @click="goLogin">已有账号，去登录</n-button>
          </div>
        </n-form>
      </div>
    </section>
  </main>
</template>

<style scoped>
.register-shell {
  position: relative;
  display: grid;
  place-items: center;
  overflow: hidden;
}

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

.register-panel {
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

.email-row {
  width: 100%;
  display: flex;
  gap: 10px;
}

.email-row :deep(.n-input) {
  flex: 1;
}

.email-send-btn {
  min-width: 116px;
}

.action-row {
  display: grid;
  gap: 10px;
  margin-top: 14px;
}

@media (max-width: 840px) {
  .register-panel {
    grid-template-columns: 1fr;
  }

  .left-banner {
    padding: 28px 22px;
  }

  .right-form {
    padding: 26px 22px;
  }

  .email-row {
    flex-direction: column;
  }

  .email-send-btn {
    width: 100%;
  }
}
</style>
