import type { TooltipProps } from 'tdesign-vue-next'

// Table ellipsis uses the application's tooltip, never a native title popup.
export const tableTooltip: TooltipProps = {
  theme: 'light',
  placement: 'top-left',
  attach: 'body',
  showArrow: false,
  overlayInnerStyle: {
    maxWidth: 'min(480px, calc(100vw - 32px))',
    maxHeight: '320px',
    overflowY: 'auto',
    whiteSpace: 'pre-wrap',
    overflowWrap: 'anywhere',
    lineHeight: '1.7',
  },
}
