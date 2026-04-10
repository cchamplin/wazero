package wasm

import (
	"errors"
	"fmt"
)

// deleteModule makes the moduleName available for instantiation again.
func (s *Store) deleteModule(m *ModuleInstance) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	// Remove this module name.
	if m.prev != nil {
		m.prev.next = m.next
	}
	if m.next != nil {
		m.next.prev = m.prev
	}
	if s.moduleList == m {
		s.moduleList = m.next
	}
	// Clear the m state so it does not enter any other branch
	// on subsequent calls to deleteModule.
	m.prev = nil
	m.next = nil

	if m.ModuleName != "" {
		delete(s.nameToModule, m.ModuleName)

		// Shrink the map if it's allocated more than twice the size of the list
		newCap := len(s.nameToModule)
		if newCap < nameToModuleShrinkThreshold {
			newCap = nameToModuleShrinkThreshold
		}
		if newCap*2 <= s.nameToModuleCap {
			nameToModule := make(map[string]*ModuleInstance, newCap)
			for k, v := range s.nameToModule {
				nameToModule[k] = v
			}
			s.nameToModule = nameToModule
			s.nameToModuleCap = newCap
		}
	}
	return nil
}

// module returns the module of the given name or error if not in this store
func (s *Store) module(moduleName string) (*ModuleInstance, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	m, ok := s.nameToModule[moduleName]
	if !ok {
		return nil, fmt.Errorf("module[%s] not instantiated", moduleName)
	}
	return m, nil
}

// registerModule registers a ModuleInstance into the store.
// This makes the ModuleInstance visible for import if it's not anonymous, and ensures it is closed when the store is.
func (s *Store) registerModule(m *ModuleInstance) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	if s.nameToModule == nil {
		return errors.New("already closed")
	}

	if m.ModuleName != "" {
		if _, ok := s.nameToModule[m.ModuleName]; ok {
			return fmt.Errorf("module[%s] has already been instantiated", m.ModuleName)
		}
		s.nameToModule[m.ModuleName] = m
		if len(s.nameToModule) > s.nameToModuleCap {
			s.nameToModuleCap = len(s.nameToModule)
		}
	}

	// Add the newest node to the moduleNamesList as the head.
	m.next = s.moduleList
	if m.next != nil {
		m.next.prev = m
	}
	s.moduleList = m
	return nil
}

// AliasModule adds a name -> module mapping so that the given module instance
// can also be resolved by moduleName during import resolution. This is used by
// the component model to support empty-string module names for inline core
// instances, which the normal registerModule path skips to preserve the
// anonymous-module semantics used by the public API.
func (s *Store) AliasModule(moduleName string, m *ModuleInstance) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.nameToModule != nil {
		s.nameToModule[moduleName] = m
	}
}

// UnaliasModule removes a name -> module mapping previously added by
// AliasModule. It does NOT close or unregister the module instance itself.
func (s *Store) UnaliasModule(moduleName string) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.nameToModule != nil {
		delete(s.nameToModule, moduleName)
	}
}

// Module implements wazero.Runtime Module
func (s *Store) Module(moduleName string) *ModuleInstance {
	m, err := s.module(moduleName)
	if err != nil {
		return nil
	}
	return m
}
