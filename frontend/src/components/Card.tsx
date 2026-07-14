import type { ReactNode } from 'react';
import { Card as WcCard } from '@zeturn/watercolor-react';

interface CardProps {
  title?: ReactNode;
  extra?: ReactNode;
  children: ReactNode;
  className?: string;
}

export const Card: React.FC<CardProps> = ({
  title,
  extra,
  children,
  className = '',
}) => {
  const headerContent = (title || extra) ? (
    <div className="flex items-center justify-between gap-3">
      {title && <div className="text-lg font-semibold text-neutral-800 dark:text-neutral-100">{title}</div>}
      {extra && <div className="shrink-0">{extra}</div>}
    </div>
  ) : undefined;

  return (
    <WcCard
      variant="elevated"
      color="default"
      interactive={false}
      className={`rounded-2xl border border-neutral-200 ${className}`}
      header={headerContent}
    >
      {children}
    </WcCard>
  );
};

interface TagProps {
  color?: string;
  children: ReactNode;
}

export const Tag: React.FC<TagProps> = ({ color, children }) => {
  const colorMap: Record<string, string> = {
    green: 'bg-green-50 text-green-700 border-green-200 dark:bg-green-500/15 dark:text-green-300 dark:border-green-500/30',
    red: 'bg-red-50 text-red-700 border-red-200 dark:bg-red-500/15 dark:text-red-300 dark:border-red-500/30',
    orange: 'bg-orange-50 text-orange-700 border-orange-200 dark:bg-orange-500/15 dark:text-orange-300 dark:border-orange-500/30',
    blue: 'bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-500/15 dark:text-blue-300 dark:border-blue-500/30',
    gold: 'bg-yellow-50 text-yellow-700 border-yellow-200 dark:bg-yellow-500/15 dark:text-yellow-300 dark:border-yellow-500/30',
    default: 'bg-neutral-100 text-neutral-700 border-neutral-300 dark:bg-neutral-700/40 dark:text-neutral-200 dark:border-neutral-600',
  };

  const colorClass = colorMap[color || 'default'] || colorMap.default;

  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-md text-xs font-medium border ${colorClass}`}>
      {children}
    </span>
  );
};
