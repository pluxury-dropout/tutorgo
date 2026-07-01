// frontend/src/components/call/DeviceSettings.tsx
'use client'

import { useMediaDeviceSelect } from '@livekit/components-react'
import type { CallTheme } from './callTheme'

function DeviceGroup({
  kind, label, theme,
}: { kind: 'audioinput' | 'videoinput'; label: string; theme: CallTheme }) {
  const { devices, activeDeviceId, setActiveMediaDevice } = useMediaDeviceSelect({ kind })
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4, padding: '4px 6px' }}>
      <div style={{ fontSize: 11, fontWeight: 600, color: theme.muted }}>{label}</div>
      {devices.map((d) => {
        const active = d.deviceId === activeDeviceId
        return (
          <button
            key={d.deviceId}
            onClick={() => setActiveMediaDevice(d.deviceId)}
            style={{
              textAlign: 'left',
              border: 'none',
              background: active ? theme.accentBg : 'transparent',
              color: active ? theme.accent : theme.text,
              fontSize: 12,
              padding: '7px 9px',
              borderRadius: 8,
              cursor: 'pointer',
              fontFamily: 'inherit',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              maxWidth: 220,
            }}
          >
            {d.label || 'Устройство'}
          </button>
        )
      })}
    </div>
  )
}

export function DeviceSettings({ theme }: { theme: CallTheme }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <DeviceGroup kind="audioinput" label="Микрофон" theme={theme} />
      <div style={{ height: 1, background: theme.border, margin: '2px 6px' }} />
      <DeviceGroup kind="videoinput" label="Камера" theme={theme} />
    </div>
  )
}
