import type { ReactNode } from 'react';

interface FormItemProps {
  label?: string;
  required?: boolean;
  error?: string;
  extra?: string;
  children: ReactNode;
  className?: string;
}

export const FormItem: React.FC<FormItemProps> = ({
  label,
  required,
  error,
  extra,
  children,
  className = '',
}) => {
  return (
    <div className={`mb-4 ${className}`}>
      {label && (
        <label className="block text-sm font-medium text-neutral-700 mb-2 dark:text-neutral-200">
          {label}
          {required && <span className="text-red-500 ml-1">*</span>}
        </label>
      )}
      {children}
      {extra && <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">{extra}</p>}
      {error && <p className="mt-1 text-xs text-red-500 dark:text-red-400">{error}</p>}
    </div>
  );
};

interface InputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'prefix'> {
  prefix?: ReactNode;
}

export const Input: React.FC<InputProps> = ({
  prefix,
  className = '',
  ...props
}) => {
  const baseClass = 'w-full px-4 py-2.5 border border-neutral-300 rounded-lg bg-white text-neutral-900 placeholder-neutral-400 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-100 transition-all dark:bg-neutral-900 dark:border-neutral-600 dark:text-neutral-100 dark:placeholder-neutral-500 dark:focus:ring-primary-900/40';
  
  if (prefix) {
    return (
      <div className="relative">
        <div className="absolute left-3 top-1/2 transform -translate-y-1/2 text-neutral-400 dark:text-neutral-500">
          {prefix}
        </div>
        <input className={`${baseClass} pl-10 ${className}`} {...props} />
      </div>
    );
  }

  return <input className={`${baseClass} ${className}`} {...props} />;
};

interface TextAreaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {}

export const TextArea: React.FC<TextAreaProps> = ({
  className = '',
  ...props
}) => {
  return (
    <textarea
      className={`w-full px-4 py-2.5 border border-neutral-300 rounded-lg bg-white text-neutral-900 placeholder-neutral-400 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-100 transition-all resize-vertical dark:bg-neutral-900 dark:border-neutral-600 dark:text-neutral-100 dark:placeholder-neutral-500 dark:focus:ring-primary-900/40 ${className}`}
      {...props}
    />
  );
};

interface SelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  options?: { label: string; value: string | number }[];
}

export const Select: React.FC<SelectProps> = ({
  options = [],
  className = '',
  children,
  ...props
}) => {
  return (
    <select
      className={`w-full px-4 py-2.5 border border-neutral-300 rounded-lg bg-white text-neutral-900 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-100 transition-all dark:bg-neutral-900 dark:border-neutral-600 dark:text-neutral-100 dark:focus:ring-primary-900/40 ${className}`}
      {...props}
    >
      {options.map((opt) => (
        <option key={opt.value} value={opt.value}>
          {opt.label}
        </option>
      ))}
      {children}
    </select>
  );
};

interface InputNumberProps extends React.InputHTMLAttributes<HTMLInputElement> {
  min?: number;
  max?: number;
  step?: number;
}

export const InputNumber: React.FC<InputNumberProps> = ({
  className = '',
  ...props
}) => {
  return (
    <input
      type="number"
      className={`w-full px-4 py-2.5 border border-neutral-300 rounded-lg bg-white text-neutral-900 focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-100 transition-all dark:bg-neutral-900 dark:border-neutral-600 dark:text-neutral-100 dark:focus:ring-primary-900/40 ${className}`}
      {...props}
    />
  );
};
