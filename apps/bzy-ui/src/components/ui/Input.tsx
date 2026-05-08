import { forwardRef } from 'react'
import type { InputHTMLAttributes, TextareaHTMLAttributes } from 'react'

const baseClass =
  'w-full rounded-md border border-zinc-700 bg-zinc-800 px-3 py-2 text-sm text-zinc-100 ' +
  'placeholder:text-zinc-500 focus:outline-none focus:ring-2 focus:ring-indigo-500 ' +
  'disabled:cursor-not-allowed disabled:opacity-50'

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(
  ({ className = '', ...rest }, ref) => (
    <input ref={ref} className={`${baseClass} ${className}`} {...rest} />
  ),
)
Input.displayName = 'Input'

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(
  ({ className = '', ...rest }, ref) => (
    <textarea ref={ref} className={`${baseClass} resize-none ${className}`} {...rest} />
  ),
)
Textarea.displayName = 'Textarea'
