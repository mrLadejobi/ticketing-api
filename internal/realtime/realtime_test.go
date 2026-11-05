package realtime_test

import (
	"testing"
	"time"

	"ticketing-api/internal/realtime"
)

func TestHub_Register_Broadcast_And_Targeting(t *testing.T) {
	hub := realtime.NewHub()

	client1 := &realtime.Client{
		UserID: 1,
		Send:   make(chan realtime.Event, 10),
		Done:   make(chan struct{}),
	}
	client2 := &realtime.Client{
		UserID: 2,
		Send:   make(chan realtime.Event, 10),
		Done:   make(chan struct{}),
	}

	pID := 10
	client3 := &realtime.Client{
		UserID:    3,
		ProjectID: &pID,
		Send:      make(chan realtime.Event, 10),
		Done:      make(chan struct{}),
	}

	hub.Register(client1)
	hub.Register(client2)
	hub.Register(client3)

	if hub.ActiveClientsCount() != 3 {
		t.Fatalf("expected 3 active clients, got %d", hub.ActiveClientsCount())
	}

	// 1. Broadcast targeted to User 1
	hub.BroadcastToUser(1, realtime.Event{
		Type:    "notification.new",
		Payload: map[string]string{"title": "Mentioned"},
	})

	select {
	case ev := <-client1.Send:
		if ev.Type != "notification.new" {
			t.Fatalf("expected event notification.new, got %s", ev.Type)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("client 1 did not receive targeted event")
	}

	// Ensure Client 2 did NOT receive it
	select {
	case ev := <-client2.Send:
		t.Fatalf("client 2 should not have received user 1 event, got %+v", ev)
	default:
	}

	// 2. Broadcast targeted to Project 10
	hub.BroadcastToProject(10, realtime.Event{
		Type:    "ticket.updated",
		Payload: map[string]string{"key": "ENG-10"},
	})

	select {
	case ev := <-client3.Send:
		if ev.Type != "ticket.updated" {
			t.Fatalf("expected event ticket.updated, got %s", ev.Type)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("client 3 did not receive project event")
	}

	// 3. System-wide broadcast
	hub.Broadcast(realtime.Event{
		Type:    "system.announcement",
		Payload: "maintenance scheduled",
	})

	for _, c := range []*realtime.Client{client1, client2, client3} {
		select {
		case ev := <-c.Send:
			if ev.Type != "system.announcement" {
				t.Fatalf("expected system.announcement, got %s", ev.Type)
			}
		case <-time.After(50 * time.Millisecond):
			t.Fatalf("client did not receive system broadcast")
		}
	}

	// 4. Unregister
	hub.Unregister(client1)
	if hub.ActiveClientsCount() != 2 {
		t.Fatalf("expected 2 active clients after unregister, got %d", hub.ActiveClientsCount())
	}
}
