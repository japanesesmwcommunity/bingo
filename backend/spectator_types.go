package main

// Public readers use the same room payload as authenticated participants.
// Session identity is only attached to authenticated participant connections.
type spectatorEvent = roomEvent
