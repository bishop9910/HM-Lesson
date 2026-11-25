<template>
  <view class="page">
    <view class="head">
      <text class="title">资料库</text>
      <text class="hint">{{ materials.length }} 份 · {{ publishedCount }} 已发布</text>
    </view>

    <scroll-view class="chips" scroll-x>
      <view
        v-for="chip in categoryChips"
        :key="chip.id"
        class="chip"
        :class="{ active: chip.id === categoryID }"
        @tap="selectCategory(chip.id)"
      >
        {{ chip.name }}
      </view>
    </scroll-view>

    <view class="toolbar">
      <view class="btn" hover-class="press" @tap="load">{{ loading ? '刷新中…' : '刷新' }}</view>
      <view class="btn primary" hover-class="press" @tap="openCreate">新建资料</view>
    </view>

    <view v-if="loading && !materials.length" class="state">加载中…</view>
    <view v-else-if="error" class="state" @tap="load">{{ error }}（点击重试）</view>
    <view v-else-if="!materials.length" class="state">还没有资料，点「新建资料」开始</view>

    <view v-for="item in materials" :key="item.id" class="card">
      <view class="card-head">
        <text class="card-title">{{ item.title }}</text>
        <text class="badge" :class="item.status">{{ statusLabel[item.status] }}</text>
      </view>
      <text class="meta">{{ item.category_name }} · {{ gradeLabel(item.grade) }} · {{ item.files.length }} 个附件</text>
      <text v-if="item.description" class="desc">{{ item.description }}</text>
      <view class="actions">
        <button v-if="item.status !== 'published'" class="mini" @tap="publish(item)">发布</button>
        <button v-if="item.status === 'published'" class="mini" @tap="archive(item)">归档</button>
        <button class="mini danger" @tap="remove(item)">删除</button>
      </view>
    </view>

    <view v-if="sheet" class="mask" @tap="closeSheet">
      <view class="sheet" @tap.stop>
        <text class="sheet-title">新建资料</text>
        <input v-model="form.title" class="field" placeholder="标题（必填，≤200 字）" maxlength="200" />
        <input
          v-model="form.description"
          class="field"
          placeholder="说明（可选）"
          maxlength="200"
        />
        <view class="row">
          <picker :range="categoryNames" @change="onCategoryChange">
            <view class="field picker">{{ currentCategoryName || "选择分类" }}</view>
          </picker>
          <picker :range="gradeNames" @change="onGradeChange">
            <view class="field picker">{{ gradeLabel(form.grade) || "年级（可选）" }}</view>
          </picker>
        </view>
        <view class="files">
          <view v-for="(file, index) in form.files" :key="file.file_id" class="file">
            <text class="file-name">{{ file.name }}</text>
            <text class="file-del" @tap="removeFile(index)">移除</text>
          </view>
          <view v-if="form.files.length < 9" class="btn" hover-class="press" @tap="chooseFiles">添加文件（最多 9 个）</view>
        </view>
        <view class="btn primary" hover-class="press" @tap="submit">{{ saving ? '保存中…' : '保存为草稿' }}</view>
        <view class="btn" hover-class="press" @tap="closeSheet">取消</view>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from "vue";
import { onShow } from "@dcloudio/uni-app";

import { ApiError, request } from "@/utils/request";
import { uploadMaterial } from "@/utils/cloud";

interface MaterialFile {
  name: string;
  file_id: string;
  size?: number;
}

interface Material {
  id: string;
  category_id: string;
  category_name: string;
  title: string;
  description: string;
  grade: string;
  status: "draft" | "published" | "archived";
  files: MaterialFile[];
  published_at: string;
}

interface Category {
  id: string;
  code: string;
  name: string;
  visibility: Record<string, boolean>;
}

interface ChooseFileResult {
  tempFiles: { name: string; path: string; size: number }[];
}

interface ChooseFileApi {
  chooseMessageFile(options: {
    count: number;
    type: string;
    extension: string[];
    success: (res: ChooseFileResult) => void;
    fail: () => void;
  }): void;
}

const statusLabel: Record<string, string> = { draft: "草稿", published: "已发布", archived: "已归档" };
const gradeNames = ["通用", "高一", "高二", "高三", "其他"];
const gradeValues = ["", "g1", "g2", "g3", "other"];

const materials = ref<Material[]>([]);
const categories = ref<Category[]>([]);
const categoryID = ref("");
const loading = ref(false);
const saving = ref(false);
const error = ref("");
const sheet = ref(false);

