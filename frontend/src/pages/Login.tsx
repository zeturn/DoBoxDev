import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { Container as ContainerIcon, AlertCircle } from 'lucide-react';
import { useAuth } from '../hooks/useAuth';
import { Input, Button, Card } from '@zeturn/watercolor-react';
import ThemeToggle from '../components/ThemeToggle';

const Login = () => {
  const [loading, setLoading] = useState(false);
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [usernameError, setUsernameError] = useState(false);
  const [passwordError, setPasswordError] = useState(false);
  const { login } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setUsernameError(false);
    setPasswordError(false);
    
    if (!username || !password) {
      setError('请填写所有字段');
      if (!username) setUsernameError(true);
      if (!password) setPasswordError(true);
      return;
    }

    setLoading(true);
    try {
      await login(username, password);
      navigate('/');
    } catch (error: any) {
      setError(error.response?.data?.error || '登录失败，请检查用户名和密码');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="relative min-h-screen flex items-center justify-center bg-gradient-to-br from-primary-50 via-neutral-50 to-primary-100 p-6 sm:p-8 dark:from-primary-950 dark:via-neutral-950 dark:to-neutral-900">
      <div className="absolute top-4 right-4">
        <ThemeToggle />
      </div>
      <div className="w-full max-w-md">
        {/* Header */}
        <div className="text-center mb-6">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-primary-500 rounded-2xl mb-4 shadow-lg shadow-primary-500/30">
            <ContainerIcon className="w-10 h-10 text-white" />
          </div>
          <h1 className="text-3xl font-bold text-neutral-800 dark:text-neutral-100">Docker 沙盒管理</h1>
          <p className="text-neutral-600 mt-1 dark:text-neutral-300">登录到您的账户</p>
        </div>

        {/* Login Card */}
        <Card 
          variant="elevated" 
          color="default"
          className="p-6 sm:p-7 rounded-2xl"
          interactive={false}
        >
          <form onSubmit={handleSubmit} className="space-y-4">
            {/* Error Message */}
            {error && (
              <div className="flex items-start space-x-2 p-4 bg-red-50 border border-red-200 rounded-lg text-red-700 text-sm dark:bg-red-950/40 dark:border-red-900 dark:text-red-300">
                <AlertCircle className="w-5 h-5 flex-shrink-0 mt-0.5" />
                <span>{error}</span>
              </div>
            )}

            {/* Username Input */}
            <div className="space-y-1.5">
              <Input
                label="用户名"
                type="text"
                value={username}
                onChange={(e: any) => setUsername(e.target.value)}
                placeholder="请输入用户名"
                fullWidth
                variant="outlined"
                error={usernameError}
                helperText={usernameError ? '请输入用户名' : undefined}
                maxLength={50}
                minLength={1}
              />
            </div>

            {/* Password Input */}
            <div className="space-y-1.5">
              <Input
                label="密码"
                type="password"
                value={password}
                onChange={(e: any) => setPassword(e.target.value)}
                placeholder="请输入密码"
                fullWidth
                variant="outlined"
                error={passwordError}
                helperText={passwordError ? '请输入密码' : undefined}
                maxLength={100}
                minLength={1}
              />
            </div>

            {/* Submit Button */}
            <Button
              type="submit"
              variant="primary"
              buttonStyle="filled"
              size="lg"
              fullWidth
              loading={loading}
              disabled={loading}
              className="mt-1"
            >
              {loading ? '登录中...' : '登录'}
            </Button>

            {/* Register Link */}
            <div className="text-center pt-1">
              <span className="text-neutral-600 text-sm">还没有账户？ </span>
              <Link 
                to="/register" 
                className="text-primary-600 hover:text-primary-700 font-medium text-sm transition-colors"
              >
                立即注册
              </Link>
            </div>
          </form>
        </Card>
      </div>
    </div>
  );
};

export default Login;
