'use client'

import { useEffect, useState } from 'react'
import {
  VideoTrack,
  isTrackReference,
  useTracks,
  useIsSpeaking,
  useTrackMutedIndicator,
  type TrackReferenceOrPlaceholder,
} from '@livekit/components-react'
import { Track } from 'livekit-client'
import type { ExcalidrawImperativeAPI, SocketId } from '@excalidraw/excalidraw/types'
import { uidOf, initialsOf, colorOf, peerNames, shallowEqual } from './callParticipants'
import { ICONS } from './CallToolbar'
import type { BoardIdentity } from '@/lib/hooks/useBoardDisplayName'

// Доска всегда в светлой теме Excalidraw — панель поверх неё тоже светлая.
const NAME = '#1B1C1F'
const MUTED = '#646670'
const SPEAK_RING = '0 0 0 3px rgba(59,165,93,0.95)'

interface CallParticipantsProps {
  /** null, пока Excalidraw не отдал api — тогда follow просто недоступен. */
  excalidrawApi: ExcalidrawImperativeAPI | null
  /** Своя личность: имя себя LiveKit не знает (у него только роль «Репетитор»). */
  identity?: BoardIdentity
}

/**
 * Discord-подобный список участников слева снизу (над зумом доски).
 * Заменяет и круглые PIP-камеры, и встроенный UserList Excalidraw:
 * клик по участнику включает/выключает follow-режим за ним.
 */
export function CallParticipants({ excalidrawApi, identity }: CallParticipantsProps) {
  const tracks = useTracks([{ source: Track.Source.Camera, withPlaceholder: true }], {
    onlySubscribed: false,
  })

  // ponytail: withPlaceholder может на миг отдать placeholder + реальный трек
  // для одного participant+source (гонка при перепубликации камеры) → дубль ключа.
  // Дедупим, предпочитая реальный трек (с publication) заглушке.
  const byKey = new Map<string, (typeof tracks)[number]>()
  for (const t of tracks) {
    const key = `${t.participant.identity}-${t.source}`
    const existing = byKey.get(key)
    if (!existing || ('publication' in t && t.publication)) byKey.set(key, t)
  }

  // Имена пиров и текущий follow — оба живут в appState доски, читаем одной
  // подпиской. Имя человека знает только доска: LiveKit хранит роль («Репетитор»,
  // «Ученик»), а настоящее имя пиры шлют друг другу в cursor-сообщениях, откуда
  // оно и попадает в collaborators (см. useExcalidrawSync).
  const [names, setNames] = useState<Record<string, string>>({})
  const [followingUid, setFollowingUid] = useState<string | null>(null)
  useEffect(() => {
    if (!excalidrawApi) return
    return excalidrawApi.onChange((_els, appState) => {
      const next = peerNames(appState.collaborators)
      // onChange бьёт на каждый штрих — обновляем стейт только на реальной смене.
      setNames((prev) => (shallowEqual(prev, next) ? prev : next))

      const socketId = appState.userToFollow?.socketId
      const collab = socketId
        ? appState.collaborators.get(socketId as SocketId)
        : undefined
      setFollowingUid(collab?.id ?? null)
    })
  }, [excalidrawApi])

  // Имя себя доска знает из BoardIdentity, чужие — из collaborators по uid.
  // Фолбэк на LiveKit-роль: аноним по ссылке (без uid) или пир, ещё не открывший
  // доску, — там имени просто нет.
  const nameOf = (lkIdentity: string, lkName: string, isLocal: boolean): string => {
    if (isLocal) return identity?.name || lkName
    const uid = uidOf(lkIdentity)
    return (uid && names[uid]) || lkName
  }

  const toggleFollow = (identity: string, name: string) => {
    const api = excalidrawApi
    if (!api) return
    const uid = uidOf(identity)
    if (!uid) return

    if (followingUid === uid) {
      api.updateScene({ appState: { userToFollow: null } })
      return
    }
    const entry = [...api.getAppState().collaborators.entries()].find(
      ([, c]) => c.id === uid && !c.isCurrentUser
    )
    if (!entry) return // участник звонка ещё не открыл доску
    api.updateScene({
      appState: { userToFollow: { socketId: entry[0], username: name } },
    })
  }

  return (
    <div
      style={{
        position: 'fixed',
        left: 24,
        bottom: 104, // над панелью зума Excalidraw
        zIndex: 9999,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'flex-start',
        gap: 10,
      }}
    >
      {[...byKey.values()].map((track) => {
        const p = track.participant
        return (
          <ParticipantRow
            key={`${p.identity}-${track.source}`}
            trackRef={track}
            name={nameOf(p.identity, p.name || p.identity, p.isLocal)}
            following={followingUid !== null && followingUid === uidOf(p.identity)}
            onToggleFollow={toggleFollow}
          />
        )
      })}
    </div>
  )
}

