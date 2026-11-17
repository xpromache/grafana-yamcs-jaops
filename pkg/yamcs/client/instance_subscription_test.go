package client

import (
	"testing"
	"time"

	"github.com/jaops-space/grafana-yamcs-jaops/api/yamcs/api"
	"github.com/jaops-space/grafana-yamcs-jaops/api/yamcs/protobuf/instances"
	"google.golang.org/protobuf/types/known/anypb"
)

func instanceMessage(t *testing.T, name string, state instances.YamcsInstance_InstanceState) *api.ServerMessage {
	t.Helper()
	data, err := anypb.New(&instances.YamcsInstance{Name: &name, State: &state})
	if err != nil {
		t.Fatal(err)
	}
	return &api.ServerMessage{Type: "instances", Data: data}
}

func closed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestInstanceRestartForgetsItsSubscriptions(t *testing.T) {
	c := &YamcsClient{
		ParameterSubscriptions: map[int32]*ParameterSubscription{
			1: {subscriptionID: 1, Instance: "sat-1", Processor: "realtime"},
			2: {subscriptionID: 2, Instance: "sat-2", Processor: "realtime"},
		},
		AlarmSubscriptions:             map[int32]*AlarmSubscription{3: {callID: 3, instance: "sat-2"}},
		ProcessorSubscriptions:         map[int32]*ProcessorSubscription{4: {subscriptionID: 4, Instance: "sat-2", Processor: "realtime"}},
		CommandHistorySubscriptions:    map[int32]*CommandHistorySubscription{},
		EventSubscriptions:             map[int32]*EventSubscription{},
		GlobalAlarmStatusSubscriptions: map[int32]*GlobalStatusSubscription{},
		TimeSubscriptions:              map[int32]*TimeSubscription{},
		LinkSubscriptions:              map[int32]*LinkSubscription{},
		subscribeCooldowns: map[string]time.Time{
			subscribeCooldownKey("parameters", "sat-2", "realtime"): time.Now().Add(time.Minute),
			subscribeCooldownKey("parameters", "sat-1", "realtime"): time.Now().Add(time.Minute),
		},
	}
	runningAgain := make(chan string, 1)
	c.SetInstanceRunningHandler(func(instance string) { runningAgain <- instance })

	// Yamcs tells changes only: an instance not heard of yet is running.
	c.HandleInstanceMessage(instanceMessage(t, "sat-1", instances.YamcsInstance_RUNNING))
	changed := c.InstanceChanged("sat-2")
	other := c.InstanceChanged("sat-1")
	if closed(other) || len(c.ParameterSubscriptions) != 2 {
		t.Fatal("RUNNING of a running instance changed something")
	}

	// sat-2 stops (its first state heard): its subscriptions and cooldowns
	// go, its signal fires.
	c.HandleInstanceMessage(instanceMessage(t, "sat-2", instances.YamcsInstance_STOPPING))
	if !closed(changed) {
		t.Error("sat-2's signal not closed")
	}
	if closed(other) {
		t.Error("sat-1's signal closed")
	}
	if _, ok := c.FindParameterSubscription("sat-2", "realtime"); ok {
		t.Error("sat-2's parameter subscription kept")
	}
	if _, ok := c.FindParameterSubscription("sat-1", "realtime"); !ok {
		t.Error("sat-1's parameter subscription removed")
	}
	if len(c.AlarmSubscriptions) != 0 || len(c.ProcessorSubscriptions) != 0 {
		t.Error("sat-2's alarm or processor subscription kept")
	}
	if c.checkSubscribeCooldown(subscribeCooldownKey("parameters", "sat-2", "realtime")) != nil {
		t.Error("sat-2's cooldown kept")
	}
	if c.checkSubscribeCooldown(subscribeCooldownKey("parameters", "sat-1", "realtime")) == nil {
		t.Error("sat-1's cooldown removed")
	}

	// Further states while stopped change nothing.
	again := c.InstanceChanged("sat-2")
	c.HandleInstanceMessage(instanceMessage(t, "sat-2", instances.YamcsInstance_OFFLINE))
	c.HandleInstanceMessage(instanceMessage(t, "sat-2", instances.YamcsInstance_STARTING))
	if closed(again) {
		t.Error("signal closed while still stopped")
	}

	// Running again: the signal fires again and the handler is called.
	c.HandleInstanceMessage(instanceMessage(t, "sat-2", instances.YamcsInstance_RUNNING))
	if !closed(again) {
		t.Error("signal not closed when running again")
	}
	select {
	case got := <-runningAgain:
		if got != "sat-2" {
			t.Errorf("handler called for %s", got)
		}
	case <-time.After(time.Second):
		t.Error("handler not called")
	}
}
