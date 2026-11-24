<script setup lang="ts">
import { computed, ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import { ApiError, request } from "@/utils/request";

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
  student_name: string;
  status: string;
  due_at?: string;
  submitted_at?: string;
  score?: number;
  comment: string;
  return_reason: string;
  submission?: Submission;
  history?: Submission[];
}
interface Task {
  id: string;
  title: string;
  description: string;
  type: string;
  status: string;
  due_at?: string;
  assigned_count: number;
  submitted_count: number;
  pending_review: number;
  overdue_count: number;
  submit_rate: number;
}
interface Stats {
  assigned: number;
  submitted: number;
  reviewed: number;
  returned: number;
  pending: number;
  pending_review: number;
  overdue: number;
  rate: number;
}

const typeLabels: Record<string, string> = {
  homework: "常规作业",
  correction: "试卷订正",
  trial: "试听任务",
  checkin: "打卡",
};
const statusLabels: Record<string, string> = {
  pending: "待提交",
  submitted: "已提交",
  reviewed: "已批阅",
  returned: "已打回",
};

const id = ref("");
const task = ref<Task | null>(null);
const assignments = ref<Assignment[]>([]);
const stats = ref<Stats | null>(null);
const loading = ref(true);
const error = ref("");
const active = ref<Assignment | null>(null);
const score = ref("");
const comment = ref("");
const returnReason = ref("");
const saving = ref(false);

function fmt(iso?: string): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return `${d.getMonth() + 1}月${d.getDate()}日 ${`${d.getHours()}`.padStart(2, "0")}:${`${d.getMinutes()}`.padStart(2, "0")}`;
}

async function load(): Promise<void> {
  loading.value = true;
  try {
    const res = await request<{ task: Task; assignments: Assignment[]; stats: Stats }>(`/api/v1/tasks/${id.value}`, "GET");
    task.value = res.task;
    assignments.value = res.assignments;
    stats.value = res.stats;
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : "加载失败";
  } finally {
    loading.value = false;
  }
}

function openReview(a: Assignment): void {
  active.value = a;
  score.value = a.score === undefined ? "" : String(a.score);
  comment.value = a.comment || "";
  returnReason.value = "";
}

async function submitReview(): Promise<void> {
  if (!active.value) return;
  saving.value = true;
  try {
    const payload: Record<string, unknown> = { comment: comment.value.trim(), return_reason: returnReason.value.trim() };
    if (score.value.trim() !== "") payload.score = Number(score.value);
    await request(`/api/v1/assignments/${active.value.id}/review`, "POST", payload);
    uni.showToast({ title: returnReason.value.trim() ? "已打回" : "已批阅", icon: "success" });
    active.value = null;
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "操作失败", icon: "none" });
  } finally {
    saving.value = false;
  }
}

async function closeTask(): Promise<void> {
  const confirmed = await new Promise<boolean>((resolve) => {
    uni.showModal({
      title: "结束任务",
      content: "结束后家长仍可查看，但不再接收新提交。确定？",
      success: (r) => resolve(Boolean(r.confirm)),
      fail: () => resolve(false),
    });
  });
  if (!confirmed) return;
  try {
    await request(`/api/v1/tasks/${id.value}/close`, "POST");
    uni.showToast({ title: "已结束", icon: "success" });
    await load();
  } catch (e) {
    uni.showToast({ title: e instanceof ApiError ? e.message : "操作失败", icon: "none" });
  }
}

function preview(urls: string[], current: string): void {
  uni.previewImage({ urls, current });
}

const submittedList = computed(() => assignments.value.filter((a) => a.status === "submitted"));

