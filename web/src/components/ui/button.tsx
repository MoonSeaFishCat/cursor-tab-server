import { Slot } from '@radix-ui/react-slot'
import { cva, type VariantProps } from 'class-variance-authority'
import type { ButtonHTMLAttributes } from 'react'

const buttonVariants = cva('button', {
  variants: {
    variant: {
      default: 'button',
      secondary: 'button secondary',
      destructive: 'button danger',
      ghost: 'button ghost',
    },
    size: {
      default: '',
      small: 'sm',
    },
  },
  defaultVariants: { variant: 'default', size: 'default' },
})

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants> & { asChild?: boolean }

export function Button({ className = '', variant, size, asChild, ...props }: ButtonProps) {
  const Component = asChild ? Slot : 'button'
  return <Component className={`${buttonVariants({ variant, size })} ${className}`.trim()} {...props} />
}
