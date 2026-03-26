import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Play,
  Pause,
  Square,
  RotateCw,
  Trash2,
  Plus,
  Eye,
  RefreshCw,
} from 'lucide-react';
import type { Container } from '../types';
import api from '../services/api';
import { Card, Tag } from '../components/Card';
import { Table, type Column } from '../components/Table';
import { Button, IconButton } from '../components/Button';
import { Modal, ConfirmModal } from '../components/Modal';
import { FormItem, Input, TextArea, InputNumber } from '../components/Form';
import { message } from '../utils/message';

const ContainerList = () => {
  const [containers, setContainers] = useState<Container[]>([]);
  const [loading, setLoading] = useState(false);
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);
  const [containerToDelete, setContainerToDelete] = useState<number | null>(null);
  const navigate = useNavigate();

  // Form state
  const [formData, setFormData] = useState({
    name: '',
    image: '',
    container_port: '',
    host_port: '',
    env: '',
    volumes: '',
    command: '',
    working_dir: '',
    network_mode: 'bridge',
    restart_policy: 'unless-stopped',
    cpu_limit: 0,
    memory_limit: 0,
  });

  const fetchContainers = async () => {
    setLoading(true);
    try {
      const data = await api.listContainers();
      setContainers(data);
    } catch (error: any) {
      message.error('获取容器列表失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchContainers();
    const interval = setInterval(fetchContainers, 5000);
    return () => clearInterval(interval);
  }, []);

  const handleStart = async (id: number) => {
    try {
      await api.startContainer(id);
      message.success('容器启动成功');
      fetchContainers();
    } catch (error: any) {
      message.error(error.response?.data?.error || '启动失败');
    }
  };

  const handleStop = async (id: number) => {
    try {
      await api.stopContainer(id);
      message.success('容器停止成功');
      fetchContainers();
    } catch (error: any) {
      message.error(error.response?.data?.error || '停止失败');
    }
  };

  const handlePause = async (id: number) => {
    try {
      await api.pauseContainer(id);
      message.success('容器暂停成功');
      fetchContainers();
    } catch (error: any) {
      message.error(error.response?.data?.error || '暂停失败');
    }
  };

  const handleUnpause = async (id: number) => {
    try {
      await api.unpauseContainer(id);
      message.success('容器恢复成功');
      fetchContainers();
    } catch (error: any) {
      message.error(error.response?.data?.error || '恢复失败');
    }
  };

  const handleDelete = async () => {
    if (!containerToDelete) return;
    try {
      await api.deleteContainer(containerToDelete);
      message.success('容器删除成功');
      setDeleteModalOpen(false);
      setContainerToDelete(null);
      fetchContainers();
    } catch (error: any) {
      message.error(error.response?.data?.error || '删除失败');
    }
  };

  const handleRestart = async (id: number) => {
    try {
      await api.restartContainer(id);
      message.success('容器重启成功');
      fetchContainers();
    } catch (error: any) {
      message.error(error.response?.data?.error || '重启失败');
    }
  };

  const handleCreateContainer = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!formData.name || !formData.image) {
      message.error('请填写必填项');
      return;
    }

    setCreateLoading(true);
    try {
      const ports: Record<string, string> = {};
      if (formData.container_port && formData.host_port) {
        ports[formData.container_port] = formData.host_port;
      }

      const env: string[] = [];
      if (formData.env) {
        env.push(...formData.env.split('\n').filter((line: string) => line.trim()));
      }

      const volumes: string[] = [];
      if (formData.volumes) {
        volumes.push(...formData.volumes.split('\n').filter((line: string) => line.trim()));
      }

      const command: string[] = formData.command
        ? formData.command.split(' ').filter((part: string) => part.trim())
        : [];

      await api.createContainer({
        name: formData.name,
        image: formData.image,
        ports,
        env,
        volumes,
        command,
        working_dir: formData.working_dir || '',
        restart_policy: formData.restart_policy || 'unless-stopped',
        network_mode: formData.network_mode || 'bridge',
        cpu_limit: formData.cpu_limit || 0,
        memory_limit: formData.memory_limit ? formData.memory_limit * 1024 * 1024 : 0,
      });

      message.success('容器创建成功');
      setCreateModalVisible(false);
      setFormData({
        name: '',
        image: '',
        container_port: '',
        host_port: '',
        env: '',
        volumes: '',
        command: '',
        working_dir: '',
        network_mode: 'bridge',
        restart_policy: 'unless-stopped',
        cpu_limit: 0,
        memory_limit: 0,
      });
      fetchContainers();
    } catch (error: any) {
      message.error(error.response?.data?.error || '创建失败');
    } finally {
      setCreateLoading(false);
    }
  };

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      running: { color: 'green', text: '运行中' },
      exited: { color: 'default', text: '已停止' },
      paused: { color: 'orange', text: '已暂停' },
      created: { color: 'blue', text: '已创建' },
      restarting: { color: 'gold', text: '重启中' },
      dead: { color: 'red', text: '异常' },
    };

    const statusInfo = statusMap[status] || { color: 'default', text: status };
    return <Tag color={statusInfo.color}>{statusInfo.text}</Tag>;
  };

  const columns: Column<Container>[] = [
    {
      key: 'name',
      title: '名称',
      dataIndex: 'name',
      render: (text: string) => <strong className="text-neutral-800">{text}</strong>,
    },
    {
      key: 'image',
      title: '镜像',
      dataIndex: 'image',
    },
    {
      key: 'status',
      title: '状态',
      dataIndex: 'status',
      render: (status: string) => getStatusTag(status),
    },
    {
      key: 'cpu_limit',
      title: 'CPU限制',
      dataIndex: 'cpu_limit',
      render: (limit: number) => (limit > 0 ? `${limit} 核` : '-'),
    },
    {
      key: 'memory_limit',
      title: '内存限制',
      dataIndex: 'memory_limit',
      render: (limit: number) =>
        limit > 0 ? `${(limit / 1024 / 1024).toFixed(0)} MB` : '-',
    },
    {
      key: 'created_at',
      title: '创建时间',
      dataIndex: 'created_at',
      render: (date: string) => new Date(date).toLocaleString('zh-CN'),
    },
    {
      key: 'actions',
      title: '操作',
      render: (_: any, record: Container) => (
        <div className="flex items-center gap-1">
          <IconButton
            icon={<Eye className="w-4 h-4" />}
            tooltip="查看详情"
            onClick={() => navigate(`/containers/${record.id}`)}
          />

          {record.status === 'running' && (
            <>
              <IconButton
                icon={<Pause className="w-4 h-4" />}
                tooltip="暂停"
                onClick={() => handlePause(record.id)}
              />
              <IconButton
                icon={<Square className="w-4 h-4" />}
                variant="danger"
                tooltip="停止"
                onClick={() => handleStop(record.id)}
              />
              <IconButton
                icon={<RotateCw className="w-4 h-4" />}
                tooltip="重启"
                onClick={() => handleRestart(record.id)}
              />
            </>
          )}

          {record.status === 'paused' && (
            <IconButton
              icon={<Play className="w-4 h-4" />}
              variant="primary"
              tooltip="恢复"
              onClick={() => handleUnpause(record.id)}
            />
          )}

          {(record.status === 'exited' || record.status === 'created') && (
            <IconButton
              icon={<Play className="w-4 h-4" />}
              variant="primary"
              tooltip="启动"
              onClick={() => handleStart(record.id)}
            />
          )}

          <IconButton
            icon={<Trash2 className="w-4 h-4" />}
            variant="danger"
            tooltip="删除"
            onClick={() => {
              setContainerToDelete(record.id);
              setDeleteModalOpen(true);
            }}
          />
        </div>
      ),
    },
  ];

  return (
    <div className="p-6">
      <Card
        title="我的容器"
        extra={
          <div className="flex gap-2">
            <Button
              variant="secondary"
              icon={<RefreshCw className="w-4 h-4" />}
              onClick={fetchContainers}
            >
              刷新
            </Button>
            <Button
              variant="primary"
              icon={<Plus className="w-4 h-4" />}
              onClick={() => setCreateModalVisible(true)}
            >
              创建容器
            </Button>
          </div>
        }
      >
        <Table
          columns={columns}
          dataSource={containers}
          loading={loading}
          rowKey="id"
        />
      </Card>

      {/* Create Container Modal */}
      <Modal
        open={createModalVisible}
        onClose={() => setCreateModalVisible(false)}
        title="创建新容器"
        width="max-w-3xl"
      >
        <form onSubmit={handleCreateContainer}>
          <div className="space-y-4">
            <FormItem label="容器名称" required>
              <Input
                placeholder="my-container"
                value={formData.name}
                onChange={(e) => setFormData({ ...formData, name: e.target.value })}
              />
            </FormItem>

            <FormItem label="镜像" required>
              <Input
                placeholder="nginx:latest"
                value={formData.image}
                onChange={(e) => setFormData({ ...formData, image: e.target.value })}
              />
            </FormItem>

            <div className="grid grid-cols-2 gap-4">
              <FormItem label="容器端口">
                <Input
                  placeholder="80"
                  value={formData.container_port}
                  onChange={(e) =>
                    setFormData({ ...formData, container_port: e.target.value })
                  }
                />
              </FormItem>

              <FormItem label="主机端口">
                <Input
                  placeholder="8080"
                  value={formData.host_port}
                  onChange={(e) =>
                    setFormData({ ...formData, host_port: e.target.value })
                  }
                />
              </FormItem>
            </div>

            <FormItem label="环境变量" extra="每行一个，格式: KEY=value">
              <TextArea
                rows={3}
                placeholder="KEY1=value1&#10;KEY2=value2"
                value={formData.env}
                onChange={(e) => setFormData({ ...formData, env: e.target.value })}
              />
            </FormItem>

            <FormItem
              label="挂载卷"
              extra="每行一个，格式: hostPath:containerPath[:ro|rw]"
            >
              <TextArea
                rows={2}
                placeholder="/host/data:/app/data&#10;/host/logs:/var/log:ro"
                value={formData.volumes}
                onChange={(e) =>
                  setFormData({ ...formData, volumes: e.target.value })
                }
              />
            </FormItem>

            <FormItem label="启动命令" extra="空格分隔参数">
              <Input
                placeholder="nginx -g 'daemon off;'"
                value={formData.command}
                onChange={(e) =>
                  setFormData({ ...formData, command: e.target.value })
                }
              />
            </FormItem>

            <div className="grid grid-cols-2 gap-4">
              <FormItem label="工作目录">
                <Input
                  placeholder="/app"
                  value={formData.working_dir}
                  onChange={(e) =>
                    setFormData({ ...formData, working_dir: e.target.value })
                  }
                />
              </FormItem>
              <FormItem label="网络模式">
                <Input
                  placeholder="bridge / host / none"
                  value={formData.network_mode}
                  onChange={(e) =>
                    setFormData({ ...formData, network_mode: e.target.value })
                  }
                />
              </FormItem>
            </div>

            <FormItem label="重启策略">
              <Input
                placeholder="no / always / unless-stopped / on-failure"
                value={formData.restart_policy}
                onChange={(e) =>
                  setFormData({ ...formData, restart_policy: e.target.value })
                }
              />
            </FormItem>

            <div className="grid grid-cols-2 gap-4">
              <FormItem label="CPU限制 (核)">
                <InputNumber
                  min={0}
                  max={32}
                  step={0.5}
                  value={formData.cpu_limit}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      cpu_limit: parseFloat(e.target.value) || 0,
                    })
                  }
                />
              </FormItem>

              <FormItem label="内存限制 (MB)">
                <InputNumber
                  min={0}
                  max={32768}
                  step={128}
                  value={formData.memory_limit}
                  onChange={(e) =>
                    setFormData({
                      ...formData,
                      memory_limit: parseInt(e.target.value) || 0,
                    })
                  }
                />
              </FormItem>
            </div>

            <div className="flex justify-end gap-3 pt-4">
              <Button
                variant="secondary"
                onClick={() => setCreateModalVisible(false)}
              >
                取消
              </Button>
              <Button variant="primary" type="submit" loading={createLoading}>
                创建
              </Button>
            </div>
          </div>
        </form>
      </Modal>

      {/* Delete Confirmation Modal */}
      <ConfirmModal
        open={deleteModalOpen}
        onConfirm={handleDelete}
        onCancel={() => {
          setDeleteModalOpen(false);
          setContainerToDelete(null);
        }}
        title="删除容器"
        message="确定要删除这个容器吗？此操作不可恢复。"
      />
    </div>
  );
};

export default ContainerList;
