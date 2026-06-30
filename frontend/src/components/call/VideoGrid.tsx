'use client'

import {
  GridLayout,
  ParticipantTile,
  ControlBar,
  useTracks,
} from '@livekit/components-react'
import { Track } from 'livekit-client'

export function VideoGrid() {
  const tracks = useTracks(
    [
      { source: Track.Source.Camera, withPlaceholder: true },
      { source: Track.Source.ScreenShare, withPlaceholder: false },
    ],
    { onlySubscribed: false },
  )

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
      {/* minHeight:0 is required — a flex child defaults to min-height:auto and
          won't shrink below its content, so the video tile pushes ControlBar
          past the bottom (clipped by the call layout's overflow-hidden). */}
      <GridLayout tracks={tracks} style={{ flex: 1, minHeight: 0 }}>
        <ParticipantTile />
      </GridLayout>
      <ControlBar />
    </div>
  )
}
