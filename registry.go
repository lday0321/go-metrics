package metrics

import (
	"fmt"
	"reflect"
	"sync"
)

// DuplicateMetric is the error returned by Registry.Register when a metric
// already exists.  If you mean to Register that metric you must first
// Unregister the existing metric.
type DuplicateMetric string

func (err DuplicateMetric) Error() string {
	return fmt.Sprintf("duplicate metric: %s", string(err))
}

// A Registry holds references to a set of metrics by name and can iterate
// over them, calling callback functions provided by the user.
//
// This is an interface so as to encourage other structs to implement
// the Registry API as appropriate.
type Registry interface {

	// Call the given function for each registered metric.
	Each(func(string, interface{}))

	// Get the metric by the given name or nil if none is registered.
	Get(string) interface{}

	// Gets an existing metric or registers the given one.
	// The interface can be the metric to register if not found in registry,
	// or a function returning the metric for lazy instantiation.
	GetOrRegister(string, interface{}) interface{}

	// Register the given metric under the given name.
	Register(string, interface{}) error

	// Run all registered healthchecks.
	RunHealthchecks()

	// Unregister the metric with the given name.
	Unregister(string)

	// Unregister all metrics.  (Mostly for testing.)
	UnregisterAll()
}

// The standard implementation of a Registry is a mutex-protected map
// of names to metrics.
type StandardRegistry struct {
	metrics map[string]interface{}
	mutex   sync.RWMutex
}

// Create a new registry.
func NewRegistry() Registry {
	return &StandardRegistry{metrics: make(map[string]interface{})}
}

// Call the given function for each registered metric.
func (r *StandardRegistry) Each(f func(string, interface{})) {
	for name, i := range r.registered() {
		f(name, i)
	}
}

// Get the metric by the given name or nil if none is registered.
func (r *StandardRegistry) Get(name string) interface{} {
	r.mutex.RLock()
	m := r.metrics[name]
	r.mutex.RUnlock()
	return m
}

// Gets an existing metric or creates and registers a new one. Threadsafe
// alternative to calling Get and Register on failure.
// The interface can be the metric to register if not found in registry,
// or a function returning the metric for lazy instantiation.
func (r *StandardRegistry) GetOrRegister(name string, i interface{}) interface{} {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if metric, ok := r.metrics[name]; ok {
		return metric
	}
	if v := reflect.ValueOf(i); v.Kind() == reflect.Func {
		i = v.Call(nil)[0].Interface()
	}
	r.register(name, i)
	return i
}

// Register the given metric under the given name.  Returns a DuplicateMetric
// if a metric by the given name is already registered.
func (r *StandardRegistry) Register(name string, i interface{}) error {
	r.mutex.Lock()
	err := r.register(name, i)
	r.mutex.Unlock()
	return err
}

// Run all registered healthchecks.
func (r *StandardRegistry) RunHealthchecks() {
	r.mutex.RLock()
	for _, i := range r.metrics {
		if h, ok := i.(Healthcheck); ok {
			h.Check()
		}
	}
	r.mutex.RUnlock()
}

// Unregister the metric with the given name.
func (r *StandardRegistry) Unregister(name string) {
	r.mutex.Lock()
	delete(r.metrics, name)
	r.mutex.Unlock()
}

// Unregister all metrics.  (Mostly for testing.)
func (r *StandardRegistry) UnregisterAll() {
	r.mutex.Lock()
	for name, _ := range r.metrics {
		delete(r.metrics, name)
	}
	r.mutex.Unlock()
}

func (r *StandardRegistry) register(name string, i interface{}) error {
	if _, ok := r.metrics[name]; ok {
		return DuplicateMetric(name)
	}
	switch i.(type) {
	case Counter, Gauge, GaugeFloat64, Healthcheck, Histogram, Meter, Timer:
		r.metrics[name] = i
	}
	return nil
}

func (r *StandardRegistry) registered() map[string]interface{} {
	r.mutex.RLock()
	metrics := make(map[string]interface{}, len(r.metrics))
	for name, i := range r.metrics {
		metrics[name] = i
	}
	r.mutex.RUnlock()
	return metrics
}

type PrefixedRegistry struct {
	underlying Registry
	prefix     string
}

func NewPrefixedRegistry(prefix string) Registry {
	return &PrefixedRegistry{
		underlying: NewRegistry(),
		prefix:     prefix,
	}
}

func NewPrefixedChildRegistry(parent Registry, prefix string) Registry {
	return &PrefixedRegistry{
		underlying: parent,
		prefix:     prefix,
	}
}

// Call the given function for each registered metric.
func (r *PrefixedRegistry) Each(fn func(string, interface{})) {
	r.underlying.Each(fn)
}

// Get the metric by the given name or nil if none is registered.
func (r *PrefixedRegistry) Get(name string) interface{} {
	return r.underlying.Get(name)
}

// Gets an existing metric or registers the given one.
// The interface can be the metric to register if not found in registry,
// or a function returning the metric for lazy instantiation.
func (r *PrefixedRegistry) GetOrRegister(name string, metric interface{}) interface{} {
	realName := r.prefix + name
	return r.underlying.GetOrRegister(realName, metric)
}

// Register the given metric under the given name. The name will be prefixed.
func (r *PrefixedRegistry) Register(name string, metric interface{}) error {
	realName := r.prefix + name
	return r.underlying.Register(realName, metric)
}

// Run all registered healthchecks.
func (r *PrefixedRegistry) RunHealthchecks() {
	r.underlying.RunHealthchecks()
}

// Unregister the metric with the given name. The name will be prefixed.
func (r *PrefixedRegistry) Unregister(name string) {
	realName := r.prefix + name
	r.underlying.Unregister(realName)
}

// Unregister all metrics.  (Mostly for testing.)
func (r *PrefixedRegistry) UnregisterAll() {
	r.underlying.UnregisterAll()
}
