type HttpMethod = "GET" | "POST" | "PATCH" | "PUT" | "DELETE";
type RequestData = string | Record<string, unknown> | ArrayBuffer;

interface CloudCallOptions {
  config: { env: string };
  path: string;
  method: HttpMethod;
  header?: Record<string, string>;
  data?: RequestData;
  timeout?: number;
  success: (res: { statusCode: number; data: unknown }) => void;
  fail: (err: { errMsg: string }) => void;
}

interface CloudApi {
  init(options: { env: string }): void;
  callContainer(options: CloudCallOptions): void;
}

const cloud = (globalThis as unknown as { wx?: { cloud?: CloudApi } }).wx?.cloud;
// 生产默认值：当前只有一个云托管环境；本地/未来多环境可用 VITE_* 覆盖。
const cloudEnv = import.meta.env.VITE_WX_CLOUD_ENV_ID ?? "prod-d8gq8vzftc39caa5b";
const serviceName = import.meta.env.VITE_WX_SERVICE ?? "hm-lesson";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly reason: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export function initCloud(): void {
  if (cloud && cloudEnv) {
    cloud.init({ env: cloudEnv });
  }
}

function authHeader(): Record<string, string> {
  const token = (uni.getStorageSync("hm_token") as string) || "";
  return token ? { "X-HM-Token": token } : {};
}

function parseError(status: number, body: unknown): ApiError {
  if (body && typeof body === "object") {
    const record = body as Record<string, unknown>;
    const reason = typeof record.reason === "string" ? record.reason : "UNKNOWN";
    const message = typeof record.message === "string" ? record.message : "请求失败";
    return new ApiError(status, reason, message);
  }
  return new ApiError(status, "UNKNOWN", `请求失败（HTTP ${status}）`);
}

export function request<T>(path: string, method: HttpMethod, data?: RequestData): Promise<T> {
  const header: Record<string, string> = { ...authHeader() };
  const timeout = 15000;

  if (cloud && cloudEnv) {
    return new Promise<T>((resolve, reject) => {
      cloud.callContainer({
        config: { env: cloudEnv },
        path,
        method,
        header: { ...header, "X-WX-SERVICE": serviceName },
        data,
        timeout,
        success: (res) => {
          if (res.statusCode >= 200 && res.statusCode < 300) {
            resolve(res.data as T);
            return;
          }
          reject(parseError(res.statusCode, res.data));
        },
        fail: (err) => reject(new ApiError(0, "NETWORK", err.errMsg)),
      });
    });
  }

  // H5/本地调试：uni.request；生产小程序不会走这里。
  // uni.request 的类型不含 PATCH，H5 调试时降级为 POST + 覆盖头。
  const isPatch = method === "PATCH";
  const uniMethod = (isPatch ? "POST" : method) as "GET" | "POST" | "PUT" | "DELETE";
  const uniHeader = isPatch ? { ...header, "X-HTTP-Method-Override": "PATCH" } : header;
  return new Promise<T>((resolve, reject) => {
    uni.request({
      url: path,
      method: uniMethod,
      header: uniHeader,
      data,
      timeout,
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(res.data as T);
          return;
        }
        reject(parseError(res.statusCode, res.data));
      },
      fail: (err) => reject(new ApiError(0, "NETWORK", err.errMsg)),
    });
  });
}