const form = reactive<{ title: string; description: string; category_id: string; grade: string; files: MaterialFile[] }>({
  title: "",
  description: "",
  category_id: "",
  grade: "",
  files: [],
});

const publishedCount = computed(() => materials.value.filter((item) => item.status === "published").length);
const categoryChips = computed(() => [{ id: "", name: "全部" }, ...categories.value.map((c) => ({ id: c.id, name: c.name }))]);
const categoryNames = computed(() => categories.value.map((c) => c.name));
const currentCategoryName = computed(() => categories.value.find((c) => c.id === form.category_id)?.name || "");

function gradeLabel(value: string): string {
  const index = gradeValues.indexOf(value);
  return index >= 0 ? gradeNames[index] : "";
}

async function load() {
  loading.value = true;
  error.value = "";
  try {
    const query = categoryID.value ? `?category_id=${categoryID.value}` : "";
    const res = await request<{ items: Material[] }>(`/api/v1/materials${query}`, "GET");
    materials.value = res.items || [];
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

async function loadCategories() {
  try {
    const res = await request<{ items: Category[] }>("/api/v1/material-categories", "GET");
    categories.value = res.items || [];
    if (!form.category_id && categories.value.length) form.category_id = categories.value[0].id;
  } catch {
    // 分类加载失败不阻塞列表。
  }
}

function selectCategory(id: string) {
  categoryID.value = id;
  void load();
}

function openCreate() {
  form.title = "";
  form.description = "";
  form.grade = "";
  form.files = [];
  if (categories.value.length) form.category_id = categories.value[0].id;
  sheet.value = true;
}

function closeSheet() {
  if (!saving.value) sheet.value = false;
}

function onCategoryChange(event: { detail: { value: number } }) {
  const picked = categories.value[event.detail.value];
  if (picked) form.category_id = picked.id;
}

function onGradeChange(event: { detail: { value: number } }) {
  form.grade = gradeValues[event.detail.value] || "";
}

function chooseFiles() {
  const wxApi = (globalThis as { wx?: ChooseFileApi }).wx;
  if (!wxApi?.chooseMessageFile) {
    uni.showToast({ title: "当前环境不支持选择文件", icon: "none" });
    return;
  }
  wxApi.chooseMessageFile({
    count: 1,
    type: "file",
    extension: ["pdf", "doc", "docx", "ppt", "pptx", "xls", "xlsx", "jpg", "jpeg", "png", "zip"],
    success: async (res) => {
      const file = res.tempFiles?.[0];
      if (!file) return;
      uni.showLoading({ title: "上传中" });
      try {
        const fileID = await uploadMaterial(file.path, file.name);
        form.files.push({ name: file.name, file_id: fileID, size: file.size });
      } catch (err) {
        uni.showToast({ title: err instanceof Error ? err.message : "上传失败", icon: "none" });
      } finally {
        uni.hideLoading();
      }
    },
    fail: () => undefined,
  });
}

function removeFile(index: number) {
  form.files.splice(index, 1);
}

async function submit() {
  if (!form.title.trim()) {
    uni.showToast({ title: "请填写标题", icon: "none" });
    return;
  }
  if (!form.category_id) {
    uni.showToast({ title: "请选择分类", icon: "none" });
    return;
  }
  saving.value = true;
  try {
    await request("/api/v1/materials", "POST", {
      category_id: form.category_id,
      title: form.title,
      description: form.description,
      grade: form.grade,
      files: form.files,
    });
    uni.showToast({ title: "已保存为草稿", icon: "success" });
    sheet.value = false;
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "保存失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

async function publish(item: Material) {
  try {
    await request(`/api/v1/materials/${item.id}/publish`, "POST", {});
    uni.showToast({ title: "已发布", icon: "success" });
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "发布失败", icon: "none" });
  }
}

async function archive(item: Material) {
  try {
    await request(`/api/v1/materials/${item.id}/archive`, "POST", {});
    uni.showToast({ title: "已归档", icon: "success" });
    await load();
  } catch (err) {
    uni.showToast({ title: err instanceof ApiError ? err.message : "归档失败", icon: "none" });
  }
}

function remove(item: Material) {
  uni.showModal({
    title: "删除资料",
    content: `确定删除「${item.title}」？下载记录也会一起清掉。`,
    confirmColor: "#b91c1c",
    success: async (res) => {
      if (!res.confirm) return;
      try {
        await request(`/api/v1/materials/${item.id}`, "DELETE");
        uni.showToast({ title: "已删除", icon: "success" });
        await load();
      } catch (err) {
        uni.showToast({ title: err instanceof ApiError ? err.message : "删除失败", icon: "none" });
      }
    },
  });
}

onShow(() => {
  void loadCategories();
  void load();
});
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
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 16rpx;
}

.title {
  font-size: 40rpx;
  font-weight: 600;
  color: $hm-ink;
}

.hint {
  font-size: 24rpx;
  color: $hm-muted;
}

.chips {
  white-space: nowrap;
  margin-bottom: 16rpx;
}

.chip {
  display: inline-block;
  padding: 10rpx 24rpx;
  margin-right: 12rpx;
  border-radius: 999rpx;
  border: 1rpx solid $hm-rule;
  font-size: 26rpx;
  color: $hm-ink;
  background: $hm-surface;

  &.active {
    background: $hm-primary;
    border-color: $hm-primary;
    color: #fff;
  }
}

.toolbar {
  display: flex;
  margin-bottom: 16rpx;
}

/* 工具栏用 view 磁贴，不用原生 button：原生 button 在 flex 里会被压成一条，
   文字竖排/裁字（v1.3.6 前的老问题）。两个一组、等宽铺满。 */
.btn {
  flex: 1;
  height: 80rpx;
  line-height: 80rpx;
  margin-right: 16rpx;
  border-radius: 16rpx;
  background: $hm-surface;
  border: 1rpx solid $hm-border;
  color: $hm-ink;
  font-size: 28rpx;
  text-align: center;
  white-space: nowrap;
}

.btn:last-child {
  margin-right: 0;
}

.btn.primary {
  background: $hm-primary;
  border-color: $hm-primary;
  color: #fff;
}

.press {
  opacity: 0.72;
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
  font-size: 32rpx;
  font-weight: 600;
  color: $hm-ink;
}

.badge {
  font-size: 22rpx;
  padding: 4rpx 14rpx;
  border-radius: 999rpx;
  background: rgba(15, 118, 110, 0.12);
  color: $hm-primary;

  &.draft {
    background: rgba(217, 119, 6, 0.14);
    color: $hm-accent;
  }

  &.archived {
    background: rgba(34, 48, 42, 0.1);
    color: $hm-muted;
  }
}

.meta {
  display: block;
  margin-top: 8rpx;
  font-size: 24rpx;
  color: $hm-muted;
}

.desc {
  display: block;
  margin-top: 8rpx;
  font-size: 26rpx;
  color: $hm-ink;
  line-height: 1.5;
}

.actions {
  display: flex;
  margin-top: 16rpx;
}

.state {
  padding: 60rpx 0;
  text-align: center;
  font-size: 28rpx;
  color: $hm-muted;
}

.mask {
  position: fixed;
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  background: rgba(18, 24, 22, 0.45);
  display: flex;
  align-items: flex-end;
}

.sheet {
  width: 100%;
  max-height: 88vh;
  overflow-y: auto;
  padding: 28rpx 24rpx 40rpx;
  box-sizing: border-box;
  border-radius: $hm-radius $hm-radius 0 0;
  background: $hm-surface;
}

.sheet-title {
  display: block;
  font-size: 32rpx;
  font-weight: 600;
  color: $hm-ink;
  margin-bottom: 16rpx;
}

.field {
  width: 100%;
  height: 88rpx;
  line-height: 88rpx;
  padding: 0 22rpx;
  margin-bottom: 14rpx;
  box-sizing: border-box;
  border-radius: 14rpx;
  border: 1rpx solid $hm-rule;
  background: #fff;
  font-size: 28rpx;
  color: $hm-ink;
}

.picker {
  color: $hm-ink;
}

.row {
  display: flex;
}

.row .field {
  flex: 1;
}

.files {
  margin: 8rpx 0 16rpx;
}

.file {
  display: flex;
  justify-content: space-between;
  padding: 12rpx 0;
  font-size: 26rpx;
  border-bottom: 1rpx dashed $hm-rule;
}

.file-name {
  flex: 1;
  color: $hm-ink;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.file-del {
  margin-left: 16rpx;
  color: $hm-danger;
}

.sheet .btn {
  margin-right: 0;
  margin-top: 16rpx;
  flex: none;
  width: 100%;
}

.mini {
  flex: 1;
  height: 64rpx;
  line-height: 64rpx;
  padding: 0;
  margin-right: 12rpx;
  font-size: 24rpx;
  background: transparent;
  color: $hm-primary;
  border: 1rpx solid $hm-primary;
  border-radius: 12rpx;
}

.mini.danger {
  color: $hm-danger;
  border-color: rgba(185, 28, 28, 0.5);
}
</style>
