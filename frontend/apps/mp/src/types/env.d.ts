/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_WX_CLOUD_ENV_ID?: string;
  readonly VITE_WX_SERVICE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
