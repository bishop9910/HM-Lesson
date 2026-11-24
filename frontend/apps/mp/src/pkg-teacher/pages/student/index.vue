<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad, onShow } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";
import { useSessionStore } from "@/stores/session";

interface Student {
  id: string;
  name: string;
  grade: string;
  school: string;
  group_code: string;
  status: string;
  tags: string[];
  next_follow_up_at?: string;
  note: string;
}
interface FollowUp {
  id: string;
  method: string;
  content: string;
  concern: string;
  next_action: string;
  created_at: string;
}

const groupLabels: Record<string, string> = {
  class_active: "班课在读",
  one_on_one: "一对一在读",
  intent: "待转化意向",
  visitor: "咨询访客",
};
const statusOptions = [
  { value: "active", label: "在读" },
  { value: "graduated", label: "结课归档" },
  { value: "lost", label: "流失" },
];
const methods = [
  { value: "wechat", label: "微信" },
  { value: "phone", label: "电话" },
  { value: "offline", label: "面谈" },
  { value: "other", label: "其他" },
];

const session = useSessionStore();
const studentId = ref("");
const student = ref<Student | null>(null);
const follows = ref<FollowUp[]>([]);
const loading = ref(false);
const errorText = ref("");
const saving = ref(false);
const followOpen = ref(false);
const follow = ref({ method: "wechat", content: "", concern: "", next_action: "" });
const code = ref("");
const codeExpires = ref("");

const statusLabel = computed(() => {
  if (!student.value) return "";
  return statusOptions.find((item) => item.value === student.value?.status)?.label ?? student.value.status;
});

