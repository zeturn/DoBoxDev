import { useEffect, useState } from 'react';
import {
  Trash2,
  Eye,
  Plus,
  Upload,
  Wrench,
  Tag as TagIcon,
  RefreshCw,
} from 'lucide-react';
import type { ImageSummary } from '../types';
import api from '../services/api';
import { Card, Tag } from '../components/Card';
import { Table, type Column } from '../components/Table';
import { Button } from '../components/Button';
import { Modal, ConfirmModal } from '../components/Modal';
import { FormItem, Input, TextArea } from '../components/Form';
import { message } from '../utils/message';

const formatBytes = (bytes: number) => {
  if (!bytes) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
};

const ImageList = () => {
  const [images, setImages] = useState<ImageSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [pullVisible, setPullVisible] = useState(false);
  const [tagVisible, setTagVisible] = useState(false);
  const [pushVisible, setPushVisible] = useState(false);
  const [buildVisible, setBuildVisible] = useState(false);
  const [inspectVisible, setInspectVisible] = useState(false);
  const [inspectData, setInspectData] = useState<any>(null);
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);
  const [imageToDelete, setImageToDelete] = useState<string>('');

  // Form states
  const [pullImage, setPullImage] = useState('');
  const [tagSource, setTagSource] = useState('');
  const [tagTarget, setTagTarget] = useState('');
  const [pushImage, setPushImage] = useState('');
  const [pushUsername, setPushUsername] = useState('');
  const [pushPassword, setPushPassword] = useState('');
  const [pushServer, setPushServer] = useState('');
  const [buildContext, setBuildContext] = useState('.');
  const [buildDockerfile, setBuildDockerfile] = useState('Dockerfile');
  const [buildTag, setBuildTag] = useState('');
  const [buildArgs, setBuildArgs] = useState('');
  const [buildNoCache, setBuildNoCache] = useState(false);

  const fetchImages = async () => {
    setLoading(true);
    try {
      const data = await api.listImages();
      setImages(data);
    } catch (e: any) {
      message.error(e.response?.data?.error || '获取镜像列表失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchImages();
  }, []);

  const handlePull = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!pullImage) {
      message.error('请输入镜像名');
      return;
    }
    try {
      await api.pullImage(pullImage);
      message.success('拉取镜像成功');
      setPullVisible(false);
      setPullImage('');
      fetchImages();
    } catch (e: any) {
      message.error(e.response?.data?.error || '拉取失败');
    }
  };

  const handleTag = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!tagSource || !tagTarget) {
      message.error('请填写完整');
      return;
    }
    try {
      await api.tagImage(tagSource, tagTarget);
      message.success('打 Tag 成功');
      setTagVisible(false);
      setTagSource('');
      setTagTarget('');
      fetchImages();
    } catch (e: any) {
      message.error(e.response?.data?.error || '打 Tag 失败');
    }
  };

  const handlePush = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!pushImage) {
      message.error('请输入镜像名');
      return;
    }
    try {
      await api.pushImage(pushImage, pushUsername, pushPassword, pushServer);
      message.success('推送请求已完成');
      setPushVisible(false);
      setPushImage('');
      setPushUsername('');
      setPushPassword('');
      setPushServer('');
    } catch (e: any) {
      message.error(e.response?.data?.error || '推送失败');
    }
  };

  const handleBuild = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!buildContext || !buildDockerfile || !buildTag) {
      message.error('请填写必填项');
      return;
    }
    try {
      const argsObj: Record<string, string> = {};
      if (buildArgs) {
        buildArgs.split('\n').forEach((line) => {
          const [k, ...rest] = line.split('=');
          if (k?.trim()) argsObj[k.trim()] = rest.join('=').trim();
        });
      }
      await api.buildImage({
        context_path: buildContext,
        dockerfile: buildDockerfile,
        tag: buildTag,
        build_args: argsObj,
        no_cache: buildNoCache,
      });
      message.success('构建镜像成功');
      setBuildVisible(false);
      setBuildContext('.');
      setBuildDockerfile('Dockerfile');
      setBuildTag('');
      setBuildArgs('');
      setBuildNoCache(false);
      fetchImages();
    } catch (e: any) {
      message.error(e.response?.data?.error || '构建失败');
    }
  };

  const handleInspect = async (ref: string) => {
    try {
      const data = await api.inspectImage(ref);
      setInspectData(data);
      setInspectVisible(true);
    } catch (e: any) {
      message.error(e.response?.data?.error || '获取详情失败');
    }
  };

  const handleDelete = async () => {
    if (!imageToDelete) return;
    try {
      await api.deleteImage(imageToDelete, true);
      message.success('删除镜像成功');
      setDeleteModalOpen(false);
      setImageToDelete('');
      fetchImages();
    } catch (e: any) {
      message.error(e.response?.data?.error || '删除失败');
    }
  };

  const columns: Column<ImageSummary>[] = [
    {
      key: 'id',
      title: '镜像ID',
      dataIndex: 'id',
      render: (v: string) => (
        <code className="text-xs bg-neutral-100 px-2 py-1 rounded">
          {v.replace('sha256:', '').slice(0, 12)}
        </code>
      ),
    },
    {
      key: 'repo_tags',
      title: 'Tags',
      dataIndex: 'repo_tags',
      render: (tags: string[]) => (
        <div className="flex flex-wrap gap-1">
          {(tags || ['<none>:<none>']).map((t) => (
            <Tag key={t}>{t}</Tag>
          ))}
        </div>
      ),
    },
    {
      key: 'size',
      title: '大小',
      dataIndex: 'size',
      render: (v: number) => formatBytes(v),
    },
    {
      key: 'created',
      title: '创建时间',
      dataIndex: 'created',
      render: (v: number) => new Date(v * 1000).toLocaleString('zh-CN'),
    },
    {
      key: 'actions',
      title: '操作',
      render: (_: any, row: ImageSummary) => {
        const ref = row.repo_tags?.[0] || row.id;
        return (
          <div className="flex gap-1">
            <button
              onClick={() => handleInspect(ref)}
              className="p-2 hover:bg-neutral-100 rounded-lg transition-colors"
              title="查看详情"
            >
              <Eye className="w-4 h-4 text-neutral-600" />
            </button>
            <button
              onClick={() => {
                setImageToDelete(ref);
                setDeleteModalOpen(true);
              }}
              className="p-2 hover:bg-red-50 rounded-lg transition-colors"
              title="删除"
            >
              <Trash2 className="w-4 h-4 text-red-600" />
            </button>
          </div>
        );
      },
    },
  ];

  return (
    <div className="p-6">
      <Card
        title="镜像管理"
        extra={
          <div className="flex gap-2">
            <Button
              variant="secondary"
              size="sm"
              icon={<RefreshCw className="w-4 h-4" />}
              onClick={fetchImages}
            >
              刷新
            </Button>
            <Button
              variant="secondary"
              size="sm"
              icon={<Plus className="w-4 h-4" />}
              onClick={() => setPullVisible(true)}
            >
              拉取镜像
            </Button>
            <Button
              variant="secondary"
              size="sm"
              icon={<TagIcon className="w-4 h-4" />}
              onClick={() => setTagVisible(true)}
            >
              打 Tag
            </Button>
            <Button
              variant="secondary"
              size="sm"
              icon={<Upload className="w-4 h-4" />}
              onClick={() => setPushVisible(true)}
            >
              推送镜像
            </Button>
            <Button
              variant="secondary"
              size="sm"
              icon={<Wrench className="w-4 h-4" />}
              onClick={() => setBuildVisible(true)}
            >
              构建镜像
            </Button>
          </div>
        }
      >
        <Table rowKey="id" loading={loading} columns={columns} dataSource={images} />
      </Card>

      {/* Pull Modal */}
      <Modal open={pullVisible} onClose={() => setPullVisible(false)} title="拉取镜像">
        <form onSubmit={handlePull}>
          <FormItem label="镜像名" required>
            <Input
              placeholder="nginx:alpine"
              value={pullImage}
              onChange={(e) => setPullImage(e.target.value)}
            />
          </FormItem>
          <div className="flex justify-end gap-2 pt-4">
            <Button variant="secondary" onClick={() => setPullVisible(false)}>
              取消
            </Button>
            <Button variant="primary" type="submit">
              拉取
            </Button>
          </div>
        </form>
      </Modal>

      {/* Tag Modal */}
      <Modal open={tagVisible} onClose={() => setTagVisible(false)} title="镜像打 Tag">
        <form onSubmit={handleTag}>
          <FormItem label="源镜像" required>
            <Input
              placeholder="nginx:alpine"
              value={tagSource}
              onChange={(e) => setTagSource(e.target.value)}
            />
          </FormItem>
          <FormItem label="目标 Tag" required>
            <Input
              placeholder="myrepo/nginx:prod"
              value={tagTarget}
              onChange={(e) => setTagTarget(e.target.value)}
            />
          </FormItem>
          <div className="flex justify-end gap-2 pt-4">
            <Button variant="secondary" onClick={() => setTagVisible(false)}>
              取消
            </Button>
            <Button variant="primary" type="submit">
              打 Tag
            </Button>
          </div>
        </form>
      </Modal>

      {/* Push Modal */}
      <Modal open={pushVisible} onClose={() => setPushVisible(false)} title="推送镜像">
        <form onSubmit={handlePush}>
          <FormItem label="镜像名" required>
            <Input
              placeholder="myrepo/myimage:tag"
              value={pushImage}
              onChange={(e) => setPushImage(e.target.value)}
            />
          </FormItem>
          <FormItem label="用户名">
            <Input value={pushUsername} onChange={(e) => setPushUsername(e.target.value)} />
          </FormItem>
          <FormItem label="密码">
            <Input
              type="password"
              value={pushPassword}
              onChange={(e) => setPushPassword(e.target.value)}
            />
          </FormItem>
          <FormItem label="仓库地址">
            <Input
              placeholder="https://index.docker.io/v1/"
              value={pushServer}
              onChange={(e) => setPushServer(e.target.value)}
            />
          </FormItem>
          <div className="flex justify-end gap-2 pt-4">
            <Button variant="secondary" onClick={() => setPushVisible(false)}>
              取消
            </Button>
            <Button variant="primary" type="submit">
              推送
            </Button>
          </div>
        </form>
      </Modal>

      {/* Build Modal */}
      <Modal open={buildVisible} onClose={() => setBuildVisible(false)} title="构建镜像">
        <form onSubmit={handleBuild}>
          <FormItem label="构建上下文路径" required>
            <Input
              value={buildContext}
              onChange={(e) => setBuildContext(e.target.value)}
            />
          </FormItem>
          <FormItem label="Dockerfile 路径(相对上下文)" required>
            <Input
              value={buildDockerfile}
              onChange={(e) => setBuildDockerfile(e.target.value)}
            />
          </FormItem>
          <FormItem label="镜像Tag" required>
            <Input
              placeholder="myapp:latest"
              value={buildTag}
              onChange={(e) => setBuildTag(e.target.value)}
            />
          </FormItem>
          <FormItem label="Build Args" extra="每行 KEY=VALUE">
            <TextArea
              rows={3}
              value={buildArgs}
              onChange={(e) => setBuildArgs(e.target.value)}
            />
          </FormItem>
          <FormItem label="不使用缓存">
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={buildNoCache}
                onChange={(e) => setBuildNoCache(e.target.checked)}
                className="w-4 h-4 text-primary-600 border-neutral-300 rounded focus:ring-primary-500"
              />
              <span className="text-sm text-neutral-700">禁用构建缓存</span>
            </label>
          </FormItem>
          <div className="flex justify-end gap-2 pt-4">
            <Button variant="secondary" onClick={() => setBuildVisible(false)}>
              取消
            </Button>
            <Button variant="primary" type="submit">
              构建
            </Button>
          </div>
        </form>
      </Modal>

      {/* Inspect Modal */}
      <Modal
        open={inspectVisible}
        onClose={() => setInspectVisible(false)}
        title="镜像详情"
        width="max-w-4xl"
      >
        <pre className="bg-neutral-900 text-green-400 p-4 rounded-lg max-h-[60vh] overflow-auto text-xs font-mono">
          {inspectData ? JSON.stringify(inspectData, null, 2) : ''}
        </pre>
      </Modal>

      {/* Delete Confirmation */}
      <ConfirmModal
        open={deleteModalOpen}
        onConfirm={handleDelete}
        onCancel={() => {
          setDeleteModalOpen(false);
          setImageToDelete('');
        }}
        title="删除镜像"
        message="确定删除此镜像吗？此操作不可恢复。"
      />
    </div>
  );
};

export default ImageList;
