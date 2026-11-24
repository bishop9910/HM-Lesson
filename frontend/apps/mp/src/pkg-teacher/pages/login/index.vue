<script setup lang="ts">
import { ref } from "vue";
import { ApiError, request } from "@/utils/request";
import { useSessionStore } from "@/stores/session";

interface LoginResponse {
  token: string;
  expires_at: string;
  staff: { id: string; name: string; phone: string; role: "owner" | "assistant"; status: string };
}

const session = useSessionStore();
const phone = ref("");
const password = ref("");
const loading = ref(false);
const error = ref("");

async function submit(): Promise<void> {
  if (loading.value) return;
  error.value = "";
  if (phone.value.trim().length < 6 || password.value.length < 8) {
    error.value = "请输入手机号和至少 8 位密码";
    return;
  }
  loading.value = true;
  try {
    const res = await request<LoginResponse>("/api/v1/auth/login", "POST", {
      phone: phone.value.trim(),
      password: password.value,
    });
    session.setSession(res.token, res.staff.role, res.staff.id, res.staff.name);
    uni.reLaunch({ url: "/pkg-teacher/pages/workbench/index" });
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : "登录失败，请稍后重试";
  } finally {
    loading.value = false;
  }
}

function back(): void {
  uni.reLaunch({ url: "/pages/index/index" });
}
</script>

<template>
  <view class="page">
    <view class="hero">
      <text class="eyebrow">教师入口</text>
      <text class="title">登录后管理作业、预约与课时</text>
      <text class="subtitle">负责人可添加其他老师；助教不能管理成员与收费。</text>
    </view>

    <view class="form">
      <text class="label">手机号</text>
      <input v-model="phone" class="input" type="number" maxlength="11" placeholder="请输入手机号" />
      <text class="label">密码</text>
      <input v-model="password" class="input" password maxlength="64" placeholder="至少 8 位" />
      <text v-if="error" class="error">{{ error }}</text>
      <button class="primary" :loading="loading" :disabled="loading" @click="submit">
        {{ loading ? "登录中…" : "登录" }}
      </button>
      <button class="ghost" @click="back">返回家长端</button>
    </view>

    <text class="footnote">家长端与教师端是同一个工具，账号由负责人创建，不开放自助注册。</text>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 48rpx $hm-space 64rpx;
  box-sizing: border-box;
  background: $hm-bg;
}

.hero {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
  padding: 40rpx 32rpx;
  border-radius: 32rpx;
  background: linear-gradient(135deg, #0f766e, #134e4a);
  color: #f0fdfa;
}

.eyebrow {
  font-size: 24rpx;
  letter-spacing: 4rpx;
  opacity: 0.8;
}

.title {
  font-size: 40rpx;
  font-weight: 700;
  line-height: 1.35;
}

.subtitle {
  font-size: 24rpx;
  opacity: 0.85;
  line-height: 1.5;
}

.form {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
  margin-top: 32rpx;
  padding: 32rpx 28rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  border-radius: $hm-radius;
}

.label {
  margin-top: 12rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.input {
  height: 84rpx;
  padding: 0 20rpx;
  background: $hm-bg;
  border: 1rpx solid $hm-border;
  border-radius: 16rpx;
  font-size: 30rpx;
}

.error {
  margin-top: 8rpx;
  font-size: 24rpx;
  color: #b91c1c;
}

.primary,
.ghost {
  height: 88rpx;
  line-height: 88rpx;
  border-radius: 44rpx;
  font-size: 30rpx;
}

.primary {
  margin-top: 20rpx;
  background: $hm-primary;
  color: #ffffff;
}

.ghost {
  margin-top: 4rpx;
  background: $hm-surface;
  color: $hm-primary-strong;
  border: 1rpx solid $hm-primary;
}

.footnote {
  display: block;
  margin-top: 28rpx;
  font-size: 22rpx;
  color: $hm-muted;
  line-height: 1.6;
}
</style>
