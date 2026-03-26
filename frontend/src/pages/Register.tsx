import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import { Container as ContainerIcon, AlertCircle } from 'lucide-react';
import { useAuth } from '../hooks/useAuth';
import { Input, Button, Card } from '@zeturn/watercolor-react';

const Register = () => {
  const [loading, setLoading] = useState(false);
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [errors, setErrors] = useState({ username: false, email: false, password: false });
  const { register } = useAuth();
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setErrors({ username: false, email: false, password: false });
    
    const newErrors = {
      username: !username,
      email: !email,
      password: !password
    };

    if (!username || !email || !password) {
      setError('请填写所有字段');
      setErrors(newErrors);
      return;
    }

    if (password.length < 6) {
      setError('密码至少需要6个字符');
      setErrors({ ...errors, password: true });
      return;
    }

    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      setError('请输入有效的邮箱地址');
      setErrors({ ...errors, email: true });
      return;
    }

    setLoading(true);
    try {
      await register(username, email, password);
      navigate('/');
    } catch (error: any) {
      setError(error.response?.data?.error || '注册失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-gradient-to-br from-primary-50 via-neutral-50 to-primary-100 p-6 sm:p-8">
      <div className="w-full max-w-md">
        {/* Header */}
        <div className="text-center mb-6">
          <div className="inline-flex items-center justify-center w-16 h-16 bg-primary-500 rounded-2xl mb-4">
            <ContainerIcon className="w-10 h-10 text-white" />
          </div>
          <h1 className="text-3xl font-bold text-neutral-800">Docker 沙盒管理</h1>
          <p className="text-neutral-600 mt-1">创建新账户</p>
        </div>

        {/* Register Card */}
        <Card 
          variant="elevated" 
          color="default"
          className="p-6 sm:p-7 rounded-2xl"
          interactive={false}
        >
          <form onSubmit={handleSubmit} className="space-y-4">
            {/* Error Message */}
            {error && (
              <div className="flex items-start space-x-2 p-4 bg-red-50 border border-red-200 rounded-lg text-red-700 text-sm">
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
                error={errors.username}
                helperText={errors.username ? '请输入用户名' : undefined}
                maxLength={50}
                minLength={1}
              />
            </div>

            {/* Email Input */}
            <div className="space-y-1.5">
              <Input
                label="邮箱"
                type="email"
                value={email}
                onChange={(e: any) => setEmail(e.target.value)}
                placeholder="请输入邮箱"
                fullWidth
                variant="outlined"
                error={errors.email}
                helperText={errors.email ? '请输入有效的邮箱地址' : undefined}
                maxLength={100}
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
                placeholder="请输入密码（至少6个字符）"
                fullWidth
                variant="outlined"
                error={errors.password}
                helperText={errors.password ? '密码至少需要6个字符' : undefined}
                maxLength={100}
                minLength={6}
              />
            </div>

            {/* Submit Button */}
            <Button
              type="submit"
              variant="success"
              buttonStyle="filled"
              size="lg"
              fullWidth
              loading={loading}
              disabled={loading}
              className="mt-1"
            >
              {loading ? '注册中...' : '注册'}
            </Button>

            {/* Login Link */}
            <div className="text-center pt-1">
              <span className="text-neutral-600 text-sm">已有账户？ </span>
              <Link 
                to="/login" 
                className="text-primary-600 hover:text-primary-700 font-medium text-sm transition-colors"
              >
                立即登录
              </Link>
            </div>
          </form>
        </Card>
      </div>
    </div>
  );
};

export default Register;
