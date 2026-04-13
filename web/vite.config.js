import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';
export default defineConfig({
    plugins: [
        vue({
            template: {
                compilerOptions: {
                    isCustomElement: (tag) => tag.startsWith('md-')
                }
            }
        })
    ],
    build: {
        outDir: '../server/public',
        emptyOutDir: false // 避免删除 public 中原本的文件
    }
});
