<script setup lang="ts">
import { computed, ref } from "vue";
import { ApiError, request } from "@/utils/request";

const phone = ref("");
const submitting = ref(false);
const result = ref("");
const canSubmit = computed(() => /^1\d{10}$/.test(phone.value));

async function submit(): Promise<void> {
  if (!canSubmit.value || submitting.value) return;
  submitting.value = true;
  result.value = "";
  try {
    const res = await request<{ status: string }>("/api/v1/auth/bind-request", "POST", {
      phone: phone.value,
    });
    const messages: Record<string, string> = {
      active: "该手机号已绑定，请让老师确认学员关系。",
      pending: "申请已提交，等待老师确认。",
      rejected: "申请未通过，请联系老师。",
      disabled: "该账号已停用，请联系老师。",
    };
    result.value = messages[res.status] ?? "申请已提交，等待老师确认。";
  } catch (err) {
    result.value = err instanceof ApiError ? err.message : "提交失败，请稍后重试。";
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <view class="page">
    <text class="title">绑定学员</text>
    <text class="hint">输入老师在学员档案里登记的手机号，提交后由老师确认。</text>

    <view class="field">
      <text class="label">家长手机号</text>
      <input v-model="phone" class="input" type="number" maxlength="11" placeholder="11 位手机号" />
    </view>

    <button class="primary" :disabled="!canSubmit || submitting" @click="submit">
      {{ submitting ? "提交中…" : "提交申请" }}
    </button>
    <text v-if="result" class="result">{{ result }}</text>
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

.result {
  display: block;
  margin-top: 24rpx;
  font-size: 24rpx;
  color: $hm-muted;
  text-align: center;
}
</style>
