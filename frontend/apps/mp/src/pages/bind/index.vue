<script setup lang="ts">
import { computed, ref } from "vue";

const phone = ref("");
const code = ref("");
const submitting = ref(false);
const canSubmit = computed(() => /^1\d{10}$/.test(phone.value));

async function submit(): Promise<void> {
  if (!canSubmit.value || submitting.value) return;
  submitting.value = true;
  try {
    // M2 接入：POST /api/v1/auth/bind，body { phone, binding_code }
    uni.showToast({ title: "绑定接口待接入", icon: "none" });
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <view class="page">
    <text class="title">绑定学员</text>
    <text class="hint">输入老师在学员档案里登记的手机号。老师审批后即可查看孩子的作业与课时。</text>

    <view class="field">
      <text class="label">家长手机号</text>
      <input v-model="phone" class="input" type="number" maxlength="11" placeholder="11 位手机号" />
    </view>

    <view class="field">
      <text class="label">绑定码（可选）</text>
      <input v-model="code" class="input" type="number" maxlength="6" placeholder="老师提供的 6 位码" />
    </view>

    <button class="primary" :disabled="!canSubmit || submitting" @click="submit">
      {{ submitting ? "提交中…" : "提交绑定申请" }}
    </button>
    <text class="footnote">不会向你索要验证码；绑定全程由老师审批。</text>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 48rpx $hm-space;
  box-sizing: border-box;
  background: $hm-bg;
}

.title {
  display: block;
  font-size: 44rpx;
  font-weight: 700;
  color: $hm-text;
}

.hint {
  display: block;
  margin-top: 16rpx;
  font-size: 26rpx;
  line-height: 1.7;
  color: $hm-muted;
}

.field {
  margin-top: 40rpx;
  display: flex;
  flex-direction: column;
  gap: 12rpx;
}

.label {
  font-size: 26rpx;
  color: $hm-text;
}

.input {
  height: 88rpx;
  padding: 0 24rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  border-radius: 16rpx;
  font-size: 30rpx;
}

.primary {
  margin-top: 48rpx;
  height: 92rpx;
  line-height: 92rpx;
  border-radius: 46rpx;
  background: $hm-primary;
  color: #ffffff;
  font-size: 30rpx;
}

.primary[disabled] {
  opacity: 0.45;
}

.footnote {
  display: block;
  margin-top: 24rpx;
  font-size: 22rpx;
  color: $hm-muted;
  text-align: center;
}
</style>