interface ParticipantRowProps {
  trackRef: TrackReferenceOrPlaceholder
  /** Имя из личности доски — не LiveKit-роль. */
  name: string
  following: boolean
  onToggleFollow: (identity: string, name: string) => void
}

function ParticipantRow({ trackRef, name, following, onToggleFollow }: ParticipantRowProps) {
  const participant = trackRef.participant
  const isLocal = participant.isLocal
  const speaking = useIsSpeaking(participant)
  const { isMuted } = useTrackMutedIndicator({
    participant,
    source: Track.Source.Microphone,
  })
  // Своё видео показываем зеркально — привычнее, чем «чужой» вид себя.
  const hasVideo = isTrackReference(trackRef) && !trackRef.publication.isMuted
  // Себя не «следим»: follow за собственным курсором ничего не даёт.
  const followable = !isLocal && uidOf(participant.identity) !== null

  const ring = speaking ? SPEAK_RING : following ? '0 0 0 3px #2B65F1' : undefined
  const pulse = speaking ? 'speakPulse 1.6s ease-in-out infinite' : undefined
  const click = followable ? () => onToggleFollow(participant.identity, name) : undefined
  const title = followable
    ? following
      ? 'Перестать следить'
      : 'Следить за участником'
    : undefined

  if (hasVideo) {
    return (
      <div
        onClick={click}
        title={title}
        style={{
          width: 208,
          height: 126,
          borderRadius: 12,
          overflow: 'hidden',
          position: 'relative',
          background: 'linear-gradient(160deg,#3c3c44,#16181d)',
          cursor: followable ? 'pointer' : 'default',
          boxShadow: ring
            ? `${ring}, 0 4px 14px rgba(0,0,0,0.25)`
            : '0 4px 14px rgba(0,0,0,0.25)',
          animation: speaking ? 'speakPulseBox 1.6s ease-in-out infinite' : undefined,
        }}
      >
        <VideoTrack
          trackRef={trackRef}
          style={{
            width: '100%',
            height: '100%',
            objectFit: 'cover',
            transform: isLocal ? 'scaleX(-1)' : undefined,
          }}
        />
        <div
          style={{
            position: 'absolute',
            bottom: 7,
            left: 7,
            display: 'flex',
            alignItems: 'center',
            gap: 5,
            background: 'rgba(0,0,0,0.55)',
            borderRadius: 6,
            padding: '3px 8px 3px 7px',
          }}
        >
          {isMuted && <MicOffIcon size={12} color="#E9898C" />}
          <span style={{ color: '#F1F1F0', fontSize: 12.5, fontWeight: 500 }}>{name}</span>
        </div>
      </div>
    )
  }

  return (
    <div
      onClick={click}
      title={title}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 9,
        borderRadius: 10,
        padding: '7px 12px 7px 7px',
        minWidth: 196,
        cursor: followable ? 'pointer' : 'default',
      }}
    >
      <div
        style={{
          width: 34,
          height: 34,
          borderRadius: '50%',
          background: colorOf(participant.identity),
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          color: '#FFFFFF',
          fontWeight: 700,
          fontSize: 13,
          flex: 'none',
          boxShadow: ring,
          animation: pulse,
        }}
      >
        {initialsOf(name)}
      </div>
      <span style={{ color: NAME, fontSize: 14, fontWeight: 600, flex: 1 }}>{name}</span>
      {isMuted && <MicOffIcon size={14} color={MUTED} />}
    </div>
  )
}

// Тот же перечёркнутый микрофон, что в тулбаре звонка — один жест не должен
// выглядеть двумя разными иконками.
function MicOffIcon({ size, color }: { size: number; color: string }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke={color}
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{ flex: 'none' }}
    >
      {ICONS.micOff}
    </svg>
  )
}
