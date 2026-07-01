// frontend/src/components/call/CallTile.tsx
'use client'

import {
  VideoTrack,
  isTrackReference,
  useTrackMutedIndicator,
  type TrackReferenceOrPlaceholder,
} from '@livekit/components-react'
import { Track } from 'livekit-client'
import { TILE_BG, AVATAR, GLYPH } from './callTheme'

interface Props {
  trackRef: TrackReferenceOrPlaceholder
  variant: 'speaker' | 'grid'
}

export function CallTile({ trackRef, variant }: Props) {
  const speaker = variant === 'speaker'
  const name = trackRef.participant.name || trackRef.participant.identity
  const { isMuted } = useTrackMutedIndicator({
    participant: trackRef.participant,
    source: Track.Source.Microphone,
  })
  const hasVideo = isTrackReference(trackRef)
  const avatarSize = speaker ? 168 : 62
  const glyphSize = speaker ? 94 : 34

  return (
    <div
      style={{
        position: 'relative',
        width: '100%',
        height: '100%',
        borderRadius: speaker ? 0 : 12,
        overflow: 'hidden',
        background: TILE_BG,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
      }}
    >
      {hasVideo ? (
        <VideoTrack
          trackRef={trackRef}
          style={{ width: '100%', height: '100%', objectFit: 'cover' }}
        />
      ) : (
        <div
          style={{
            width: avatarSize,
            height: avatarSize,
            borderRadius: '50%',
            background: AVATAR,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <svg width={glyphSize} height={glyphSize} viewBox="0 0 24 24" fill={GLYPH}>
            <circle cx="12" cy="8" r="4.1" />
            <path d="M4 21c0-4.4 3.6-8 8-8s8 3.6 8 8Z" />
          </svg>
        </div>
      )}

      {/* Плашка имени — сверху-слева (низ занят тулбаром) */}
      <div
        style={{
          position: 'absolute',
          left: speaker ? 18 : 10,
          top: speaker ? 18 : 10,
          display: 'flex',
          alignItems: 'center',
          gap: speaker ? 7 : 5,
          background: 'rgba(14,14,15,0.68)',
          padding: speaker ? '6px 12px' : '4px 9px',
          borderRadius: speaker ? 9 : 7,
        }}
      >
        {isMuted && (
          <svg
            width={speaker ? 14 : 12}
            height={speaker ? 14 : 12}
            viewBox="0 0 24 24"
            fill="none"
            stroke="#fff"
            strokeWidth={speaker ? 1.8 : 2}
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M9 9v3a3 3 0 0 0 4.6 2.5" />
            <path d="M15 9.34V5a3 3 0 0 0-5.94-.6" />
            <path d="M19 10v1a7 7 0 0 1-.32 2.1" />
            <path d="M5 10v1a7 7 0 0 0 11.3 5.5" />
            <path d="M12 18v4" />
            <path d="M8 22h8" />
            <path d="M2 2l20 20" />
          </svg>
        )}
        <span style={{ color: '#fff', fontSize: speaker ? 13 : 12, fontWeight: 500 }}>
          {name}
        </span>
      </div>
    </div>
  )
}
