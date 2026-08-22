package handlers

import "encoding/json"

// peerPresence — последнее эфемерное состояние одного соединения.
type peerPresence struct {
	cursor   json.RawMessage // последний cursor (уже с peerId)
	viewport json.RawMessage // последний viewport (уже с peerId)
}

// presenceRegistry — авторитетное эфемерное состояние одной страницы доски.
// НЕ потокобезопасен: владелец — единственная run()-горутина хаба.
type presenceRegistry struct {
	peers       map[string]*peerPresence
	following   map[string]string // follower peerID -> target peerID
	cursorDirty map[string]bool
	vpDirty     map[string]bool
}

func newPresenceRegistry() *presenceRegistry {
	return &presenceRegistry{
		peers:       map[string]*peerPresence{},
		following:   map[string]string{},
		cursorDirty: map[string]bool{},
		vpDirty:     map[string]bool{},
	}
}

func (r *presenceRegistry) peer(id string) *peerPresence {
	p := r.peers[id]
	if p == nil {
		p = &peerPresence{}
		r.peers[id] = p
	}
	return p
}

func (r *presenceRegistry) setCursor(peerID string, msg json.RawMessage) {
	r.peer(peerID).cursor = msg
	r.cursorDirty[peerID] = true
}

func (r *presenceRegistry) setViewport(peerID string, msg json.RawMessage) {
	r.peer(peerID).viewport = msg
	r.vpDirty[peerID] = true
}

// follow фиксирует follower->target и возвращает текущий viewport цели для
// мгновенного снапа (nil, если цель ещё не вещала вьюпорт).
func (r *presenceRegistry) follow(follower, target string) json.RawMessage {
	r.following[follower] = target
	if p := r.peers[target]; p != nil {
		return p.viewport
	}
	return nil
}

func (r *presenceRegistry) unfollow(follower string) {
	delete(r.following, follower)
}

// remove убирает presence пира и любые follow-связи, его касающиеся.
func (r *presenceRegistry) remove(peerID string) {
	delete(r.peers, peerID)
	delete(r.cursorDirty, peerID)
	delete(r.vpDirty, peerID)
	delete(r.following, peerID) // если пир был подписчиком
	for f, t := range r.following {
		if t == peerID { // пиры, следившие за ушедшим
			delete(r.following, f)
		}
	}
}

// followerCounts — сколько пиров следит за каждой целью (нулевые не кладём).
// Клиенту нужно, чтобы нарисовать «за этим человеком смотрят N».
func (r *presenceRegistry) followerCounts() map[string]int {
	out := map[string]int{}
	for _, t := range r.following {
		out[t]++
	}
	return out
}

func (r *presenceRegistry) followersOf(peerID string) []string {
	var out []string
	for f, t := range r.following {
		if t == peerID {
			out = append(out, f)
		}
	}
	return out
}

// presenceOut — одна адресованная рассылка из flush.
// toAll=true: всем, кроме origin. Иначе — только пирам из to.
type presenceOut struct {
	data   json.RawMessage
	origin string
	to     []string
	toAll  bool
}

// flush отдаёт коалесцированные рассылки за тик и сбрасывает dirty-множества.
// Курсоры идут всем (Excalidraw показывает все курсоры), вьюпорт — только
// подписчикам данного пира.
func (r *presenceRegistry) flush() []presenceOut {
	var out []presenceOut
	for id := range r.cursorDirty {
		if p := r.peers[id]; p != nil && p.cursor != nil {
			out = append(out, presenceOut{data: p.cursor, origin: id, toAll: true})
		}
		delete(r.cursorDirty, id)
	}
	for id := range r.vpDirty {
		p := r.peers[id]
		if p != nil && p.viewport != nil {
			if fs := r.followersOf(id); len(fs) > 0 {
				out = append(out, presenceOut{data: p.viewport, to: fs})
			}
		}
		delete(r.vpDirty, id)
	}
	return out
}
