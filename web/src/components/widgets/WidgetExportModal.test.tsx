import { describe, it, expect, vi, beforeEach } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import { WidgetExportModal } from './WidgetExportModal'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => String(options?.filename ?? options?.defaultValue ?? key),
    i18n: { language: 'en', changeLanguage: vi.fn() },
  }),
}))

vi.mock('../../lib/modals', () => {
  const BaseModal = ({ isOpen, children }: { isOpen: boolean; children: React.ReactNode }) => (
    isOpen ? <div role="dialog">{children}</div> : null
  )
  BaseModal.Header = ({ title }: { title: string }) => <div>{title}</div>
  BaseModal.Content = ({ children }: { children: React.ReactNode }) => <div>{children}</div>
  return { BaseModal }
})

vi.mock('../../lib/widgets/widgetRegistry', () => ({
  WIDGET_TEMPLATES: {
    cluster_overview: {
      templateId: 'cluster_overview',
      displayName: 'Cluster Overview',
      description: 'Template description',
      cards: ['pod_health'],
      stats: ['cpu_usage'],
      size: { width: 400, height: 300 },
      layout: 'grid',
    },
  },
  WIDGET_CARDS: {
    pod_health: {
      cardType: 'pod_health',
      displayName: 'Pod Health',
      description: 'Card description',
      defaultSize: { width: 320, height: 240 },
      category: 'Health',
    },
  },
  WIDGET_STATS: {
    cpu_usage: {
      statId: 'cpu_usage',
      displayName: 'CPU Usage',
      format: 'percent',
      size: { width: 120, height: 90 },
      color: '#60a5fa',
    },
  },
}))

vi.mock('../../lib/widgets/codeGenerator', () => ({
  generateWidget: () => 'export default {}',
  getWidgetFilename: () => 'widget.jsx',
}))

vi.mock('../../lib/analytics', () => ({
  emitWidgetDownloaded: vi.fn(),
}))

vi.mock('../../lib/clipboard', () => ({
  copyToClipboard: vi.fn().mockResolvedValue(undefined),
}))

describe('WidgetExportModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('exposes tab semantics and labeled form controls', () => {
    render(<WidgetExportModal isOpen={true} onClose={vi.fn()} />)

    const templatesTab = screen.getByRole('tab', { name: 'widgets.templates' })
    const cardTab = screen.getByRole('tab', { name: 'widgets.singleCard' })
    expect(templatesTab).toHaveAttribute('aria-selected', 'true')
    expect(cardTab).toHaveAttribute('aria-selected', 'false')

    const tabpanel = screen.getByRole('tabpanel')
    expect(tabpanel).toHaveAttribute('aria-labelledby', 'widget-export-tab-templates')
    expect(screen.getByLabelText('widgets.apiEndpoint')).toBeInTheDocument()
    expect(screen.getByLabelText('widgets.refreshInterval')).toHaveAttribute('min', '10')
  })

  it('toggles code view state and selection pressed state', () => {
    render(<WidgetExportModal isOpen={true} onClose={vi.fn()} />)

    const showCodeButton = screen.getByRole('button', { name: 'widgets.showCode' })
    expect(showCodeButton).toHaveAttribute('aria-pressed', 'false')

    fireEvent.click(showCodeButton)

    expect(screen.getByRole('button', { name: 'widgets.hideCode' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByText('export default {}')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'widgets.singleCard' }))
    expect(screen.getByRole('button', { name: /^Pod Health/ })).toHaveAttribute('aria-pressed', 'false')
  })
})
