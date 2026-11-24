<script setup lang="ts">
import { ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";
import { uploadImage } from "@/utils/cloud";

interface Submission {
  id: string;
  version: number;
  text: string;
  file_ids: string[];
  submitted_at: string;
  is_late: boolean;
}
interface Assignment {
  id: string;
  task_id: string;
  task_title: string;
  task_type: string;
  task_description?: string;
  status: string;
  due_at?: string;
  submitted_at?: string;
  reviewed_at?: string;
  score?: number;
  comment: string;
  return_reason: string;
  submission?: Submission;
  history?: Submission[];
}

const statusLabels: Record<string, string> = {
  pending: "待提交",
  submitted: "已提交",
  reviewed: "已批阅",
  returned: "已打回",
};
const id = ref("");
const item = ref<Assignment | null>(null);
const loading = ref(true);
const error = ref("");
const text = ref("");
const fileIDs = ref<string[]>([]);
const uploading = ref(false);
const submitting = ref(false);

function fmt(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  return `${d.getMonth() + 1}月${d.getDate()}日 ${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
}

function overdue(): boolean {
  return item.value?.status === "pending" && Boolean(item.value.due_at) && new Date(item.value.due_at!).getTime() < Date.now();
}

function canSubmit(): boolean {
  return item.value?.status === "pending" || item.value?.status === "returned";
}

async function load(): Promise<void> {
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ assignment: Assignment }>(`/api/v1/portal/tasks/${id.value}`, "GET", undefined, "guardian");
    item.value = res.assignment;
    if (res.assignment.status === "returned") {
      text.value = res.assignment.submission?.text || "";
      fileIDs.value = [...(res.assignment.submission?.file_ids || [])];
    }
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      uni.reLaunch({ url: "/pages/guardian-login/index" });
      return;
    }
    error.value = e instanceof ApiError ? e.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

function chooseImages(): void {
  if (uploading.value) return;
  const left = 9 - fileIDs.value.length;
  if (left <= 0) {
    uni.showToast({ title: "最多 9 张", icon: "none" });
    return;
  }
  uni.chooseImage({
    count: left,
    sizeType: ["compressed"],
    sourceType: ["camera", "album"],
    success: async (res) => {
      uploading.value = true;
      try {
        for (const path of res.tempFilePaths) {
          const fileID = await uploadImage(path);
          fileIDs.value.push(fileID);
        }
      } catch (e) {
        uni.showToast({ title: e instanceof Error ? e.message : "上传失败", icon: "none" });
      } finally {
        uploading.value = false;
      }
    },
  });
}

function removeImage(i: number): void {
  fileIDs.value.splice(i, 1);
}

function preview(urls: string[], current: string): void {
  uni.previewImage({ urls, current });
}

async function submit(): Promise<void> {
  if (uploading.value) {
    uni.showToast({ title: "图片还在上传", icon: "none" });
    return;
  }
  if (!text.value.trim() && fileIDs.value.length === 0) {
    uni.showToast({ title: "请填写内容或上传图片", icon: "none" });
    return;
  }
  submitting.value = true;
  try {
    await request(`/api/v1/portal/tasks/${id.value}/submit`, "POST", { text: text.value.trim(), file_ids: fileIDs.value }, "guardian");
    uni.showToast({ title: "已提交", icon: "success" });
    await load();
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) {
      uni.reLaunch({ url: "/pages/guardian-login/index" });
      return;
    }
    uni.showToast({ title: e instanceof ApiError ? e.message : "提交失败", icon: "none" });
  } finally {
    submitting.value = false;
  }
}

onLoad((options) => {
  id.value = (options?.id as string) || "";
  if (!id.value) {
    error.value = "缺少作业 id";
    loading.value = false;
    return;
  }
  void load();
});
</script>

<template>
  <view class="page">
    <view v-if="loading" class="hint">加载中…</view>
    <view v-else-if="error" class="hint error" @tap="load">{{ error }}，点此重试</view>
    <template v-else-if="item">
      <view class="card">
        <view class="row">
          <text class="title">{{ item.task_title }}</text>
          <text class="badge" :class="item.status">{{ statusLabels[item.status] }}</text>
        </view>
        <view class="meta">
          <text>截止 {{ fmt(item.due_at) }}</text>
          <text v-if="overdue()" class="late">已逾期</text>
          <text v-if="item.score !== undefined">评分 {{ item.score }}</text>
        </view>
        <text v-if="item.task_description" class="desc">{{ item.task_description }}</text>
      </view>

      <view v-if="item.status === 'reviewed'" class="card result">
        <text class="result-title">老师批阅</text>
        <text v-if="item.comment" class="comment">{{ item.comment }}</text>
        <text v-if="item.score !== undefined" class="score">评分 {{ item.score }}</text>
      </view>
      <view v-if="item.return_reason" class="card result warn">
        <text class="result-title">已打回</text>
        <text class="comment">{{ item.return_reason }}</text>
        <text class="tip">修改后可在下方重新提交。</text>
      </view>

      <view v-if="canSubmit()" class="card">
        <text class="section">我的提交</text>
        <textarea v-model="text" class="area" placeholder="写点什么，或上传作业照片" maxlength="5000" />
        <view class="images">
          <view v-for="(f, i) in fileIDs" :key="f" class="thumb-wrap">
            <image :src="f" mode="aspectFill" class="thumb" @tap="preview(fileIDs, f)" />
            <text class="remove" @tap="removeImage(i)">×</text>
          </view>
          <view v-if="fileIDs.length < 9" class="add" @tap="chooseImages">{{ uploading ? "…" : "+" }}</view>
        </view>
        <button class="primary" :disabled="submitting || uploading" @tap="submit">{{ item.status === "returned" ? "重新提交" : "提交作业" }}</button>
      </view>

      <view v-if="item.history?.length" class="card">
        <text class="section">提交记录</text>
        <view v-for="s in item.history" :key="s.id" class="history">
          <view class="history-head">
            <text>第 {{ s.version }} 次</text>
            <text>{{ fmt(s.submitted_at) }}<text v-if="s.is_late" class="late"> · 逾期</text></text>
          </view>
          <text v-if="s.text" class="history-text">{{ s.text }}</text>
          <view v-if="s.file_ids.length" class="images">
            <image v-for="f in s.file_ids" :key="f" :src="f" mode="aspectFill" class="thumb" @tap="preview(s.file_ids, f)" />
          </view>
        </view>
      </view>
    </template>
  </view>
</template>

<style lang="scss">
@import "@/styles/tokens.scss";
.page { min-height: 100vh; background: $hm-bg; padding: 24rpx 24rpx 60rpx; }
.card { background: rgba(255, 255, 255, 0.38); border: 1rpx solid $hm-rule; border-radius: 10rpx; padding: 26rpx; margin-bottom: 20rpx; }
.row { display: flex; justify-content: space-between; gap: 14rpx; }
.title { font-size: 32rpx; font-weight: 700; flex: 1; }
.badge { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 999rpx; background: #f1f5f9; color: $hm-muted; }
.badge.pending { background: #fef3c7; color: #92400e; }
.badge.reviewed { background: #ccfbf1; color: #115e59; }
.badge.returned { background: #fee2e2; color: #991b1b; }
.badge.submitted { background: #dbeafe; color: #1e40af; }
.meta { display: flex; gap: 18rpx; flex-wrap: wrap; color: $hm-muted; font-size: 23rpx; margin-top: 10rpx; }
.late { color: $hm-danger; }
.desc { display: block; margin-top: 14rpx; color: #334155; font-size: 25rpx; line-height: 1.7; white-space: pre-wrap; }
.result-title { display: block; font-size: 26rpx; font-weight: 700; color: $hm-primary; }
.comment { display: block; margin-top: 10rpx; color: #334155; font-size: 25rpx; line-height: 1.7; white-space: pre-wrap; }
.score { display: block; margin-top: 10rpx; color: $hm-accent; font-weight: 700; }
.result.warn { border-left: 6rpx solid $hm-danger; }
.tip { display: block; margin-top: 10rpx; color: $hm-muted; font-size: 23rpx; }
.section { display: block; font-size: 27rpx; font-weight: 700; margin-bottom: 16rpx; }
.area { width: 100%; box-sizing: border-box; height: 200rpx; background: #f8fafc; border-radius: 14rpx; padding: 20rpx; font-size: 26rpx; margin-bottom: 16rpx; }
.images { display: flex; flex-wrap: wrap; gap: 12rpx; margin-bottom: 18rpx; }
.thumb-wrap { position: relative; }
.thumb { width: 160rpx; height: 160rpx; border-radius: 12rpx; }
.remove { position: absolute; top: -10rpx; right: -10rpx; width: 36rpx; height: 36rpx; line-height: 32rpx; text-align: center; border-radius: 50%; background: rgba(15, 23, 42, 0.7); color: #fff; font-size: 26rpx; }
.add { width: 160rpx; height: 160rpx; border-radius: 12rpx; border: 1rpx dashed #cbd5e1; display: flex; align-items: center; justify-content: center; color: $hm-muted; font-size: 44rpx; }
.primary { background: $hm-primary; color: #fff; border-radius: 16rpx; font-size: 27rpx; }
.history { padding: 18rpx 0; border-top: 1rpx solid $hm-border; }
.history:first-of-type { border-top: none; }
.history-head { display: flex; justify-content: space-between; color: $hm-muted; font-size: 22rpx; }
.history-text { display: block; margin-top: 8rpx; font-size: 24rpx; color: #334155; line-height: 1.6; white-space: pre-wrap; }
.hint { color: $hm-muted; text-align: center; padding: 40rpx 0; }
.hint.error { color: $hm-danger; }
</style>
