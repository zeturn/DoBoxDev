import type { ReactNode } from 'react';
import { Button as WcButton } from '@zeturn/watercolor-react';

interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost';
  size?: 'sm' | 'md' | 'lg';
  icon?: ReactNode;
  loading?: boolean;
  children?: ReactNode;
}

export const Button: React.FC<ButtonProps> = ({
  variant = 'secondary',
  size = 'md',
  icon,
  loading,
  children,
  className = '',
  disabled,
  onClick,
  ...props
}) => {
  const variantMap = {
    primary: { variant: 'primary', buttonStyle: 'filled' as const },
    secondary: { variant: 'default', buttonStyle: 'outlined' as const },
    danger: { variant: 'error', buttonStyle: 'filled' as const },
    ghost: { variant: 'default', buttonStyle: 'text' as const },
  };

  const mapped = variantMap[variant];

  return (
    <WcButton
      variant={mapped.variant as any}
      buttonStyle={mapped.buttonStyle}
      size={size}
      startIcon={!loading ? icon : undefined}
      loading={!!loading}
      disabled={disabled || loading}
      className={className}
      onClick={onClick as any}
      {...props}
    >
      {children}
    </WcButton>
  );
};

interface IconButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  icon: ReactNode;
  variant?: 'default' | 'danger' | 'primary';
  size?: 'sm' | 'md';
  tooltip?: string;
}

export const IconButton: React.FC<IconButtonProps> = ({
  icon,
  variant = 'default',
  size = 'md',
  tooltip,
  className = '',
  ...props
}) => {
  const variants = {
    default: { variant: 'default', buttonStyle: 'text' as const },
    danger: { variant: 'error', buttonStyle: 'text' as const },
    primary: { variant: 'primary', buttonStyle: 'text' as const },
  };

  const sizes = {
    sm: 'sm' as const,
    md: 'md' as const,
  };

  return (
    <WcButton
      variant={variants[variant].variant as any}
      buttonStyle={variants[variant].buttonStyle}
      size={sizes[size]}
      className={className}
      onClick={props.onClick as any}
      title={tooltip}
      {...props}
    >
      {icon}
    </WcButton>
  );
};
