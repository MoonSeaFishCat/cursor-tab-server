import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Button } from './button'

describe('Button', () => {
  it('applies the destructive action style', () => {
    render(<Button variant="destructive">停用</Button>)
    expect(screen.getByRole('button', { name: '停用' }).className).toContain('danger')
  })
})
