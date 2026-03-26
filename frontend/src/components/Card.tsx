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
      {title && <div className="text-lg font-semibold text-neutral-800">{title}</div>}
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
    green: 'bg-green-50 text-green-700 border-green-200',
    red: 'bg-red-50 text-red-700 border-red-200',
    orange: 'bg-orange-50 text-orange-700 border-orange-200',
    blue: 'bg-blue-50 text-blue-700 border-blue-200',
    gold: 'bg-yellow-50 text-yellow-700 border-yellow-200',
    default: 'bg-neutral-100 text-neutral-700 border-neutral-300',
  };

  const colorClass = colorMap[color || 'default'] || colorMap.default;

  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-md text-xs font-medium border ${colorClass}`}>
      {children}
    </span>
  );
};
