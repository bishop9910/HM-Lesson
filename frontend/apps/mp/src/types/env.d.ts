/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_WX_CLOUD_ENV_ID?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
