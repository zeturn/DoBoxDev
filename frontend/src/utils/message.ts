// Toast notification system to replace antd message
type ToastType = 'success' | 'error' | 'info' | 'warning';

interface ToastOptions {
  message: string;
  type: ToastType;
  duration?: number;
}

class ToastManager {
  private container: HTMLDivElement | null = null;

  private getContainer() {
    if (!this.container) {
      this.container = document.createElement('div');
      this.container.className = 'fixed top-4 right-4 z-50 flex flex-col gap-2';
      document.body.appendChild(this.container);
    }
    return this.container;
  }

  private show({ message, type, duration = 3000 }: ToastOptions) {
    const container = this.getContainer();
    const toast = document.createElement('div');
    
    const colors = {
      success: 'bg-green-50 border-green-500 text-green-800 dark:bg-green-500/15 dark:border-green-400 dark:text-green-300',
      error: 'bg-red-50 border-red-500 text-red-800 dark:bg-red-500/15 dark:border-red-400 dark:text-red-300',
      info: 'bg-blue-50 border-blue-500 text-blue-800 dark:bg-blue-500/15 dark:border-blue-400 dark:text-blue-300',
      warning: 'bg-yellow-50 border-yellow-500 text-yellow-800 dark:bg-yellow-500/15 dark:border-yellow-400 dark:text-yellow-300',
    };

    const icons = {
      success: '✓',
      error: '✕',
      info: 'ℹ',
      warning: '⚠',
    };

    toast.className = `
      ${colors[type]}
      px-4 py-3 rounded-lg border-l-4 min-w-[300px] max-w-[500px]
      animate-[slideIn_0.3s_ease-out] flex items-center gap-3
    `;
    
    toast.innerHTML = `
      <span class="text-lg font-bold">${icons[type]}</span>
      <span class="flex-1">${message}</span>
    `;

    container.appendChild(toast);

    setTimeout(() => {
      toast.style.animation = 'slideOut 0.3s ease-in forwards';
      setTimeout(() => {
        container.removeChild(toast);
        if (container.children.length === 0) {
          document.body.removeChild(container);
          this.container = null;
        }
      }, 300);
    }, duration);
  }

  success(message: string, duration?: number) {
    this.show({ message, type: 'success', duration });
  }

  error(message: string, duration?: number) {
    this.show({ message, type: 'error', duration });
  }

  info(message: string, duration?: number) {
    this.show({ message, type: 'info', duration });
  }

  warning(message: string, duration?: number) {
    this.show({ message, type: 'warning', duration });
  }
}

export const message = new ToastManager();

// Add animations to CSS
if (typeof document !== 'undefined') {
  const style = document.createElement('style');
  style.textContent = `
    @keyframes slideIn {
      from {
        transform: translateX(100%);
        opacity: 0;
      }
      to {
        transform: translateX(0);
        opacity: 1;
      }
    }
    @keyframes slideOut {
      from {
        transform: translateX(0);
        opacity: 1;
      }
      to {
        transform: translateX(100%);
        opacity: 0;
      }
    }
  `;
  document.head.appendChild(style);
}
