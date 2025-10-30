/// <reference types="vite/client" />

interface ImportMetaEnv {
    readonly VITE_API_HOST: string;
    readonly VITE_WS_HOST: string;
}

interface ImportMeta {
    readonly env: ImportMetaEnv;
}

// DOMPurify type shim if types are not installed
declare module "dompurify" {
    const DOMPurify: {
        sanitize(dirty: string, config?: { ALLOWED_TAGS?: string[]; ALLOWED_ATTR?: string[] }): string;
    };
    export default DOMPurify;
}