import { useEffect, useState } from 'react';
import { Eye, RefreshCw, Trash2, Link2 } from 'lucide-react';
import type { NetworkSummary, VolumeSummary } from '../types';
import api from '../services/api';
import { Card, Tag } from '../components/Card';
import { Table, type Column } from '../components/Table';
import { Button, IconButton } from '../components/Button';
import { Modal, ConfirmModal } from '../components/Modal';
import { message } from '../utils/message';

const NetworkVolume = () => {
  const [networks, setNetworks] = useState<NetworkSummary[]>([]);
  const [volumes, setVolumes] = useState<VolumeSummary[]>([]);
  const [loading, setLoading] = useState(false);
  const [inspectOpen, setInspectOpen] = useState(false);
  const [inspectTitle, setInspectTitle] = useState('');
  const [inspectData, setInspectData] = useState<any>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [pendingDelete, setPendingDelete] = useState<{ type: 'network' | 'volume'; id: string; name: string } | null>(null);

  const loadAll = async () => {
    setLoading(true);
    try {
      const [n, v] = await Promise.all([api.listNetworks(), api.listVolumes()]);
      setNetworks(n || []);
      setVolumes(v || []);
    } catch (e: any) {
      message.error(e.response?.data?.error || '加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { loadAll(); }, []);

  const formatDate = (value?: string | null) => {
    if (!value) return '-';
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? '-' : date.toLocaleString('zh-CN');
  };

  const handleInspectNetwork = async (network: NetworkSummary) => {
    try {
      const data = await api.inspectNetwork(network.id);
      setInspectTitle(`网络详情：${network.name}`);
      setInspectData(data);
      setInspectOpen(true);
    } catch (e: any) {
      message.error(e.response?.data?.error || '获取网络详情失败');
    }
  };

  const handleInspectVolume = async (volume: VolumeSummary) => {
    try {
      const data = await api.inspectVolume(volume.name);
      setInspectTitle(`卷详情：${volume.name}`);
      setInspectData(data);
      setInspectOpen(true);
    } catch (e: any) {
      message.error(e.response?.data?.error || '获取卷详情失败');
    }
  };

  const handleVolumeRelations = async (volume: VolumeSummary) => {
    try {
      const data = await api.getVolumeRelations(volume.name);
      setInspectTitle(`卷挂载关系：${volume.name}`);
      setInspectData(data);
      setInspectOpen(true);
    } catch (e: any) {
      message.error(e.response?.data?.error || '获取卷挂载关系失败');
    }
  };

  const askDeleteNetwork = (network: NetworkSummary) => {
    setPendingDelete({ type: 'network', id: network.id, name: network.name });
    setConfirmOpen(true);
  };

  const askDeleteVolume = (volume: VolumeSummary) => {
    setPendingDelete({ type: 'volume', id: volume.name, name: volume.name });
    setConfirmOpen(true);
  };

  const handleConfirmDelete = async () => {
    if (!pendingDelete) return;
    try {
      if (pendingDelete.type === 'network') {
        await api.deleteNetwork(pendingDelete.id);
        message.success(`网络 ${pendingDelete.name} 已删除`);
      } else {
        await api.deleteVolume(pendingDelete.id, true);
        message.success(`卷 ${pendingDelete.name} 已删除`);
      }
      setConfirmOpen(false);
      setPendingDelete(null);
      await loadAll();
    } catch (e: any) {
      message.error(e.response?.data?.error || '删除失败');
    }
  };

  const networkColumns: Column<NetworkSummary>[] = [
    { key: 'id', title: '网络ID', dataIndex: 'id', render: (v: string) => <code className="text-xs bg-neutral-100 px-2 py-1 rounded dark:bg-neutral-800 dark:text-neutral-300">{v.slice(0, 12)}</code> },
    { key: 'name', title: '名称', dataIndex: 'name', render: (text: string) => <strong>{text}</strong> },
    { key: 'driver', title: '驱动', dataIndex: 'driver', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { key: 'scope', title: '范围', dataIndex: 'scope' },
    { key: 'created', title: '创建时间', dataIndex: 'created', render: (v: string) => formatDate(v) },
    {
      key: 'actions',
      title: '操作',
      render: (_: unknown, row: NetworkSummary) => (
        <div className="flex items-center gap-1">
          <IconButton icon={<Eye className="w-4 h-4" />} tooltip="查看详情" onClick={() => handleInspectNetwork(row)} />
          <IconButton icon={<Trash2 className="w-4 h-4" />} tooltip="删除网络" variant="danger" onClick={() => askDeleteNetwork(row)} />
        </div>
      ),
    },
  ];

  const volumeColumns: Column<VolumeSummary>[] = [
    { key: 'name', title: '名称', dataIndex: 'name', render: (text: string) => <strong>{text}</strong> },
    { key: 'driver', title: '驱动', dataIndex: 'driver', render: (v: string) => <Tag color="blue">{v}</Tag> },
    { key: 'mountpoint', title: '挂载点', dataIndex: 'mountpoint', render: (v: string) => <code className="text-xs bg-neutral-100 px-2 py-1 rounded dark:bg-neutral-800 dark:text-neutral-300">{v}</code> },
    { key: 'created_at', title: '创建时间', dataIndex: 'created_at', render: (v: string) => formatDate(v) },
    {
      key: 'actions',
      title: '操作',
      render: (_: unknown, row: VolumeSummary) => (
        <div className="flex items-center gap-1">
          <IconButton icon={<Eye className="w-4 h-4" />} tooltip="查看详情" onClick={() => handleInspectVolume(row)} />
          <IconButton icon={<Link2 className="w-4 h-4" />} tooltip="挂载关系" onClick={() => handleVolumeRelations(row)} />
          <IconButton icon={<Trash2 className="w-4 h-4" />} tooltip="删除卷" variant="danger" onClick={() => askDeleteVolume(row)} />
        </div>
      ),
    },
  ];

  return (
    <div className="p-6 sm:p-8 space-y-6">
      <Card title="Docker 网络" extra={<Button variant="secondary" size="sm" icon={<RefreshCw className="w-4 h-4" />} onClick={loadAll}>刷新</Button>}>
        <Table rowKey="id" loading={loading} columns={networkColumns} dataSource={networks} />
      </Card>
      <Card title="Docker 卷">
        <Table rowKey="name" loading={loading} columns={volumeColumns} dataSource={volumes} />
      </Card>

      <Modal open={inspectOpen} onClose={() => setInspectOpen(false)} title={inspectTitle || '详情'} width="max-w-4xl">
        <pre className="bg-neutral-900 text-green-400 p-4 rounded-lg max-h-[60vh] overflow-auto text-xs font-mono">
          {inspectData ? JSON.stringify(inspectData, null, 2) : ''}
        </pre>
      </Modal>

      <ConfirmModal
        open={confirmOpen}
        onConfirm={handleConfirmDelete}
        onCancel={() => {
          setConfirmOpen(false);
          setPendingDelete(null);
        }}
        title={pendingDelete?.type === 'network' ? '删除网络' : '删除卷'}
        message={`确定删除${pendingDelete?.type === 'network' ? '网络' : '卷'} "${pendingDelete?.name || ''}" 吗？此操作不可恢复。`}
      />
    </div>
  );
};

export default NetworkVolume;
