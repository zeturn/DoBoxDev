import type { ReactNode } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';

export interface Column<T = any> {
  key: string;
  title: string;
  dataIndex?: string;
  render?: (value: any, record: T, index: number) => ReactNode;
  width?: string;
}

interface TableProps<T = any> {
  columns: Column<T>[];
  dataSource: T[];
  loading?: boolean;
  rowKey: string;
  pagination?: {
    current: number;
    pageSize: number;
    total: number;
    onChange: (page: number) => void;
  };
}

export const Table = <T extends Record<string, any>>({
  columns,
  dataSource,
  loading,
  rowKey,
  pagination,
}: TableProps<T>) => {
  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="flex flex-col items-center gap-3">
          <div className="w-10 h-10 border-4 border-primary-500 border-t-transparent rounded-full animate-spin"></div>
          <p className="text-neutral-500 dark:text-neutral-400">加载中...</p>
        </div>
      </div>
    );
  }

  if (dataSource.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-neutral-500 dark:text-neutral-400">
        暂无数据
      </div>
    );
  }

  const totalPages = pagination ? Math.ceil(pagination.total / pagination.pageSize) : 1;

  return (
    <div className="w-full">
      <div className="overflow-x-auto border border-neutral-200 rounded-xl bg-white dark:border-neutral-700 dark:bg-neutral-900">
        <table className="w-full">
          <thead className="bg-neutral-50/80 border-b border-neutral-200 dark:bg-neutral-800/80 dark:border-neutral-700">
            <tr>
              {columns.map((col) => (
                <th
                  key={col.key}
                  className="px-4 py-3.5 text-left text-xs font-semibold text-neutral-500 uppercase tracking-wide dark:text-neutral-400"
                  style={{ width: col.width }}
                >
                  {col.title}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-neutral-200 dark:bg-neutral-900 dark:divide-neutral-700">
            {dataSource.map((record, index) => (
              <tr key={record[rowKey]} className="hover:bg-primary-50/30 transition-colors dark:hover:bg-primary-900/20">
                {columns.map((col) => {
                  const value = col.dataIndex ? record[col.dataIndex] : undefined;
                  const content = col.render
                    ? col.render(value, record, index)
                    : value;

                  return (
                    <td key={col.key} className="px-4 py-3.5 text-sm text-neutral-700 align-middle dark:text-neutral-300">
                      {content}
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {pagination && totalPages > 1 && (
        <div className="flex items-center justify-between mt-4 px-2">
          <p className="text-sm text-neutral-600 dark:text-neutral-400">
            共 {pagination.total} 条数据
          </p>
          <div className="flex items-center gap-2">
            <button
              onClick={() => pagination.onChange(pagination.current - 1)}
              disabled={pagination.current === 1}
              className="p-2 rounded-lg border border-neutral-300 hover:bg-neutral-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors dark:border-neutral-600 dark:hover:bg-neutral-800"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <span className="text-sm text-neutral-600 dark:text-neutral-400">
              {pagination.current} / {totalPages}
            </span>
            <button
              onClick={() => pagination.onChange(pagination.current + 1)}
              disabled={pagination.current === totalPages}
              className="p-2 rounded-lg border border-neutral-300 hover:bg-neutral-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors dark:border-neutral-600 dark:hover:bg-neutral-800"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
