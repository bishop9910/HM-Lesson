interface UploadResult {
  fileID: string;
}

interface UploadOptions {
  cloudPath: string;
  filePath: string;
  success: (res: UploadResult) => void;
  fail: (err: { errMsg: string }) => void;
}

interface CloudUploadApi {
  init?(options: { env?: string }): void;
  uploadFile(options: UploadOptions): void;
}

const cloud = (globalThis as unknown as { wx?: { cloud?: CloudUploadApi } }).wx?.cloud;
const cloudEnv = (import.meta.env.VITE_WX_CLOUD_ENV_ID as string | undefined) || "prod-d8gq8vzftc39caa5b";
let cloudReady = false;

function ensureCloud(): CloudUploadApi {
  if (!cloud) throw new Error("当前环境不支持云存储上传");
  if (!cloudReady) {
    cloud.init?.({ env: cloudEnv });
    cloudReady = true;
  }
  return cloud;
}

/** 上传图片到云存储，返回 fileID；H5/本地调试无云能力时抛错。 */
export function uploadImage(filePath: string): Promise<string> {
  const api = ensureCloud();
  const ext = filePath.split(".").pop()?.toLowerCase() || "jpg";
  const cloudPath = `hm/submissions/${Date.now()}-${Math.random().toString(36).slice(2, 10)}.${ext}`;
  return new Promise<string>((resolve, reject) => {
    api.uploadFile({
      cloudPath,
      filePath,
      success: (res) => resolve(res.fileID),
      fail: (err) => reject(new Error(err.errMsg || "上传失败")),
    });
  });
}
