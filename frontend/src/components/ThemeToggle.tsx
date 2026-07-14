import { Moon, Sun } from 'lucide-react';
import { useTheme } from '../theme';

export default function ThemeToggle({ className = '' }: { className?: string }) {
  const { theme, toggle } = useTheme();
  const isDark = theme === 'dark';
  return (
    <button
      type="button"
      onClick={toggle}
      title={isDark ? '切换到浅色模式' : '切换到暗色模式'}
      aria-label="切换主题"
      className={
        'inline-flex items-center justify-center w-9 h-9 rounded-lg border border-neutral-200 bg-white text-neutral-600 ' +
        'hover:bg-neutral-100 hover:text-neutral-900 transition-colors ' +
        'dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-300 dark:hover:bg-neutral-700 dark:hover:text-white ' +
        className
      }
    >
      {isDark ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
    </button>
  );
}
