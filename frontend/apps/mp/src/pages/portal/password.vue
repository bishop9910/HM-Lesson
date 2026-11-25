<script setup lang="ts">
import { ref } from "vue";
import { ApiError, request } from "@/utils/request";

const oldPassword = ref("");
const newPassword = ref("");
const confirmPassword = ref("");
const loading = ref(false);
const errorText = ref("");

function back(): void {
  uni.navigateBack();
}

async function submit(): Promise<void> {
  const next = newPassword.value.trim();
  if (next.length < 6 || next.length > 32) {
    uni.showToast({ title: "新密码需要 6~32 位", icon: "none" });
    return;
  }
  if (next !== confirmPassword.value.trim()) {
    uni.showToast({ title: "两次输入的新密码不一致", icon: "none" });
    return;
  }
  loading.value = true;
  errorText.value = "";
  try {
    await request<{ ok: boolean }>(
      "/api/v1/auth/guardian/password",
      "POST",
      { old_password: oldPassword.value.trim(), new_password: next },
      "guardian",
    );
    uni.showToast({ title: "密码已更新", icon: "success" });
    setTimeout(() => back(), 600);
  } catch (err) {
    errorText.value = err instanceof ApiError ? err.message : "修改失败";
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <view class="page">
    <view class="card">
      <text class="title">修改密码</text>
      <text class="hint">第一次设置密码时，旧密码留空。</text>
      <input
        v-model="oldPassword"
        class="field"
        password
        maxlength="32"
        placeholder="旧密码（首次设置可留空）"
      />
      <input v-model="newPassword" class="field" password maxlength="32" placeholder="新密码（6~32 位）" />
      <input
        v-model="confirmPassword"
        class="field"
        password
        maxlength="32"
        placeholder="再输一次新密码"
      />
      <text v-if="errorText" class="error">{{ errorText }}</text>
      <button class="primary" :loading="loading" @click="submit">保存</button>
    </view>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 32rpx $hm-space calc(32rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
  background: $hm-bg;
}

.card {
  padding: 32rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.title {
  display: block;
  font-size: 34rpx;
  font-weight: 700;
  color: $hm-text;
}

.hint {
  display: block;
  margin: 12rpx 0 20rpx;
  font-size: 22rpx;
  line-height: 1.5;
  color: $hm-muted;
}

.field {
  height: 88rpx;
  margin-bottom: 16rpx;
  padding: 0 24rpx;
  box-sizing: border-box;
  border-radius: $hm-radius;
  background: $hm-bg;
  border: 1rpx solid $hm-border;
  font-size: 28rpx;
}

.error {
  display: block;
  margin-top: 4rpx;
  font-size: 24rpx;
  color: #b91c1c;
}

.primary {
  height: 88rpx;
  margin-top: 24rpx;
  line-height: 88rpx;
  border-radius: 44rpx;
  background: $hm-primary;
  color: #fff;
  font-size: 28rpx;
}
</style>
