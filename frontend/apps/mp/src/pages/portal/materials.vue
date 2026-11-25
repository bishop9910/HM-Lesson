<template>
  <view class="page">
    <view class="head">
      <text class="title">学习资料</text>
      <text class="hint">老师发布给你的讲义与真题</text>
    </view>

    <view v-if="loading && !items.length" class="state">加载中…</view>
    <view v-else-if="error" class="state" @tap="load">{{ error }}（点击重试）</view>
    <view v-else-if="!items.length" class="state">还没有可下载的资料</view>

    <view v-for="group in groups" :key="group.name" class="group">
      <text class="group-title">{{ group.name }}</text>
      <view v-for="item in group.items" :key="item.id" class="card">
        <view class="card-head">
          <text class="card-title">{{ item.title }}</text>
          <text class="date">{{ shortDate(item.published_at) }}</text>
        </view>
        <text v-if="item.description" class="desc">{{ item.description }}</text>
        <view class="files">
          <view v-for="(file, index) in item.files" :key="file.file_id" class="file">
            <text class="file-name" @tap="download(item, index)">{{ file.name || "附件" }}</text>
            <text class="file-tag" @tap="download(item, index)">下载</text>
          </view>
        </view>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";

import { ApiError, request } from "@/utils/request";

interface MaterialFile {
  name: string;
  file_id: string;
}

interface Material {
  id: string;
  category_name: string;
  title: string;
  description: string;
  published_at: string;
  files: MaterialFile[];
}

interface DownloadResult {
  tempFilePath: string;
}

interface CloudDownloadApi {
  init?(options: { env?: string }): void;
  downloadFile(options: { fileID: string; success: (res: DownloadResult) => void; fail: (err: { errMsg: string }) => void }): void;
}

interface OpenDocumentApi {
  openDocument(options: { filePath: string; fileType?: string; showMenu?: boolean; fail?: () => void }): void;
}

const items = ref<Material[]>([]);
const loading = ref(false);
const error = ref("");

const groups = computed(() => {
  const buckets = new Map<string, Material[]>();
  for (const item of items.value) {
    const key = item.category_name || "其他资料";
    const list = buckets.get(key) || [];
    list.push(item);
    buckets.set(key, list);
  }
  return Array.from(buckets, ([name, list]) => ({ name, items: list }));
});

function shortDate(iso: string): string {
  if (!iso) return "";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return "";
  return `${date.getMonth() + 1}月${date.getDate()}日`;
}

async function load() {
  const token = uni.getStorageSync("hm_guardian_token") as string | undefined;
  if (!token) {
    uni.reLaunch({ url: "/pages/guardian-login/index" });
    return;
  }
  loading.value = true;
  error.value = "";
  try {
    const res = await request<{ items: Material[] }>("/api/v1/portal/materials", "GET", undefined, "guardian");
    items.value = res.items || [];
  } catch (err) {
    if (err instanceof ApiError && err.status === 401) {
      uni.removeStorageSync("hm_guardian_token");
      uni.reLaunch({ url: "/pages/guardian-login/index" });
      return;
    }
    error.value = err instanceof ApiError ? err.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

async function download(material: Material, index: number) {
  const file = material.files[index];
  if (!file) return;
  const cloud = (globalThis as { wx?: { cloud?: CloudDownloadApi } }).wx?.cloud;
  if (!cloud) {
    uni.showToast({ title: "请在小程序里下载", icon: "none" });
    return;
  }
  uni.showLoading({ title: "准备中" });
  try {
    // 先让后端校验可见性并记录下载日志，再真正拉文件。
    await request(`/api/v1/portal/materials/${material.id}/download`, "POST", {}, "guardian");
    const tempPath = await new Promise<string>((resolve, reject) => {
      cloud.downloadFile({
        fileID: file.file_id,
        success: (res) => resolve(res.tempFilePath),
        fail: (err) => reject(new Error(err.errMsg || "下载失败")),
      });
    });
    const suffix = file.name.split(".").pop()?.toLowerCase() || "pdf";
    openTemp(tempPath, suffix);
  } catch (err) {
    uni.showToast({ title: err instanceof Error ? err.message : "下载失败", icon: "none" });
  } finally {
    uni.hideLoading();
  }
}

function openTemp(filePath: string, suffix: string) {
  const wxApi = (globalThis as { wx?: { openDocument?: OpenDocumentApi["openDocument"] } }).wx;
  if (!wxApi?.openDocument) {
    uni.showToast({ title: "已下载，请用微信打开", icon: "none" });
    return;
  }
  wxApi.openDocument({
    filePath,
    fileType: suffix,
    showMenu: true,
    fail: () => uni.showToast({ title: "该格式暂不支持预览", icon: "none" }),
  });
}

onShow(load);
</script>

<style lang="scss" scoped>
@import "@/styles/tokens.scss";

.page {
  min-height: 100vh;
  padding: 24rpx;
  box-sizing: border-box;
  background: $hm-bg;
}

.head {
  margin-bottom: 20rpx;
}

.title {
  display: block;
  font-size: 40rpx;
  font-weight: 600;
  color: $hm-ink;
}

.hint {
  font-size: 24rpx;
  color: $hm-muted;
}

.group {
  margin-bottom: 24rpx;
}

.group-title {
  display: block;
  margin-bottom: 12rpx;
  font-size: 26rpx;
  font-weight: 600;
  color: $hm-primary;
}

.card {
  padding: 24rpx;
  margin-bottom: 16rpx;
  border-radius: $hm-radius;
  border: 1rpx solid $hm-rule;
  background: rgba(255, 255, 255, 0.55);
}

.card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-title {
  flex: 1;
  font-size: 30rpx;
  font-weight: 600;
  color: $hm-ink;
}

.date {
  margin-left: 12rpx;
  font-size: 22rpx;
  color: $hm-muted;
}

.desc {
  display: block;
  margin-top: 8rpx;
  font-size: 26rpx;
  color: $hm-muted;
  line-height: 1.5;
}

.files {
  margin-top: 12rpx;
}

.file {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12rpx 0;
  border-top: 1rpx dashed $hm-rule;
}

.file-name {
  flex: 1;
  font-size: 26rpx;
  color: $hm-ink;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.file-tag {
  margin-left: 16rpx;
  font-size: 24rpx;
  color: $hm-primary;
}

.state {
  padding: 80rpx 0;
  text-align: center;
  font-size: 28rpx;
  color: $hm-muted;
}
</style>
