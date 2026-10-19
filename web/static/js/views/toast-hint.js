import { appCtx } from '../store/index.js';

export const ToastHint = {
    name: 'ToastHint',
    setup: () => appCtx(),
    template: `
    <div class="toast" :class="{ visible: toast.visible, error: toast.error }">{{ toast.message }}</div>
`,
};