async function load(): Promise<void> {
  if (!studentId.value) return;
  loading.value = true;
  errorText.value = "";
  try {
    const res = await request<{ student: Student; follow_ups: FollowUp[] }>(
      `/api/v1/students/${studentId.value}`,
      "GET",
    );
    student.value = res.student;
    follows.value = res.follow_ups;
  } catch (err) {
    errorText.value = err instanceof ApiError ? err.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

onLoad((options) => {
  studentId.value = options?.id ?? "";
});

onShow(() => {
  void load();
});

async function changeStatus(value: string): Promise<void> {
  if (!student.value || student.value.status === value) return;
  saving.value = true;
  try {
    const res = await request<{ student: Student }>(`/api/v1/students/${student.value.id}`, "PATCH", { status: value });
    student.value = res.student;
    uni.showToast({ title: "已更新", icon: "success" });
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "更新失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

function openFollow(): void {
  followOpen.value = true;
  follow.value = { method: "wechat", content: "", concern: "", next_action: "" };
}

function onMethodChange(e: { detail: { value: number } }): void {
  follow.value.method = methods[e.detail.value]?.value ?? "wechat";
}

async function submitFollow(): Promise<void> {
  if (!student.value || !follow.value.content.trim()) {
    uni.showToast({ title: "请填写沟通内容", icon: "none" });
    return;
  }
  saving.value = true;
  try {
    await request(`/api/v1/students/${student.value.id}/follow-ups`, "POST", {
      method: follow.value.method,
      content: follow.value.content.trim(),
      concern: follow.value.concern.trim(),
      next_action: follow.value.next_action.trim(),
    });
    followOpen.value = false;
    await load();
    uni.showToast({ title: "已记录", icon: "success" });
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "记录失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

async function createCode(): Promise<void> {
  if (!student.value) return;
  try {
    const res = await request<{ code: string; expires_at: string }>(
      `/api/v1/students/${student.value.id}/binding-codes`,
      "POST",
    );
    code.value = res.code;
    codeExpires.value = res.expires_at.slice(0, 10);
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "生成失败", icon: "none" });
  }
}

function copyCode(): void {
  if (!code.value) return;
  uni.setClipboardData({ data: code.value });
}

function goBack(): void {
  uni.navigateBack();
}
</script>

<template>
  <view class="page">
    <view v-if="errorText" class="error">{{ errorText }}</view>
    <view v-else-if="loading && !student" class="hint">正在加载…</view>

    <template v-if="student">
      <view class="hero">
        <view class="hero-top">
          <text class="name">{{ student.name }}</text>
          <text class="badge">{{ groupLabels[student.group_code] || student.group_code }}</text>
        </view>
        <text class="meta">{{ student.grade }} · {{ student.school || "未填学校" }}</text>
        <text class="meta">状态：{{ statusLabel }}</text>
        <text v-if="student.note" class="note">{{ student.note }}</text>
      </view>

      <view class="section">
        <text class="section-title">状态</text>
        <view class="status-row">
          <button
            v-for="item in statusOptions"
            :key="item.value"
            class="status-btn"
            :class="{ active: student.status === item.value }"
            :disabled="saving"
            @click="changeStatus(item.value)"
          >{{ item.label }}</button>
        </view>
      </view>

      <view class="section">
        <view class="section-head">
          <text class="section-title">跟进记录</text>
          <text v-if="session.isOwner" class="link" @click="openFollow">+ 新增</text>
        </view>
        <view v-if="!follows.length" class="hint">还没有跟进记录。</view>
        <view v-for="item in follows" :key="item.id" class="follow">
          <view class="follow-top">
            <text class="method">{{ methods.find((m) => m.value === item.method)?.label || item.method }}</text>
            <text class="time">{{ item.created_at.slice(0, 10) }}</text>
          </view>
          <text class="content">{{ item.content }}</text>
          <text v-if="item.next_action" class="next">下一步：{{ item.next_action }}</text>
        </view>
      </view>

      <view v-if="session.isOwner" class="section">
        <view class="section-head">
          <text class="section-title">家长绑定码</text>
          <text class="link" @click="createCode">生成</text>
        </view>
        <view v-if="code" class="code-box" @click="copyCode">
          <text class="code">{{ code }}</text>
          <text class="code-hint">有效期至 {{ codeExpires }}，点击复制。家长用手机号 + 此码登录。</text>
        </view>
        <view v-else class="hint">生成 6 位一次性绑定码，交给家长完成绑定。</view>
      </view>

      <button class="ghost" @click="goBack">返回</button>
    </template>

    <view v-if="followOpen" class="mask" @click.self="followOpen = false">
      <view class="sheet">
        <text class="sheet-title">新增跟进</text>
        <picker :range="methods" range-key="label" @change="onMethodChange">
          <view class="field-row">
            <text class="field-label">方式</text>
            <text class="picker">{{ methods.find((m) => m.value === follow.method)?.label }}</text>
          </view>
        </picker>
        <textarea v-model="follow.content" class="textarea" placeholder="沟通内容 / 学情反馈" />
        <input v-model="follow.concern" class="field" placeholder="顾虑（可选）" />
        <input v-model="follow.next_action" class="field" placeholder="下一步（可选）" />
        <button class="primary" :loading="saving" @click="submitFollow">保存</button>
        <button class="ghost" @click="followOpen = false">取消</button>
      </view>
    </view>
  </view>
</template>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 32rpx $hm-space calc(48rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
  background: $hm-bg;
}

.hero {
  padding: 28rpx;
  border-radius: 28rpx;
  background: linear-gradient(135deg, #0f766e, #134e4a);
  color: #f0fdfa;
}

.hero-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.name {
  font-size: 40rpx;
  font-weight: 700;
}

.badge {
  padding: 6rpx 16rpx;
  border-radius: 20rpx;
  font-size: 22rpx;
  background: rgba(240, 253, 250, 0.18);
}

.meta {
  display: block;
  margin-top: 10rpx;
  font-size: 24rpx;
  opacity: 0.85;
}

.note {
  display: block;
  margin-top: 14rpx;
  font-size: 24rpx;
  opacity: 0.8;
}

.section {
  margin-top: 24rpx;
  padding: 24rpx;
  border-radius: $hm-radius;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.section-title {
  font-size: 30rpx;
  font-weight: 600;
}

.link {
  font-size: 26rpx;
  color: $hm-primary;
}

.status-row {
  display: flex;
  gap: 12rpx;
  margin-top: 16rpx;
}

.status-btn {
  flex: 1;
  height: 72rpx;
  line-height: 72rpx;
  border-radius: 36rpx;
  font-size: 24rpx;
  background: $hm-bg;
  border: 1rpx solid $hm-border;
}

.status-btn.active {
  background: $hm-primary;
  border-color: $hm-primary;
  color: #fff;
}

.hint {
  margin-top: 12rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.error {
  font-size: 26rpx;
  color: #b91c1c;
}

.follow {
  margin-top: 16rpx;
  padding-top: 16rpx;
  border-top: 1rpx dashed $hm-border;
}

.follow-top {
  display: flex;
  justify-content: space-between;
}

.method {
  font-size: 24rpx;
  color: $hm-primary-strong;
}

.time {
  font-size: 22rpx;
  color: $hm-muted;
}

.content {
  display: block;
  margin-top: 8rpx;
  font-size: 28rpx;
  line-height: 1.5;
}

.next {
  display: block;
  margin-top: 6rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.code-box {
  margin-top: 16rpx;
  padding: 24rpx;
  border-radius: $hm-radius;
  background: #f0fdfa;
  border: 1rpx dashed $hm-primary;
}

.code {
  display: block;
  font-size: 56rpx;
  font-weight: 700;
  letter-spacing: 12rpx;
  color: $hm-primary-strong;
}

.code-hint {
  display: block;
  margin-top: 8rpx;
  font-size: 22rpx;
  color: $hm-muted;
}

.ghost {
  height: 88rpx;
  margin-top: 28rpx;
  line-height: 88rpx;
  border-radius: 44rpx;
  font-size: 28rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
}

.mask {
  position: fixed;
  inset: 0;
  z-index: 10;
  display: flex;
  align-items: flex-end;
  background: rgba(15, 23, 42, 0.45);
}

.sheet {
  width: 100%;
  padding: 32rpx $hm-space calc(32rpx + env(safe-area-inset-bottom));
  box-sizing: border-box;
  border-radius: 32rpx 32rpx 0 0;
  background: $hm-surface;
}

.sheet-title {
  display: block;
  margin-bottom: 16rpx;
  font-size: 32rpx;
  font-weight: 700;
}

.field,
.textarea,
.field-row {
  width: 100%;
  margin-top: 16rpx;
  padding: 20rpx 24rpx;
  box-sizing: border-box;
  border-radius: $hm-radius;
  border: 1rpx solid $hm-border;
  background: $hm-bg;
  font-size: 28rpx;
}

.textarea {
  height: 160rpx;
}

.field-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.field-label {
  color: $hm-muted;
}

.picker {
  color: $hm-primary-strong;
}

.primary {
  height: 88rpx;
  margin-top: 20rpx;
  line-height: 88rpx;
  border-radius: 44rpx;
  font-size: 28rpx;
  background: $hm-primary;
  color: #fff;
}
</style>
