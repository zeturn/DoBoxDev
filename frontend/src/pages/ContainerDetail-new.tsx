import { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { ArrowLeft, Play, Square, Trash2, RefreshCw } from 'lucide-react';
import type { Container } from '../types';
import api from '../services/api';
import { Card, Tag } from '../components/Card';
import { Button } from '../components/Button';
import { message } from '../utils/message';

const ContainerDetail = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [container, setContainer] = useState<Container | null>(null);
  const [loading, setLoading] = useState(false);

  const fetchContainer = async () => {
    if (!id) return;
    setLoading(true);
    try {
      const data = await api.getContainer(parseInt(id));
      setContainer(data);
    } catch (error: any) {
      message.error('获取容器详情失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchContainer();
  }, [id]);

  const handleStart = async () => {
    if (!container) return;
    try {
      await api.startContainer(container.id);
      message.success('容器启动成功');
      fetchContainer();
    } catch (error: any) {
      message.error(error.response?.data?.error || '启动失败');
    }
  };

  const handleStop = async () => {
    if (!container) return;
    try {
      await api.stopContainer(container.id);
      message.success('容器停止成功');
      fetchContainer();
    } catch (error: any) {
      message.error(error.response?.data?.error || '停止失败');
    }
  };

  const handleDelete = async () => {
    if (!container) return;
    try {
      await api.deleteContainer(container.id);
      message.success('容器删除成功');
      navigate('/');
    } catch (error: any) {
      message.error(error.response?.data?.error || '删除失败');
    }
  };

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      running: { color: 'green', text: '运行中' },
      exited: { color: 'default', text: '已停止' },
      paused: { color: 'orange', text: '已暂停' },
      created: { color: 'blue', text: '已创建' },
    };
    const statusInfo = statusMap[status] || { color: 'default', text: status };
    return <Tag color={statusInfo.color}>{statusInfo.text}</Tag>;
  };

  if (loading || !container) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center">
          <div className="inline-block w-12 h-12 border-4 border-primary-500 border-t-transparent rounded-full animate-spin"></div>
          <p className="mt-4 text-neutral-600">加载中...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <Button variant="ghost" icon={<ArrowLeft className="w-4 h-4" />} onClick={() => navigate('/')}>
          返回列表
        </Button>
        <div className="flex gap-2">
          <Button variant="secondary" icon={<RefreshCw className="w-4 h-4" />} onClick={fetchContainer}>刷新</Button>
          {container.status === 'running' && (
            <Button variant="secondary" icon={<Square className="w-4 h-4" />} onClick={handleStop}>停止</Button>
          )}
          {(container.status === 'exited' || container.status === 'created') && (
            <Button variant="primary" icon={<Play className="w-4 h-4" />} onClick={handleStart}>启动</Button>
          )}
          <Button variant="danger" icon={<Trash2 className="w-4 h-4" />} onClick={handleDelete}>删除</Button>
        </div>
      </div>

      <Card title={`容器: ${container.name}`} extra={getStatusTag(container.status)}>
        <div className="grid grid-cols-2 gap-6">
          <div>
            <p className="text-sm text-neutral-500 mb-1">容器ID</p>
            <code className="text-sm bg-neutral-100 px-2 py-1 rounded">{container.container_id?.slice(0, 12)}</code>
          </div>
          <div>
            <p className="text-sm text-neutral-500 mb-1">镜像</p>
            <p className="text-sm font-medium">{container.image}</p>
          </div>
          <div>
            <p className="text-sm text-neutral-500 mb-1">CPU限制</p>
            <p className="text-sm">{container.cpu_limit > 0 ? `${container.cpu_limit} 核` : '无限制'}</p>
          </div>
          <div>
            <p className="text-sm text-neutral-500 mb-1">内存限制</p>
            <p className="text-sm">
              {container.memory_limit > 0 ? `${(container.memory_limit / 1024 / 1024).toFixed(0)} MB` : '无限制'}
            </p>
          </div>
          <div className="col-span-2">
            <p className="text-sm text-neutral-500 mb-1">创建时间</p>
            <p className="text-sm">{new Date(container.created_at).toLocaleString('zh-CN')}</p>
          </div>
        </div>
      </Card>
    </div>
  );
};

export default ContainerDetail;
