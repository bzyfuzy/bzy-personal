import { forwardRef } from 'react'
import type { ButtonHTMLAttributes } from 'react'
import { Spinner } from './Spinner'

type Variant = 'primary' | 'ghost' | 'danger'
type Size = 'sm' | 'md' | 'lg'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  loading?: boolean
}

const variantClasses: Record<Variant, string> = {
  primary:
    'bg-indigo-600 text-white hover:bg-indigo-500 disabled:bg-indigo-800',
  ghost:
    'bg-transparent text-zinc-300 hover:bg-zinc-700 disabled:text-zinc-600',
  danger:
    'bg-red-700 text-white hover:bg-red-600 disabled:bg-red-900',
}

const sizeClasses: Record<Size, string> = {
  sm: 'px-2.5 py-1 text-xs',
  md: 'px-4 py-2 text-sm',
  lg: 'px-5 py-2.5 text-base',
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = 'primary', size = 'md', loading, children, className = '', disabled, ...rest }, ref) => (
    <button
      ref={ref}
      disabled={disabled || loading}
      className={[
        'inline-flex items-center justify-center gap-2 rounded-md font-medium',
        'transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-indigo-500',
        'disabled:cursor-not-allowed',
        variantClasses[variant],
        sizeClasses[size],
        className,
      ].join(' ')}
      {...rest}
    >
      {loading && <Spinner size="sm" />}
      {children}
    </button>
  ),
)
Button.displayName = 'Button'
