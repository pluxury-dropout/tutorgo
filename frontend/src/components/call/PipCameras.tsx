'use client'

import { ParticipantTile, useTracks } from '@livekit/components-react'
import { Track } from 'livekit-client'

export function PipCameras() {
  const tracks = useTracks(
    [{ source: Track.Source.Camera, withPlaceholder: true }],
    { onlySubscribed: false },
  )

  // ponytail: withPlaceholder может на миг отдать placeholder + реальный трек
  // для одного participant+source (гонка при перепубликации камеры) → дубль ключа.
  // Дедупим, предпочитая реальный трек (с publication) заглушке.
  const byKey = new Map<string, (typeof tracks)[number]>()
  for (const t of tracks) {
    const key = `${t.participant.identity}-${t.source}`
    const existing = byKey.get(key)
    if (!existing || ('publication' in t && t.publication)) byKey.set(key, t)
  }
  const uniqueTracks = [...byKey.values()]

  return (
    <div
      style={{
        position: 'fixed',
        top: 72, // под доком
        right: 16,
        zIndex: 9999,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        gap: 10,
        pointerEvents: 'auto',
      }}
    >
      {uniqueTracks.map((track) => (
        <div
          key={`${track.participant.identity}-${track.source}`}
          style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}
        >
          <div
            style={{
              width: 86,
              height: 86,
              borderRadius: '50%',
              overflow: 'hidden',
              border: '3px solid rgba(255,255,255,.7)',
              boxShadow: '0 10px 28px rgba(30,40,60,.22)',
              background: 'linear-gradient(150deg,#3c3c44,#1f1f24)',
            }}
          >
            <ParticipantTile
              trackRef={track}
              style={{ width: '100%', height: '100%', borderRadius: 0 }}
            />
          </div>
          <div
            style={{
              display: 'flex', alignItems: 'center', gap: 6,
              background: 'rgba(255,255,255,.8)', padding: '4px 11px',
              borderRadius: 999, boxShadow: '0 4px 14px rgba(30,40,60,.12)',
            }}
          >
            <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="#2f333b" strokeWidth="2.1" strokeLinecap="round">
              <path d="M12 2a3 3 0 0 0-3 3v6a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3z" />
              <path d="M5 11a7 7 0 0 0 14 0M12 18v3" />
            </svg>
            <span style={{ color: '#2f333b', fontSize: 11.5, fontWeight: 600 }}>Репетитор</span>
          </div>
        </div>
      ))}
    </div>
  )
}
