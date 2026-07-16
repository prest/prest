/// <reference types="vite/client" />

interface ImportMetaEnv {
	readonly VITE_PREST_PROXY_TARGET?: string
}

interface ImportMeta {
	readonly env: ImportMetaEnv
}
