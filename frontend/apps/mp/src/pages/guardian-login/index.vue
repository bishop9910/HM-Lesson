<script setup lang="ts">
import { ref } from "vue";
import { ApiError, request } from "@/utils/request";

const phone = ref("");
const code = ref("");
const loading = ref(false);
const errorText = ref("");

async function login(): Promise<void> {
  if (!/^1\d{10}$/.test(phone.value.trim())) {
    uni.showToast({ title: "请输入 11 位手机号", icon: "none" });
    return;
  }
  loading.value = true;
  errorText.value = "";
  try {
    const res = await request<{ token: string; guardian: { phone: string } }>(
      "/api/v1/auth/guardian/login",
      "POST",
      { phone: phone.value.trim(), code: code.value.trim() },
      "none",
    );
    uni.setStorageSync("hm_guardian_token", res.token);
    uni.setStorageSync("hm_guardian_phone", res.guardian.phone);
    uni.reLaunch({ url: "/pages/portal/index" });
  } catch (err) {
    errorText.value = err instanceof ApiError ? err.message : "登录失败";
  } finally {
    loading.value = false;
  }
}

function goBind(): void {
  uni.navigateTo({ url: "/pages/bind/index" });
}
</script>

<template>
  <view class="page">
    <view class="hero">
      <text class="title">家长登录</text>
      <text class="subtitle">查看孩子的作业、课时与上课记录。</text>
    </view>

    <view class="form">
      <input v-model="phone" class="field" type="number" maxlength="11" placeholder="家长手机号" />
      <input v-model="code" class="field" type="number" maxlength="6" placeholder="绑定码（首次绑定必填）" />
      <text class="tip">已通过老师审批的家长，只填手机号即可登录；首次绑定请输入老师给的 6 位绑定码。</text>
      <text v-if="errorText" class="error">{{ errorText }}</text>
      <button class="primary" :loading="loading" @click="login">登录</button>
      <button class="ghost" @click="goBind">还没有绑定？提交申请</button>
    </view>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 48rpx $hm-space calc(48rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
  background: $hm-bg;
}

.hero {
  padding: 40rpx 32rpx;
  border-radius: 32rpx;
  background: linear-gradient(135deg, #0f766e, #134e4a);
  color: #f0fdfa;
}

.title {
  display: block;
  font-size: 40rpx;
  font-weight: 700;
}

.subtitle {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  opacity: 0.85;
}

.form {
  margin-top: 28rpx;
  padding: 28rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
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

.tip {
  display: block;
  font-size: 22rpx;
  line-height: 1.5;
  color: $hm-muted;
}

.error {
  display: block;
  margin-top: 12rpx;
  font-size: 24rpx;
  color: #b91c1c;
}

.primary,
.ghost {
  height: 88rpx;
  margin-top: 20rpx;
  line-height: 88rpx;
  border-radius: 44rpx;
  font-size: 28rpx;
}

.primary {
  background: $hm-primary;
  color: #fff;
}

.ghost {
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-muted;
}
</style>
