package handlers

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPresenceCursorFlushBroadcastsOnceThenClears(t *testing.T) {
	r := newPresenceRegistry()
	r.setCursor("A", json.RawMessage(`{"type":"cursor","peerId":"A"}`))
	out := r.flush()
	assert.Len(t, out, 1)
	assert.True(t, out[0].toAll)
	assert.Equal(t, "A", out[0].origin)
	assert.Empty(t, r.flush(), "dirty должен очиститься после flush")
}

func TestPresenceFollowReturnsTargetViewportSnap(t *testing.T) {
	r := newPresenceRegistry()
	r.setViewport("A", json.RawMessage(`{"type":"viewport","peerId":"A"}`))
	snap := r.follow("B", "A")
	assert.JSONEq(t, `{"type":"viewport","peerId":"A"}`, string(snap))
}

func TestPresenceFollowNoViewportYetReturnsNil(t *testing.T) {
	r := newPresenceRegistry()
	assert.Nil(t, r.follow("B", "A"))
}

func TestPresenceViewportRoutedOnlyToFollowers(t *testing.T) {
	r := newPresenceRegistry()
	r.follow("B", "A")
	r.setViewport("A", json.RawMessage(`{"type":"viewport","peerId":"A"}`))
	out := r.flush()
	assert.Len(t, out, 1)
	assert.False(t, out[0].toAll)
	assert.Equal(t, []string{"B"}, out[0].to)
}

func TestPresenceViewportNoFollowersEmitsNothing(t *testing.T) {
	r := newPresenceRegistry()
	r.setViewport("A", json.RawMessage(`{"type":"viewport","peerId":"A"}`))
	assert.Empty(t, r.flush())
}

func TestPresenceUnfollowStopsRouting(t *testing.T) {
	r := newPresenceRegistry()
	r.follow("B", "A")
	r.unfollow("B")
	r.setViewport("A", json.RawMessage(`{"type":"viewport","peerId":"A"}`))
	assert.Empty(t, r.flush())
}

func TestPresenceRemoveTargetClearsFollowers(t *testing.T) {
	r := newPresenceRegistry()
	r.follow("B", "A")
	r.remove("A")
	assert.Empty(t, r.followersOf("A"))
	_, stillFollowing := r.following["B"]
	assert.False(t, stillFollowing)
}
