import type { ReactNode } from 'react';
import { useNavigate, useLocation } from 'react-router-dom';
import { Container, Image, Network, LogOut, User } from 'lucide-react';
import { useAuth } from '../../hooks/useAuth';
import ThemeToggle from '../ThemeToggle';

interface MainLayoutProps {
  children: ReactNode;
}

const MainLayout = ({ children }: MainLayoutProps) => {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const menuItems = [
    {
      key: '/',
      icon: Container,
      label: '容器列表',
      path: '/',
    },
    {
      key: '/images',
      icon: Image,
      label: '镜像管理',
      path: '/images',
    },
    {
      key: '/infrastructure',
      icon: Network,
      label: '网络与卷',
      path: '/infrastructure',
    },
  ];

  return (
    <div className="min-h-screen flex flex-col bg-gradient-to-br from-neutral-50 via-white to-primary-50/40 dark:from-neutral-950 dark:via-neutral-950 dark:to-primary-950/30">
      {/* Header */}
      <header className="bg-white/95 backdrop-blur-sm border-b border-neutral-200 sticky top-0 z-30 dark:bg-neutral-900/95 dark:border-neutral-800">
        <div className="px-8 py-3.5 flex items-center justify-between gap-6">
          <div className="flex items-center gap-10">
            {/* Logo */}
            <div className="flex items-center gap-2.5">
              <Container className="w-8 h-8 text-primary-500" />
              <span className="text-[28px] leading-none font-bold text-neutral-800 dark:text-neutral-100">Docker 沙盒管理</span>
            </div>

            {/* Navigation */}
            <nav className="flex items-center gap-2">
              {menuItems.map((item) => {
                const Icon = item.icon;
                const isActive = location.pathname === item.path;
                return (
                  <button
                    key={item.key}
                    onClick={() => navigate(item.path)}
                    className={`
                      flex items-center gap-2 px-4 py-2.5 rounded-lg
                      transition-colors duration-200 font-medium bg-transparent
                      ${isActive 
                        ? '!bg-primary-600 !text-white border border-primary-600' 
                        : 'text-neutral-600 hover:bg-neutral-100 border border-transparent dark:text-neutral-300 dark:hover:bg-neutral-800'
                      }
                    `}
                  >
                    <Icon className="w-4 h-4" />
                    <span>{item.label}</span>
                  </button>
                );
              })}
            </nav>
          </div>

          {/* User Menu */}
          <div className="flex items-center gap-2.5">
            <ThemeToggle />
            <div className="flex items-center gap-2 px-3 py-1.5 bg-neutral-100 rounded-lg border border-neutral-200 dark:bg-neutral-800 dark:border-neutral-700">
              <div className="w-8 h-8 rounded-full bg-primary-100 border border-primary-200 flex items-center justify-center dark:bg-primary-900/40 dark:border-primary-800">
                <User className="w-4 h-4 text-primary-600 dark:text-primary-300" />
              </div>
                <span className="text-sm font-medium text-neutral-700 dark:text-neutral-200">{user?.username || '用户'}</span>
            </div>
            <button
              onClick={handleLogout}
              className="flex items-center gap-2 px-3.5 py-2 text-neutral-600 hover:text-red-600 hover:bg-red-50 rounded-lg border border-transparent hover:border-red-200 transition-colors duration-200 dark:text-neutral-300 dark:hover:text-red-400 dark:hover:bg-red-950/40 dark:hover:border-red-900"
            >
              <LogOut className="w-4 h-4" />
              <span className="text-sm font-medium">退出</span>
            </button>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1">
        {children}
      </main>

      {/* Footer */}
      <footer className="bg-white/90 border-t border-neutral-200 py-4 dark:bg-neutral-900/90 dark:border-neutral-800">
        <div className="px-6 text-center text-sm text-neutral-500 dark:text-neutral-400">
          Docker 沙盒管理工具 ©{new Date().getFullYear()} - 基于 Go Fiber + React
        </div>
      </footer>
    </div>
  );
};

export default MainLayout;
