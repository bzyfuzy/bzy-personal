type Color = 'gray' | 'green' | 'yellow' | 'red' | 'blue' | 'purple'

interface BadgeProps {
  color?: Color
  children: React.ReactNode
  className?: string
}

const colorClasses: Record<Color, string> = {
  gray:   'bg-zinc-700 text-zinc-300',
  green:  'bg-emerald-900 text-emerald-300',
  yellow: 'bg-amber-900 text-amber-300',
  red:    'bg-red-900 text-red-300',
  blue:   'bg-blue-900 text-blue-300',
  purple: 'bg-purple-900 text-purple-300',
}

export function Badge({ color = 'gray', children, className = '' }: BadgeProps) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium ${colorClasses[color]} ${className}`}
    >
      {children}
    </span>
  )
}
