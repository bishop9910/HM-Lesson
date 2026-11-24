import { defineStore } from "pinia";
import { ApiError, request } from "@/utils/request";

interface SessionState {
  token: string;
  role: "" | "owner" | "assistant" | "guardian";
  staffId: string;
  staffName: string;
}

export const useSessionStore = defineStore("session", {
  state: (): SessionState => ({
    token: (uni.getStorageSync("hm_token") as string) || "",
    role: (uni.getStorageSync("hm_role") as SessionState["role"]) || "",
    staffId: (uni.getStorageSync("hm_staff_id") as string) || "",
    staffName: (uni.getStorageSync("hm_staff_name") as string) || "",
  }),
  getters: {
    isTeacher: (state): boolean => state.role === "owner" || state.role === "assistant",
    isOwner: (state): boolean => state.role === "owner",
  },
  actions: {
    setSession(token: string, role: SessionState["role"], staffId: string, staffName: string): void {
      this.token = token;
      this.role = role;
      this.staffId = staffId;
      this.staffName = staffName;
      uni.setStorageSync("hm_token", token);
      uni.setStorageSync("hm_role", role);
      uni.setStorageSync("hm_staff_id", staffId);
      uni.setStorageSync("hm_staff_name", staffName);
    },
    // 用服务端身份覆盖本地缓存，避免旧缓存里的姓名/角色继续显示。
    async refresh(): Promise<boolean> {
      if (!this.token) return false;
      try {
        const res = await request<{ staff: { id: string; name: string; role: SessionState["role"] } }>(
          "/api/v1/auth/me",
          "GET",
        );
        this.setSession(this.token, res.staff.role, res.staff.id, res.staff.name);
        return true;
      } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
          this.clear();
        }
        return false;
      }
    },
    clear(): void {
      this.token = "";
      this.role = "";
      this.staffId = "";
      this.staffName = "";
      uni.removeStorageSync("hm_token");
      uni.removeStorageSync("hm_role");
      uni.removeStorageSync("hm_staff_id");
      uni.removeStorageSync("hm_staff_name");
    },
  },
});
