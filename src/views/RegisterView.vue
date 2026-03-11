<script setup lang="ts">
import { reactive, ref } from "vue";
import { useRouter } from "vue-router";
import { createDiscreteApi } from "naive-ui";
import { register } from "@/api/modules/auth";

const router = useRouter();
const { message } = createDiscreteApi(["message"]);
const englishUsernamePattern = /^[A-Za-z]{4,}$/;

const loading = ref(false);
const form = reactive({
  username: "",
  email: "",
  password: "",
});

function isValidUsername(raw: string) {
  const value = raw.trim();
  if (!value) return false;
  if (englishUsernamePattern.test(value)) return true;
  const chars = [...value];
  return chars.length >= 2 && chars.every((ch) => /^[\u4E00-\u9FFF]$/.test(ch));
}

function isValidEmail(raw: string) {
  const value = raw.trim();
  return value.length > 3 && value.includes("@");
}

async function onRegister() {
  const username = form.username.trim();
  const email = form.email.trim();
  if (!isValidUsername(username)) {
    message.warning("用户名需至少4个英文字符，或至少2个中文字符");
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

  loading.value = true;
  try {
    await register({
      username,
      email,
      password: form.password,
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
</script>

<template>
  <main class="page-shell register-shell">
    <div class="orb orb-a" />
    <div class="orb orb-b" />

    <section class="register-panel glass-card" v-motion-slide-visible-once-bottom>
      <div class="left-banner">
        <p class="badge">MUSIC PARSER</p>
        <h1>创建新账号</h1>
        <p class="desc">注册后即可使用系统功能。若注册被禁用，请联系管理员在后台开启。</p>
      </div>
      <div class="right-form">
        <h2>用户注册</h2>
        <n-form :show-feedback="false" label-placement="top">
          <n-form-item label="用户名">
            <n-input v-model:value="form.username" placeholder="至少4个英文字符，或至少2个中文字符" size="large" />
          </n-form-item>
          <n-form-item label="邮箱">
            <n-input v-model:value="form.email" placeholder="请输入邮箱地址" size="large" />
          </n-form-item>
          <n-form-item label="密码">
            <n-input v-model:value="form.password" type="password" show-password-on="click" placeholder="请输入密码（至少 8 位）" size="large" />
          </n-form-item>
          <div class="action-row">
            <n-button type="primary" size="large" block :loading="loading" @click="onRegister">立即注册</n-button>
            <n-button tertiary size="large" block @click="goLogin">已有账号，去登录</n-button>
          </div>
          <p class="hint">用户名与密码规则与管理员账号保持一致。</p>
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

.action-row {
  display: grid;
  gap: 10px;
}

.hint {
  margin: 12px 0 0;
  font-size: 12px;
  color: var(--text-2);
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
}
</style>
