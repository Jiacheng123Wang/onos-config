// Copyright 2019-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package device

import (
	"github.com/onosproject/onos-config/pkg/types"
	"github.com/onosproject/onos-config/pkg/types/change"
	devicechange "github.com/onosproject/onos-config/pkg/types/change/device"
	"github.com/onosproject/onos-topo/pkg/northbound/device"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestDeviceStore(t *testing.T) {
	node, conn := startLocalNode()
	defer node.Stop()
	defer conn.Close()

	store1, err := newLocalStore(conn)
	assert.NoError(t, err)
	defer store1.Close()

	store2, err := newLocalStore(conn)
	assert.NoError(t, err)
	defer store2.Close()

	device1 := device.ID("device-1")
	device2 := device.ID("device-2")

	lastIndex, err := store1.LastIndex(device1)
	assert.NoError(t, err)
	assert.Equal(t, devicechange.Index(0), lastIndex)

	lastIndex, err = store1.LastIndex(device2)
	assert.NoError(t, err)
	assert.Equal(t, devicechange.Index(0), lastIndex)

	ch := make(chan *devicechange.Change)
	err = store2.Watch(ch)
	assert.NoError(t, err)

	change1 := &devicechange.Change{
		NetworkChangeID: types.ID("network-change-1"),
		DeviceID:        device1,
		Values: []*devicechange.Value{
			{
				Path:  "foo",
				Value: []byte("Hello world!"),
				Type:  devicechange.ValueType_STRING,
			},
			{
				Path:  "bar",
				Value: []byte("Hello world again!"),
				Type:  devicechange.ValueType_STRING,
			},
		},
	}

	change2 := &devicechange.Change{
		NetworkChangeID: types.ID("network-change-2"),
		DeviceID:        device1,
		Values: []*devicechange.Value{
			{
				Path:  "baz",
				Value: []byte("Goodbye world!"),
				Type:  devicechange.ValueType_STRING,
			},
		},
	}

	// Create a new change
	err = store1.Create(change1)
	assert.NoError(t, err)
	assert.Equal(t, devicechange.ID("device-1:1"), change1.ID)
	assert.Equal(t, devicechange.Index(1), change1.Index)
	assert.NotEqual(t, devicechange.Revision(0), change1.Revision)

	// Get the change
	change1, err = store2.Get(devicechange.ID("device-1:1"))
	assert.NoError(t, err)
	assert.NotNil(t, change1)
	assert.Equal(t, devicechange.ID("device-1:1"), change1.ID)
	assert.Equal(t, devicechange.Index(1), change1.Index)
	assert.NotEqual(t, devicechange.Revision(0), change1.Revision)

	// Append another change
	err = store2.Create(change2)
	assert.NoError(t, err)
	assert.Equal(t, devicechange.ID("device-1:2"), change2.ID)
	assert.Equal(t, devicechange.Index(2), change2.Index)
	assert.NotEqual(t, devicechange.Revision(0), change2.Revision)

	change3 := &devicechange.Change{
		NetworkChangeID: types.ID("network-change-3"),
		DeviceID:        device1,
		Values: []*devicechange.Value{
			{
				Path:    "foo",
				Removed: true,
			},
		},
	}

	// Append another change
	err = store1.Create(change3)
	assert.NoError(t, err)
	assert.Equal(t, devicechange.ID("device-1:3"), change3.ID)
	assert.Equal(t, devicechange.Index(3), change3.Index)
	assert.NotEqual(t, devicechange.Revision(0), change3.Revision)

	// For two devices
	change4 := &devicechange.Change{
		NetworkChangeID: types.ID("network-change-3"),
		DeviceID:        device2,
		Values: []*devicechange.Value{
			{
				Path:  "foo",
				Value: []byte("bar"),
				Type:  devicechange.ValueType_STRING,
			},
		},
	}

	// Append another change
	err = store1.Create(change4)
	assert.NoError(t, err)
	assert.Equal(t, devicechange.ID("device-2:1"), change4.ID)
	assert.Equal(t, devicechange.Index(1), change4.Index)
	assert.NotEqual(t, devicechange.Revision(0), change4.Revision)

	// Verify events were received for the changes
	select {
	case change := <-ch:
		assert.Equal(t, devicechange.ID("device-1:1"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	select {
	case change := <-ch:
		assert.Equal(t, devicechange.ID("device-1:2"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	select {
	case change := <-ch:
		assert.Equal(t, devicechange.ID("device-1:3"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	select {
	case change := <-ch:
		assert.Equal(t, devicechange.ID("device-2:1"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}

	// Update one of the changes
	change2.Status.State = change.State_APPLYING
	revision := change2.Revision
	err = store1.Update(change2)
	assert.NoError(t, err)
	assert.NotEqual(t, revision, change2.Revision)

	// Read and then update the change
	change2, err = store2.Get(devicechange.ID("device-1:2"))
	assert.NoError(t, err)
	assert.NotNil(t, change2)
	change2.Status.State = change.State_SUCCEEDED
	revision = change2.Revision
	err = store1.Update(change2)
	assert.NoError(t, err)
	assert.NotEqual(t, revision, change2.Revision)

	// Verify that concurrent updates fail
	change31, err := store1.Get(devicechange.ID("device-1:3"))
	assert.NoError(t, err)
	change32, err := store2.Get(devicechange.ID("device-1:3"))
	assert.NoError(t, err)

	change31.Status.State = change.State_APPLYING
	err = store1.Update(change31)
	assert.NoError(t, err)

	change32.Status.State = change.State_FAILED
	err = store2.Update(change32)
	assert.Error(t, err)

	// Verify device events were received again
	select {
	case change := <-ch:
		assert.Equal(t, devicechange.ID("device-1:2"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	select {
	case change := <-ch:
		assert.Equal(t, devicechange.ID("device-1:2"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	select {
	case change := <-ch:
		assert.Equal(t, devicechange.ID("device-1:3"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}

	// List the changes for a device
	changes := make(chan *devicechange.Change)
	err = store1.List(device1, changes)
	assert.NoError(t, err)

	select {
	case listChange := <-changes:
		assert.Equal(t, devicechange.ID("device-1:1"), listChange.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	select {
	case change := <-changes:
		assert.Equal(t, devicechange.ID("device-1:2"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	select {
	case change := <-changes:
		assert.Equal(t, devicechange.ID("device-1:3"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	select {
	case _, ok := <-changes:
		assert.False(t, ok)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}

	// Replay changes from a specific index
	changes = make(chan *devicechange.Change)
	err = store1.Replay(device1, 2, changes)
	assert.NoError(t, err)

	select {
	case change := <-changes:
		assert.Equal(t, devicechange.ID("device-1:2"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	select {
	case change := <-changes:
		assert.Equal(t, devicechange.ID("device-1:3"), change.ID)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}
	select {
	case _, ok := <-changes:
		assert.False(t, ok)
	case <-time.After(5 * time.Second):
		t.FailNow()
	}

	// Delete a change
	err = store1.Delete(change2)
	assert.NoError(t, err)
	change2, err = store2.Get("device-1:2")
	assert.NoError(t, err)
	assert.Nil(t, change2)
}