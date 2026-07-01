'use client'

import { useState } from 'react'
import type { ReactNode } from 'react'
import {
  ParticipantTile,
  VideoTrack,
  ParticipantPlaceholder,
  isTrackReference,
  useTracks,
  useLocalParticipant,
  useTrackMutedIndicator,
  type TrackReferenceOrPlaceholder,
} from '@livekit/components-react'
import { Track, type LocalParticipant } from 'livekit-client'

const PILL_BG = 'rgba(255,255,255,.8)'
const FG = '#2f333b'

export function PipCameras() {
  const tracks = useTracks(
    [{ source: Track.Source.Camera, withPlaceholder: true }],
    { onlySubscribed: false },
  )
  const { localParticipant, isCameraEnabled, isMicrophoneEnabled } = useLocalParticipant()

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
        <PipTile
          key={`${track.participant.identity}-${track.source}`}
          trackRef={track}
          localParticipant={localParticipant}
          camOn={isCameraEnabled}
          micOn={isMicrophoneEnabled}
        />
      ))}
    </div>
  )
}

interface PipTileProps {
  trackRef: TrackReferenceOrPlaceholder
  localParticipant: LocalParticipant
  camOn: boolean
  micOn: boolean
}

function PipTile({ trackRef, localParticipant, camOn, micOn }: PipTileProps) {
  const [hover, setHover] = useState(false)
  const isLocal = trackRef.participant.isLocal
  const name = trackRef.participant.name || trackRef.participant.identity
  const { isMuted } = useTrackMutedIndicator({
    participant: trackRef.participant,
    source: Track.Source.Microphone,
  })
  const micIsOn = isLocal ? micOn : !isMuted

  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 8 }}>
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
        <ParticipantTile trackRef={trackRef} style={{ width: '100%', height: '100%', borderRadius: 0 }}>
          {isTrackReference(trackRef) ? (
            <VideoTrack trackRef={trackRef} style={{ width: '100%', height: '100%', objectFit: 'cover' }} />
          ) : (
            <ParticipantPlaceholder />
          )}
        </ParticipantTile>
      </div>

      {/* Плашка имени + значок микрофона (реальное состояние) + (для себя) кнопки по наведению */}
      <div
        onMouseEnter={() => setHover(true)}
        onMouseLeave={() => setHover(false)}
        style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 6 }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            background: PILL_BG,
            padding: '4px 11px',
            borderRadius: 999,
            boxShadow: '0 4px 14px rgba(30,40,60,.12)',
          }}
        >
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke={FG} strokeWidth="2.1" strokeLinecap="round">
            <path d="M12 2a3 3 0 0 0-3 3v6a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3z" />
            <path d="M5 11a7 7 0 0 0 14 0M12 18v3" />
            {!micIsOn && <line x1="2" y1="2" x2="22" y2="22" />}
          </svg>
          <span style={{ color: FG, fontSize: 11.5, fontWeight: 600 }}>{name}</span>
        </div>

        {isLocal && hover && (
          <div style={{ display: 'flex', gap: 8 }}>
            <MediaButton
              on={micOn}
              title={micOn ? 'Выключить микрофон' : 'Включить микрофон'}
              onClick={() => localParticipant.setMicrophoneEnabled(!micOn)}
            >
              <path d="M12 2a3 3 0 0 0-3 3v6a3 3 0 0 0 6 0V5a3 3 0 0 0-3-3z" />
              <path d="M5 11a7 7 0 0 0 14 0M12 18v3" />
            </MediaButton>
            <MediaButton
              on={camOn}
              title={camOn ? 'Выключить камеру' : 'Включить камеру'}
              onClick={() => localParticipant.setCameraEnabled(!camOn)}
            >
              <path d="M23 7l-7 5 7 5V7z" />
              <rect x="1" y="5" width="15" height="14" rx="2" />
            </MediaButton>
          </div>
        )}
      </div>
    </div>
  )
}

interface MediaButtonProps {
  on: boolean
  title: string
  onClick: () => void
  children: ReactNode
}

function MediaButton({ on, title, onClick, children }: MediaButtonProps) {
  return (
    <button
      onClick={onClick}
      title={title}
      style={{
        width: 30,
        height: 30,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        border: 'none',
        borderRadius: '50%',
        cursor: 'pointer',
        background: PILL_BG,
        color: FG,
        boxShadow: '0 4px 14px rgba(30,40,60,.12)',
      }}
    >
      <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        {children}
        {/* косая черта = выключено */}
        {!on && <line x1="2" y1="2" x2="22" y2="22" />}
      </svg>
    </button>
  )
}
