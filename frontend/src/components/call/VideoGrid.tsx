'use client'

import { useEffect } from 'react'
import {
  GridLayout,
  ParticipantTile,
  ControlBar,
  RoomAudioRenderer,
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
      <GridLayout tracks={tracks} style={{ flex: 1 }}>
        <ParticipantTile />
      </GridLayout>
      <ControlBar />
      <RoomAudioRenderer />
    </div>
  )
}