onLoad((options) => {
  id.value = (options?.id as string) || "";
  if (!id.value) {
    error.value = "缺少任务 id";
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
    <template v-else-if="task">
      <view class="card">
        <view class="row">
          <text class="title">{{ task.title }}</text>
          <text class="badge">{{ task.status === "closed" ? "已结束" : task.status === "draft" ? "草稿" : "进行中" }}</text>
        </view>
        <view class="meta">
          <text>{{ typeLabels[task.type] || task.type }}</text>
          <text>截止 {{ fmt(task.due_at) }}</text>
        </view>
        <text v-if="task.description" class="desc">{{ task.description }}</text>
        <view class="stats">
          <view class="stat"><text class="num">{{ stats?.assigned }}</text><text class="lbl">下发</text></view>
          <view class="stat"><text class="num">{{ stats?.submitted }}</text><text class="lbl">已交</text></view>
          <view class="stat"><text class="num">{{ stats?.pending_review }}</text><text class="lbl">待批</text></view>
          <view class="stat danger"><text class="num">{{ stats?.overdue }}</text><text class="lbl">逾期</text></view>
        </view>
        <view class="bar"><view class="bar-in" :style="{ width: `${Math.round((stats?.rate || 0) * 100)}%` }" /></view>
        <button v-if="task.status === 'published'" class="ghost close" @tap="closeTask">结束任务</button>
      </view>

      <view class="section-head">
        <text>学员提交（{{ assignments.length }}）</text>
        <text class="sub">待批 {{ submittedList.length }}</text>
      </view>

      <view v-for="a in assignments" :key="a.id" class="card assignment">
        <view class="row">
          <text class="name">{{ a.student_name }}</text>
          <text class="badge" :class="a.status">{{ statusLabels[a.status] }}</text>
        </view>
        <view class="meta">
          <text>提交 {{ fmt(a.submitted_at) }}</text>
          <text v-if="a.score !== undefined">评分 {{ a.score }}</text>
          <text v-if="a.submission?.is_late" class="late">逾期提交</text>
        </view>
        <view v-if="a.submission" class="submission">
          <text v-if="a.submission.text" class="text">{{ a.submission.text }}</text>
          <view v-if="a.submission.file_ids.length" class="images">
            <image v-for="f in a.submission.file_ids" :key="f" :src="f" mode="aspectFill" class="thumb" @tap="preview(a.submission!.file_ids, f)" />
          </view>
          <text class="version">第 {{ a.submission.version }} 次提交</text>
        </view>
        <view v-if="a.comment || a.return_reason" class="reviewed">
          <text v-if="a.comment">评语：{{ a.comment }}</text>
          <text v-if="a.return_reason" class="danger-text">打回理由：{{ a.return_reason }}</text>
        </view>
        <button v-if="a.status === 'submitted'" class="primary review" @tap="openReview(a)">批阅</button>
        <button v-else-if="a.status === 'returned'" class="ghost review" disabled>已打回，等待重交</button>
      </view>

      <view v-if="assignments.length === 0" class="hint">还没有下发记录</view>
    </template>

    <view v-if="active" class="mask" @tap.self="active = null">
      <view class="sheet">
        <view class="sheet-head">
          <text class="sheet-title">批阅 · {{ active.student_name }}</text>
          <text class="close" @tap="active = null">关闭</text>
        </view>
        <view v-if="active.submission?.text" class="quote">{{ active.submission.text }}</view>
        <input v-model="score" class="field" type="digit" placeholder="评分 0–100（可空）" />
        <textarea v-model="comment" class="field area" placeholder="评语（家长可见）" maxlength="2000" />
        <textarea v-model="returnReason" class="field area danger" placeholder="打回理由（填写即打回，家长可重交）" maxlength="1000" />
        <view class="actions">
          <button class="ghost" :disabled="saving" @tap="active = null">取消</button>
          <button class="primary" :disabled="saving" @tap="submitReview">提交批阅</button>
        </view>
      </view>
    </view>
  </view>
</template>

<style lang="scss">
@import "@/styles/tokens.scss";
.page { min-height: 100vh; background: $hm-bg; padding: 24rpx 24rpx 60rpx; }
.card { background: #fff; border-radius: $hm-radius; padding: 26rpx; margin-bottom: 20rpx; }
.row { display: flex; justify-content: space-between; gap: 16rpx; align-items: center; }
.title { font-size: 32rpx; font-weight: 700; color: $hm-text; flex: 1; }
.badge { font-size: 22rpx; padding: 4rpx 14rpx; border-radius: 999rpx; background: #f1f5f9; color: $hm-muted; }
.badge.submitted { background: #fef3c7; color: #92400e; }
.badge.reviewed { background: #ccfbf1; color: #115e59; }
.badge.returned { background: #fee2e2; color: #991b1b; }
.meta { display: flex; gap: 18rpx; flex-wrap: wrap; color: $hm-muted; font-size: 23rpx; margin-top: 10rpx; }
.desc { display: block; color: #334155; font-size: 25rpx; line-height: 1.7; margin-top: 14rpx; white-space: pre-wrap; }
.stats { display: flex; gap: 34rpx; margin-top: 20rpx; }
.stat { display: flex; flex-direction: column; }
.num { font-size: 34rpx; font-weight: 700; }
.lbl { font-size: 21rpx; color: $hm-muted; }
.stat.danger .num { color: $hm-danger; }
.bar { height: 8rpx; background: #e2e8f0; border-radius: 999rpx; margin-top: 18rpx; overflow: hidden; }
.bar-in { height: 100%; background: $hm-primary; }
.ghost { background: #fff; color: $hm-primary; border: 1rpx solid $hm-border; border-radius: 16rpx; font-size: 25rpx; }
.primary { background: $hm-primary; color: #fff; border-radius: 16rpx; font-size: 25rpx; }
.close { margin-top: 18rpx; font-size: 24rpx; }
.section-head { display: flex; justify-content: space-between; margin: 10rpx 0 16rpx; font-size: 27rpx; font-weight: 600; }
.sub { color: $hm-muted; font-size: 23rpx; font-weight: 400; }
.name { font-size: 28rpx; font-weight: 600; }
.submission { margin-top: 14rpx; background: #f8fafc; border-radius: 14rpx; padding: 18rpx; }
.text { font-size: 25rpx; line-height: 1.7; color: #334155; white-space: pre-wrap; }
.images { display: flex; flex-wrap: wrap; gap: 10rpx; margin-top: 12rpx; }
.thumb { width: 150rpx; height: 150rpx; border-radius: 10rpx; }
.version { display: block; font-size: 21rpx; color: $hm-muted; margin-top: 10rpx; }
.reviewed { margin-top: 12rpx; font-size: 24rpx; color: #334155; display: flex; flex-direction: column; gap: 6rpx; }
.danger-text, .late { color: $hm-danger; }
.review { margin-top: 16rpx; }
.hint { color: $hm-muted; text-align: center; padding: 40rpx 0; }
.hint.error { color: $hm-danger; }
.mask { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.45); display: flex; align-items: flex-end; z-index: 20; }
.sheet { width: 100%; background: #fff; border-radius: 32rpx 32rpx 0 0; padding: 30rpx 24rpx calc(30rpx + env(safe-area-inset-bottom)); box-sizing: border-box; }
.sheet-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 18rpx; }
.sheet-title { font-size: 30rpx; font-weight: 700; }
.quote { background: #f8fafc; border-radius: 14rpx; padding: 18rpx; color: #334155; font-size: 24rpx; margin-bottom: 16rpx; max-height: 180rpx; overflow: auto; }
.field { width: 100%; box-sizing: border-box; background: #f8fafc; border-radius: 14rpx; padding: 20rpx; font-size: 26rpx; margin-bottom: 14rpx; }
.area { height: 120rpx; }
.field.danger { border: 1rpx solid #fecaca; }
.actions { display: flex; gap: 16rpx; margin-top: 10rpx; }
.actions button { flex: 1; }
</style>
