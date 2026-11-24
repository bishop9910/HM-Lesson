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
  uploadFile(options: UploadOptions): void;
}

const cloud = (globalThis as unknown as { wx?: { cloud?: CloudUploadApi } }).wx?.cloud;

/** 上传图片到云存储，返回 fileID；H5/本地调试无云能力时抛错。 */
export function uploadImage(filePath: string): Promise<string> {
  if (!cloud) {
    return Promise.reject(new Error("当前环境不支持云存储上传"));
  }
  const ext = filePath.split(".").pop()?.toLowerCase() || "jpg";
  const cloudPath = `hm/submissions/${Date.now()}-${Math.random().toString(36).slice(2, 10)}.${ext}`;
  return new Promise<string>((resolve, reject) => {
    cloud.uploadFile({
      cloudPath,
      filePath,
      success: (res) => resolve(res.fileID),
      fail: (err) => reject(new Error(err.errMsg || "上传失败")),
    });
  });
}
