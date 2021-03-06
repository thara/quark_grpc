package gameserver

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActor_Leave(t *testing.T) {
	r := NewRoom()
	defer r.Stop()

	a := NewActor()

	a.JoinTo(r)
	require.True(t, a.InRoom())

	ok := a.Leave()
	assert.True(t, ok)

	assert.False(t, a.InRoom())

	_, ok = <-a.Inbox()
	assert.False(t, ok)

	ok = a.Leave()
	assert.False(t, ok)
}

func TestActor_BroadcastToRoom(t *testing.T) {
	r := NewRoom()
	defer r.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	a1 := NewActor()
	a2 := NewActor()
	a3 := NewActor()

	a1.JoinTo(r)
	a2.JoinTo(r)
	{
		m := <-a1.Inbox()
		require.IsType(t, m, JoinRoomEvent{})
		assert.Len(t, m.(JoinRoomEvent).ActorList, 2)
	}
	a3.JoinTo(r)
	{
		m := <-a1.Inbox()
		require.IsType(t, m, JoinRoomEvent{})
		assert.Len(t, m.(JoinRoomEvent).ActorList, 3)
	}
	{
		m := <-a2.Inbox()
		require.IsType(t, m, JoinRoomEvent{})
		assert.Len(t, m.(JoinRoomEvent).ActorList, 3)
	}

	body := make([]byte, 1024)
	rand.Read(body)
	a3.BroadcastToRoom(Payload{0x01, body})

	n := 0
L:
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
			return
		case m := <-a1.Inbox():
			require.IsType(t, m, ActorMessage{})
			am := m.(ActorMessage)
			assert.Equal(t, a3.ActorID(), am.Sender)
			assert.EqualValues(t, 0x01, am.Code)
			assert.Equal(t, body, am.Payload)
			n += 1
		case m := <-a2.Inbox():
			require.IsType(t, m, ActorMessage{})
			am := m.(ActorMessage)
			assert.Equal(t, a3.ActorID(), am.Sender)
			assert.EqualValues(t, 0x01, am.Code)
			assert.Equal(t, body, am.Payload)
			n += 1
		case m := <-a3.Inbox():
			require.IsType(t, m, ActorMessage{})
			am := m.(ActorMessage)
			assert.Equal(t, a3.ActorID(), am.Sender)
			assert.EqualValues(t, 0x01, am.Code)
			assert.Equal(t, body, am.Payload)
			n += 1
		default:
			if n == 3 {
				break L
			}
		}
	}

	a2.Leave()
	{
		m := <-a1.Inbox()
		require.IsType(t, m, LeaveRoomEvent{})
		assert.Len(t, m.(LeaveRoomEvent).ActorList, 2)
	}
	{
		m := <-a3.Inbox()
		require.IsType(t, m, LeaveRoomEvent{})
		assert.Len(t, m.(LeaveRoomEvent).ActorList, 2)
	}

	body = make([]byte, 1024)
	rand.Read(body)
	a3.BroadcastToRoom(Payload{0x02, body})

	n = 0
M:
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
			return
		case m := <-a1.Inbox():
			require.IsType(t, m, ActorMessage{})
			am := m.(ActorMessage)
			assert.Equal(t, a3.ActorID(), am.Sender)
			assert.EqualValues(t, 0x02, am.Code)
			assert.Equal(t, body, am.Payload)
			n += 1
		case m := <-a3.Inbox():
			require.IsType(t, m, ActorMessage{})
			am := m.(ActorMessage)
			assert.Equal(t, a3.ActorID(), am.Sender)
			assert.EqualValues(t, 0x02, am.Code)
			assert.Equal(t, body, am.Payload)
			n += 1
		default:
			if n >= 2 {
				break M
			}
		}
	}

	a4 := NewActor()
	a4.JoinTo(r)
	{
		m := <-a1.Inbox()
		require.IsType(t, m, JoinRoomEvent{})
		assert.Len(t, m.(JoinRoomEvent).ActorList, 3)
	}
	{
		m := <-a3.Inbox()
		require.IsType(t, m, JoinRoomEvent{})
		assert.Len(t, m.(JoinRoomEvent).ActorList, 3)
	}

	body = make([]byte, 1024)
	rand.Read(body)
	a4.BroadcastToRoom(Payload{0x03, body})

	n = 0
N:
	for {
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
			return
		case m := <-a1.Inbox():
			require.IsType(t, m, ActorMessage{})
			am := m.(ActorMessage)
			assert.Equal(t, a4.ActorID(), am.Sender)
			assert.EqualValues(t, 0x03, am.Code)
			assert.Equal(t, body, am.Payload)
			n += 1
		case m := <-a3.Inbox():
			require.IsType(t, m, ActorMessage{})
			am := m.(ActorMessage)
			assert.Equal(t, a4.ActorID(), am.Sender)
			assert.EqualValues(t, 0x03, am.Code)
			assert.Equal(t, body, am.Payload)
			n += 1
		case m := <-a4.Inbox():
			require.IsType(t, m, ActorMessage{})
			am := m.(ActorMessage)
			assert.Equal(t, a4.ActorID(), am.Sender)
			assert.EqualValues(t, 0x03, am.Code)
			assert.Equal(t, body, am.Payload)
			n += 1
		default:
			if n == 3 {
				break N
			}
		}
	}
}
