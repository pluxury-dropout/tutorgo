'use client'

import { ParticipantTile, useTracks } from '@livekit/components-react'
import { Track } from 'livekit-client'

export function PipCameras() {
  const tracks = useTracks(
    [{ source: Track.Source.Camera, withPlaceholder: true }],
    { onlySubscribed: false },
  )

  return (
    <div
      style={{
        position: 'absolute',
        top: 12,
        right: 12,
        zIndex: 50,
        display: 'flex',
        flexDirection: 'column',
        gap: 8,
        pointerEvents: 'auto',
      }}
    >
      {tracks.map((track) => (
        <ParticipantTile
          key={`${track.participant.identity}-${track.source}`}
          trackRef={track}
          style={{
            width: 120,
            height: 80,
            borderRadius: 8,
            overflow: 'hidden',
            flexShrink: 0,
          }}
        />
      ))}
    </div>
  )
}
