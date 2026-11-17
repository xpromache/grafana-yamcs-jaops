package client

import (
	"context"
	"sync"

	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/jaops-space/grafana-yamcs-jaops/api/yamcs/api"
	"github.com/jaops-space/grafana-yamcs-jaops/api/yamcs/protobuf/instances"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Instance restarts.
//
// Yamcs ends every subscription of an instance when the instance stops: its
// processors are gone, and a restarted instance has new ones. The WebSocket
// stays connected, so nothing else tells the plugin: without this, a
// restarted instance's panels keep their old (dead) subscriptions and stop
// updating until Grafana is restarted. The client therefore follows the
// instances' states (the Yamcs topic "instances") and, when an instance
// leaves RUNNING, forgets its subscriptions and closes its InstanceChanged
// signal, so its streams end and Grafana starts them again, subscribing
// afresh; when it is RUNNING again, the instanceRunning handler (the
// multiplexer's) sets its endpoints up again.

// instanceTracker holds the state of the instances, as last told by Yamcs,
// and a signal per instance closed when it stops or starts.
type instanceTracker struct {
	mu      sync.Mutex
	states  map[string]instances.YamcsInstance_InstanceState
	signals map[string]chan struct{}
	running func(instance string)
}

func (client *YamcsClient) tracker() *instanceTracker {
	client.instancesOnce.Do(func() {
		client.instances = &instanceTracker{
			states:  map[string]instances.YamcsInstance_InstanceState{},
			signals: map[string]chan struct{}{},
		}
	})
	return client.instances
}

// InstanceChanged returns a channel closed the next time the instance stops
// or starts again (after it was RUNNING). Like Disconnected, it is valid for
// one change: call it again after it fires.
func (client *YamcsClient) InstanceChanged(instance string) <-chan struct{} {
	t := client.tracker()
	t.mu.Lock()
	defer t.mu.Unlock()
	ch, ok := t.signals[instance]
	if !ok {
		ch = make(chan struct{})
		t.signals[instance] = ch
	}
	return ch
}

// SetInstanceRunningHandler sets what to call when an instance is RUNNING
// again after it stopped (in its own goroutine: it may make requests).
func (client *YamcsClient) SetInstanceRunningHandler(handler func(instance string)) {
	t := client.tracker()
	t.mu.Lock()
	t.running = handler
	t.mu.Unlock()
}

// subscribeInstances subscribes to the instances' state changes. Called
// after each connect; a failure is only logged (the panels then keep the old
// behaviour: a restarted instance needs a reconnect).
func (client *YamcsClient) subscribeInstances(ctx context.Context) {
	t := client.tracker()
	t.mu.Lock()
	// a new connection: the states from before are of no use
	t.states = map[string]instances.YamcsInstance_InstanceState{}
	t.mu.Unlock()

	options, err := anypb.New(&emptypb.Empty{})
	if err != nil {
		return
	}
	if _, _, _, err := client.WebSocket.SendSync(ctx, &api.ClientMessage{Type: "instances", Options: options}); err != nil {
		backend.Logger.Warn("Could not subscribe to instance changes: a restarted instance's panels stop updating until reconnected", "server", client.ServerAddress, "error", err)
	}
}

// HandleInstanceMessage follows an instance's state (the "instances" topic).
// It runs on the WebSocket's read loop: no network I/O here.
func (client *YamcsClient) HandleInstanceMessage(message *api.ServerMessage) {
	instance := &instances.YamcsInstance{}
	if err := message.Data.UnmarshalTo(instance); err != nil {
		backend.Logger.Debug("Error unmarshalling instance data", "error", err)
		return
	}
	client.instanceState(instance.GetName(), instance.GetState())
}

func (client *YamcsClient) instanceState(name string, state instances.YamcsInstance_InstanceState) {
	t := client.tracker()
	t.mu.Lock()
	previous, known := t.states[name]
	t.states[name] = state
	running := t.running
	// Yamcs tells changes only, not the states at subscribe time: an
	// instance not heard of yet is taken as RUNNING (as it is when its
	// endpoints are set up), so its first change, a stop, counts.
	wasRunning := !known || previous == instances.YamcsInstance_RUNNING
	isRunning := state == instances.YamcsInstance_RUNNING
	// Stopped, or running again.
	changed := wasRunning != isRunning
	if changed {
		if ch, ok := t.signals[name]; ok {
			close(ch)
			delete(t.signals, name)
		}
	}
	t.mu.Unlock()
	if !changed {
		return
	}

	backend.Logger.Info("Yamcs instance changed state: its subscriptions are made again", "server", client.ServerAddress, "instance", name, "state", state.String())
	client.forgetInstanceSubscriptions(name)
	if isRunning && running != nil {
		go running(name)
	}
}

// forgetInstanceSubscriptions removes the instance's subscriptions, which
// Yamcs ended with the instance, so the next request makes new ones.
func (client *YamcsClient) forgetInstanceSubscriptions(instance string) {
	client.subsMu.Lock()
	defer client.subsMu.Unlock()
	for id, s := range client.ParameterSubscriptions {
		if s.Instance == instance {
			delete(client.ParameterSubscriptions, id)
		}
	}
	for id, s := range client.CommandHistorySubscriptions {
		if s.Instance == instance {
			delete(client.CommandHistorySubscriptions, id)
		}
	}
	for id, s := range client.EventSubscriptions {
		if s.Instance == instance {
			delete(client.EventSubscriptions, id)
		}
	}
	for id, s := range client.AlarmSubscriptions {
		if s.instance == instance {
			delete(client.AlarmSubscriptions, id)
		}
	}
	for id, s := range client.GlobalAlarmStatusSubscriptions {
		if s.instance == instance {
			delete(client.GlobalAlarmStatusSubscriptions, id)
		}
	}
	for id, s := range client.TimeSubscriptions {
		if s.Instance == instance {
			delete(client.TimeSubscriptions, id)
		}
	}
	for id, s := range client.LinkSubscriptions {
		if s.Instance == instance {
			delete(client.LinkSubscriptions, id)
		}
	}
	for id, s := range client.ProcessorSubscriptions {
		if s.Instance == instance {
			delete(client.ProcessorSubscriptions, id)
		}
	}
	for key := range client.subscribeCooldowns {
		if cooldownInstance(key) == instance {
			delete(client.subscribeCooldowns, key)
		}
	}
}

// FindProcessorSubscription returns the subscription to the processor's
// updates, if there is one.
func (client *YamcsClient) FindProcessorSubscription(instance, processor string) (*ProcessorSubscription, bool) {
	client.subsMu.RLock()
	defer client.subsMu.RUnlock()
	for _, s := range client.ProcessorSubscriptions {
		if s.Instance == instance && s.Processor == processor {
			return s, true
		}
	}
	return nil, false
}
