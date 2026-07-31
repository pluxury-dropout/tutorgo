// frontend/src/components/call/CallTile.tsx
'use client'

import {
  VideoTrack,
  isTrackReference,
  type TrackReferenceOrPlaceholder,
} from '@livekit/components-react'
import { User } from 'lucide-react'
import { TILE_BG, AVATAR, GLYPH } from './callTheme'

interface Props {
  trackRef: TrackReferenceOrPlaceholder
  variant: 'speaker' | 'grid'
}

// ponytail: без плашки имени — LiveKit знает только роль («Репетитор»), а не
// человека; имена и мьют показывает CallParticipants поверх доски.
export function CallTile({ trackRef, variant }: Props) {
  const speaker = variant === 'speaker'
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
          <User size={glyphSize} color={GLYPH} strokeWidth={1.5} />
        </div>
      )}
    </div>
  )
}
