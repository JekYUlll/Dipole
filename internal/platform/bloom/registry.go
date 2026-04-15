package bloom

import (
	"fmt"
	"sync"

	"github.com/JekYUlll/Dipole/internal/model"
	"github.com/JekYUlll/Dipole/internal/store"
)

const (
	defaultFalsePositiveRate = 0.01
	defaultUserCapacity      = 10000
	defaultGroupCapacity     = 2000
)

type Registry struct {
	users      *Filter
	groups     *Filter
	userCount  int
	groupCount int
}

var (
	globalMu sync.RWMutex
	global   *Registry
)

func Init() error {
	if store.DB == nil {
		return fmt.Errorf("mysql not initialized")
	}

	var userUUIDs []string
	if err := store.DB.Model(&model.User{}).Pluck("uuid", &userUUIDs).Error; err != nil {
		return fmt.Errorf("load user uuids for bloom filter: %w", err)
	}

	var groupUUIDs []string
	if err := store.DB.Model(&model.Group{}).Pluck("uuid", &groupUUIDs).Error; err != nil {
		return fmt.Errorf("load group uuids for bloom filter: %w", err)
	}

	Load(userUUIDs, groupUUIDs)
	return nil
}

func Load(userUUIDs, groupUUIDs []string) {
	userCapacity := uint64(len(userUUIDs))
	if userCapacity < defaultUserCapacity {
		userCapacity = defaultUserCapacity
	}
	groupCapacity := uint64(len(groupUUIDs))
	if groupCapacity < defaultGroupCapacity {
		groupCapacity = defaultGroupCapacity
	}

	registry := &Registry{
		users:      NewFilter(userCapacity, defaultFalsePositiveRate),
		groups:     NewFilter(groupCapacity, defaultFalsePositiveRate),
		userCount:  len(userUUIDs),
		groupCount: len(groupUUIDs),
	}
	for _, uuid := range userUUIDs {
		registry.users.Add(uuid)
	}
	for _, uuid := range groupUUIDs {
		registry.groups.Add(uuid)
	}

	globalMu.Lock()
	global = registry
	globalMu.Unlock()
}

func Reset() {
	globalMu.Lock()
	global = nil
	globalMu.Unlock()
}

func UserMayExist(uuid string) bool {
	registry := snapshot()
	if registry == nil || registry.users == nil {
		return true
	}

	return registry.users.Test(uuid)
}

func GroupMayExist(uuid string) bool {
	registry := snapshot()
	if registry == nil || registry.groups == nil {
		return true
	}

	return registry.groups.Test(uuid)
}

func AddUser(uuid string) {
	registry := snapshot()
	if registry == nil || registry.users == nil {
		return
	}

	registry.users.Add(uuid)
	registry.incrementUsers()
}

func AddGroup(uuid string) {
	registry := snapshot()
	if registry == nil || registry.groups == nil {
		return
	}

	registry.groups.Add(uuid)
	registry.incrementGroups()
}

func Stats() (userCount int, groupCount int) {
	registry := snapshot()
	if registry == nil {
		return 0, 0
	}

	return registry.userCount, registry.groupCount
}

func snapshot() *Registry {
	globalMu.RLock()
	defer globalMu.RUnlock()

	return global
}

func (r *Registry) incrementUsers() {
	globalMu.Lock()
	defer globalMu.Unlock()

	if global == r {
		global.userCount++
	}
}

func (r *Registry) incrementGroups() {
	globalMu.Lock()
	defer globalMu.Unlock()

	if global == r {
		global.groupCount++
	}
}
