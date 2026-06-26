'use client'

import { useEffect } from 'react'
import {
  GridLayout,
  ParticipantTile,
  ControlBar,
  useTracks,
  useRoomContext,
} from '@livekit/components-react'
import { Track } from 'livekit-client'

export function VideoGrid() {
  const room = useRoomContext()
  const tracks = useTracks(
    [
      { source: Track.Source.Camera, withPlaceholder: true },
      { source: Track.Source.ScreenShare, withPlaceholder: false },
    ],
    { onlySubscribed: false },
  )

  useEffect(() => {
    room.localParticipant.enableCameraAndMicrophone().catch(() => {})
  }, [room])

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
